package validate_test

// FF-01 part 2: the content fix. Part 1 pinned the catalog as found
// (the cse113 doubled-backslash defect inventory and a full set of
// provenance gaps); these tests pin the end state the contract
// requires: no defects, no provenance gaps, the canonical projectile
// conditions stated, and the enforcement gate biting on new content
// that lacks provenance.

import (
	"strings"
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
	"github.com/FaiyazEnayet/formulaforge/internal/validate"
)

// TestStage3Formulaforge01Part2CatalogClean requires the whole
// catalog audit to come back empty: no invalid LaTeX, no doubled
// backslashes, no missing or invalid provenance, no missing
// conditions on the quantitative sheets.
func TestStage3Formulaforge01Part2CatalogClean(t *testing.T) {
	findings, err := validate.AuditCatalog()
	if err != nil {
		t.Fatalf("audit catalog: %v", err)
	}
	if len(findings) > 0 {
		counts := validate.ClassCounts(findings)
		var sample []string
		for i, f := range findings {
			if i >= 10 {
				break
			}
			sample = append(sample, f.Sheet+"/"+f.ID+" ("+f.Class+": "+f.Detail+")")
		}
		t.Fatalf("catalog audit not clean: %d findings %v, first: %s",
			len(findings), counts, strings.Join(sample, "; "))
	}
}

// TestStage3Formulaforge01Part2Cse113LatexFixed pins the cse113 fix
// at the parsed-value level: the sheet was written as single-quoted
// YAML whose LaTeX was escaped for double quotes, so every formula
// parsed with doubled backslashes. After the fix the parsed values
// carry single backslashes, the truth table keeps its line breaks,
// and every formula passes the LaTeX checker.
func TestStage3Formulaforge01Part2Cse113LatexFixed(t *testing.T) {
	sheet, err := models.LoadSheetByName("cse113")
	if err != nil {
		t.Fatalf("load cse113: %v", err)
	}
	if len(sheet.Formulas) != 32 {
		t.Fatalf("cse113 has %d formulas, want 32", len(sheet.Formulas))
	}
	for i := range sheet.Formulas {
		f := &sheet.Formulas[i]
		if problems := validate.CheckLatex(f.Latex); len(problems) > 0 {
			t.Errorf("cse113/%s: latex fails validation: %v", f.ID, problems)
		}
		if strings.Contains(f.Latex, `\\lnot`) || strings.Contains(f.Latex, `\\land`) ||
			strings.Contains(f.Latex, `\\frac`) || strings.Contains(f.Latex, `\\begin`) {
			t.Errorf("cse113/%s: parsed latex still carries doubled backslashes: %q", f.ID, f.Latex)
		}
	}
	demorgan, err := sheet.ByID("demorgan")
	if err != nil {
		t.Fatalf("demorgan: %v", err)
	}
	if !strings.Contains(demorgan.Latex, `\lnot (p \land q)`) {
		t.Errorf("demorgan latex = %q, want single-backslash De Morgan statement", demorgan.Latex)
	}
	truth, err := sheet.ByID("truth-table")
	if err != nil {
		t.Fatalf("truth-table: %v", err)
	}
	if !strings.Contains(truth.Latex, `\begin{array}`) || !strings.Contains(truth.Latex, `\hline`) {
		t.Errorf("truth-table latex = %q, want an array with a \\hline rule", truth.Latex)
	}
	if !strings.Contains(truth.Latex, `\\ `) && !strings.Contains(truth.Latex, `\\ \hline`) &&
		!strings.Contains(truth.Latex, `T \\`) {
		t.Errorf("truth-table latex = %q, want row line breaks preserved", truth.Latex)
	}
}

// TestStage3Formulaforge01Part2ProjectileConditions pins the
// contract's canonical case: the projectile entry claims range is
// maximized at 45 degrees, so its conditions must state the
// assumptions that claim needs (equal launch and landing heights,
// no air resistance).
func TestStage3Formulaforge01Part2ProjectileConditions(t *testing.T) {
	sheet, err := models.LoadSheetByName("phy131")
	if err != nil {
		t.Fatalf("load phy131: %v", err)
	}
	f, err := sheet.ByID("projectile")
	if err != nil {
		t.Fatalf("projectile: %v", err)
	}
	cond := strings.ToLower(f.Conditions)
	for _, want := range []string{"equal", "height", "air resistance", "45"} {
		if !strings.Contains(cond, want) {
			t.Errorf("projectile conditions %q missing %q", f.Conditions, want)
		}
	}
	if f.Source == "" {
		t.Error("projectile carries no source")
	}
}

