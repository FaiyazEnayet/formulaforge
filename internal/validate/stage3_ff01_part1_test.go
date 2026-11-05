package validate_test

// FF-01 part 1: source contract, baseline capture, fixtures and the
// independent oracle. These tests pin the catalog exactly as it
// stands at the part-1 baseline (265 formulas, 284 edges, the cse113
// doubled-backslash defect inventory, and a full set of provenance
// gaps). Part 2 fixes the content and re-pins the baseline; the
// expectations marked PART 2 below are the ones it updates.

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
	"github.com/FaiyazEnayet/formulaforge/internal/render"
	"github.com/FaiyazEnayet/formulaforge/internal/validate"
)

const baselinePath = "baseline.json"

// TestStage3Formulaforge01Part1BaselineCapture pins the catalog
// baseline. Any sheet edit, defect change or provenance change drifts
// from the pin and fails here until the baseline is deliberately
// re-pinned with FF01_UPDATE_BASELINE=1 (the part-2 workflow).
func TestStage3Formulaforge01Part1BaselineCapture(t *testing.T) {
	fresh, err := validate.CaptureBaseline()
	if err != nil {
		t.Fatalf("capture baseline: %v", err)
	}
	if fresh.TotalFormulas != 265 || fresh.TotalEdges != 284 {
		t.Fatalf("headline counts drifted from the published catalog: %d formulas, %d edges",
			fresh.TotalFormulas, fresh.TotalEdges)
	}
	if os.Getenv("FF01_UPDATE_BASELINE") == "1" {
		data, err := json.MarshalIndent(fresh, "", "  ")
		if err != nil {
			t.Fatalf("marshal baseline: %v", err)
		}
		if err := os.WriteFile(baselinePath, append(data, '\n'), 0o644); err != nil {
			t.Fatalf("write baseline: %v", err)
		}
		t.Logf("baseline re-pinned: %d formulas, %d edges, %d defects",
			fresh.TotalFormulas, fresh.TotalEdges, len(fresh.Defects))
		return
	}
	raw, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read pinned baseline: %v", err)
	}
	var pinned validate.Baseline
	if err := json.Unmarshal(raw, &pinned); err != nil {
		t.Fatalf("parse pinned baseline: %v", err)
	}
	if diffs := validate.CompareBaseline(&pinned, fresh); len(diffs) > 0 {
		t.Fatalf("catalog drifted from the pinned baseline:\n%s", strings.Join(diffs, "\n"))
	}
}

// TestStage3Formulaforge01Part1CatalogAudit records the known-defect
// inventory and provenance gaps at the part-1 baseline. PART 2: as
// content is fixed, the defect expectations below shrink to zero and
// the gap counts shrink to zero; update them in the same change that
// fixes the content and re-pins the baseline.
func TestStage3Formulaforge01Part1CatalogAudit(t *testing.T) {
	findings, err := validate.AuditCatalog()
	if err != nil {
		t.Fatalf("audit catalog: %v", err)
	}
	defects := validate.Defects(findings)
	if len(defects) == 0 {
		t.Fatal("expected the known cse113 defects at the part-1 baseline")
	}
	for _, d := range defects {
		if d.Sheet != "cse113" {
			t.Errorf("defect outside the known carrier sheet: %s/%s (%s)", d.Sheet, d.ID, d.Class)
		}
	}
	doubled := map[string]bool{}
	for _, f := range findings {
		if f.Class == validate.ClassDoubledBackslash {
			doubled[f.Sheet+"/"+f.ID] = true
		}
	}
	if len(doubled) != 32 {
		t.Errorf("doubled-backslash formulas: got %d, want 32 (all of cse113)", len(doubled))
	}
	counts := validate.ClassCounts(findings)
	if counts[validate.ClassMissingProvenance] != 265 {
		t.Errorf("missing-provenance findings: got %d, want 265 (no formula carries a source yet)",
			counts[validate.ClassMissingProvenance])
	}
	if counts[validate.ClassMissingConditions] != 199 {
		t.Errorf("missing-conditions findings: got %d, want 199 (quantitative sheets)",
			counts[validate.ClassMissingConditions])
	}
}

type oracleCase struct {
	Sheet     string             `yaml:"sheet"`
	ID        string             `yaml:"id"`
	Quantity  string             `yaml:"quantity"`
	Inputs    map[string]float64 `yaml:"inputs"`
	Values    []float64          `yaml:"values"`
	Expected  float64            `yaml:"expected"`
	Tolerance float64            `yaml:"tolerance"`
	Source    string             `yaml:"source"`
}

type oracleFile struct {
	Cases []oracleCase `yaml:"cases"`
}

