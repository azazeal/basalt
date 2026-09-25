// Command basalt resolves the palette, reports it, and writes the export and
// the sheet.
//
// The report exists so the palette can be looked at while it is argued about:
// every entry with its position, the color that works out to, and the contrast
// each rendition achieves against the surface it is read on.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/azazeal/basalt/internal/export"
	"github.com/azazeal/basalt/internal/oklch"
	"github.com/azazeal/basalt/internal/palette"
	"github.com/azazeal/basalt/internal/render"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "basalt: %v\n", err)

		os.Exit(1)
	}
}

func run() error {
	path := flag.String("palette", "palette/basalt.toml", "the palette to resolve")
	svg := flag.String("svg", "", "also draw the palette as a sheet, and write it here")
	out := flag.String("json", "", "also write the palette as JSON, and write it here")
	flag.Parse()

	p, err := palette.Load(*path)
	if err != nil {
		return err
	}

	// first, so a palette the report cannot measure writes nothing
	if err := report(os.Stdout, p); err != nil {
		return err
	}

	if *out != "" {
		data, err := export.JSON(p)
		if err != nil {
			return fmt.Errorf("failed rendering the palette as JSON: %w", err)
		}

		if err := os.WriteFile(*out, data, 0o644); err != nil {
			return fmt.Errorf("failed writing the palette: %w", err)
		}
	}

	if *svg != "" {
		if err := os.WriteFile(*svg, render.SVG(p), 0o644); err != nil {
			return fmt.Errorf("failed writing the sheet: %w", err)
		}
	}

	return nil
}

func report(w io.Writer, p *palette.Palette) error {
	var page, sunk, bright, muted, body palette.Color

	for _, s := range []struct {
		name string
		c    *palette.Color
	}{
		{"page", &page},
		{"sunk", &sunk},
		{"bright", &bright},
		{"muted", &muted},
		{"body", &body},
	} {
		c, ok := p.Surface(s.name)
		if !ok {
			return fmt.Errorf("the palette has no surface named %q to measure against", s.name)
		}

		*s.c = c
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintln(tw, "SURFACES\tHEX\tL\tC\tvs PAGE\t")
	for _, c := range p.Surfaces {
		fmt.Fprintf(tw, "%s\t%s\t%.1f\t%.4f\t%.2f\t%s\n",
			c.Name, c.Hex(), c.LCh.L*100, c.LCh.C, oklch.Contrast(c.RGB(), page.RGB()), c.Note)
	}

	fmt.Fprintln(tw, "\nACCENTS\tHUE\tTEXT\ton PAGE\tsunk on IT\tDEEP\tbright on IT\t"+
		"WASH\tcomment on IT\tCONTAINER\tbody on IT\t")
	for _, a := range p.Accents {
		fmt.Fprintf(tw, "%s\t%.1f\t%s\t%.2f\t%.2f\t%s\t%.2f\t%s\t%.2f\t%s\t%.2f\t\n",
			a.Name,
			a.Hue,
			a.Text.Hex(), oklch.Contrast(a.Text.RGB(), page.RGB()), oklch.Contrast(sunk.RGB(), a.Text.RGB()),
			a.Deep.Hex(), oklch.Contrast(bright.RGB(), a.Deep.RGB()),
			a.Wash.Hex(), oklch.Contrast(muted.RGB(), a.Wash.RGB()),
			a.Container.Hex(), oklch.Contrast(body.RGB(), a.Container.RGB()),
		)
	}

	return tw.Flush()
}
