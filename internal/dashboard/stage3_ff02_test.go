// Stage 3 FF-02 dashboard check: the node payload carries the typed
// relationships, so the explorer can show conceptual links and
// unreviewed derivations without treating either as a derivation
// edge. Data only; the dashboard UI itself is a separate owner step.
package dashboard

import (
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

const stage3FF02DashboardFixture = `
course: TEST
title: FF-02 fixture sheet
description: typed relationships fixture
formulas:
  - id: a
    name: Alpha
    topic: Basics
    latex: a
    derives_from: []
  - id: b
    name: Beta
    topic: Basics
    latex: b
    derives_from: []
  - id: c
    name: Gamma
    topic: Basics
    latex: c
    derives_from: []
  - id: t1
    name: Tee One
    topic: Basics
    latex: t1
    derives_from: [a, b]
    conceptual_prerequisites: [c]
  - id: t3
    name: Tee Three
    topic: Basics
    latex: t3
    derives_from: [a]
    derivation_rules:
      - {statement: "t3 follows from a", status: unreviewed}
  - id: t5
    name: Tee Five
    topic: Basics
    latex: t5
    conceptual_prerequisites: [t1]
`

func TestStage3Formulaforge02DashboardPayload(t *testing.T) {
	s, err := models.Load([]byte(stage3FF02DashboardFixture))
	if err != nil {
		t.Fatal(err)
	}
	p, err := payloadFor(s)
	if err != nil {
		t.Fatal(err)
	}
	n1 := p.Nodes["t1"]
	if len(n1.ConceptualPrereqs) != 1 || n1.ConceptualPrereqs[0] != "c" {
		t.Errorf("t1 conceptual_prereqs = %v, want [c]", n1.ConceptualPrereqs)
	}
	if len(n1.Prereqs) != 2 || n1.Prereqs[0] != "a" || n1.Prereqs[1] != "b" {
		t.Errorf("t1 prereqs = %v, want [a b]: the conceptual link must not appear as a derivation prerequisite", n1.Prereqs)
	}
	if n1.Unreviewed {
		t.Errorf("t1 is marked unreviewed, want false")
	}
	if n3 := p.Nodes["t3"]; !n3.Unreviewed {
		t.Errorf("t3 is not marked unreviewed, want true")
	}
	n5 := p.Nodes["t5"]
	if len(n5.ConceptualPrereqs) != 1 || n5.ConceptualPrereqs[0] != "t1" {
		t.Errorf("t5 conceptual_prereqs = %v, want [t1]", n5.ConceptualPrereqs)
	}
	if len(n5.Prereqs) != 0 {
		t.Errorf("t5 prereqs = %v, want none: a conceptual-only formula has no derivation prerequisites", n5.Prereqs)
	}
	for _, e := range p.Edges {
		if e.From == "c" && e.To == "t1" {
			t.Errorf("dashboard edges contain the conceptual edge c->t1 as a derivation edge")
		}
		if e.To == "t3" {
			t.Errorf("dashboard edges contain an edge into t3, whose only derivation is unreviewed")
		}
	}
	if p.Stats.Edges != 2 {
		t.Errorf("payload stats edges = %d, want 2 (t1 a,b; nothing into t3 or t5)", p.Stats.Edges)
	}
}
