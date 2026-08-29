package oklch

import (
	"math"
	"strconv"
	"testing"
)

// near reports whether two floats agree to within tol, and is what every
// assertion here is written in terms of: the conversions are irrational and
// exact equality would only ever test the compiler.
func near(t *testing.T, what string, got, want, tol float64) {
	t.Helper()

	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.6f, want %.6f (tolerance %g)", what, got, want, tol)
	}
}

func TestLCh(t *testing.T) {
	cases := []struct {
		hex     string
		l, c, h float64
	}{
		0: { // the page
			hex: "#0F1219",
			l:   0.182389,
			c:   0.015198,
			h:   266.7409,
		},
		1: { // the foreground
			hex: "#ABB2BF",
			l:   0.762093,
			c:   0.020162,
			h:   262.9873,
		},
		2: { // an accent
			hex: "#EF5F6B",
			l:   0.672893,
			c:   0.177473,
			h:   18.2977,
		},
		3: { // another, at the far side of the wheel
			hex: "#5AB0F6",
			l:   0.733190,
			c:   0.131715,
			h:   245.5411,
		},
		4: { // the chrome's critical fill, which sits on the gamut ceiling
			hex: "#9D0006",
			l:   0.437370,
			c:   0.178914,
			h:   28.2597,
		},
		5: { // white, which has no chroma to carry a hue
			hex: "#FFFFFF",
			l:   1,
		},
		6: { // black, likewise
			hex: "#000000",
		},
	}

	for caseIndex, kase := range cases {
		t.Run(strconv.Itoa(caseIndex), func(t *testing.T) {
			rgb, err := ParseHex(kase.hex)
			if err != nil {
				t.Fatalf("ParseHex(%q) failed: %v", kase.hex, err)
			}

			got := rgb.LCh()

			near(t, "L", got.L, kase.l, 1e-5)
			near(t, "C", got.C, kase.c, 1e-5)

			// Hue is meaningless without chroma to carry it, and the two
			// achromatic cases above leave it to whatever the rounding says.
			if kase.c > 0 {
				near(t, "H", got.H, kase.h, 1e-3)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	// A coarse sweep of the cube: every conversion in the package is used on
	// the way out and on the way back, so a sign error anywhere shows up here.
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

				out := in.LCh().RGB()

				near(t, "R", out.R, in.R, tol)
				near(t, "G", out.G, in.G, tol)
				near(t, "B", out.B, in.B, tol)
			}
		}
	}
}

func TestMaxChroma(t *testing.T) {
	cases := []struct {
		l, h float64
		want float64
	}{
		0: { // the chrome's critical fill sits exactly on the ceiling
			l:    0.437289,
			h:    28.2971,
			want: 0.178915,
		},
		1: { // blue at the accent lightness, the narrowest of the accents
			l:    0.73,
			h:    245,
			want: 0.150143,
		},
	}

	// The tolerance is looser than elsewhere because the ceiling is defined
	// against [RGB.InGamut], so it moves with that function's own tolerance.
	// It still pins the answer far below anything an 8-bit channel can show.
	const tol = 1e-4

	for caseIndex, kase := range cases {
		t.Run(strconv.Itoa(caseIndex), func(t *testing.T) {
			near(t, "MaxChroma", MaxChroma(kase.l, kase.h), kase.want, tol)
		})
	}
}

func TestMaxChromaIsTheCeiling(t *testing.T) {
	// Whatever the ceiling is, sitting on it must be showable and stepping
	// past it must not be. This is the property the palette leans on when it
	// asks for a fraction of the ceiling, so it is checked across the wheel
	// rather than at the two points the table above pins.
	for h := 0.0; h < 360; h += 15 {
		for _, l := range []float64{0.2, 0.45, 0.73, 0.9} {
			max := MaxChroma(l, h)

			if !(LCh{L: l, C: max, H: h}).RGB().InGamut() {
				t.Errorf("L=%.2f H=%.0f: chroma %.6f is the ceiling but falls outside sRGB", l, h, max)
			}

			if (LCh{L: l, C: max + 1e-3, H: h}).RGB().InGamut() {
				t.Errorf("L=%.2f H=%.0f: chroma %.6f is past the ceiling but fits sRGB", l, h, max+1e-3)
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

	fg, _ := ParseHex("#ABB2BF")
	bg, _ := ParseHex("#0F1219")
	near(t, "the foreground on the page", Contrast(fg, bg), 8.789643, 1e-5)
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

func TestParseHex(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		0: { // the canonical form
			in:   "#0F1219",
			want: "#0F1219",
		},
		1: { // the # is optional
			in:   "0f1219",
			want: "#0F1219",
		},
		2: { // surrounding space is trimmed
			in:   "  #0F1219  ",
			want: "#0F1219",
		},
		3: { // empty
			in:      "",
			wantErr: true,
		},
		4: { // the three-digit shorthand is not accepted
			in:      "#FFF",
			wantErr: true,
		},
		5: { // not hex digits
			in:      "#GGGGGG",
			wantErr: true,
		},
		6: { // one digit too many
			in:      "#0F12199",
			wantErr: true,
		},
	}

	for caseIndex, kase := range cases {
		t.Run(strconv.Itoa(caseIndex), func(t *testing.T) {
			got, err := ParseHex(kase.in)

			switch {
			case kase.wantErr && err == nil:
				t.Fatalf("ParseHex(%q) succeeded, want an error", kase.in)
			case kase.wantErr:
				return
			case err != nil:
				t.Fatalf("ParseHex(%q) failed: %v", kase.in, err)
			}

			if got.Hex() != kase.want {
				t.Errorf("ParseHex(%q).Hex() = %s, want %s", kase.in, got.Hex(), kase.want)
			}
		})
	}
}

func TestHexClamps(t *testing.T) {
	// A color outside sRGB still has to render, and Hex clamps rather than
	// wrapping, so a component past the end lands on the end.
	if got, want := (RGB{1.4, -0.2, 0.5}).Hex(), "#FF0080"; got != want {
		t.Errorf("Hex() = %s, want %s", got, want)
	}
}
