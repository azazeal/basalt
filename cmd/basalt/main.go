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
	"os"
	"text/tabwriter"

	"github.com/azazeal/basalt/internal/export"
	"github.com/azazeal/basalt/internal/oklch"
	"github.com/azazeal/basalt/internal/palette"
	"github.com/azazeal/basalt/internal/render"
)

func main() {
	path := flag.String("palette", "palette/basalt.toml", "the palette to resolve")
	svg := flag.String("svg", "", "also draw the palette as a sheet, and write it here")
	out := flag.String("json", "", "also write the palette as JSON, and write it here")
	flag.Parse()

	if *path == "" {
		fmt.Fprintln(os.Stderr, "basalt: -palette needs the path to a palette")

		os.Exit(1)
	}

	p, err := palette.Load(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		os.Exit(1)
	}

	if *out != "" {
		data, err := export.JSON(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "basalt: rendering the palette as JSON: %v\n", err)

			os.Exit(1)
		}

		if err := os.WriteFile(*out, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "basalt: writing the palette: %v\n", err)

			os.Exit(1)
		}
	}

	if *svg != "" {
		if err := os.WriteFile(*svg, render.SVG(p), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "basalt: writing the sheet: %v\n", err)

			os.Exit(1)
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	report(w, p)
}

func report(w *tabwriter.Writer, p *palette.Palette) {
	page, ok := p.Surface("page")
	if !ok {
		fmt.Fprintln(os.Stderr, "the palette has no surface named \"page\" to measure against")

		os.Exit(1)
	}

	sunk, _ := p.Surface("sunk")
	bright, _ := p.Surface("bright")

	fmt.Fprintln(w, "SURFACES\tHEX\tL\tC\tvs PAGE\t")
	for _, c := range p.Surfaces {
		fmt.Fprintf(w, "%s\t%s\t%.1f\t%.4f\t%.2f\t%s\n",
			c.Name, c.Hex(), c.LCh.L*100, c.LCh.C, oklch.Contrast(c.RGB(), page.RGB()), c.Note)
	}

	// The four numbers an accent has to survive. Text is read on the page;
	// the same value is filled with and read against the deepest surface;
	// deep is filled with and read against the lightest; and the text value
	// is what sits on the container.
	// A wash is measured against the dimmest thing it must leave readable, and
	// a container against the ordinary text that sits on one.
	muted, _ := p.Surface("muted")
	body, _ := p.Surface("body")

	fmt.Fprintln(w, "\nACCENTS\tHUE\tTEXT\ton PAGE\tsunk on IT\tDEEP\tbright on IT\tWASH\tcomment on IT\tCONTAINER\tbody on IT\t")
	for _, a := range p.Accents {
		fmt.Fprintf(w, "%s\t%.1f\t%s\t%.2f\t%.2f\t%s\t%.2f\t%s\t%.2f\t%s\t%.2f\t\n",
			a.Name,
			a.Hue,
			a.Text.Hex(), oklch.Contrast(a.Text.RGB(), page.RGB()), oklch.Contrast(sunk.RGB(), a.Text.RGB()),
			a.Deep.Hex(), oklch.Contrast(bright.RGB(), a.Deep.RGB()),
			a.Wash.Hex(), oklch.Contrast(muted.RGB(), a.Wash.RGB()),
			a.Container.Hex(), oklch.Contrast(body.RGB(), a.Container.RGB()),
		)
	}
}
