// Package anki exports a formula sheet as a tab-separated file that
// Anki imports natively (File > Import). Three columns: Front, Back,
// Topic. Math uses \( \) / \[ \] delimiters, which Anki renders with
// its built-in MathJax — no LaTeX installation needed.
//
// Two rows per formula:
//  1. "State the formula": name (+ description) on the front, formula on the back.
//  2. "Name this formula": the formula on the front, the name on the back.
package anki

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// mathJax wraps display math in \[ \] for Anki's MathJax.
func mathJax(latex string) string {
	return "\\[" + latex + "\\]"
}

// ExportTSV writes an Anki-importable TSV. Returns the number of rows.
func ExportTSV(sheet *models.Sheet, path string) (int, error) {
	fh, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer fh.Close()
	w := csv.NewWriter(fh)
	w.Comma = '\t'
	// Anki reads the first row as field names when the header matches.
	if err := w.Write([]string{"Front", "Back", "Topic"}); err != nil {
		return 0, err
	}
	nameOf := map[string]string{}
	for i := range sheet.Formulas {
		nameOf[sheet.Formulas[i].ID] = sheet.Formulas[i].Name
	}
	rows := 0
	for i := range sheet.Formulas {
		f := &sheet.Formulas[i]
		prereq := ""
		if len(f.DerivesFrom) > 0 {
			names := make([]string, 0, len(f.DerivesFrom))
			for _, d := range f.DerivesFrom {
				names = append(names, nameOf[d])
			}
			prereq = " (Builds on: " + join(names, ", ") + ")"
		}
		front := f.Name + prereq
		if f.Description != "" {
			front += " — " + f.Description
		}
		back := mathJax(f.Latex)
		if err := w.Write([]string{front, back, f.Topic}); err != nil {
			return 0, err
		}
		if err := w.Write([]string{back, f.Name, f.Topic}); err != nil {
			return 0, err
		}
		rows += 2
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return 0, fmt.Errorf("write tsv: %w", err)
	}
	return rows, nil
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