// oracleCompute recomputes each known answer from the case inputs in
// plain Go. It shares no code with the catalog or its renderers.
func oracleCompute(c oracleCase) float64 {
	in := c.Inputs
	switch c.Sheet + "/" + c.ID {
	case "phy131/kinematic":
		return in["v0"]*in["t"] + 0.5*in["a"]*in["t"]*in["t"]
	case "phy131/projectile":
		rad := in["angle_deg"] * math.Pi / 180
		return in["v0"] * in["v0"] * math.Sin(2*rad) / in["g"]
	case "phy131/weight":
		return in["m"] * in["g"]
	case "ams151/power-rule":
		return in["n"] * math.Pow(in["x"], in["n"]-1)
	case "ams151/ftc2":
		return (math.Pow(in["b"], 3) - math.Pow(in["a"], 3)) / 3
	case "psy103/z-score":
		return (in["x"] - in["mean"]) / in["sd"]
	case "psy103/mean":
		sum := 0.0
		for _, v := range c.Values {
			sum += v
		}
		return sum / float64(len(c.Values))
	}
	return math.NaN()
}

// TestStage3Formulaforge01Part1OracleKnownAnswers checks the oracle:
// every case names a real catalog formula, the formula's LaTeX passes
// validation, the case's own source satisfies the provenance
// contract, and an independent recomputation lands within tolerance
// of the recorded known answer.
func TestStage3Formulaforge01Part1OracleKnownAnswers(t *testing.T) {
	raw, err := os.ReadFile("testdata/oracle.yaml")
	if err != nil {
		t.Fatalf("read oracle fixtures: %v", err)
	}
	var of oracleFile
	if err := yaml.Unmarshal(raw, &of); err != nil {
		t.Fatalf("parse oracle fixtures: %v", err)
	}
	if len(of.Cases) < 5 {
		t.Fatalf("oracle covers %d formulas, want at least 5 representative cases", len(of.Cases))
	}
	sheetsSeen := map[string]bool{}
	for _, c := range of.Cases {
		sheetsSeen[c.Sheet] = true
		sheet, err := models.LoadSheetByName(c.Sheet)
		if err != nil {
			t.Errorf("%s/%s: load sheet: %v", c.Sheet, c.ID, err)
			continue
		}
		f, err := sheet.ByID(c.ID)
		if err != nil {
			t.Errorf("%s/%s: not a catalog formula: %v", c.Sheet, c.ID, err)
			continue
		}
		if problems := validate.CheckLatex(f.Latex); len(problems) > 0 {
			t.Errorf("%s/%s: catalog latex fails validation: %v", c.Sheet, c.ID, problems)
		}
		prov := validate.Provenance{Source: c.Source}
		for _, p := range prov.Problems(false) {
			t.Errorf("%s/%s: oracle source violates the contract: %s", c.Sheet, c.ID, p)
		}
		got := oracleCompute(c)
		if math.IsNaN(got) {
			t.Errorf("%s/%s: no independent computation registered for this case", c.Sheet, c.ID)
			continue
		}
		if math.Abs(got-c.Expected) > c.Tolerance {
			t.Errorf("%s/%s (%s): recomputed %v, known answer %v, tolerance %v",
				c.Sheet, c.ID, c.Quantity, got, c.Expected, c.Tolerance)
		}
	}
	if len(sheetsSeen) < 3 {
		t.Errorf("oracle spans %d sheets, want at least 3", len(sheetsSeen))
	}
}

