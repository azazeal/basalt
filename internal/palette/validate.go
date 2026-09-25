package palette

import (
	"errors"
	"fmt"
	"slices"
)

// NOTE(@azazeal): between these two nothing reads on a fill, dark text and
// light alike, with the floor around 52. Anything filled with is checked
// against the band rather than trusted.
const trapLo, trapHi = 49.0, 58.0

// validate rejects a palette that cannot mean what it says. Loud on purpose:
// each check catches a typo that would otherwise render as a plausible color.
func (s *spec) validate() error {
	if err := s.Surfaces.validate(); err != nil {
		return err
	}

	if err := s.Renditions.validate(s.Surfaces.Steps); err != nil {
		return err
	}

	if len(s.Accents) == 0 {
		return errors.New("the wheel has no accents on it")
	}

	seen := make(map[string]struct{}, len(s.Accents))
	for _, a := range s.Accents {
		if a.Name == "" {
			return errors.New("an accent has no name")
		}

		if _, dup := seen[a.Name]; dup {
			return fmt.Errorf("accent %q is named twice", a.Name)
		}
		seen[a.Name] = struct{}{}

		if a.Hue < 0 || a.Hue >= 360 {
			return fmt.Errorf("accent %q sits at hue %g, which is not a place on the wheel", a.Name, a.Hue)
		}

		if a.Lightness == 0 {
			continue
		}

		if a.Lightness < 0 || a.Lightness > 100 {
			return fmt.Errorf("accent %q has lightness %g, which is off the scale", a.Name, a.Lightness)
		}

		// The text rendition is filled with as well as read, so an override
		// has to clear the same band the rendition itself does.
		if a.Lightness > trapLo && a.Lightness < trapHi {
			return fmt.Errorf(
				"accent %q has lightness %g, which is in the band from %g to %g where nothing reads on a fill",
				a.Name, a.Lightness, trapLo, trapHi,
			)
		}

		// An override with no reason is a preference in a rule's clothes, and
		// the next reader cannot tell the difference.
		if a.Why == "" {
			return fmt.Errorf("accent %q overrides the common lightness without saying why", a.Name)
		}
	}

	return nil
}

func (s *surfacesSpec) validate() error {
	if s.Hue < 0 || s.Hue >= 360 {
		return fmt.Errorf("the surfaces sit at hue %g, which is not a place on the wheel", s.Hue)
	}

	if s.Chroma <= 0 {
		return fmt.Errorf("the surfaces carry chroma %g at their peak, which is no color at all", s.Chroma)
	}

	if len(s.Steps) == 0 {
		return errors.New("the surface ladder has no steps")
	}

	seen := make(map[string]struct{}, len(s.Steps))
	prev := -1.0

	for _, step := range s.Steps {
		if step.Name == "" {
			return errors.New("a surface step has no name")
		}

		if _, dup := seen[step.Name]; dup {
			return fmt.Errorf("surface step %q is named twice", step.Name)
		}
		seen[step.Name] = struct{}{}

		if step.Lightness < 0 || step.Lightness > 100 {
			return fmt.Errorf("surface step %q has lightness %g, which is off the scale", step.Name, step.Lightness)
		}

		// A ladder that doubles back is always a mistake, and two steps at one
		// lightness are one step wearing two names.
		if step.Lightness <= prev {
			return fmt.Errorf("surface step %q is not lighter than the step before it", step.Name)
		}
		prev = step.Lightness
	}

	return nil
}

func (s *renditionsSpec) validate(surfaces []stepSpec) error {
	for _, r := range []struct {
		name string
		spec renditionSpec
	}{
		{"text", s.Text},
		{"deep", s.Deep},
	} {
		if err := r.spec.validate(); err != nil {
			return fmt.Errorf("rendition %q: %w", r.name, err)
		}

		if l := r.spec.Lightness; l > trapLo && l < trapHi {
			return fmt.Errorf(
				"rendition %q has lightness %g, which is in the band from %g to %g"+
					" where neither dark nor light text reads on a fill",
				r.name, l, trapLo, trapHi,
			)
		}
	}

	for _, g := range []struct {
		name string
		spec groundSpec
	}{
		{"wash", s.Wash},
		{"container", s.Container},
	} {
		if err := g.spec.validate(surfaces); err != nil {
			return fmt.Errorf("the %s grounds: %w", g.name, err)
		}
	}

	// The one looked through has to be the darker of the two.
	if s.Wash.Lightness >= s.Container.Lightness {
		return fmt.Errorf(
			"the wash sits at lightness %g and the container at %g, so the one meant to be seen through is not the lighter",
			s.Wash.Lightness, s.Container.Lightness,
		)
	}

	return nil
}

func (g groundSpec) validate(surfaces []stepSpec) error {
	if g.Lightness < 0 || g.Lightness > 100 {
		return fmt.Errorf("lightness %g is off the scale", g.Lightness)
	}

	if g.Over <= 0 {
		return fmt.Errorf("%g more color than the ladder would leave them the ladder", g.Over)
	}

	if g.Clear < 0 {
		return fmt.Errorf("clear %g is less than none", g.Clear)
	}

	// Either one alone would be read and then do nothing.
	if (g.Clear > 0) != (g.Against != "") {
		return errors.New("clear and against are given together or not at all")
	}

	if g.Against != "" && !slices.ContainsFunc(surfaces, func(s stepSpec) bool { return s.Name == g.Against }) {
		return fmt.Errorf("against names %q, which is not a surface", g.Against)
	}

	return nil
}

func (r renditionSpec) validate() error {
	if r.Lightness < 0 || r.Lightness > 100 {
		return fmt.Errorf("lightness %g is off the scale", r.Lightness)
	}

	if r.Chroma <= 0 || r.Chroma > 1 {
		return fmt.Errorf("chroma %g is not a fraction of what sRGB can show", r.Chroma)
	}

	if r.Max < 0 {
		return fmt.Errorf("the chroma cap %g is less than none", r.Max)
	}

	return nil
}
