// Stage 3 FO2 regression: empty-sheet and invalid-query behavior.
// Contract: empty YAML, unknown prior/target IDs and malformed
// alternatives return stable documented errors or finite values;
// JSON export never contains NaN.
package graph

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

const stage3FO2Fixture = `
course: TEST
title: FO2 fixture sheet
description: three-formula chain with one alternative path
formulas:
  - id: a
    name: Alpha
    topic: Basics
    latex: a
    description: root formula
    derives_from: []
  - id: b
    name: Beta
    topic: Basics
    latex: b
    description: builds on a
    derives_from: [a]
  - id: c
    name: Gamma
    topic: Basics
    latex: c
    description: builds on a and b, or on a alone
    derives_from: [a, b]
    alt_derives_from: [[a]]
`

func loadStage3FO2Fixture(t *testing.T) *Graph {
	t.Helper()
	s, err := models.Load([]byte(stage3FO2Fixture))
	if err != nil {
		t.Fatalf("fixture sheet failed to load: %v", err)
	}
	return New(s)
}

func TestStage3Formulaforge2(t *testing.T) {
	t.Run("EmptyYAMLRejected", func(t *testing.T) {
		for name, data := range map[string]string{
			"empty document":   "",
			"course only":      "course: TEST\ntitle: Empty\n",
			"empty formulas":   "course: TEST\ntitle: Empty\nformulas: []\n",
			"null formulas":    "course: TEST\ntitle: Empty\nformulas:\n",
		} {
			if _, err := models.Load([]byte(data)); err == nil {
				t.Errorf("%s: Load accepted an empty sheet, want a stable error", name)
			}
		}
	})

	t.Run("MalformedYAMLRejected", func(t *testing.T) {
		for name, data := range map[string]string{
			"not yaml":        "[[[",
			"scalar document": "just a string",
		} {
			if _, err := models.Load([]byte(data)); err == nil {
				t.Errorf("%s: Load accepted malformed YAML, want a stable error", name)
			}
		}
	})

	t.Run("MalformedAlternativeRejected", func(t *testing.T) {
		data := `
course: TEST
title: Bad alt
formulas:
  - id: a
    name: Alpha
    topic: T
    latex: a
    description: root
    alt_derives_from: [[]]
`
		if _, err := models.Load([]byte(data)); err == nil {
			t.Error("Load accepted an empty alternative path, want a stable error")
		}
	})

	t.Run("ZeroNodeCoverageFinite", func(t *testing.T) {
		g := New(&models.Sheet{})
		curve := g.CoverageCurve()
		if len(curve) == 0 {
			t.Fatal("zero-node graph must still return a coverage point")
		}
		for _, p := range curve {
			if math.IsNaN(p.Fraction) || math.IsInf(p.Fraction, 0) {
				t.Fatalf("nonfinite coverage fraction on empty graph: %+v", p)
			}
		}
		if curve[0].K != 0 || curve[0].Fraction != 1.0 {
			t.Errorf("empty graph coverage must start at (0, 1.0), got (%d, %v)",
				curve[0].K, curve[0].Fraction)
		}
	})

	t.Run("UnknownTargetIDError", func(t *testing.T) {
		g := loadStage3FO2Fixture(t)
		if _, err := g.MinMemorizeFor([]string{"nope"}, nil); err == nil {
			t.Error("MinMemorizeFor accepted an unknown target id, want an error")
		} else if !strings.Contains(err.Error(), "nope") {
			t.Errorf("error should name the unknown id, got: %v", err)
		}
	})

	t.Run("UnknownPriorIDError", func(t *testing.T) {
		g := loadStage3FO2Fixture(t)
		if _, err := g.MinMemorizeFor([]string{"c"}, []string{"ghost"}); err == nil {
			t.Error("MinMemorizeFor accepted an unknown prior id, want an error")
		} else if !strings.Contains(err.Error(), "ghost") {
			t.Errorf("error should name the unknown prior id, got: %v", err)
		}
	})

	t.Run("MinMemorizeSetIgnoresUnknownPrior", func(t *testing.T) {
		g := loadStage3FO2Fixture(t)
		plain := g.MinMemorizeSet([]string{"a"})
		withGhost := g.MinMemorizeSet([]string{"a", "ghost"})
		if strings.Join(plain, ",") != strings.Join(withGhost, ",") {
			t.Errorf("unknown prior id changed the result: %v vs %v", plain, withGhost)
		}
	})

	t.Run("ClosureBoolMapContract", func(t *testing.T) {
		g := loadStage3FO2Fixture(t)
		// A false entry is not knowledge.
		if got := g.Closure(map[string]bool{"a": false}); got["a"] {
			t.Error("Closure treated a false entry as known")
		}
		// True entries count; unknown ids are ignored, not fatal.
		got := g.Closure(map[string]bool{"a": true, "ghost": true})
		if !got["a"] || !got["b"] || !got["c"] {
			t.Errorf("Closure(a) should derive the whole fixture, got %v", got)
		}
	})

	t.Run("CoverageCurveJSONNeverNaN", func(t *testing.T) {
		graphs := []*Graph{loadStage3FO2Fixture(t), New(&models.Sheet{})}
		for i, g := range graphs {
			fractions := make([]float64, 0)
			for _, p := range g.CoverageCurve() {
				fractions = append(fractions, p.Fraction)
			}
			if _, err := json.Marshal(fractions); err != nil {
				t.Errorf("graph %d: coverage fractions are not JSON-safe: %v", i, err)
			}
		}
	})
}
