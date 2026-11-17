// Stage 3 FF-02 regression: typed relationships.
//
// Contract: conceptual prerequisites are pedagogical links that never
// grant derivability; only certified (reviewed) derivation rules enter
// Closure and optimization; an unreviewed rule suppresses mathematical
// proof claims in render and Anki output; cycles and missing
// references are rejected; the migration report enumerates every
// changed root, edge, and coverage-curve point instead of hiding
// changes. Reviewed-compatible fixtures keep their existing semantics.
package graph_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/anki"
	"github.com/FaiyazEnayet/formulaforge/internal/graph"
	"github.com/FaiyazEnayet/formulaforge/internal/models"
	"github.com/FaiyazEnayet/formulaforge/internal/render"
)

const stage3FF02Fixture = `
course: TEST
title: FF-02 fixture sheet
description: typed relationships fixture
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
    description: root formula
    derives_from: []
  - id: c
    name: Gamma
    topic: Basics
    latex: c
    description: root formula
    derives_from: []
  - id: d
    name: Delta
    topic: Basics
    latex: d
    description: root formula
    derives_from: []
  - id: t1
    name: Tee One
    topic: Basics
    latex: t1
    description: AND derivation plus a conceptual link
    derives_from: [a, b]
    conceptual_prerequisites: [c]
  - id: t2
    name: Tee Two
    topic: Basics
    latex: t2
    description: OR derivation, both paths certified
    derives_from: [a]
    alt_derives_from: [[b]]
  - id: t3
    name: Tee Three
    topic: Basics
    latex: t3
    description: only derivation is unreviewed
    derives_from: [a]
    derivation_rules:
      - statement: "t3 follows from a"
        reference: "Test notes, Section 1"
        assumptions: "a is known"
        status: unreviewed
  - id: t4
    name: Tee Four
    topic: Basics
    latex: t4
    description: unreviewed primary path, reviewed alternative
    derives_from: [a]
    alt_derives_from: [[b]]
    derivation_rules:
      - {statement: "t4 follows from a", status: unreviewed}
      - {statement: "t4 follows from b", status: reviewed}
  - id: t5
    name: Tee Five
    topic: Basics
    latex: t5
    description: conceptual link only
    conceptual_prerequisites: [t1]
`

func ff02LoadFixture(t *testing.T) (*models.Sheet, *graph.Graph) {
	t.Helper()
	s, err := models.Load([]byte(stage3FF02Fixture))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return s, graph.New(s)
}

func ff02HasChange(t *testing.T, rep *graph.MigrationReport, kind, detail string) {
	t.Helper()
	for _, ch := range rep.Changes {
		if ch.Kind == kind && ch.Detail == detail {
			return
		}
	}
	t.Errorf("migration report for %s lacks change %s %q; got %v", rep.Sheet, kind, detail, rep.Changes)
}

