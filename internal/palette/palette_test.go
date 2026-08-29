package palette

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// sound is a spec that passes every check, for a case to spoil one field of.
func sound() spec {
	return spec{
		Surfaces: surfacesSpec{
			Hue:    264,
			Chroma: 0.042,
			Steps: []stepSpec{
				{Name: "page", Lightness: 18},
				{Name: "raised", Lightness: 26},
				{Name: "body", Lightness: 76.8},
			},
		},
		Renditions: renditionsSpec{
			Text:      renditionSpec{Lightness: 74, Chroma: 0.90, Max: 0.160},
			Deep:      renditionSpec{Lightness: 43, Chroma: 0.85, Max: 0.150},
			Wash:      groundSpec{Lightness: 26, Over: 0.040},
			Container: groundSpec{Lightness: 34, Over: 0.040},
		},
		Accents: []accentSpec{
			{Name: "red", Hue: 18.3},
			{Name: "blue", Hue: 245.5},
		},
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		spoil   func(*spec)
		wantErr bool
	}{
		0: { // the sound spec passes
			spoil: func(*spec) {},
		},
		1: { // the surfaces sit off the wheel
			spoil:   func(s *spec) { s.Surfaces.Hue = 400 },
			wantErr: true,
		},
		2: { // no ladder at all
			spoil:   func(s *spec) { s.Surfaces.Steps = nil },
			wantErr: true,
		},
		3: { // a step with no name
			spoil:   func(s *spec) { s.Surfaces.Steps[1].Name = "" },
			wantErr: true,
		},
		4: { // one name on two steps
			spoil:   func(s *spec) { s.Surfaces.Steps[1].Name = "page" },
			wantErr: true,
		},
		5: { // the ladder doubles back
			spoil:   func(s *spec) { s.Surfaces.Steps[1].Lightness = 10 },
			wantErr: true,
		},
		6: { // two steps at one lightness, which is one step wearing two names
			spoil:   func(s *spec) { s.Surfaces.Steps[1].Lightness = 18 },
			wantErr: true,
		},
		7: { // lightness off the scale
			spoil:   func(s *spec) { s.Surfaces.Steps[2].Lightness = 140 },
			wantErr: true,
		},
		8: { // a ladder carrying no color at all
			spoil:   func(s *spec) { s.Surfaces.Chroma = 0 },
			wantErr: true,
		},
		9: { // an empty wheel
			spoil:   func(s *spec) { s.Accents = nil },
			wantErr: true,
		},
		10: { // an accent with no name
			spoil:   func(s *spec) { s.Accents[0].Name = "" },
			wantErr: true,
		},
		11: { // one name on two accents
			spoil:   func(s *spec) { s.Accents[1].Name = "red" },
			wantErr: true,
		},
		12: { // an accent off the wheel
			spoil:   func(s *spec) { s.Accents[0].Hue = -5 },
			wantErr: true,
		},
		13: { // chroma given as an amount rather than a fraction
			spoil:   func(s *spec) { s.Renditions.Text.Chroma = 1.5 },
			wantErr: true,
		},
		14: { // a rendition with no color at all
			spoil:   func(s *spec) { s.Renditions.Deep.Chroma = 0 },
			wantErr: true,
		},
		15: { // text parked in the band where nothing reads on a fill
			spoil:   func(s *spec) { s.Renditions.Text.Lightness = 52 },
			wantErr: true,
		},
		16: { // deep parked in the same band
			spoil:   func(s *spec) { s.Renditions.Deep.Lightness = 55 },
			wantErr: true,
		},
		17: { // the container may sit anywhere, since nothing is filled with it
			spoil: func(s *spec) { s.Renditions.Container.Lightness = 52 },
		},
		18: { // a container carrying no more color than the ladder it sits on
			spoil:   func(s *spec) { s.Renditions.Container.Over = 0 },
			wantErr: true,
		},
		19: { // the wash is not below the container, so the wrong one is subtle
			spoil:   func(s *spec) { s.Renditions.Wash.Lightness = 40 },
			wantErr: true,
		},
	}

	for caseIndex, kase := range cases {
		t.Run(strconv.Itoa(caseIndex), func(t *testing.T) {
			s := sound()
			kase.spoil(&s)

			switch err := s.validate(); {
			case kase.wantErr && err == nil:
				t.Error("validate() passed, want an error")
			case !kase.wantErr && err != nil:
				t.Errorf("validate() failed: %v", err)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	s := sound()
	p := s.resolve()

	if got, want := len(p.Surfaces), len(s.Surfaces.Steps); got != want {
		t.Fatalf("resolved %d surfaces, want %d", got, want)
	}

	// Every surface takes the one hue the ladder was given, which is what
	// keeps the greys from drifting apart as they lighten.
	for _, c := range p.Surfaces {
		if math.Abs(c.LCh.H-s.Surfaces.Hue) > 1e-9 {
			t.Errorf("surface %q sits at hue %.4f, want %.4f", c.Name, c.LCh.H, s.Surfaces.Hue)
		}
	}

	for _, a := range p.Accents {
		// All three renditions of an accent are the same accent, so they share
		// its place on the wheel and differ only in lightness and chroma.
		for _, r := range []struct {
			what string
			c    Color
		}{
			{"text", a.Text},
			{"deep", a.Deep},
			{"wash", a.Wash},
			{"container", a.Container},
		} {
			if math.Abs(r.c.LCh.H-a.Hue) > 1e-9 {
				t.Errorf("accent %q's %s sits at hue %.4f, want %.4f", a.Name, r.what, r.c.LCh.H, a.Hue)
			}

			if !r.c.LCh.RGB().InGamut() {
				t.Errorf("accent %q's %s is a color sRGB cannot show", a.Name, r.what)
			}
		}

		if !(a.Deep.LCh.L < a.Text.LCh.L) {
			t.Errorf("accent %q's deep rendition is not darker than its text one", a.Name)
		}

		if !(a.Container.LCh.L < a.Deep.LCh.L) {
			t.Errorf("accent %q's container is not darker than its deep rendition", a.Name)
		}

		// The one you look through has to sit under the one you look at.
		if !(a.Wash.LCh.L < a.Container.LCh.L) {
			t.Errorf("accent %q's wash is not darker than its container", a.Name)
		}
	}
}

func TestResolveCapsChroma(t *testing.T) {
	s := sound()

	// Magenta has room to spare at the text lightness and cyan does not, so
	// the cap is what stops one becoming twice the accent the other is.
	s.Accents = []accentSpec{
		{Name: "cyan", Hue: 206.5},
		{Name: "magenta", Hue: 318},
	}

	p := s.resolve()

	magenta, _ := p.Accent("magenta")
	if got, want := magenta.Text.LCh.C, s.Renditions.Text.Max; math.Abs(got-want) > 1e-9 {
		t.Errorf("magenta's text chroma is %.4f, want it held at the cap of %.4f", got, want)
	}

	cyan, _ := p.Accent("cyan")
	if cyan.Text.LCh.C >= s.Renditions.Text.Max {
		t.Errorf("cyan's text chroma is %.4f, want it under the cap at %.4f", cyan.Text.LCh.C, s.Renditions.Text.Max)
	}
}

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "palette.toml")

	const src = `
[surfaces]
hue = 264.0
chroma = 0.042

[[surfaces.step]]
name = "page"
lightness = 18.0

[[surfaces.step]]
name = "body"
lightness = 76.8

[renditions]
text = { lightness = 74.0, chroma = 0.90, max = 0.160 }
deep = { lightness = 43.0, chroma = 0.85, max = 0.150 }
wash = { lightness = 26.0, over = 0.040 }
container = { lightness = 34.0, over = 0.040 }

[[accent]]
name = "blue"
hue = 245.5
note = "functions, focus"
`

	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("writing the palette failed: %v", err)
	}

	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	page, ok := p.Surface("page")
	if !ok {
		t.Fatal("the resolved palette has no surface named \"page\"")
	}

	if got, want := page.Hex(), "#0F1217"; got != want {
		t.Errorf("page = %s, want %s", got, want)
	}

	blue, ok := p.Accent("blue")
	if !ok {
		t.Fatal("the resolved palette has no accent named \"blue\"")
	}

	if got, want := blue.Note, "functions, focus"; got != want {
		t.Errorf("blue's note = %q, want %q", got, want)
	}
}

func TestLoadRejectsAnUnsoundPalette(t *testing.T) {
	path := filepath.Join(t.TempDir(), "palette.toml")

	// A ladder that doubles back, which validate has to catch on the way in
	// rather than leave for a port to render.
	const src = `
[surfaces]
hue = 264.0
chroma = 0.042

[[surfaces.step]]
name = "page"
lightness = 40.0

[[surfaces.step]]
name = "body"
lightness = 20.0

[renditions]
text = { lightness = 74.0, chroma = 0.90, max = 0.160 }
deep = { lightness = 43.0, chroma = 0.85, max = 0.150 }
wash = { lightness = 26.0, over = 0.040 }
container = { lightness = 34.0, over = 0.040 }

[[accent]]
name = "blue"
hue = 245.5
`

	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("writing the palette failed: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Error("Load accepted a ladder that doubles back")
	}
}
