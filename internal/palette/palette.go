// Package palette reads the written-down palette and resolves it into colors.
//
// What is written down is positions, not hexes: a lightness and an amount of
// color for each surface, a place on the wheel for each accent, and the rules
// that turn an accent's place into the renditions it is used through. Nothing
// downstream of here chooses a color; the ports render what this produces.
package palette

import (
	"fmt"
	"math"

	"github.com/BurntSushi/toml"

	"github.com/azazeal/basalt/internal/oklch"
)

// Load reads a palette from a TOML file and resolves it.
func Load(path string) (*Palette, error) {
	var s spec

	md, err := toml.DecodeFile(path, &s)
	if err != nil {
		return nil, fmt.Errorf("failed reading %s: %w", path, err)
	}

	// A misspelled key is not an error to the decoder: the field it meant
	// keeps its zero, which reads as "not set" and resolves to a plausible
	// color.
	if keys := md.Undecoded(); len(keys) > 0 {
		return nil, fmt.Errorf("%s: unknown key %q", path, keys[0].String())
	}

	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	p, err := s.resolve()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return p, nil
}

// Palette is a resolved palette: the surface ladder in order from the deepest
// to the lightest, and the accent wheel in the order it was written down.
type Palette struct {
	Surfaces []Color
	Accents  []Accent
}

func (p *Palette) Surface(name string) (Color, bool) {
	for _, c := range p.Surfaces {
		if c.Name == name {
			return c, true
		}
	}

	return Color{}, false
}

func (p *Palette) Accent(name string) (Accent, bool) {
	for _, a := range p.Accents {
		if a.Name == name {
			return a, true
		}
	}

	return Accent{}, false
}

// Color is one step of the ladder, with the note saying what it is for.
type Color struct {
	Name string
	Note string
	LCh  oklch.LCh
}

func (c Color) Hex() string {
	return c.LCh.Hex()
}

func (c Color) RGB() oklch.RGB {
	return c.LCh.RGB()
}

// Accent is one place on the wheel and the four renditions it resolves to.
type Accent struct {
	Name string
	Note string

	// Why is the reason the accent overrides the common text lightness, and is
	// empty for every accent that does not.
	Why string

	Hue float64

	Text      oklch.LCh
	Deep      oklch.LCh
	Wash      oklch.LCh
	Container oklch.LCh
}

type spec struct {
	Surfaces   surfacesSpec   `toml:"surfaces"`
	Renditions renditionsSpec `toml:"renditions"`
	Accents    []accentSpec   `toml:"accent"`
}

func (s *spec) resolve() (*Palette, error) {
	p := &Palette{
		Surfaces: make([]Color, len(s.Surfaces.Steps)),
		Accents:  make([]Accent, len(s.Accents)),
	}

	for i, step := range s.Surfaces.Steps {
		p.Surfaces[i] = Color{
			Name: step.Name,
			Note: step.Note,
			LCh: oklch.LCh{
				L: step.Lightness / 100,
				C: s.Surfaces.chromaAt(step.Lightness),
				H: s.Surfaces.Hue,
			}.Fit(),
		}
	}

	for i, a := range s.Accents {
		text := s.Renditions.Text

		// NOTE: the override moves the text and nothing else. It is about how a
		// hue reads at the text lightness, so carrying it into the grounds
		// would put them at four lightnesses and invert severity.
		if a.Lightness > 0 {
			text.Lightness = a.Lightness
		}

		wash, err := s.ground(s.Renditions.Wash, a.Hue, p)
		if err != nil {
			return nil, fmt.Errorf("accent %q's wash: %w", a.Name, err)
		}

		container, err := s.ground(s.Renditions.Container, a.Hue, p)
		if err != nil {
			return nil, fmt.Errorf("accent %q's container: %w", a.Name, err)
		}

		p.Accents[i] = Accent{
			Name: a.Name,
			Note: a.Note,
			Why:  a.Why,
			Hue:  a.Hue,

			Text:      text.at(a.Hue),
			Deep:      s.Renditions.Deep.at(a.Hue),
			Wash:      wash,
			Container: container,
		}
	}

	return p, nil
}

