// Package render turns formula sheets into beautiful output:
//
//   - HTML: printable page with KaTeX-rendered math, topic sections, and
//     "builds on" chips linking each formula to its prerequisites.
//   - LaTeX: compilable .tex document (run pdflatex yourself).
//   - PDF: paginated PDF via gofpdf. Math is shown as LaTeX source in
//     monospace — gofpdf has no TeX engine, so this is a clean reading
//     copy, not typeset math. Use the .tex export for real typesetting.
package render

import (
	"fmt"
	"html"
	"os"
	"strings"
	"text/template"

	"github.com/jung-kurt/gofpdf"

	"github.com/FaiyazEnayet/formulaforge/internal/graph"
	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

const katexCSS = "https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/katex.min.css"
const katexJS = "https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/katex.min.js"
const katexAuto = "https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/contrib/auto-render.min.js"

const pageCSS = `
:root { --ink: #1a1a2e; --accent: #4a6fa5; --paper: #ffffff; --wash: #f4f6fa; }
* { box-sizing: border-box; }
body { font-family: Georgia, 'Times New Roman', serif; color: var(--ink);
       max-width: 860px; margin: 0 auto; padding: 2rem 1.5rem; background: var(--paper); }
header.sheet-head { border-bottom: 3px solid var(--accent); margin-bottom: 1.5rem; padding-bottom: 1rem; }
header.sheet-head h1 { margin: 0 0 .25rem; font-size: 2rem; }
header.sheet-head .course { color: var(--accent); font-weight: bold; letter-spacing: .08em;
  text-transform: uppercase; font-size: .85rem; font-family: system-ui, sans-serif; }
header.sheet-head p.desc { color: #444; font-style: italic; }
.stats { display: flex; gap: .6rem; flex-wrap: wrap; margin: .8rem 0; font-family: system-ui, sans-serif; }
.stat { background: var(--wash); border-radius: 8px; padding: .35rem .7rem; font-size: .85rem; }
.stat b { color: var(--accent); }
h2.topic { font-family: system-ui, sans-serif; color: var(--accent); margin: 2rem 0 .8rem;
  font-size: 1.15rem; border-bottom: 1px solid #dde3ee; padding-bottom: .3rem; }
.card { background: var(--wash); border-radius: 10px; padding: .9rem 1.1rem; margin: 0 0 .9rem; }
.card h3 { margin: 0 0 .4rem; font-size: 1.05rem; font-family: system-ui, sans-serif; }
.card .math { font-size: 1.15rem; overflow-x: auto; padding: .3rem 0; }
.card p.desc { margin: .4rem 0; color: #333; font-size: .95rem; }
.card .meta { font-family: system-ui, sans-serif; font-size: .78rem; color: #666; margin-top: .5rem; }
.chip { display: inline-block; background: #e2e9f5; color: #2c4a73; border-radius: 20px;
  padding: .1rem .55rem; margin: .1rem .15rem .1rem 0; font-size: .75rem; }
footer { margin-top: 2.5rem; padding-top: 1rem; border-top: 1px solid #dde3ee;
  font-family: system-ui, sans-serif; font-size: .8rem; color: #888; }
@media print {
  body { max-width: 100%; padding: 0; }
  .card { break-inside: avoid; background: #fff; border: 1px solid #ddd; }
}
`

// HTML renders a full standalone HTML page for a sheet.
func HTML(sheet *models.Sheet) (string, error) {
	g := graph.New(sheet)
	st, err := g.Stats()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html lang='en'><head><meta charset='utf-8'>\n")
	b.WriteString("<meta name='viewport' content='width=device-width, initial-scale=1'>\n")
	b.WriteString("<title>" + html.EscapeString(sheet.Title) + " — formulaforge</title>\n")
	b.WriteString("<link rel='stylesheet' href='" + katexCSS + "'>\n")
	b.WriteString("<script defer src='" + katexJS + "'></script>\n")
	b.WriteString("<script defer src='" + katexAuto + "'></script>\n")
	b.WriteString(`<script>document.addEventListener('DOMContentLoaded',function(){` +
		`renderMathInElement(document.body,{delimiters:[` +
		`{left:'$$',right:'$$',display:true},{left:'\\(',right:'\\)',display:false}` +
		`]});});</script>` + "\n")
	b.WriteString("<style>" + pageCSS + "</style></head><body>\n")
	b.WriteString("<header class='sheet-head'>\n")
	b.WriteString("<div class='course'>" + html.EscapeString(sheet.Course) + "</div>\n")
	b.WriteString("<h1>" + html.EscapeString(sheet.Title) + "</h1>\n")
	b.WriteString("<p class='desc'>" + html.EscapeString(sheet.Description) + "</p>\n")
	b.WriteString("<div class='stats'>\n")
	fmt.Fprintf(&b, "<span class='stat'><b>%d</b> formulas</span>\n", st.Nodes)
	fmt.Fprintf(&b, "<span class='stat'><b>%d</b> derivations</span>\n", st.Edges)
	fmt.Fprintf(&b, "<span class='stat'><b>%d</b> study waves</span>\n", st.Waves)
	fmt.Fprintf(&b, "<span class='stat'>memorize <b>%d</b>, derive the rest</span>\n", len(st.MinMemorizeSet))
	b.WriteString("</div></header>\n")

	nameOf := map[string]string{}
	for i := range sheet.Formulas {
		nameOf[sheet.Formulas[i].ID] = sheet.Formulas[i].Name
	}
	for _, topic := range sheet.Topics() {
		b.WriteString("<h2 class='topic'>" + html.EscapeString(topic) + "</h2>\n")
		for i := range sheet.Formulas {
			f := &sheet.Formulas[i]
			if f.Topic != topic {
				continue
			}
			b.WriteString("<div class='card' id='" + html.EscapeString(f.ID) + "'>\n")
			b.WriteString("<h3>" + html.EscapeString(f.Name) + "</h3>\n")
			b.WriteString("<div class='math'>$$" + f.Latex + "$$</div>\n")
			if f.Description != "" {
				b.WriteString("<p class='desc'>" + html.EscapeString(f.Description) + "</p>\n")
			}
			b.WriteString("<div class='meta'>\n")
			if len(f.DerivesFrom) > 0 {
				b.WriteString("Builds on: ")
				for _, d := range f.DerivesFrom {
					b.WriteString("<span class='chip'>" + html.EscapeString(nameOf[d]) + "</span>")
				}
				b.WriteString("\n")
			} else {
				b.WriteString("Foundation formula — learn first\n")
			}
			for _, t := range f.Tags {
				b.WriteString("<span class='chip'>#" + html.EscapeString(t) + "</span>")
			}
			b.WriteString("</div></div>\n")
		}
	}
	b.WriteString("<footer>Built with formulaforge — write sheets in YAML + LaTeX, study the dependency graph.</footer>\n")
	b.WriteString("</body></html>\n")
	return b.String(), nil
}

// texEscape escapes the few characters that are special outside math mode.
func texEscape(s string) string {
	r := strings.NewReplacer(
		`&`, `\&`, `%`, `\%`, `$`, `\$`, `#`, `\#`, `_`, `\_`,
		`{`, `\{`, `}`, `\}`, `~`, `\textasciitilde{}`, `^`, `\textasciicircum{}`,
	)
	return r.Replace(s)
}

var texTmpl = template.Must(template.New("tex").Parse(`\documentclass[11pt]{article}
\usepackage[margin=1in]{geometry}
\usepackage{amsmath,amssymb}
\usepackage{hyperref}
\usepackage{xcolor}
\definecolor{accent}{HTML}{4a6fa5}
\title{ {{.Title}} }
\author{ {{.Course}} \textemdash\ formulaforge}
\date{}
\begin{document}
\maketitle
\textit{ {{.Description}} }

\noindent\textbf{ {{.Stats.Nodes}} formulas \textbar\ {{.Stats.Edges}} derivations \textbar\ {{.Stats.Waves}} study waves \textbar\ memorize {{.Stats.MinSize}}, derive the rest}
\vspace{1em}

{{range .Topics}}
\section*{ {{.Name}} }
{{range .Formulas}}
\subsection*{ {{.Name}} }
\begin{equation*}
 {{.Latex}}
\end{equation*}
{{.Description}}

\noindent\textcolor{accent}{\small {{.PrereqLine}} }
{{end}}
{{end}}
\end{document}
`))

type texFormula struct {
	Name        string
	Latex       string
	Description string
	PrereqLine  string
}

type texTopic struct {
	Name     string
	Formulas []texFormula
}

type texData struct {
	Course      string
	Title       string
	Description string
	Stats       struct {
		Nodes   int
		Edges   int
		Waves   int
		MinSize int
	}
	Topics []texTopic
}

// Latex renders a compilable .tex document for a sheet.
func Latex(sheet *models.Sheet) (string, error) {
	g := graph.New(sheet)
	st, err := g.Stats()
	if err != nil {
		return "", err
	}
	nameOf := map[string]string{}
	for i := range sheet.Formulas {
		nameOf[sheet.Formulas[i].ID] = sheet.Formulas[i].Name
	}
	data := texData{
		Course:      texEscape(sheet.Course),
		Title:       texEscape(sheet.Title),
		Description: texEscape(sheet.Description),
	}
	data.Stats.Nodes = st.Nodes
	data.Stats.Edges = st.Edges
	data.Stats.Waves = st.Waves
	data.Stats.MinSize = len(st.MinMemorizeSet)
	for _, topic := range sheet.Topics() {
		tt := texTopic{Name: texEscape(topic)}
		for i := range sheet.Formulas {
			f := &sheet.Formulas[i]
			if f.Topic != topic {
				continue
			}
			line := "Foundation formula."
			if len(f.DerivesFrom) > 0 {
				names := make([]string, 0, len(f.DerivesFrom))
				for _, d := range f.DerivesFrom {
					names = append(names, texEscape(nameOf[d]))
				}
				line = "Builds on: " + strings.Join(names, ", ") + "."
			}
			tt.Formulas = append(tt.Formulas, texFormula{
				Name:        texEscape(f.Name),
				Latex:       f.Latex,
				Description: texEscape(f.Description),
				PrereqLine:  line,
			})
		}
		data.Topics = append(data.Topics, tt)
	}
	var b strings.Builder
	if err := texTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// PDFInfo describes what the PDF export produced.
type PDFInfo struct {
	Pages int
	Note  string
}

// PDF writes a paginated reading copy of the sheet. Math appears as
// LaTeX source in monospace (gofpdf has no TeX engine); for typeset
// math use the LaTeX export instead.
func PDF(sheet *models.Sheet, path string) (PDFInfo, error) {
	g := graph.New(sheet)
	st, err := g.Stats()
	if err != nil {
		return PDFInfo{}, err
	}
	pdf := gofpdf.New("P", "mm", "Letter", "")
	pdf.SetTitle(sheet.Title+" — formulaforge", true)
	pdf.SetAuthor("formulaforge", true)

	// title page
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(74, 111, 165)
	pdf.CellFormat(0, 12, sheet.Course, "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "B", 26)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(0, 14, sheet.Title, "", 1, "C", false, 0, "")
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "I", 11)
	pdf.SetTextColor(85, 85, 85)
	pdf.MultiCell(0, 6, sheet.Description, "", "C", false)
	pdf.Ln(6)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(51, 51, 51)
	pdf.CellFormat(0, 8, fmt.Sprintf("%d formulas  |  %d derivations  |  %d study waves  |  memorize %d, derive the rest",
		st.Nodes, st.Edges, st.Waves, len(st.MinMemorizeSet)), "", 1, "C", false, 0, "")
	pdf.SetY(-20)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(136, 136, 136)
	pdf.CellFormat(0, 8, "Built with formulaforge", "", 0, "C", false, 0, "")

	pages := 1
	nameOf := map[string]string{}
	for i := range sheet.Formulas {
		nameOf[sheet.Formulas[i].ID] = sheet.Formulas[i].Name
	}
	for _, topic := range sheet.Topics() {
		var chunk []*models.Formula
		for i := range sheet.Formulas {
			if sheet.Formulas[i].Topic == topic {
				chunk = append(chunk, &sheet.Formulas[i])
			}
		}
		for i := 0; i < len(chunk); i += 4 {
			pdf.AddPage()
			pages++
			pdf.SetFont("Helvetica", "B", 15)
			pdf.SetTextColor(74, 111, 165)
			pdf.CellFormat(0, 10, topic, "", 1, "L", false, 0, "")
			pdf.Ln(2)
			for _, f := range chunk[i:min(i+4, len(chunk))] {
				pdf.SetFont("Helvetica", "B", 12)
				pdf.SetTextColor(0, 0, 0)
				pdf.CellFormat(0, 8, f.Name, "", 1, "L", false, 0, "")
				pdf.SetFont("Courier", "", 10)
				pdf.SetTextColor(30, 30, 30)
				pdf.MultiCell(0, 5.5, f.Latex, "", "L", false)
				pdf.Ln(1)
				if f.Description != "" {
					pdf.SetFont("Helvetica", "", 9.5)
					pdf.SetTextColor(51, 51, 51)
					pdf.MultiCell(0, 5, f.Description, "", "L", false)
				}
				if len(f.DerivesFrom) > 0 {
					names := make([]string, 0, len(f.DerivesFrom))
					for _, d := range f.DerivesFrom {
						names = append(names, nameOf[d])
					}
					pdf.SetFont("Helvetica", "I", 8.5)
					pdf.SetTextColor(74, 111, 165)
					pdf.CellFormat(0, 6, "Builds on: "+strings.Join(names, ", "), "", 1, "L", false, 0, "")
				}
				pdf.Ln(4)
			}
			pdf.SetY(-15)
			pdf.SetFont("Helvetica", "", 8)
			pdf.SetTextColor(170, 170, 170)
			pdf.CellFormat(0, 8, "formulaforge", "", 0, "C", false, 0, "")
		}
	}
	if err := pdf.OutputFileAndClose(path); err != nil {
		return PDFInfo{}, err
	}
	// count bytes written for the report
	fi, _ := os.Stat(path)
	_ = fi
	return PDFInfo{Pages: pages, Note: "math shown as LaTeX source; use the .tex export for typeset math"}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