// TestStage3Formulaforge01Part2RepresentativeSources spot-checks one
// entry per sheet (two for phy131, whose sheet spans both University
// Physics volumes) and requires the source to name the reference the
// entry actually comes from, so a single blanket citation pasted
// across the catalog cannot satisfy the gate.
func TestStage3Formulaforge01Part2RepresentativeSources(t *testing.T) {
	cases := []struct {
		sheet, id, want string
	}{
		{"ams151", "ftc2", "Calculus Volume 1"},
		{"ams161", "taylor", "Calculus Volume 2"},
		{"ams210", "gram-schmidt", "Linear Algebra and Its Applications"},
		{"ams261", "green", "Calculus Volume 3"},
		{"cse113", "quotient-remainder", "Discrete Mathematics with Applications"},
		{"cse213", "bayes", "Discrete Mathematics and Its Applications"},
		{"phy131", "projectile", "University Physics Volume 1"},
		{"phy131", "carnot", "University Physics Volume 2"},
		{"psy103", "z-score", "Introductory Statistics"},
	}
	for _, c := range cases {
		sheet, err := models.LoadSheetByName(c.sheet)
		if err != nil {
			t.Errorf("%s: load: %v", c.sheet, err)
			continue
		}
		f, err := sheet.ByID(c.id)
		if err != nil {
			t.Errorf("%s/%s: %v", c.sheet, c.id, err)
			continue
		}
		if !strings.Contains(f.Source, c.want) {
			t.Errorf("%s/%s: source %q does not cite %q", c.sheet, c.id, f.Source, c.want)
		}
	}
}

// TestStage3Formulaforge01Part2BaselineEndState requires a fresh
// baseline capture to show the contract's end state: headline counts
// unchanged, defect list empty, and no findings of any class on any
// sheet.
func TestStage3Formulaforge01Part2BaselineEndState(t *testing.T) {
	b, err := validate.CaptureBaseline()
	if err != nil {
		t.Fatalf("capture baseline: %v", err)
	}
	if b.TotalFormulas != 265 || b.TotalEdges != 284 {
		t.Errorf("headline counts: %d formulas, %d edges, want 265/284", b.TotalFormulas, b.TotalEdges)
	}
	if len(b.Defects) != 0 {
		t.Errorf("fresh capture still lists %d defects, want 0", len(b.Defects))
	}
	for _, s := range b.Sheets {
		for class, n := range s.FindingsByClass {
			if n != 0 {
				t.Errorf("sheet %s still has %d %s findings", s.Name, n, class)
			}
		}
	}
}

// TestStage3Formulaforge01Part2MissingProvenanceFailsGate proves the
// enforcement side of part 2: a newly written formula without a
// source, or with a source that names no section or URL, fails the
// audit on any sheet, and a quantitative-sheet formula without
// conditions fails as well. This is what makes missing references
// fail the curated-content suite after the catalog itself is clean.
func TestStage3Formulaforge01Part2MissingProvenanceFailsGate(t *testing.T) {
	sheetYAML := []byte(`
course: "FIXTURE"
title: "Fixture"
description: "provenance gate fixture"
formulas:
  - id: bare
    name: "Bare"
    topic: "T"
    latex: 'x = 1'
    description: "fixture"
    derives_from: []
  - id: sectionless
    name: "Sectionless"
    topic: "T"
    latex: 'y = 2'
    description: "fixture"
    source: "some calculus textbook"
    derives_from: []
  - id: complete
    name: "Complete"
    topic: "T"
    latex: 'z = 3'
    description: "fixture"
    source: "OpenStax Calculus Volume 1, Section 3.3"
    conditions: "holds for all real x"
    derives_from: []
`)
	sheet, err := models.Load(sheetYAML)
	if err != nil {
		t.Fatalf("load fixture sheet: %v", err)
	}
	classes := map[string]map[string]bool{}
	for _, f := range validate.AuditSheet("ams151", sheet) {
		if classes[f.ID] == nil {
			classes[f.ID] = map[string]bool{}
		}
		classes[f.ID][f.Class] = true
	}
	if !classes["bare"][validate.ClassMissingProvenance] {
		t.Errorf("bare formula not flagged missing-provenance: %v", classes["bare"])
	}
	if !classes["bare"][validate.ClassMissingConditions] {
		t.Errorf("bare formula on a quantitative sheet not flagged missing-conditions: %v", classes["bare"])
	}
	if !classes["sectionless"][validate.ClassMissingProvenance] {
		t.Errorf("sectionless source not flagged: %v", classes["sectionless"])
	}
	if len(classes["complete"]) != 0 {
		t.Errorf("complete formula flagged: %v", classes["complete"])
	}
	// On a logic sheet the same bare formula needs a source but is
	// not required to state conditions.
	logicClasses := map[string]bool{}
	for _, f := range validate.AuditSheet("cse113", sheet) {
		if f.ID == "bare" {
			logicClasses[f.Class] = true
		}
	}
	if !logicClasses[validate.ClassMissingProvenance] || logicClasses[validate.ClassMissingConditions] {
		t.Errorf("logic-sheet bare formula findings = %v, want missing-provenance only", logicClasses)
	}
}
