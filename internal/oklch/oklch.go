// Package oklch maps sRGB to OKLCh and vice-versa, and answers the two
// questions the palette is built on: how much color a hue can hold at a given
// lightness, and how far apart two colors read.
//
// OKLCh is a cylindrical form of OKLab, where lightness tracks what the eye
// reports rather than what the encoding says. Two colors at one L look equally
// bright, which is what lets accents be placed by rule.
package oklch

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RGB is a gamma-encoded sRGB color. Each component runs 0 to 1, and a value
// outside that range is a color sRGB cannot show.
type RGB struct {
	R, G, B float64
}

// LCh is a color in OKLCh: L is perceptual lightness from 0 to 1, C is chroma
// from 0 upward with no fixed ceiling, and H is hue in degrees.
type LCh struct {
	L, C, H float64
}

// ParseHex reads a "#RRGGBB" color. The leading # is optional.
func ParseHex(s string) (RGB, error) {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) != 6 {
		return RGB{}, fmt.Errorf("oklch: %q is not a 6-digit hex color", s)
	}

	var c [3]float64
	for i := range c {
		v, err := strconv.ParseUint(h[i*2:i*2+2], 16, 8)
		if err != nil {
			return RGB{}, fmt.Errorf("oklch: %q is not a 6-digit hex color", s)
		}

		c[i] = float64(v) / 255
	}

	return RGB{c[0], c[1], c[2]}, nil
}

// Hex renders the color as "#RRGGBB", clamping any component that falls
// outside sRGB. Use [RGB.InGamut] first where the difference matters.
func (c RGB) Hex() string {
	q := func(v float64) int {
		return int(math.Round(math.Min(1, math.Max(0, v)) * 255))
	}

	return fmt.Sprintf("#%02X%02X%02X", q(c.R), q(c.G), q(c.B))
}

// InGamut reports whether every component lies within sRGB. The tolerance
// absorbs the rounding that a round trip through OKLab leaves behind.
func (c RGB) InGamut() bool {
	const tol = 1e-6

	for _, v := range [3]float64{c.R, c.G, c.B} {
		if v < -tol || v > 1+tol {
			return false
		}
	}

	return true
}

// linear undoes the sRGB transfer function, giving light rather than signal.
func linear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}

	return math.Pow((v+0.055)/1.055, 2.4)
}

// encode applies the sRGB transfer function, the inverse of [linear].
func encode(v float64) float64 {
	if v <= 0.0031308 {
		return v * 12.92
	}

	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

// LCh converts the color to OKLCh.
func (c RGB) LCh() LCh {
	r, g, b := linear(c.R), linear(c.G), linear(c.B)

	l := 0.4122214708*r + 0.5363325363*g + 0.0514459929*b
	m := 0.2119034982*r + 0.6806995451*g + 0.1073969566*b
	s := 0.0883024619*r + 0.2817188376*g + 0.6299787005*b

	l, m, s = math.Cbrt(l), math.Cbrt(m), math.Cbrt(s)

	lightness := 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
	a := 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	bb := 0.0259040371*l + 0.7827717662*m - 0.8086757660*s

	h := math.Mod(math.Atan2(bb, a)*180/math.Pi, 360)
	if h < 0 {
		h += 360
	}

	return LCh{L: lightness, C: math.Hypot(a, bb), H: h}
}

// RGB converts the color to sRGB. The result may fall outside the gamut; see
// [RGB.InGamut] and [LCh.Fit].
func (c LCh) RGB() RGB {
	rad := c.H * math.Pi / 180
	a, bb := c.C*math.Cos(rad), c.C*math.Sin(rad)

	l := c.L + 0.3963377774*a + 0.2158037573*bb
	m := c.L - 0.1055613458*a - 0.0638541728*bb
	s := c.L - 0.0894841775*a - 1.2914855480*bb

	l, m, s = l*l*l, m*m*m, s*s*s

	return RGB{
		R: encode(4.0767416621*l - 3.3077115913*m + 0.2309699292*s),
		G: encode(-1.2684380046*l + 2.6097574011*m - 0.3413193965*s),
		B: encode(-0.0041960863*l - 0.7034186147*m + 1.7076147010*s),
	}
}

// Hex renders the color as "#RRGGBB" after fitting it to sRGB.
func (c LCh) Hex() string {
	return c.Fit().RGB().Hex()
}

// maxChromaSteps is the bisection depth [MaxChroma] runs to. Fifty halvings
// land well inside the rounding of an 8-bit channel.
const maxChromaSteps = 50

// MaxChroma returns the most chroma sRGB can express at a lightness and hue.
// Asking for a fraction of it keeps every hue as colorful as it can be, rather
// than holding them all to whatever the narrowest allows.
func MaxChroma(l, h float64) float64 {
	lo, hi := 0.0, 0.5
	for range maxChromaSteps {
		mid := (lo + hi) / 2
		if (LCh{L: l, C: mid, H: h}).RGB().InGamut() {
			lo = mid
		} else {
			hi = mid
		}
	}

	return lo
}

// Fit reduces chroma until sRGB can show the color. Lightness and hue are left
// alone: dropping chroma dulls a color, clamping channels changes it.
func (c LCh) Fit() LCh {
	if c.RGB().InGamut() {
		return c
	}

	c.C = MaxChroma(c.L, c.H)

	return c
}

// luminance is WCAG's relative luminance, not OKLCh's lightness: it weights
// channels by emitted light rather than by how bright they look.
func (c RGB) luminance() float64 {
	r, g, b := linear(c.R), linear(c.G), linear(c.B)

	return 0.2126*r + 0.7152*g + 0.0722*b
}

// Difference returns how far apart two colors look, as their distance in
// OKLab. Below about 0.10 a pair starts to blur into one color.
//
// The palette needs this as well as [Contrast]. Contrast counts only lightness,
// so it calls two hues at one lightness identical; that is right for text on a
// ground and wrong for two grounds side by side with nothing written on them.
func Difference(a, b LCh) float64 {
	ax, ay := a.C*math.Cos(a.H*math.Pi/180), a.C*math.Sin(a.H*math.Pi/180)
	bx, by := b.C*math.Cos(b.H*math.Pi/180), b.C*math.Sin(b.H*math.Pi/180)

	return math.Sqrt((a.L-b.L)*(a.L-b.L) + (ax-bx)*(ax-bx) + (ay-by)*(ay-by))
}

// Contrast returns the WCAG contrast ratio between two colors, from 1 for a
// pair that cannot be told apart to 21 for black against white.
func Contrast(a, b RGB) float64 {
	x, y := a.luminance(), b.luminance()
	if x < y {
		x, y = y, x
	}

	return (x + 0.05) / (y + 0.05)
}
