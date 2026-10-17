package render

import (
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

func loadSheet(t *testing.T, name string) *models.Sheet {
	t.Helper()
	s, err := models.LoadSheetByName(name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestHTMLHasKaTeXAndFormulas(t *testing.T) {
	for _, name := range models.ListSheets() {
		s := loadSheet(t, name)
		out, err := HTML(s)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"katex.min.css", "renderMathInElement", "@media print"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: HTML missing %q", name, want)
			}
		}
		for _, f := range s.Formulas {
			if !strings.Contains(out, html.EscapeString(f.Name)) {
				t.Errorf("%s: HTML missing formula name %q", name, f.Name)
			}
			if !strings.Contains(out, f.Latex) {
				t.Errorf("%s: HTML missing latex for %s", name, f.ID)
			}
		}
	}
}

func TestHTMLShowsPrereqChips(t *testing.T) {
	for _, name := range models.ListSheets() {
		s := loadSheet(t, name)
		out, err := HTML(s)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "Builds on:") && !strings.Contains(out, "Foundation formula") {
			t.Errorf("%s: HTML missing prereq chips", name)
		}
	}
}

func TestHTMLStatsPresent(t *testing.T) {
	s := loadSheet(t, "psy103")
	out, err := HTML(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<b>22</b> formulas", "<b>26</b> derivations", "memorize <b>5</b>"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML missing stat %q", want)
		}
	}
}

func TestLatexCompilableStructure(t *testing.T) {
	s := loadSheet(t, "cse213")
	doc, err := Latex(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`\documentclass`, `\begin{document}`, `\end{document}`,
		`\usepackage{amsmath`, `\maketitle`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("tex missing %q", want)
		}
	}
	// every formula's latex appears verbatim inside an equation
	for _, f := range s.Formulas {
		if !strings.Contains(doc, f.Latex) {
			t.Errorf("tex missing latex for %s", f.ID)
		}
	}
	// braces balanced (rough sanity)
	if strings.Count(doc, "{") != strings.Count(doc, "}") {
		t.Error("unbalanced braces in tex output")
	}
}

func TestLatexPrereqLines(t *testing.T) {
	s := loadSheet(t, "psy103")
	doc, err := Latex(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc, "Builds on:") {
		t.Error("tex missing prereq lines")
	}
	if !strings.Contains(doc, "Foundation formula.") {
		t.Error("tex missing foundation line")
	}
}

func TestPDFCreatesFile(t *testing.T) {
	s := loadSheet(t, "psy103")
	out := filepath.Join(t.TempDir(), "s.pdf")
	info, err := PDF(s, out)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() < 5000 {
		t.Errorf("PDF too small: %d bytes", fi.Size())
	}
	if info.Pages < 2 {
		t.Errorf("expected multiple pages, got %d", info.Pages)
	}
	// PDF magic bytes
	fh, _ := os.Open(out)
	head := make([]byte, 5)
	_, _ = fh.Read(head)
	fh.Close()
	if string(head) != "%PDF-" {
		t.Error("not a PDF file")
	}
}

func TestPDFAllSheets(t *testing.T) {
	for _, name := range models.ListSheets() {
		s := loadSheet(t, name)
		out := filepath.Join(t.TempDir(), name+".pdf")
		info, err := PDF(s, out)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if info.Pages < 2 {
			t.Errorf("%s: only %d pages", name, info.Pages)
		}
	}
}

func TestTexEscape(t *testing.T) {
	if got := texEscape("100% & #1"); got != `100\% \& \#1` {
		t.Errorf("got %q", got)
	}
}
