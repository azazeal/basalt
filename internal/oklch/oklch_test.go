package oklch

import (
	"math"
	"strconv"
	"testing"
)

// The reference values below come from culori 4.0.2, an implementation that
// shares no code with this one.
var (
	red   = RGB{R: 1}
	green = RGB{G: 1}
	blue  = RGB{B: 1}

	redLCh   = LCh{L: 0.627955, C: 0.257683, H: 29.2339}
	greenLCh = LCh{L: 0.866440, C: 0.294827, H: 142.4953}
	blueLCh  = LCh{L: 0.452014, C: 0.313214, H: 264.0520}
)

func TestLCh(t *testing.T) {
	cases := []struct {
		in   RGB
		want LCh
	}{
		0: { // red
			in:   red,
			want: redLCh,
		},
		1: { // green
			in:   green,
			want: greenLCh,
		},
		2: { // blue
			in:   blue,
			want: blueLCh,
		},
		3: { // white, which has no chroma to carry a hue
			in:   RGB{1, 1, 1},
			want: LCh{L: 1},
		},
		4: { // black, likewise
		},
	}

	for caseIndex, kase := range cases {
		t.Run(strconv.Itoa(caseIndex), func(t *testing.T) {
			got := lch(kase.in)

			near(t, "L", got.L, kase.want.L, 1e-5)
			near(t, "C", got.C, kase.want.C, 1e-5)

			// Hue is meaningless without chroma to carry it, and the two
			// achromatic cases above leave it to whatever the rounding says.
			if kase.want.C > 0 {
				near(t, "H", got.H, kase.want.H, 1e-3)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	// A coarse sweep of the cube, out through lch and back through RGB, so a
	// sign error in either shows up here.
	//
	// The tolerance is loose next to the others because the trip runs through
	// a cube root and back. It is still two and a half orders of magnitude
	// under the 1/255 an 8-bit channel can express, so nothing that survives
	// this can move a rendered color.
	const tol = 1e-5

	for r := 0; r < 256; r += 17 {
		for g := 0; g < 256; g += 17 {
			for b := 0; b < 256; b += 17 {
				in := RGB{float64(r) / 255, float64(g) / 255, float64(b) / 255}

				out := lch(in).RGB()

				near(t, "R", out.R, in.R, tol)
				near(t, "G", out.G, in.G, tol)
				near(t, "B", out.B, in.B, tol)
			}
		}
	}
}

func TestMaxChroma(t *testing.T) {
	// A primary is a corner of the sRGB cube, so no color at its lightness and
	// hue holds more chroma than it does.
	//
	// NOTE: blue is not a case, and fails as one. The line of constant OKLCh
	// hue that ends in pure blue bends outside sRGB on the way and touches it
	// again only at blue itself, so the ceiling at blue's lightness and hue is
	// the edge where the line first leaves, around 0.266.
	cases := []LCh{
		0: redLCh,
		1: greenLCh,
	}

	// The tolerance is looser than elsewhere because the ceiling is defined
	// against [RGB.InGamut], so it moves with that function's own tolerance.
	// It still pins the answer far below anything an 8-bit channel can show.
	const tol = 1e-4

	for caseIndex, kase := range cases {
		t.Run(strconv.Itoa(caseIndex), func(t *testing.T) {
			near(t, "MaxChroma", MaxChroma(kase.L, kase.H), kase.C, tol)
		})
	}
}

func TestMaxChromaIsTheCeiling(t *testing.T) {
	// Whatever the ceiling is, sitting on it must be showable and stepping
	// past it must not be. This is the property the palette leans on when it
	// asks for a fraction of the ceiling, so it is checked across the wheel
	// rather than at the points the table above pins.
	for h := 0.0; h < 360; h += 15 {
		for _, l := range []float64{0.2, 0.45, 0.73, 0.9} {
			ceiling := MaxChroma(l, h)

			if !(LCh{L: l, C: ceiling, H: h}).RGB().InGamut() {
				t.Errorf("L=%.2f H=%.0f: chroma %.6f is the ceiling but falls outside sRGB", l, h, ceiling)
			}

			if (LCh{L: l, C: ceiling + 1e-3, H: h}).RGB().InGamut() {
				t.Errorf("L=%.2f H=%.0f: chroma %.6f is past the ceiling but fits sRGB", l, h, ceiling+1e-3)
			}
		}
	}
}

func TestFit(t *testing.T) {
	// A chroma no hue can reach, so Fit has to pull it back to the ceiling
	// while leaving lightness and hue where they were.
	const l, h = 0.6, 30.0

	got := (LCh{L: l, C: 0.4, H: h}).Fit()

	near(t, "L", got.L, l, 1e-9)
	near(t, "H", got.H, h, 1e-9)
	near(t, "C", got.C, MaxChroma(l, h), 1e-9)

	if !got.RGB().InGamut() {
		t.Error("Fit returned a color sRGB cannot show")
	}

	// A color already inside the gamut must come back untouched.
	in := LCh{L: 0.5, C: 0.02, H: 200}
	if out := in.Fit(); out != in {
		t.Errorf("Fit(%v) = %v, want it left alone", in, out)
	}
}

func TestContrast(t *testing.T) {
	white, black := RGB{1, 1, 1}, RGB{0, 0, 0}

	near(t, "white on black", Contrast(white, black), 21, 1e-9)
	near(t, "black on white", Contrast(black, white), 21, 1e-9)
	near(t, "white on white", Contrast(white, white), 1, 1e-9)

	// The darkest grey that passes 4.5:1 on white, which checkers report at
	// 4.54.
	const v = 0x76 / 255.0
	near(t, "#767676 on white", Contrast(RGB{v, v, v}, white), 4.54, 0.005)
}

func TestDifference(t *testing.T) {
	// A color is no distance from itself, whatever it is.
	c := LCh{L: 0.26, C: 0.06, H: 200}
	near(t, "a color against itself", Difference(c, c), 0, 1e-12)

	// It is symmetric, which Contrast is too but for a different reason.
	d := LCh{L: 0.30, C: 0.02, H: 40}
	near(t, "symmetry", Difference(c, d), Difference(d, c), 1e-12)

	// Two colors of one lightness are identical to Contrast and are not
	// identical here. That difference is the whole reason this exists: a
	// selection and the row under it are grounds, and nothing is written on
	// either of them for a contrast ratio to be about.
	x := LCh{L: 0.26, C: 0.06, H: 200}
	y := LCh{L: 0.26, C: 0.06, H: 20}

	near(t, "contrast cannot tell them apart", Contrast(x.RGB(), y.RGB()), 1, 0.35)

	if got := Difference(x, y); got < 0.10 {
		t.Errorf("Difference of two hues at one lightness = %.4f, want it to see them apart", got)
	}

	// Lightness alone moves it, which is what lets a wash that has run out of
	// chroma buy separation by rising.
	near(t, "pure lightness", Difference(
		LCh{L: 0.26, C: 0, H: 0},
		LCh{L: 0.36, C: 0, H: 0},
	), 0.10, 1e-12)
}

func TestHexClamps(t *testing.T) {
	// A color outside sRGB still has to render, and Hex clamps rather than
	// wrapping, so a component past the end lands on the end.
	if got, want := (RGB{1.4, -0.2, 0.5}).Hex(), "#FF0080"; got != want {
		t.Errorf("Hex() = %s, want %s", got, want)
	}
}

// near reports whether two floats agree to within tol. The conversions are
// irrational, so exact equality would only ever test the compiler.
func near(t *testing.T, what string, got, want, tol float64) {
	t.Helper()

	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.6f, want %.6f (tolerance %g)", what, got, want, tol)
	}
}

// lch converts sRGB to OKLCh, the inverse of [LCh.RGB]. Only the tests need
// it: the palette is written as positions and never read back from a hex.
func lch(c RGB) LCh {
	r, g, b := linear(c.R), linear(c.G), linear(c.B)

	l := 0.4122214708*r + 0.5363325363*g + 0.0514459929*b
	m := 0.2119034982*r + 0.6806995451*g + 0.1073969566*b
	s := 0.0883024619*r + 0.2817188376*g + 0.6299787005*b

	l, m, s = math.Cbrt(l), math.Cbrt(m), math.Cbrt(s)

	lightness := 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
	a := 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	bb := 0.0259040371*l + 0.7827717662*m - 0.8086757660*s

	h := math.Atan2(bb, a) * 180 / math.Pi
	if h < 0 {
		h += 360
	}

	return LCh{L: lightness, C: math.Hypot(a, bb), H: h}
}
