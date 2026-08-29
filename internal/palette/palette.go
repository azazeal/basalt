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

// Palette is a resolved palette: the surface ladder in order from the deepest
// to the lightest, and the accent wheel in the order it was written down.
type Palette struct {
	Surfaces []Color
	Accents  []Accent
}

// Color is one entry, with the note saying what it is for.
type Color struct {
	Name string
	Note string
	LCh  oklch.LCh
}

// Hex renders the color as "#RRGGBB".
func (c Color) Hex() string {
	return c.LCh.Hex()
}

// RGB returns the color in sRGB.
func (c Color) RGB() oklch.RGB {
	return c.LCh.Fit().RGB()
}

// Accent is one place on the wheel and the four renditions it resolves to. A
// fifth use, a bright fill with the deepest surface as its text, is
// [Accent.Text] again rather than a color of its own.
type Accent struct {
	Name string
	Note string

	// Why is the reason the accent overrides the common text lightness, and
	// is empty for every accent that does not.
	Why string

	Hue float64

	Text      Color
	Deep      Color
	Wash      Color
	Container Color
}

// Surface returns the named surface step.
func (p *Palette) Surface(name string) (Color, bool) {
	for _, c := range p.Surfaces {
		if c.Name == name {
			return c, true
		}
	}

	return Color{}, false
}

// Accent returns the named accent.
func (p *Palette) Accent(name string) (Accent, bool) {
	for _, a := range p.Accents {
		if a.Name == name {
			return a, true
		}
	}

	return Accent{}, false
}

type spec struct {
	Surfaces   surfacesSpec   `toml:"surfaces"`
	Renditions renditionsSpec `toml:"renditions"`
	Accents    []accentSpec   `toml:"accent"`
}

// surfacesSpec is the accent-free ladder. Chroma is the height of one curve
// rather than a value per step, so a step says only how light it is.
type surfacesSpec struct {
	Hue    float64    `toml:"hue"`
	Chroma float64    `toml:"chroma"`
	Steps  []stepSpec `toml:"step"`
}

// chromaAt returns how much color the ladder carries at a lightness: the peak,
// shaped by a squared sine. The ends go to nothing because a near-black cannot
// show a tint and a near-white wearing one looks dirty.
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

// groundSpec places a tinted ground. Unlike a fill it is measured against the
// surface beside it rather than against sRGB, so Over is how much more color it
// carries than the plain ladder at that lightness.
//
// There are two, and the difference is whether you look at the ground or
// through it. A container is the strip and has text on it; a wash lies under
// text that already has a color. Those pull opposite ways, so one value cannot
// be both.
type groundSpec struct {
	Lightness float64 `toml:"lightness"`
	Over      float64 `toml:"over"`

	// Clear is how far the ground must end up from the surface named in
	// Against, as [oklch.Difference]. Chroma buys it first; Lightness rises
	// only where sRGB has none left. Zero asks for nothing.
	Clear   float64 `toml:"clear"`
	Against string  `toml:"against"`
}

// ground places a ground on the wheel, over the ladder's own chroma there. Fit
// pulls it back where sRGB cannot hold that much, as it does for cyan and blue:
// a dark teal barely exists.
func (s *spec) ground(g groundSpec, hue float64) oklch.LCh {
	return oklch.LCh{
		L: g.Lightness / 100,
		C: s.Surfaces.chromaAt(g.Lightness) + g.Over,
		H: hue,
	}.Fit()
}

const (
	// riseStep is finer than an 8-bit channel can express, so the answer is
	// the lowest a ground could sit and still clear.
	riseStep = 0.01

	// ceiling stops the walk if a palette asks for a separation no lightness
	// could buy. The furthest any accent has had to rise is three and a half.
	ceiling = 60.0
)

// clearedGround places a ground and, where it has been asked to clear a surface
// and does not, raises it until it does. Hue is free and lightness is not, so
// it spends the least lightness that works and most accents never move.
func (s *spec) clearedGround(g groundSpec, hue float64, against oklch.LCh, ok bool) oklch.LCh {
	c := s.ground(g, hue)

	if g.Clear <= 0 || !ok {
		return c
	}

	for g.Lightness < ceiling && oklch.Difference(c, against) < g.Clear {
		g.Lightness += riseStep
		c = s.ground(g, hue)
	}

	return c
}

// renditionSpec places one rendition. Chroma is a fraction of what sRGB can
// show there, and Max caps it absolutely so the accents with room to spare do
// not shout over the ones without. Zero means no cap.
type renditionSpec struct {
	Lightness float64 `toml:"lightness"`
	Chroma    float64 `toml:"chroma"`
	Max       float64 `toml:"max"`
}

// accentSpec places one accent on the wheel. Lightness overrides the text
// rendition's, only where sRGB is not symmetric enough for the common one; Why
// must say what the asymmetry is, so an override cannot pass as a preference.
// Zero means no override.
type accentSpec struct {
	Name      string  `toml:"name"`
	Hue       float64 `toml:"hue"`
	Lightness float64 `toml:"lightness"`
	Note      string  `toml:"note"`
	Why       string  `toml:"why"`
}

// Load reads a palette from a TOML file and resolves it.
func Load(path string) (*Palette, error) {
	var s spec
	if _, err := toml.DecodeFile(path, &s); err != nil {
		return nil, fmt.Errorf("palette: reading %s: %w", path, err)
	}

	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("palette: %s: %w", path, err)
	}

	return s.resolve(), nil
}

func (s *spec) resolve() *Palette {
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

	// The surface a wash must stay distinguishable from, resolved once. Named
	// in the palette rather than assumed here: which ground a wash lands on is
	// a fact about the editor, not about color.
	var (
		against    oklch.LCh
		hasAgainst bool
	)

	if name := s.Renditions.Wash.Against; name != "" {
		if c, found := p.Surface(name); found {
			against, hasAgainst = c.LCh, true
		}
	}

	for i, a := range s.Accents {
		text := s.Renditions.Text

		// NOTE(@azazeal): the override moves the text and nothing else. It is
		// about how a hue reads at the text lightness, so carrying it into the
		// grounds would put them at four lightnesses and invert severity.
		if a.Lightness > 0 {
			text.Lightness = a.Lightness
		}

		p.Accents[i] = Accent{
			Name: a.Name,
			Note: a.Note,
			Why:  a.Why,
			Hue:  a.Hue,

			Text:      Color{Name: a.Name, Note: a.Note, LCh: text.at(a.Hue)},
			Deep:      Color{Name: a.Name, Note: a.Note, LCh: s.Renditions.Deep.at(a.Hue)},
			Wash:      Color{Name: a.Name, Note: a.Note, LCh: s.clearedGround(s.Renditions.Wash, a.Hue, against, hasAgainst)},
			Container: Color{Name: a.Name, Note: a.Note, LCh: s.ground(s.Renditions.Container, a.Hue)},
		}
	}

	return p
}

// at places the rendition on the wheel at the given hue.
func (r renditionSpec) at(hue float64) oklch.LCh {
	l := r.Lightness / 100

	c := r.Chroma * oklch.MaxChroma(l, hue)
	if r.Max > 0 {
		c = math.Min(c, r.Max)
	}

	return oklch.LCh{L: l, C: c, H: hue}
}