const (
	// riseStep is finer than an 8-bit channel can express, so the answer is
	// the lowest a ground could sit and still clear.
	riseStep = 0.01

	// ceiling is as far as a ground rises before the palette is rejected for
	// asking a separation no lightness could buy.
	ceiling = 60.0
)

// ground places a ground on the wheel, over the ladder's own chroma there, and
// fits it to sRGB. Where it has been asked to clear a surface and does not, it
// rises until it does: hue is free and lightness is not, so it spends the least
// lightness that works.
func (s *spec) ground(g groundSpec, hue float64, p *Palette) (oklch.LCh, error) {
	at := func(lightness float64) oklch.LCh {
		return oklch.LCh{
			L: lightness / 100,
			C: s.Surfaces.chromaAt(lightness) + g.Over,
			H: hue,
		}.Fit()
	}

	c := at(g.Lightness)
	if g.Clear == 0 {
		return c, nil
	}

	against, ok := p.Surface(g.Against)
	if !ok {
		panic("palette: validate let a ground clear a surface that does not exist")
	}

	for l := g.Lightness; oklch.Difference(c, against.LCh) < g.Clear; {
		if l >= ceiling {
			return oklch.LCh{}, fmt.Errorf("no lightness under %g clears %q by %g", ceiling, g.Against, g.Clear)
		}

		l += riseStep
		c = at(l)
	}

	return c, nil
}

// surfacesSpec is the accent-free ladder. Chroma is the height of one curve
// rather than a value per step, so a step says only how light it is.
type surfacesSpec struct {
	Hue    float64    `toml:"hue"`
	Chroma float64    `toml:"chroma"`
	Steps  []stepSpec `toml:"step"`
}

// chromaAt returns how much color the ladder carries at a lightness: the peak,
// shaped by a squared sine.
func (s *surfacesSpec) chromaAt(lightness float64) float64 {
	sine := math.Sin(math.Pi * lightness / 100)

	return s.Chroma * sine * sine
}

type stepSpec struct {
	Name      string  `toml:"name"`
	Lightness float64 `toml:"lightness"`
	Note      string  `toml:"note"`
}

type renditionsSpec struct {
	Text      renditionSpec `toml:"text"`
	Deep      renditionSpec `toml:"deep"`
	Wash      groundSpec    `toml:"wash"`
	Container groundSpec    `toml:"container"`
}

// renditionSpec places one rendition. Chroma is a fraction of what sRGB can
// show there, and Max caps it absolutely so the accents with room to spare do
// not shout over the ones without. Zero means no cap.
type renditionSpec struct {
	Lightness float64 `toml:"lightness"`
	Chroma    float64 `toml:"chroma"`
	Max       float64 `toml:"max"`
}

func (r renditionSpec) at(hue float64) oklch.LCh {
	l := r.Lightness / 100

	c := r.Chroma * oklch.MaxChroma(l, hue)
	if r.Max > 0 {
		c = math.Min(c, r.Max)
	}

	return oklch.LCh{L: l, C: c, H: hue}
}

// groundSpec places a tinted ground. Unlike a fill it is measured against the
// surface beside it rather than against sRGB, so Over is how much more color it
// carries than the plain ladder at that lightness.
type groundSpec struct {
	Lightness float64 `toml:"lightness"`
	Over      float64 `toml:"over"`

	// Clear is how far the ground must end up from the surface named in
	// Against, as [oklch.Difference]. Chroma buys it first; Lightness rises
	// only where sRGB has none left. Zero asks for nothing.
	Clear   float64 `toml:"clear"`
	Against string  `toml:"against"`
}

// accentSpec places one accent on the wheel. Lightness, when not zero,
// overrides the text rendition's, and Why says what about sRGB or vision needs
// it to.
type accentSpec struct {
	Name      string  `toml:"name"`
	Hue       float64 `toml:"hue"`
	Lightness float64 `toml:"lightness"`
	Note      string  `toml:"note"`
	Why       string  `toml:"why"`
}