func TestStage3Formulaforge02(t *testing.T) {
	t.Run("ConceptualEdgeNeverDerives", func(t *testing.T) {
		s, g := ff02LoadFixture(t)
		f, err := s.ByID("t1")
		if err != nil {
			t.Fatal(err)
		}
		if got := f.ConceptualPrerequisites; len(got) != 1 || got[0] != "c" {
			t.Fatalf("t1 conceptual prerequisites = %v, want [c]", got)
		}
		if got := g.ConceptualPrereqs("t1"); len(got) != 1 || got[0] != "c" {
			t.Fatalf("graph conceptual prereqs of t1 = %v, want [c]", got)
		}
		// Knowing only the conceptual prerequisite derives nothing.
		if closed := g.Closure(map[string]bool{"c": true}); len(closed) != 1 || !closed["c"] {
			t.Errorf("Closure({c}) = %v, want exactly {c}: a conceptual edge alone must never trigger closure", closed)
		}
		// The AND derivation still needs both certified members; the
		// conceptual link does not substitute for the missing one.
		if closed := g.Closure(map[string]bool{"a": true, "c": true}); closed["t1"] {
			t.Errorf("Closure({a, c}) derives t1: the conceptual link to c must not stand in for b in the AND path")
		}
		if closed := g.Closure(map[string]bool{"a": true, "b": true}); !closed["t1"] {
			t.Errorf("Closure({a, b}) does not derive t1")
		}
		// Conceptual links stay navigable without entering the
		// derivation structure: t5 is not a derivation descendant of t1.
		if g.Descendants("t1")["t5"] {
			t.Errorf("t5 is a derivation descendant of t1: conceptual links must not enter Children")
		}
		if got := g.ConceptualDescendants("t1"); !got["t5"] {
			t.Errorf("conceptual descendants of t1 = %v, want t5 among them", got)
		}
		if got := g.ConceptualAncestors("t5"); len(got) != 2 || !got["t1"] || !got["c"] {
			t.Errorf("conceptual ancestors of t5 = %v, want exactly {t1, c}: c is a conceptual prerequisite of t1, while t1's derivation prerequisites a and b are not conceptual ancestors", got)
		}
	})

	t.Run("AndRequiresAllRules", func(t *testing.T) {
		_, g := ff02LoadFixture(t)
		if closed := g.Closure(map[string]bool{"a": true}); closed["t1"] {
			t.Errorf("Closure({a}) derives t1: an AND derivation fires only when all rules hold")
		}
		if closed := g.Closure(map[string]bool{"b": true}); closed["t1"] {
			t.Errorf("Closure({b}) derives t1: an AND derivation fires only when all rules hold")
		}
		if closed := g.Closure(map[string]bool{"a": true, "b": true}); !closed["t1"] {
			t.Errorf("Closure({a, b}) does not derive t1")
		}
	})

	t.Run("OrFiresOnAnyCertifiedPath", func(t *testing.T) {
		_, g := ff02LoadFixture(t)
		if closed := g.Closure(map[string]bool{"a": true}); !closed["t2"] {
			t.Errorf("Closure({a}) does not derive t2 through its certified primary path")
		}
		if closed := g.Closure(map[string]bool{"b": true}); !closed["t2"] {
			t.Errorf("Closure({b}) does not derive t2 through its certified alternative path")
		}
	})

	t.Run("UnreviewedNeverDerives", func(t *testing.T) {
		s, g := ff02LoadFixture(t)
		f3, err := s.ByID("t3")
		if err != nil {
			t.Fatal(err)
		}
		if len(f3.DerivationRules) != 1 || f3.DerivationRules[0].Status != "unreviewed" {
			t.Fatalf("t3 derivation rules = %+v, want one unreviewed rule", f3.DerivationRules)
		}
		if len(f3.CertifiedPaths()) != 0 {
			t.Fatalf("t3 certified paths = %v, want none", f3.CertifiedPaths())
		}
		if !f3.HasUnreviewedDerivation() {
			t.Errorf("t3 should report an unreviewed derivation")
		}
		// An unreviewed-only formula is a root for optimization: it
		// can only be memorized, never derived.
		if closed := g.Closure(map[string]bool{"a": true}); closed["t3"] {
			t.Errorf("Closure({a}) derives t3 through its unreviewed rule")
		}
		if set, err := g.MinMemorizeFor([]string{"t3"}, nil); err != nil || len(set) != 1 || set[0] != "t3" {
			t.Errorf("MinMemorizeFor([t3]) = %v, %v, want [t3]", set, err)
		}
		// t4: the unreviewed primary path grants nothing, but the
		// certified alternative path still fires.
		if closed := g.Closure(map[string]bool{"a": true}); closed["t4"] {
			t.Errorf("Closure({a}) derives t4 through its unreviewed primary path")
		}
		if closed := g.Closure(map[string]bool{"b": true}); !closed["t4"] {
			t.Errorf("Closure({b}) does not derive t4 through its certified alternative path")
		}
		if w := g.Witnesses(map[string]bool{"a": true, "b": true}); w["t3"] != nil {
			t.Errorf("t3 has a derivation witness %v, want none", w["t3"])
		} else if got := w["t4"]; len(got) != 1 || got[0] != "b" {
			t.Errorf("witness for t4 = %v, want [b]", got)
		}
	})

	t.Run("OptimizationUsesCertifiedOnly", func(t *testing.T) {
		_, g := ff02LoadFixture(t)
		wantSet := []string{"a", "b", "c", "d", "t3", "t5"}
		if set := g.MinMemorizeSet(nil); !equalStrings(set, wantSet) {
			t.Errorf("MinMemorizeSet = %v, want %v (unreviewed-only and conceptual-only formulas are roots)", set, wantSet)
		}
		if set, err := g.MinMemorizeFor([]string{"t1"}, nil); err != nil || !equalStrings(set, []string{"t1"}) {
			t.Errorf("MinMemorizeFor([t1]) = %v, %v, want [t1]: memorizing the target beats its two-element derivation", set, err)
		}
		if set, err := g.MinMemorizeFor([]string{"t1", "t2", "t4"}, nil); err != nil || !equalStrings(set, []string{"a", "b"}) {
			t.Errorf("MinMemorizeFor([t1, t2, t4]) = %v, %v, want [a b]: the conceptual link to c must not be part of the covering set", set, err)
		}
		if set, err := g.MinMemorizeFor([]string{"t5"}, nil); err != nil || !equalStrings(set, []string{"t5"}) {
			t.Errorf("MinMemorizeFor([t5]) = %v, %v, want [t5]", set, err)
		}
		if set, err := g.MinMemorizeFor([]string{"t2"}, nil); err != nil || len(set) != 1 || (set[0] != "a" && set[0] != "b") {
			t.Errorf("MinMemorizeFor([t2]) = %v, %v, want a single-element set [a] or [b]", set, err)
		}
	})

	t.Run("StatsCountCertifiedOnly", func(t *testing.T) {
		_, g := ff02LoadFixture(t)
		st, err := g.Stats()
		if err != nil {
			t.Fatal(err)
		}
		if st.Edges != 5 {
			t.Errorf("Stats.Edges = %d, want 5 (t1 a,b; t2 a,b; t4 b; unreviewed paths excluded)", st.Edges)
		}
		if st.Roots != 6 {
			t.Errorf("Stats.Roots = %d, want 6 (a, b, c, d, t3, t5)", st.Roots)
		}
		if st.ConceptualEdges != 2 {
			t.Errorf("Stats.ConceptualEdges = %d, want 2 (c->t1, t1->t5)", st.ConceptualEdges)
		}
		if st.Depth != 1 {
			t.Errorf("Stats.Depth = %d, want 1", st.Depth)
		}
	})

	t.Run("Rejections", func(t *testing.T) {
		cases := []struct {
			name string
			yaml string
			want string
		}{
			{
				"conceptual unknown reference",
				"formulas:\n  - {id: a, name: A, latex: x}\n  - {id: b, name: B, latex: y, conceptual_prerequisites: [ghost]}\n",
				"unknown id",
			},
			{
				"conceptual self reference",
				"formulas:\n  - {id: a, name: A, latex: x, conceptual_prerequisites: [a]}\n",
				"conceptual prerequisite",
			},
			{
				"conceptual cycle",
				"formulas:\n  - {id: a, name: A, latex: x, conceptual_prerequisites: [b]}\n  - {id: b, name: B, latex: y, conceptual_prerequisites: [a]}\n",
				"cycle",
			},
			{
				"invalid rule status",
				"formulas:\n  - {id: a, name: A, latex: x}\n  - {id: b, name: B, latex: y, derives_from: [a], derivation_rules: [{status: pending}]}\n",
				"status",
			},
			{
				"orphan derivation rule",
				"formulas:\n  - {id: a, name: A, latex: x}\n  - {id: b, name: B, latex: y, derives_from: [a], derivation_rules: [{status: reviewed}, {status: reviewed}]}\n",
				"derivation rule",
			},
		}
		for _, tc := range cases {
			if _, err := models.Load([]byte(tc.yaml)); err == nil {
				t.Errorf("%s: Load accepted the sheet, want an error", tc.name)
			} else if !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Errorf("%s: error %q does not mention %q", tc.name, err, tc.want)
			}
		}
		// A derivation cycle is still rejected by the graph layer.
		s, err := models.Load([]byte("formulas:\n  - {id: a, name: A, latex: x, derives_from: [b]}\n  - {id: b, name: B, latex: y, derives_from: [a]}\n"))
		if err != nil {
			t.Fatalf("load cyclic sheet: %v", err)
		}
		if err := graph.New(s).AssertAcyclic(); err == nil {
			t.Errorf("derivation cycle accepted by AssertAcyclic")
		}
	})

	t.Run("RenderSuppressesProofClaims", func(t *testing.T) {
		s, _ := ff02LoadFixture(t)
		html, err := render.HTML(s)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(html, "Builds on:") {
			t.Errorf("HTML lost the Builds on line for certified derivations")
		}
		if !strings.Contains(html, "Derivation under review") {
			t.Errorf("HTML does not mark the unreviewed derivation as under review")
		}
		i := strings.Index(html, `id='t3'`)
		j := strings.Index(html[i:], `id='t4'`)
		if i < 0 || j < 0 {
			t.Fatalf("HTML is missing the t3 or t4 card")
		}
		seg := html[i : i+j]
		if strings.Contains(seg, "Builds on:") {
			t.Errorf("t3 card presents its unreviewed derivation as proven: %q", seg)
		}
		if !strings.Contains(seg, "Derivation under review") {
			t.Errorf("t3 card does not label its derivation as under review: %q", seg)
		}
		tex, err := render.Latex(s)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(tex, "Derivation under review") {
			t.Errorf("LaTeX export does not mark the unreviewed derivation as under review")
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "ff02.tsv")
		rows, err := anki.ExportTSV(s, path)
		if err != nil {
			t.Fatal(err)
		}
		if rows != 18 {
			t.Errorf("Anki rows = %d, want 18 (2 per formula)", rows)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		var t3front, t1front string
		for _, ln := range lines[1:] {
			fields := strings.Split(ln, "\t")
			if len(fields) < 1 {
				continue
			}
			if strings.HasPrefix(fields[0], "Tee Three") {
				t3front = fields[0]
			}
			if strings.HasPrefix(fields[0], "Tee One") {
				t1front = fields[0]
			}
		}
		if t3front == "" {
			t.Fatalf("no Anki row for Tee Three")
		}
		if strings.Contains(t3front, "Builds on:") {
			t.Errorf("Anki front for Tee Three presents its unreviewed derivation as proven: %q", t3front)
		}
		if !strings.Contains(t3front, "under review") {
			t.Errorf("Anki front for Tee Three does not mark the derivation as under review: %q", t3front)
		}
		if !strings.Contains(t1front, "Builds on: Alpha, Beta") {
			t.Errorf("Anki front for Tee One = %q, want the certified Builds on note", t1front)
		}
	})

	t.Run("MigrationReportEnumeratesChanges", func(t *testing.T) {
		s, _ := ff02LoadFixture(t)
		rep := graph.MigrationReportFor(s)
		if !rep.Changed() {
			t.Fatalf("migration report says nothing changed for the fixture, want the typed-relationship effects listed")
		}
		ff02HasChange(t, rep, "root-added", "t3")
		ff02HasChange(t, rep, "edge-removed", "a->t3")
		ff02HasChange(t, rep, "edge-removed", "a->t4")
		ff02HasChange(t, rep, "curve-changed", "K=1: 0.4444 -> 0.3333")
		ff02HasChange(t, rep, "curve-changed", "length 6 -> 7")
		foundCurve := false
		for _, ch := range rep.Changes {
			if ch.Kind == "curve-changed" && strings.Contains(ch.Detail, "K=1") {
				foundCurve = true
			}
			if strings.Contains(ch.Detail, "t5") {
				t.Errorf("migration report hides nothing but lists t5 (%s %q), which was a root before and after", ch.Kind, ch.Detail)
			}
		}
		if !foundCurve {
			t.Errorf("migration report lists no curve change at K=1; got %v", rep.Changes)
		}
	})

	t.Run("ShippedCatalogUnchanged", func(t *testing.T) {
		sheets, err := models.LoadAll()
		if err != nil {
			t.Fatal(err)
		}
		totalFormulas, totalEdges := 0, 0
		for _, name := range models.ListSheets() {
			rep := graph.MigrationReportFor(sheets[name])
			if rep.Changed() {
				t.Errorf("migration report for shipped sheet %s lists changes, want none (no shipped formula uses the new fields): %v", name, rep.Changes)
			}
			totalFormulas += len(sheets[name].Formulas)
			g := graph.New(sheets[name])
			st, err := g.Stats()
			if err != nil {
				t.Fatal(err)
			}
			totalEdges += st.Edges
		}
		if totalFormulas != 265 {
			t.Errorf("shipped catalog formulas = %d, want 265", totalFormulas)
		}
		if totalEdges != 284 {
			t.Errorf("shipped catalog certified edges = %d, want 284", totalEdges)
		}
	})
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