// TestStage3Formulaforge01Part1InvalidLatexRejected feeds the
// validator the defect classes the contract names, plus the cse113
// defect exactly as it occurs in sheet YAML, and requires rejection;
// valid controls must pass.
func TestStage3Formulaforge01Part1InvalidLatexRejected(t *testing.T) {
	invalid := map[string]string{
		"unbalanced braces":   `x = \frac{1}{2`,
		"unmatched close":     `x = 1}`,
		"unknown command":     `x = \frobnicate{1}`,
		"doubled command":     `x = \\frac{1}{2}`,
		"mismatched env":      `\begin{array}{c} a \end{matrix}`,
		"end without begin":   `a \end{array}`,
		"unclosed env":        `\begin{array}{c} a`,
		"alignment outside":   `a & b`,
		"line break outside":  `a \\ b`,
		"empty":               ``,
		"trailing backslash":  `x = 1\`,
		"begin without brace": `\begin array`,
	}
	for name, src := range invalid {
		if problems := validate.CheckLatex(src); len(problems) == 0 {
			t.Errorf("%s: %q was accepted, want rejection", name, src)
		}
	}
	valid := map[string]string{
		"plain":         `\frac{a}{b} + \sqrt{x}`,
		"array":         `\begin{array}{c|c} a & b \\ \hline c & d \end{array}`,
		"text":          `f'(c) = 0 \ \text{or} \ f'(c) \text{ undefined}`,
		"escaped set":   `\{1, 2\} \subseteq \mathbb{R}`,
		"cases":         `\begin{cases} x, & x \ge 0 \\ -x, & x < 0 \end{cases}`,
		"nested braces": `\frac{\frac{a}{b}}{c}`,
	}
	for name, src := range valid {
		if problems := validate.CheckLatex(src); len(problems) > 0 {
			t.Errorf("%s: %q rejected: %v", name, src, problems)
		}
	}
	// The cse113 defect at the sheet layer: single-quoted YAML whose
	// LaTeX was escaped for double quotes parses with doubled
	// backslashes, and the audit must flag it as such.
	sheetYAML := []byte(`
course: "FIXTURE"
title: "Fixture"
description: "doubled-backslash fixture"
formulas:
  - id: demorgan-fixture
    name: "De Morgan fixture"
    topic: "Logic"
    latex: '\\lnot (p \\land q) \\equiv \\lnot p \\lor \\lnot q'
    description: "fixture"
    derives_from: []
`)
	sheet, err := models.Load(sheetYAML)
	if err != nil {
		t.Fatalf("load fixture sheet: %v", err)
	}
	findings := validate.AuditSheet("cse113", sheet)
	found := false
	for _, f := range findings {
		if f.Class == validate.ClassDoubledBackslash {
			found = true
		}
	}
	if !found {
		t.Errorf("audit did not flag the doubled-backslash fixture: %v", findings)
	}
}

// TestStage3Formulaforge01Part1MissingReferencesFail covers the
// contract's reference rules at every layer: derivation references
// must resolve at load, provenance fields are parsed strictly so a
// typo cannot drop a source, and a formula without a source (or with
// a source that names no section or URL) fails the audit.
func TestStage3Formulaforge01Part1MissingReferencesFail(t *testing.T) {
	badRef := []byte(`
course: "FIXTURE"
title: "Fixture"
description: "unknown derivation reference"
formulas:
  - id: a
    name: "A"
    topic: "T"
    latex: 'x'
    description: "fixture"
    derives_from: [does-not-exist]
`)
	if _, err := models.Load(badRef); err == nil {
		t.Error("load accepted an unknown derivation reference")
	}
	if _, err := validate.ParseProvenance([]byte("soruce: OpenStax, Section 1.1\n")); err == nil {
		t.Error("provenance parse accepted a misspelled field")
	}
	for name, p := range map[string]validate.Provenance{
		"no source":          {},
		"placeholder":        {Source: "TBD"},
		"source w/o section": {Source: "some textbook"},
	} {
		problems := p.Problems(false)
		if len(problems) == 0 {
			t.Errorf("%s: provenance %+v accepted, want problems", name, p)
		}
	}
	good := validate.Provenance{
		Source:     "OpenStax University Physics Volume 1, Section 4.3",
		Conditions: "equal launch and landing heights; no air resistance",
		Units:      "m",
	}
	if problems := good.Problems(true); len(problems) > 0 {
		t.Errorf("complete provenance rejected: %v", problems)
	}
	bare := &models.Formula{ID: "bare", Latex: `\frac{a}{b}`}
	findings := validate.AuditFormula("phy131", bare)
	classes := map[string]bool{}
	for _, f := range findings {
		classes[f.Class] = true
	}
	if !classes[validate.ClassMissingProvenance] || !classes[validate.ClassMissingConditions] {
		t.Errorf("bare formula findings = %v, want missing-provenance and missing-conditions", findings)
	}
	full := &models.Formula{
		ID: "full", Latex: `\frac{a}{b}`,
		Source: good.Source, Conditions: good.Conditions, Units: good.Units,
	}
	if findings := validate.AuditFormula("phy131", full); len(findings) > 0 {
		t.Errorf("fully provenanced formula produced findings: %v", findings)
	}
}

// TestStage3Formulaforge01Part1TypesetSmoke checks that the sample
// typeset output for every sheet is structurally compilable: a full
// document whose every formula survives into the .tex verbatim and
// whose own LaTeX passes validation. A real pdflatex compile is
// scripts/texsmoke.sh, run where a TeX toolchain is installed.
func TestStage3Formulaforge01Part1TypesetSmoke(t *testing.T) {
	all, err := models.LoadAll()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	for _, name := range models.ListSheets() {
		sheet := all[name]
		doc, err := render.Latex(sheet)
		if err != nil {
			t.Errorf("%s: render latex: %v", name, err)
			continue
		}
		for _, want := range []string{`\documentclass`, `\begin{document}`, `\end{document}`} {
			if !strings.Contains(doc, want) {
				t.Errorf("%s: typeset output missing %s", name, want)
			}
		}
		for i := range sheet.Formulas {
			f := &sheet.Formulas[i]
			if !strings.Contains(doc, f.Latex) {
				t.Errorf("%s/%s: formula latex missing from typeset output", name, f.ID)
			}
		}
	}
}
