// Stage 3 FO3 regression: derivation witnesses for dependency closure.
// Contract: Witnesses returns one satisfied prerequisite set per derived
// formula (a derivation witness DAG), and the separate CheckWitnesses
// checker accepts only whole conjunctions, rejects a missing AND
// prerequisite, and requires a valid acyclic witness for every derived
// formula.
package graph

import (
	"reflect"
	"strings"
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

const stage3FO3Fixture = `
course: TEST
title: FO3 fixture sheet
description: chain with an alternative path and a cyclic alternative pair
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
    description: builds on a and b, or on b alone
    derives_from: [a, b]
    alt_derives_from: [[b]]
  - id: d
    name: Delta
    topic: Basics
    latex: d
    description: builds on b and c
    derives_from: [b, c]
  - id: x
    name: Xi
    topic: Basics
    latex: x
    description: builds on a, or on y
    derives_from: [a]
    alt_derives_from: [[y]]
  - id: y
    name: Upsilon
    topic: Basics
    latex: y
    description: builds on x
    derives_from: [x]
`

func loadStage3FO3Fixture(t *testing.T) *Graph {
	t.Helper()
	s, err := models.Load([]byte(stage3FO3Fixture))
	if err != nil {
		t.Fatalf("fixture sheet failed to load: %v", err)
	}
	return New(s)
}

func TestStage3Formulaforge3(t *testing.T) {
	t.Run("WitnessForEveryDerivedFormula", func(t *testing.T) {
		g := loadStage3FO3Fixture(t)
		known := map[string]bool{"a": true}
		closed := g.Closure(known)
		if len(closed) != 6 {
			t.Fatalf("closure from {a} has %d formulas, want 6", len(closed))
		}
		got := g.Witnesses(known)
		want := map[string][]string{
			"b": {"a"},
			"c": {"b"},
			"d": {"b", "c"},
			"x": {"a"},
			"y": {"x"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Witnesses = %v, want %v", got, want)
		}
		if err := g.CheckWitnesses(known, got); err != nil {
			t.Errorf("CheckWitnesses rejected the witness DAG: %v", err)
		}
	})

	t.Run("WholeConjunctionAlternativesAccepted", func(t *testing.T) {
		g := loadStage3FO3Fixture(t)
		known := map[string]bool{"a": true}
		// c may be witnessed by its primary path [a, b] as well as by
		// its alternative path [b]; both are whole conjunctions.
		witnesses := map[string][]string{
			"b": {"a"},
			"c": {"a", "b"},
			"d": {"b", "c"},
			"x": {"a"},
			"y": {"x"},
		}
		if err := g.CheckWitnesses(known, witnesses); err != nil {
			t.Errorf("CheckWitnesses rejected the primary-path witness for c: %v", err)
		}
	})

	t.Run("MissingANDPrerequisiteRejected", func(t *testing.T) {
		g := loadStage3FO3Fixture(t)
		known := map[string]bool{"a": true}
		for name, tamper := range map[string]func(map[string][]string){
			"drops one member of the AND pair for d": func(w map[string][]string) {
				w["d"] = []string{"b"}
			},
			"claims a partial path for c": func(w map[string][]string) {
				w["c"] = []string{"a"}
			},
			"adds an extra prerequisite to b": func(w map[string][]string) {
				w["b"] = []string{"a", "c"}
			},
		} {
			witnesses := map[string][]string{
				"b": {"a"},
				"c": {"b"},
				"d": {"b", "c"},
				"x": {"a"},
				"y": {"x"},
			}
			tamper(witnesses)
			if err := g.CheckWitnesses(known, witnesses); err == nil {
				t.Errorf("%s: CheckWitnesses accepted, want rejection", name)
			}
		}
	})

	t.Run("MissingWitnessRejected", func(t *testing.T) {
		g := loadStage3FO3Fixture(t)
		known := map[string]bool{"a": true}
		witnesses := g.Witnesses(known)
		delete(witnesses, "d")
		if err := g.CheckWitnesses(known, witnesses); err == nil {
			t.Errorf("CheckWitnesses accepted a witness DAG with no witness for d, want rejection")
		}
	})

	t.Run("WitnessForKnownOrUnknownRejected", func(t *testing.T) {
		g := loadStage3FO3Fixture(t)
		known := map[string]bool{"a": true}
		base := map[string][]string{
			"b": {"a"},
			"c": {"b"},
			"d": {"b", "c"},
			"x": {"a"},
			"y": {"x"},
		}
		for name, extra := range map[string]map[string][]string{
			"witness for the already known root a": {"a": {}},
			"witness for an unknown id":            {"zz": {"a"}},
		} {
			witnesses := map[string][]string{}
			for k, v := range base {
				witnesses[k] = v
			}
			for k, v := range extra {
				witnesses[k] = v
			}
			if err := g.CheckWitnesses(known, witnesses); err == nil {
				t.Errorf("%s: CheckWitnesses accepted, want rejection", name)
			}
		}
	})

	t.Run("CyclicWitnessRejected", func(t *testing.T) {
		g := loadStage3FO3Fixture(t)
		known := map[string]bool{"a": true}
		// x and y each cite the other through a real whole path, so
		// the entries pass the path check but form a cycle: neither
		// derivation is grounded in the known set.
		witnesses := map[string][]string{
			"b": {"a"},
			"c": {"b"},
			"d": {"b", "c"},
			"x": {"y"},
			"y": {"x"},
		}
		err := g.CheckWitnesses(known, witnesses)
		if err == nil {
			t.Fatalf("CheckWitnesses accepted a cyclic witness DAG, want rejection")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("rejection error %q does not mention a cycle", err.Error())
		}
	})

	t.Run("UnknownPrerequisiteRejected", func(t *testing.T) {
		g := loadStage3FO3Fixture(t)
		known := map[string]bool{"a": true}
		// c is derivable through [b], but this witness claims the
		// primary path [a, b] with b replaced by the unknown id z.
		witnesses := map[string][]string{
			"b": {"a"},
			"c": {"a", "z"},
			"d": {"b", "c"},
			"x": {"a"},
			"y": {"x"},
		}
		if err := g.CheckWitnesses(known, witnesses); err == nil {
			t.Errorf("CheckWitnesses accepted a witness citing an unknown prerequisite, want rejection")
		}
	})
}
