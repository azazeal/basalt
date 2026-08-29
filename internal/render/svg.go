// Package render draws a palette so it can be looked at.
//
// The sheet is not a picture of the theme, it is the theme: every color on it
// comes from the resolved palette, so a sheet that looks wrong is a palette
// that is wrong. A screenshot could only show one installed port, and only the
// half of the theme that lives in a text field.
package render

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/azazeal/basalt/internal/palette"
)

// The sheet is one column of sections, each a full content-width band.
const (
	margin  = 56
	content = 1096

	rowGap     = 10
	sectionGap = 46

	surfaceRow  = 40
	accentRow   = 62
	labelColumn = 268

	lineHeight = 22
	codeSize   = 13

	titleBar  = 38
	tabStrip  = 32
	statusBar = 28
	sidebar   = 208

	mono = "ui-monospace, 'JetBrains Mono', 'DejaVu Sans Mono', Menlo, Consolas, monospace"
	sans = "ui-sans-serif, system-ui, 'Inter', 'DejaVu Sans', sans-serif"
)

// sheet accumulates the drawing and tracks how far down the page it has got.
type sheet struct {
	buf bytes.Buffer
	y   int
	p   *palette.Palette
}

// SVG draws the palette as a single sheet.
func SVG(p *palette.Palette) []byte {
	s := &sheet{p: p, y: margin}

	// Drawn first: the page height is not known until the last section lands.
	s.title()
	s.surfaces()
	s.accents()
	s.window()
	s.modes()
	s.bar()
	s.footer()

	height, width := s.y+margin, content+margin*2

	var out bytes.Buffer
	fmt.Fprintf(&out,
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="%s">`,
		width, height, width, height, attr(sans),
	)
	fmt.Fprintf(&out, `<rect width="%d" height="%d" fill="%s"/>`, width, height, s.hex("sunk"))
	out.Write(s.buf.Bytes())
	out.WriteString(`</svg>`)

	return out.Bytes()
}

// esc escapes the three characters XML text cannot hold literally. Quotes are
// left alone: legal in a text node, and some renderers mangle the entity.
func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// attr escapes a string for use inside a double-quoted attribute.
func attr(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", `"`, "&quot;").Replace(s)
}

// hex returns a surface by name, or a color from nowhere in the palette, so a
// rename shows up on the sheet rather than failing quietly.
func (s *sheet) hex(name string) string {
	c, ok := s.p.Surface(name)
	if !ok {
		return "#FF00FF"
	}

	return c.Hex()
}

func (s *sheet) accentHex(name string) string {
	a, ok := s.p.Accent(name)
	if !ok {
		return "#FF00FF"
	}

	return a.Text.Hex()
}

type opt func(*strings.Builder)

func size(v int) opt { return func(b *strings.Builder) { fmt.Fprintf(b, ` font-size="%d"`, v) } }
func family(v string) opt {
	return func(b *strings.Builder) { fmt.Fprintf(b, ` font-family="%s"`, attr(v)) }
}
func anchor(v string) opt { return func(b *strings.Builder) { fmt.Fprintf(b, ` text-anchor="%s"`, v) } }
func weight(v string) opt { return func(b *strings.Builder) { fmt.Fprintf(b, ` font-weight="%s"`, v) } }
func italic() opt         { return func(b *strings.Builder) { b.WriteString(` font-style="italic"`) } }

func (s *sheet) text(x, y int, fill, str string, opts ...opt) {
	var b strings.Builder
	for _, o := range opts {
		o(&b)
	}

	fmt.Fprintf(&s.buf, `<text x="%d" y="%d" fill="%s"%s>%s</text>`, x, y, fill, b.String(), esc(str))
}

func (s *sheet) rect(x, y, w, h, r int, fill string) {
	fmt.Fprintf(&s.buf, `<rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="%s"/>`, x, y, w, h, r, fill)
}

// swatch is a rect with a thin stroke, which is what parts the deepest
// surfaces from the sheet they are drawn on.
func (s *sheet) swatch(x, y, w, h, r int, fill string) {
	fmt.Fprintf(&s.buf,
		`<rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="%s" stroke="%s" stroke-width="1"/>`,
		x, y, w, h, r, fill, s.hex("lifted"),
	)
}

// chevron draws the triangle before a folder, right when shut and down when
// open. It takes the mark step rather than the folder's color: which way it
// points is all it says, and it should not compete with the name.
func (s *sheet) chevron(x, y int, open bool) {
	var d string
	if open {
		d = fmt.Sprintf("M%d,%d L%d,%d L%d,%d Z", x-3, y-1, x+3, y-1, x, y+3)
	} else {
		d = fmt.Sprintf("M%d,%d L%d,%d L%d,%d Z", x-1, y-3, x+3, y, x-1, y+3)
	}

	fmt.Fprintf(&s.buf, `<path d="%s" fill="%s"/>`, d, s.hex("faint"))
}

// folder draws the icon before a name: filled when open, outlined when shut,
// a page for a file. Drawn rather than typed, since a glyph would want a font
// the reader has no reason to have.
func (s *sheet) folder(x, y int, dir, open bool, fill string) {
	if !dir {
		// A page, with the corner turned down.
		fmt.Fprintf(&s.buf,
			`<path d="M%d,%d h5 l3,3 v6 h-8 z" fill="none" stroke="%s" stroke-width="1"/>`,
			x-4, y-5, fill)

		return
	}

	body := fmt.Sprintf("M%d,%d h4 l1,2 h5 v6 h-10 z", x-5, y-5)
	if open {
		fmt.Fprintf(&s.buf, `<path d="%s" fill="%s"/>`, body, fill)

		return
	}

	fmt.Fprintf(&s.buf, `<path d="%s" fill="none" stroke="%s" stroke-width="1"/>`, body, fill)
}

// undercurl draws the wave under a token. A shape rather than a color change,
// because what the token is stays true while the server complains about it.
func (s *sheet) undercurl(x, y, w int, stroke string) {
	const step = 4

	d := fmt.Sprintf("M%d,%d", x, y)
	for i := 0; i < w/step; i++ {
		up := i%2 == 0

		dy := 3
		if up {
			dy = -3
		}

		d += fmt.Sprintf(" q%d,%d %d,0", step/2, dy, step)
	}

	fmt.Fprintf(&s.buf, `<path d="%s" fill="none" stroke="%s" stroke-width="1"/>`, d, stroke)
}

func (s *sheet) title() {
	s.text(margin, s.y+30, s.hex("bright"), "basalt", size(34), weight("600"))
	s.text(margin, s.y+56, s.hex("muted"),
		"a dark theme, named after what magma becomes when it cools", size(14))

	s.y += 56 + sectionGap
}

func (s *sheet) heading(str, note string) {
	s.text(margin, s.y, s.hex("dim"), strings.ToUpper(str), size(12), weight("600"))

	if note != "" {
		s.text(margin+content, s.y, s.hex("faint"), note, size(12), anchor("end"))
	}

	s.y += 22
}

// surfaces draws the ladder as one row of blocks, then again as rows carrying
// the note that says what each step is for.
func (s *sheet) surfaces() {
	s.heading("surfaces", "one hue, a ladder of lightness")

	n := len(s.p.Surfaces)
	if n == 0 {
		return
	}

	// One unbroken run, so the spacing of the steps can be read off it.
	const ladder = 56

	w := content / n
	for i, c := range s.p.Surfaces {
		r := 0
		if i == 0 || i == n-1 {
			r = 6
		}

		s.swatch(margin+i*w, s.y, w, ladder, r, c.Hex())
	}

	s.y += ladder + 22

	for _, c := range s.p.Surfaces {
		s.swatch(margin, s.y, 96, surfaceRow-rowGap, 4, c.Hex())

		mid := s.y + (surfaceRow-rowGap)/2 + 4

		s.text(margin+110, mid, s.hex("body"), c.Name, size(13), weight("500"))
		s.text(margin+200, mid, s.hex("faint"), c.Hex(), size(12), family(mono))
		s.text(margin+300, mid, s.hex("muted"), c.Note, size(12))

		s.y += surfaceRow
	}

	s.y += sectionGap - rowGap
}

// accents draws one row per accent: what it means on the left, and its three
// renditions on the right, each carrying the text that is read on it.
func (s *sheet) accents() {
	s.heading("accents", "text · deep fill · wash · container")

	w := (content - labelColumn - rowGap*3) / 4
	sunk, bright, body := s.hex("sunk"), s.hex("bright"), s.hex("body")

	for _, a := range s.p.Accents {
		top := s.y
		mid := top + accentRow/2 - 4

		s.text(margin, mid, s.hex("body"), a.Name, size(14), weight("600"))
		s.text(margin, mid+16, s.hex("muted"), wrapNote(a.Note), size(11))

		// An accent off the common lightness says so here too: the sheet is
		// what gets looked at.
		if a.Why != "" {
			s.text(margin, mid+30, s.hex("faint"), "· off the common lightness", size(10), italic())
		}

		// Drawn on the page as a token is, with the same value as a fill.
		x := margin + labelColumn
		s.swatch(x, top, w, accentRow-rowGap, 5, s.hex("page"))
		s.text(x+14, mid+4, a.Text.Hex(), "Aa", size(15), family(mono), weight("500"))
		s.text(x+44, mid+4, a.Text.Hex(), a.Text.Hex(), size(11), family(mono))
		s.rect(x+w-50, top+8, 40, accentRow-rowGap-16, 4, a.Text.Hex())
		s.text(x+w-30, mid+4, sunk, "fill", size(10), family(mono), anchor("middle"), weight("600"))

		// Deep, a fill in the serious register with bright text on it.
		x += w + rowGap
		s.swatch(x, top, w, accentRow-rowGap, 5, a.Deep.Hex())
		s.text(x+14, mid+4, bright, "Aa", size(15), family(mono), weight("500"))
		s.text(x+44, mid+4, bright, a.Deep.Hex(), size(11), family(mono))

		// Shown with a comment on it: the dimmest thing a wash has to leave
		// readable.
		x += w + rowGap
		s.swatch(x, top, w, accentRow-rowGap, 5, a.Wash.Hex())
		s.text(x+14, mid+4, s.hex("muted"), "Aa", size(15), family(mono), weight("500"))
		s.text(x+44, mid+4, s.hex("muted"), a.Wash.Hex(), size(11), family(mono))

		// The hue carries what the ground says, not the text, which is why this
		// is body and not the accent's own value.
		x += w + rowGap
		s.swatch(x, top, w, accentRow-rowGap, 5, a.Container.Hex())
		s.text(x+14, mid+4, body, "Aa", size(15), family(mono), weight("500"))
		s.text(x+44, mid+4, body, a.Container.Hex(), size(11), family(mono))

		s.y += accentRow
	}

	s.y += sectionGap - rowGap
}

// wrapNote cuts a note at the last space that fits: one line, no layout
// engine.
func wrapNote(note string) string {
	const fits = 48

	if len(note) <= fits {
		return note
	}

	if i := strings.LastIndex(note[:fits], " "); i > 0 {
		return note[:i] + "…"
	}

	return note[:fits] + "…"
}

// span is a run of characters and where its color comes from: an accent by
// name, or a surface when accent is empty.
type span struct {
	text    string
	accent  string
	surface string
	italic  bool
}

func (s *sheet) spanFill(sp span) string {
	if sp.accent != "" {
		return s.accentHex(sp.accent)
	}

	return s.hex(sp.surface)
}

// tabStop is one level of indent. SVG has no tab stops and renders a tab as a
// single space, so an indent has to be spelled out.
const tabStop = "    "

// charWidth is what one column of the sample is worth. Lines are drawn with an
// explicit textLength, pinning every character to a known column rather than to
// whatever monospace the reader has. That is what lets a selection be drawn as
// a rectangle over the right characters.
const charWidth = 8

// columns counts what a run of spans occupies, with tabs already expanded.
func columns(spans []span) int {
	n := 0
	for _, sp := range spans {
		n += len([]rune(strings.ReplaceAll(sp.text, "\t", tabStop)))
	}

	return n
}

// line draws a run of spans as one text element. Ligatures are off: a font
// that draws != as one glyph shows something the file does not say.
func (s *sheet) line(x, y int, spans []span) {
	n := columns(spans)
	if n == 0 {
		return
	}

	fmt.Fprintf(&s.buf,
		`<text x="%d" y="%d" font-size="%d" font-family="%s" textLength="%d" xml:space="preserve"`+
			` style="font-variant-ligatures:none">`,
		x, y, codeSize, attr(mono), n*charWidth,
	)

	for _, sp := range spans {
		style := ""
		if sp.italic {
			style = ` font-style="italic"`
		}

		text := strings.ReplaceAll(sp.text, "\t", tabStop)

		fmt.Fprintf(&s.buf, `<tspan fill="%s"%s>%s</tspan>`, s.spanFill(sp), style, esc(text))
	}

	s.buf.WriteString(`</text>`)
}

// window draws the theme as an editor: a title bar, a tree beside the file, a
// tab strip over it, the file itself, and a status line under it. Every one of
// those is a different step of the ladder, which is the part of the theme a
// row of swatches cannot show.
func (s *sheet) window() {
	s.heading("in a window", "the ladder doing the work a border would")

	top := s.y
	body := 13*lineHeight + 20
	h := titleBar + tabStrip + body + statusBar

	// The window itself, clipped so the corners round the whole stack rather
	// than each band of it.
	fmt.Fprintf(&s.buf, `<clipPath id="win"><rect x="%d" y="%d" width="%d" height="%d" rx="9"/></clipPath>`,
		margin, top, content, h)
	s.buf.WriteString(`<g clip-path="url(#win)">`)

	// The title bar, in the raised surface, with the three lights drawn in the
	// theme's own accents rather than the ones a Mac would use.
	s.rect(margin, top, content, titleBar, 0, s.hex("raised"))

	for i, name := range []string{"red", "yellow", "green"} {
		fmt.Fprintf(&s.buf, `<circle cx="%d" cy="%d" r="6" fill="%s"/>`,
			margin+24+i*20, top+titleBar/2, s.accentHex(name))
	}

	s.text(margin+content/2, top+titleBar/2+5, s.hex("dim"), "oklch.go — basalt",
		size(12), anchor("middle"))

	// The tree, on the surface behind the page. No separator is drawn between
	// it and the file: the step in the ladder is the separator, which is the
	// claim the whole theme rests on.
	inner := top + titleBar
	s.rect(margin, inner, sidebar, tabStrip+body, 0, s.hex("sunk"))

	s.text(margin+30, inner+22, s.accentHex("magenta"), "basalt", size(12), weight("600"))

	// Depth is a number rather than spaces in the name: SVG collapses runs of
	// whitespace unless asked not to, so an indent written into the string
	// comes out flat.
	for i, f := range []struct {
		name    string
		depth   int
		dir     bool
		open    bool
		accent  string
		surface string
	}{
		{name: "internal", depth: 1, dir: true, open: true, accent: "blue"},
		{name: "oklch", depth: 2, dir: true, accent: "blue"},
		{name: "palette", depth: 2, dir: true, accent: "blue"},
		{name: "render", depth: 2, dir: true, open: true, accent: "blue"},
		{name: "svg.go", depth: 3, accent: "green"},
		{name: "palette", depth: 1, dir: true, open: true, accent: "blue"},
		{name: "basalt.toml", depth: 2, accent: "yellow"},
		{name: "ports", depth: 1, dir: true, surface: "faint"},
		{name: "README.md", depth: 1, surface: "muted"},
		{name: "go.mod", depth: 1, surface: "body"},
	} {
		const indent = 14

		x := margin + 16 + f.depth*indent
		y := inner + 46 + i*20

		// One per level the row sits inside, in the step named for exactly
		// this.
		for lv := 1; lv <= f.depth; lv++ {
			gx := margin + 16 + lv*indent - 8
			fmt.Fprintf(&s.buf, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`,
				gx, y-14, gx, y+6, s.hex("guide"))
		}

		fill := s.hex(f.surface)
		if f.accent != "" {
			fill = s.accentHex(f.accent)
		}

		if f.dir {
			s.chevron(x+2, y-4, f.open)
		}

		s.folder(x+12, y-4, f.dir, f.open, fill)

		s.text(x+28, y, fill, f.name, size(11), family(mono))
	}

	// The open tab lifts to the status line's surface, the rest stay on the
	// tree's.
	pane := margin + sidebar
	paneW := content - sidebar

	s.rect(pane, inner, paneW, tabStrip, 0, s.hex("sunk"))
	s.rect(pane, inner, 110, tabStrip, 0, s.hex("raised"))
	s.text(pane+16, inner+20, s.hex("body"), "oklch.go", size(11), family(mono), weight("500"))
	s.text(pane+130, inner+20, s.hex("faint"), "palette.go", size(11), family(mono))
	s.text(pane+228, inner+20, s.accentHex("yellow"), "basalt.toml ●", size(11), family(mono))

	// The page.
	code := inner + tabStrip
	s.rect(pane, code, paneW, body, 0, s.hex("page"))

	lines := [][]span{
		{{text: "// Fit pulls a color back inside sRGB, dulling it", surface: "muted", italic: true}},
		{{text: "// rather than shifting the hue it was asked for.", surface: "muted", italic: true}},
		{{text: "func", accent: "magenta"}, {text: " (c ", surface: "body"}, {text: "LCh", accent: "yellow"},
			{text: ") ", surface: "body"}, {text: "Fit", accent: "blue"}, {text: "() ", surface: "body"},
			{text: "LCh", accent: "yellow"}, {text: " {", surface: "body"}},
		{{text: "\tif", accent: "magenta"}, {text: " c.", surface: "body"}, {text: "RGB", accent: "blue"},
			{text: "().", surface: "body"}, {text: "InGamut", accent: "blue"}, {text: "() {", surface: "body"}},
		{{text: "\t\treturn", accent: "magenta"}, {text: " c", surface: "body"}},
		{{text: "\t}", surface: "body"}},
		{{text: "\tvar out ", surface: "body"}, {text: "LCh", accent: "yellow"}},
		{{text: "\tc.C = ", surface: "body"}, {text: "oklch", accent: "yellow"}, {text: ".", surface: "body"},
			{text: "MaxChroma", accent: "blue"}, {text: "(c.L, c.H)", surface: "body"}},
		{{text: "\tconst", accent: "magenta"}, {text: " steps = ", surface: "body"},
			{text: "50", accent: "orange"}, {text: "   ", surface: "body"},
			{text: "// halvings", surface: "muted", italic: true}},
		{{text: "\tname := ", surface: "body"}, {text: `"basalt"`, accent: "green"},
			{text: " + ", surface: "body"}, {text: `"\n"`, accent: "cyan"}},
		{{text: "\t", surface: "body"}, {text: "//go:embed", accent: "cyan"},
			{text: " palette.toml", surface: "muted", italic: true}},
		{{text: "\tif", accent: "magenta"}, {text: " err != ", surface: "body"}, {text: "nil", accent: "orange"},
			{text: " { ", surface: "body"}, {text: "panic", accent: "red"}, {text: "(err) }", surface: "body"}},
		{{text: "}", surface: "body"}},
	}

	// The row the caret is on, one step up from the page.
	const caret = 2
	s.rect(pane, code+10+caret*lineHeight, paneW, lineHeight, 0, s.hex("row"))

	// What the wash is for: LCh is selected on the caret's line and its other
	// copies in view are marked without being selected. The copies take a
	// different accent from the selection, so where the caret is stays
	// findable.
	for _, m := range []struct {
		line, col, width int
		accent           string
	}{
		{line: 2, col: 8, width: 3, accent: "magenta"},
		{line: 2, col: 19, width: 3, accent: "cyan"},
		{line: 6, col: 12, width: 3, accent: "cyan"},
	} {
		a, ok := s.p.Accent(m.accent)
		if !ok {
			continue
		}

		s.rect(pane+68+m.col*charWidth, code+10+m.line*lineHeight,
			m.width*charWidth, lineHeight, 0, a.Wash.Hex())
	}

	for i, line := range lines {
		y := code + 26 + i*lineHeight

		fill, w := s.hex("faint"), "400"
		if i == caret {
			fill, w = s.hex("body"), "600"
		}

		s.text(pane+52, y, fill, fmt.Sprint(i+1), size(11), family(mono), anchor("end"), weight(w))

		if len(line) > 0 {
			s.line(pane+68, y, line)
		}
	}

	// The only thing in the theme that draws under the text rather than behind
	// it, and the one place `inlay` shows: what the server says is in the
	// window without being in the file.
	const diag = 11

	dy := code + 26 + diag*lineHeight
	dx := pane + 68 + 7*charWidth

	s.undercurl(dx, dy+4, 3*charWidth, s.accentHex("red"))

	hint := pane + 68 + 35*charWidth
	s.rect(hint-6, code+10+diag*lineHeight, 17*charWidth, lineHeight, 0, s.hex("inlay"))
	s.line(hint, dy, []span{{text: "undefined: err", accent: "red"}})

	// A mode block in the focus accent, the branch, then what the file is.
	st := code + body
	s.rect(margin, st, content, statusBar, 0, s.hex("raised"))
	s.rect(margin, st, 76, statusBar, 0, s.accentHex("blue"))
	s.text(margin+38, st+18, s.hex("sunk"), "NORMAL", size(11), family(mono),
		anchor("middle"), weight("700"))
	s.text(margin+92, st+18, s.accentHex("orange"), "main ●", size(11), family(mono))
	s.text(margin+content-16, st+18, s.hex("muted"), "go · utf-8 · 8:14", size(11),
		family(mono), anchor("end"))

	s.buf.WriteString(`</g>`)

	s.y = top + h + sectionGap
}

// modes draws an editor whose caret and selection change color per mode. Each
// mode fills its block and washes its selection with one accent, and every wash
// sits at one lightness, so the mode changes the hue and nothing else.
func (s *sheet) modes() {
	s.heading("one mode at a time", "the hue changes, the legibility does not")

	top := s.y

	const (
		blockW = 84
		rowH   = 34
		gap    = 8
	)

	sunk := s.hex("sunk")

	for i, m := range []struct{ name, accent string }{
		{"NORMAL", "blue"},
		{"INSERT", "green"},
		{"VISUAL", "magenta"},
		{"SELECT", "orange"},
		{"REPLACE", "red"},
		{"COMMAND", "yellow"},
	} {
		a, ok := s.p.Accent(m.accent)
		if !ok {
			continue
		}

		y := top + i*(rowH+gap)

		// The mode block: the accent filled, with the deepest surface on it.
		s.rect(margin, y, blockW, rowH, 4, a.Text.Hex())
		s.text(margin+blockW/2, y+rowH/2+4, sunk, m.name,
			size(11), family(mono), anchor("middle"), weight("700"))

		// The caret, which is the same value again in a narrower shape.
		s.rect(margin+blockW+gap, y+7, 9, rowH-14, 1, a.Text.Hex())

		// The comment is the point: it is the dimmest thing a wash ever covers,
		// and it reads the same under every one of these.
		x := margin + blockW + gap + 9 + gap
		w := content - (x - margin)

		s.rect(x, y, w, rowH, 4, s.hex("page"))
		s.rect(x+90, y, 25*charWidth, rowH, 0, a.Wash.Hex())

		s.line(x+12, y+rowH/2+5, []span{
			{text: "out := ", surface: "body"},
			{text: "Fit", accent: "blue"},
			{text: "(c)  ", surface: "body"},
			{text: "// dulled, never shifted", surface: "muted", italic: true},
		})
	}

	s.y = top + 6*(rowH+gap) - gap + sectionGap
}

// bar draws the desktop's strip, the half of the theme that needs a fill and a
// container to exist at all.
func (s *sheet) bar() {
	s.heading("on the desktop", "the same wheel, filled rather than read")

	top := s.y
	const h = 52

	s.rect(margin, top, content, h, 8, s.hex("raised"))

	x := margin + 14
	y := top + 10
	ih := h - 20

	// The focused one filled, the rest in the grey read without asking.
	for i, name := range []string{"1", "2", "3", "4"} {
		const w = 28

		if i == 0 {
			s.rect(x, y, w, ih, 5, s.accentHex("blue"))
			s.text(x+w/2, y+ih/2+4, s.hex("sunk"), name, size(12), anchor("middle"), weight("700"))
		} else {
			s.text(x+w/2, y+ih/2+4, s.hex("dim"), name, size(12), anchor("middle"))
		}

		x += w + 4
	}

	x += 16

	for _, chip := range []struct{ label, accent string }{
		{"48%", "green"},
		{"3.2 GB", "cyan"},
		{"62°C", "yellow"},
	} {
		const w = 82

		s.rect(x, y, w, ih, 5, s.hex("lifted"))
		s.text(x+w/2, y+ih/2+4, s.accentHex(chip.accent), chip.label, size(12),
			family(mono), anchor("middle"), weight("500"))

		x += w + 8
	}

	// A notice in the container register and a control that can end the session
	// in the deep one: one accent, two depths, two weights of meaning.
	right := margin + content - 14

	if a, ok := s.p.Accent("orange"); ok {
		const w = 52

		s.rect(right-w, y, w, ih, 5, a.Deep.Hex())
		s.text(right-w/2, y+ih/2+5, s.hex("bright"), "⏻", size(15), anchor("middle"))

		right -= w + 8
	}

	if a, ok := s.p.Accent("red"); ok {
		const w = 176

		s.rect(right-w, y, w, ih, 5, a.Container.Hex())
		s.text(right-w+14, y+ih/2+4, a.Text.Hex(), "3 updates pending", size(12), weight("500"))

		right -= w + 8
	}

	s.y = top + h + sectionGap
}

// footer says where the colors came from, since a sheet of hexes with no
// account of how they got there is what this repository exists to stop being.
func (s *sheet) footer() {
	// Read off the palette, so the sheet cannot claim a wheel it no longer has.
	for _, line := range []string{
		fmt.Sprintf(
			"Every color here is computed, not chosen. The surfaces are one hue at %d lightnesses; the accents are %d",
			len(s.p.Surfaces), len(s.p.Accents),
		),
		"places on the wheel, each rendered four ways: read as text, filled with," +
			" washed under text, and tinted into a ground.",
		"Nothing between lightness 49 and 58 is ever filled with, because no text reads on it.",
	} {
		s.text(margin, s.y, s.hex("faint"), line, size(11))

		s.y += 17
	}
}
