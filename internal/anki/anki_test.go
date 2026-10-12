package anki

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

func TestExportTSVRowCount(t *testing.T) {
	s, err := models.LoadSheetByName("ams261")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "deck.tsv")
	rows, err := ExportTSV(s, out)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 2*len(s.Formulas) {
		t.Errorf("rows = %d, want %d", rows, 2*len(s.Formulas))
	}
}

func TestExportTSVParses(t *testing.T) {
	s, err := models.LoadSheetByName("psy103")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "deck.tsv")
	if _, err := ExportTSV(s, out); err != nil {
		t.Fatal(err)
	}
	fh, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	r := csv.NewReader(fh)
	r.Comma = '\t'
	recs, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	// header + 2 rows per formula
	if len(recs) != 1+2*len(s.Formulas) {
		t.Errorf("records = %d, want %d", len(recs), 1+2*len(s.Formulas))
	}
	if recs[0][0] != "Front" || recs[0][1] != "Back" || recs[0][2] != "Topic" {
		t.Errorf("bad header: %v", recs[0])
	}
	// first data row: name front, mathjax back
	if !strings.Contains(recs[1][0], s.Formulas[0].Name) {
		t.Errorf("front missing formula name: %q", recs[1][0])
	}
	if !strings.HasPrefix(recs[1][1], `\[`) || !strings.HasSuffix(recs[1][1], `\]`) {
		t.Errorf("back not mathjax-wrapped: %q", recs[1][1])
	}
	// second row is the reverse card
	if !strings.Contains(recs[2][1], s.Formulas[0].Name) {
		t.Errorf("reverse card back missing name: %q", recs[2][1])
	}
	// prereq note appears for formulas with derivations
	found := false
	for _, rec := range recs[1:] {
		if strings.Contains(rec[0], "Builds on:") {
			found = true
		}
	}
	if !found {
		t.Error("no 'Builds on' prereq notes found")
	}
}

func TestExportTSVAllSheets(t *testing.T) {
	for _, name := range models.ListSheets() {
		s, err := models.LoadSheetByName(name)
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), name+".tsv")
		rows, err := ExportTSV(s, out)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if rows != 2*len(s.Formulas) {
			t.Errorf("%s: rows = %d", name, rows)
		}
	}
}
