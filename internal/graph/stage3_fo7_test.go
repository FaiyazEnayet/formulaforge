// Stage 3 FO7 fixtures: the published algorithm and schema contract.
//
// These tests pin the behaviors documented in
// docs/algorithm-contract.md: the distinction between union ancestors
// and AND/OR derivability, exact targets vs heuristic budget coverage,
// witness interpretation, search limits, tie-breaking, the
// unsupported-cycle behavior, the pinned Go dependencies, and the CLI
// examples (which run the real binary). The exact search is checked
// against an independent exhaustive oracle; closures used by the
// oracle are themselves pinned by hand-computed expectations in the
// first subtests.
package graph

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// fo7OrSheet: t needs BOTH a and b on its primary path, or r alone on
// its alternative path. Union ancestors of t are {a, b, r}.
const fo7OrSheet = `
course: TEST
title: FO7 OR fixture
description: AND path vs OR alternative
formulas:
  - id: r
    name: Root
    topic: Basics
    latex: r
    description: root formula
    derives_from: []
  - id: a
    name: Aye
    topic: Basics
    latex: a
    description: builds on r
    derives_from: [r]
  - id: b
    name: Bee
    topic: Basics
    latex: b
    description: builds on r
    derives_from: [r]
  - id: t
    name: Tee
    topic: Basics
    latex: t
    description: needs a and b, or r alone
    derives_from: [a, b]
    alt_derives_from: [[r]]
`

// fo7AltCycleSheet: the only cycle runs through a's primary path via
// b; a also has an alternative path from the root c, so under AND/OR
// semantics the whole sheet is derivable from {c}. The union graph
// still contains the cycle a <-> b.
const fo7AltCycleSheet = `
course: TEST
title: FO7 alternative cycle fixture
description: cycle only through a non-chosen alternative structure
formulas:
  - id: c
    name: Cee
    topic: Basics
    latex: c
    description: root formula
    derives_from: []
  - id: a
    name: Aye
    topic: Basics
    latex: a
    description: from b, or from c alone
    derives_from: [b]
    alt_derives_from: [[c]]
  - id: b
    name: Bee
    topic: Basics
    latex: b
    description: builds on a
    derives_from: [a]
`

// fo7GreedySheet: the standard greedy-coverage counterexample shape.
// Memorizing x gains 4 formulas at once (x plus its three dependents),
// so greedy takes it first; but a and b together unlock the joint node
// j and its two dependents, so the exact budget-2 optimum (7 formulas)
// beats the greedy budget-2 total (6 formulas). j1 and j2 each need j
// plus one root, so memorizing j directly cannot shortcut the pair.
const fo7GreedySheet = `
course: TEST
title: FO7 greedy counterexample fixture
description: greedy budget coverage is not exact
formulas:
  - id: x
    name: Ex
    topic: Basics
    latex: x
    description: root with three direct dependents
    derives_from: []
  - id: x1
    name: Ex One
    topic: Basics
    latex: x_1
    description: builds on x
    derives_from: [x]
  - id: x2
    name: Ex Two
    topic: Basics
    latex: x_2
    description: builds on x
    derives_from: [x]
  - id: x3
    name: Ex Three
    topic: Basics
    latex: x_3
    description: builds on x
    derives_from: [x]
  - id: a
    name: Aye
    topic: Basics
    latex: a
    description: root
    derives_from: []
  - id: a1
    name: Aye One
    topic: Basics
    latex: a_1
    description: builds on a
    derives_from: [a]
  - id: b
    name: Bee
    topic: Basics
    latex: b
    description: root
    derives_from: []
  - id: b1
    name: Bee One
    topic: Basics
    latex: b_1
    description: builds on b
    derives_from: [b]
  - id: j
    name: Jay
    topic: Basics
    latex: j
    description: needs both a and b
    derives_from: [a, b]
  - id: j1
    name: Jay One
    topic: Basics
    latex: j_1
    description: builds on j and a
    derives_from: [j, a]
  - id: j2
    name: Jay Two
    topic: Basics
    latex: j_2
    description: builds on j and b
    derives_from: [j, b]
`

// fo7TieSheet: sheet order is beta before alpha on purpose, so sorted
// (not insertion) order is what the assertions observe.
const fo7TieSheet = `
course: TEST
title: FO7 tie-breaking fixture
description: symmetric roots feeding one joint node
formulas:
  - id: beta
    name: Beta
    topic: Basics
    latex: \beta
    description: root listed first
    derives_from: []
  - id: alpha
    name: Alpha
    topic: Basics
    latex: \alpha
    description: root listed second
    derives_from: []
  - id: gamma
    name: Gamma
    topic: Basics
    latex: \gamma
    description: needs alpha and beta
    derives_from: [alpha, beta]
`

// fo7FIFOSheet: pins the precise TopoOrder tie-breaking rule. The
// initial ready queue is sorted by id, but nodes are then emitted in
// the order they become ready: aa becomes ready only after a is
// emitted, so it lands behind the already-queued root b. The output
// a, b, aa is therefore NOT the lexicographically smallest
// topological order (that would be a, aa, b).
const fo7FIFOSheet = `
course: TEST
title: FO7 FIFO tie fixture
description: ready-queue order vs lexicographic order
formulas:
  - id: b
    name: Bee
    topic: Basics
    latex: b
    description: root
    derives_from: []
  - id: a
    name: Aye
    topic: Basics
    latex: a
    description: root
    derives_from: []
  - id: aa
    name: Aye Aye
    topic: Basics
    latex: aa
    description: builds on a
    derives_from: [a]
`

func loadFO7Sheet(t *testing.T, data string) *Graph {
	t.Helper()
	s, err := models.Load([]byte(data))
	if err != nil {
		t.Fatalf("fixture sheet failed to load: %v", err)
	}
	return New(s)
}

func sameStringSet(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	set := map[string]bool{}
	for _, w := range want {
		set[w] = true
	}
	for _, g := range got {
		if !set[g] {
			return false
		}
	}
	return true
}

// witnessLeaves is the independent oracle for the documented witness
// interpretation: a formula is witnessed by a set of known leaves when
// it is itself known, or when one of its derivation paths has every
// member witnessed. It reads only the exported Paths, never the
// search internals.
func witnessLeaves(g *Graph, target string, known map[string]bool) (map[string]bool, bool) {
	var rec func(n string, visiting map[string]bool) (map[string]bool, bool)
	rec = func(n string, visiting map[string]bool) (map[string]bool, bool) {
		if known[n] {
			return map[string]bool{n: true}, true
		}
		if visiting[n] {
			return nil, false
		}
		visiting[n] = true
		defer delete(visiting, n)
		for _, path := range g.Paths[n] {
			leaves := map[string]bool{}
			ok := true
			for _, m := range path {
				sub, subOK := rec(m, visiting)
				if !subOK {
					ok = false
					break
				}
				for k := range sub {
					leaves[k] = true
				}
			}
			if ok {
				return leaves, true
			}
		}
		return nil, false
	}
	return rec(target, map[string]bool{})
}

// bruteForceMinMemorize is the exhaustive oracle for the exact search:
// the smallest memorize-set size whose closure covers every target.
func bruteForceMinMemorize(g *Graph, targets []string, prior []string) int {
	nodes := g.Nodes
	best := len(nodes) + 1
	for mask := 0; mask < (1 << len(nodes)); mask++ {
		known := toSet(prior)
		size := 0
		for i, n := range nodes {
			if mask>>i&1 == 1 {
				known[n] = true
				size++
			}
		}
		if size >= best {
			continue
		}
		cl := g.Closure(known)
		ok := true
		for _, tgt := range targets {
			if !cl[tgt] {
				ok = false
				break
			}
		}
		if ok {
			best = size
		}
	}
	return best
}

func TestStage3Formulaforge7(t *testing.T) {
	t.Run("UnionAncestorsDifferFromDerivability", func(t *testing.T) {
		g := loadFO7Sheet(t, fo7OrSheet)
		anc := g.Ancestors("t")
		if len(anc) != 3 || !anc["a"] || !anc["b"] || !anc["r"] {
			t.Fatalf("Ancestors(t) = %v, want the union {a, b, r}", anc)
		}
		// Derivability is per-path AND/OR, not union membership:
		// knowing only a does not derive t (the primary path also
		// needs b, the alternative needs r), while knowing r alone
		// derives t through the alternative path, without going
		// through a and b as memorized prerequisites. Ancestor
		// membership is a structural fact, not a derivability claim.
		if cl := g.Closure(map[string]bool{"a": true}); cl["t"] {
			t.Errorf("Closure({a}) derives t; one member of an AND path must not suffice")
		}
		if cl := g.Closure(map[string]bool{"a": true, "b": true}); !cl["t"] {
			t.Errorf("Closure({a, b}) does not derive t via its primary path")
		}
		if cl := g.Closure(map[string]bool{"r": true}); !cl["t"] {
			t.Errorf("Closure({r}) does not derive t via its alternative path")
		}
		got, err := g.MinMemorizeFor([]string{"t"}, nil)
		if err != nil {
			t.Fatalf("MinMemorizeFor: %v", err)
		}
		if !sameStringSet(got, "r") {
			t.Errorf("MinMemorizeFor([t]) = %v, want [r] (the OR branch)", got)
		}
	})

	t.Run("UnsupportedCycleThroughAlternative", func(t *testing.T) {
		g := loadFO7Sheet(t, fo7AltCycleSheet)
		cyc := g.FindCycle()
		if len(cyc) == 0 {
			t.Fatalf("FindCycle = nil, want the union cycle through a and b")
		}
		joined := strings.Join(cyc, " ")
		if !strings.Contains(joined, "a") || !strings.Contains(joined, "b") {
			t.Errorf("FindCycle = %v, want a cycle naming a and b", cyc)
		}
		if err := g.AssertAcyclic(); err == nil || !strings.Contains(err.Error(), "dependency cycle") {
			t.Errorf("AssertAcyclic = %v, want a dependency cycle error", err)
		}
		if _, err := g.TopoOrder(); err == nil {
			t.Errorf("TopoOrder on a union-cyclic graph returned no error")
		}
		if _, err := g.Levels(); err == nil {
			t.Errorf("Levels on a union-cyclic graph returned no error")
		}
		if _, err := g.Stats(); err == nil {
			t.Errorf("Stats on a union-cyclic graph returned no error")
		}
		// The unsupported part is the ordering layer. Derivation itself
		// still resolves the sheet from the root c, and the exact
		// search (which works on Paths, not the union order) still
		// answers.
		cl := g.Closure(map[string]bool{"c": true})
		if !cl["a"] || !cl["b"] {
			t.Errorf("Closure({c}) = %v, want a and b derivable via the alternative path", cl)
		}
		got, err := g.MinMemorizeFor([]string{"b"}, nil)
		if err != nil {
			t.Fatalf("MinMemorizeFor: %v", err)
		}
		if !sameStringSet(got, "c") {
			t.Errorf("MinMemorizeFor([b]) = %v, want [c]", got)
		}
		if ms := g.MinMemorizeSet(nil); !sameStringSet(ms, "c") {
			t.Errorf("MinMemorizeSet = %v, want [c] (the only root)", ms)
		}
	})

	t.Run("WitnessInterpretation", func(t *testing.T) {
		g := loadFO7Sheet(t, fo7OrSheet)
		ms, err := g.MinMemorizeFor([]string{"t"}, nil)
		if err != nil {
			t.Fatalf("MinMemorizeFor: %v", err)
		}
		known := toSet(ms)
		leaves, ok := witnessLeaves(g, "t", known)
		if !ok {
			t.Fatalf("no witness for t under the returned memorize set %v", ms)
		}
		if len(leaves) != 1 || !leaves["r"] {
			t.Errorf("witness leaves for t = %v, want exactly {r}", leaves)
		}
		// AND semantics inside a witness: knowing only a does not
		// witness t, because the primary path also needs b and the
		// alternative path needs r.
		if _, ok := witnessLeaves(g, "t", map[string]bool{"a": true}); ok {
			t.Errorf("witness for t accepted known={a}; AND/OR semantics violated")
		}
		// An empty memorize set witnesses nothing beyond its members.
		if _, ok := witnessLeaves(g, "t", map[string]bool{}); ok {
			t.Errorf("witness for t accepted an empty known set")
		}
	})

	t.Run("ExactSearchMatchesExhaustiveOracle", func(t *testing.T) {
		cases := []struct {
			sheet   string
			targets []string
			prior   []string
		}{
			{fo7OrSheet, []string{"t"}, nil},
			{fo7OrSheet, []string{"a", "b"}, nil},
			{fo7OrSheet, []string{"t"}, []string{"a"}},
			{fo7AltCycleSheet, []string{"b"}, nil},
			{fo7GreedySheet, []string{"j", "j1", "j2"}, nil},
			{fo7GreedySheet, []string{"x1", "j"}, nil},
		}
		for i, tc := range cases {
			g := loadFO7Sheet(t, tc.sheet)
			got, err := g.MinMemorizeFor(tc.targets, tc.prior)
			if err != nil {
				t.Fatalf("case %d: MinMemorizeFor: %v", i, err)
			}
			want := bruteForceMinMemorize(g, tc.targets, tc.prior)
			if len(got) != want {
				t.Errorf("case %d: MinMemorizeFor size = %d, exhaustive optimum = %d", i, len(got), want)
			}
			// The returned set must actually cover, and be deterministic.
			cl := g.Closure(toSet(append(append([]string{}, tc.prior...), got...)))
			for _, tgt := range tc.targets {
				if !cl[tgt] {
					t.Errorf("case %d: returned set %v does not derive target %s", i, got, tgt)
				}
			}
			again, err := g.MinMemorizeFor(tc.targets, tc.prior)
			if err != nil || strings.Join(again, ",") != strings.Join(got, ",") {
				t.Errorf("case %d: MinMemorizeFor not deterministic: %v vs %v", i, got, again)
			}
		}
	})

	t.Run("GreedyCoverageIsHeuristicNotExact", func(t *testing.T) {
		g := loadFO7Sheet(t, fo7GreedySheet)
		curve := g.CoverageCurve()
		if len(curve) < 3 {
			t.Fatalf("CoverageCurve = %v, want at least 3 points", curve)
		}
		total := float64(len(g.Nodes))
		greedyAt2 := int(curve[2].Fraction*total + 0.5)
		if greedyAt2 != 6 {
			t.Errorf("greedy coverage at budget 2 = %d formulas, want 6 (x first, then a)", greedyAt2)
		}
		// Independent budget-2 optimum by exhaustive subset search.
		best := 0
		nodes := g.Nodes
		for i := 0; i < len(nodes); i++ {
			for j := i + 1; j < len(nodes); j++ {
				cl := g.Closure(map[string]bool{nodes[i]: true, nodes[j]: true})
				if len(cl) > best {
					best = len(cl)
				}
			}
		}
		if best != 7 {
			t.Fatalf("exhaustive budget-2 optimum = %d, want 7 (a and b together)", best)
		}
		if greedyAt2 >= best {
			t.Errorf("greedy matched the exact optimum on the counterexample fixture; the heuristic gap is gone and the contract example is stale")
		}
		// Exact whole-sheet memorization is a different question with
		// an exact answer: every root, nothing else.
		if ms := g.MinMemorizeSet(nil); !sameStringSet(ms, "a", "b", "x") {
			t.Errorf("MinMemorizeSet = %v, want the three roots [a b x]", ms)
		}
	})

	t.Run("TieBreakingAndOrderingKnownAnswers", func(t *testing.T) {
		g := loadFO7Sheet(t, fo7TieSheet)
		order, err := g.TopoOrder()
		if err != nil {
			t.Fatalf("TopoOrder: %v", err)
		}
		if strings.Join(order, ",") != "alpha,beta,gamma" {
			t.Errorf("TopoOrder = %v, want [alpha beta gamma] (id tie-break, not sheet order)", order)
		}
		rank := g.LeverageRanking()
		if len(rank) != 3 || rank[0].ID != "alpha" || rank[1].ID != "beta" || rank[0].Leverage != 1 || rank[1].Leverage != 1 {
			t.Errorf("LeverageRanking = %v, want alpha then beta at leverage 1 (ties by id)", rank)
		}
		levels, err := g.Levels()
		if err != nil {
			t.Fatalf("Levels: %v", err)
		}
		if levels["alpha"] != 0 || levels["beta"] != 0 || levels["gamma"] != 1 {
			t.Errorf("Levels = %v, want alpha=0 beta=0 gamma=1", levels)
		}
		if d, err := g.Depth(); err != nil || d != 1 {
			t.Errorf("Depth = %d, %v; want 1", d, err)
		}
		if ms := g.MinMemorizeSet(nil); strings.Join(ms, ",") != "alpha,beta" {
			t.Errorf("MinMemorizeSet = %v, want [alpha beta] sorted", ms)
		}
		// The precise rule: sorted initial queue, then readiness
		// (FIFO) order. aa becomes ready after a is emitted and so
		// lands behind the already-queued root b.
		g2 := loadFO7Sheet(t, fo7FIFOSheet)
		order2, err := g2.TopoOrder()
		if err != nil {
			t.Fatalf("TopoOrder FIFO fixture: %v", err)
		}
		if strings.Join(order2, ",") != "a,b,aa" {
			t.Errorf("TopoOrder = %v, want [a b aa] (readiness order, not lexicographically smallest)", order2)
		}
	})

	t.Run("DependencyPins", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
		if err != nil {
			t.Fatalf("read go.mod: %v", err)
		}
		mod := string(data)
		if !strings.Contains(mod, "go 1.24.2") {
			t.Errorf("go.mod does not pin the go 1.24.2 toolchain directive")
		}
		direct := []string{
			"github.com/fogleman/gg v1.3.0",
			"github.com/jung-kurt/gofpdf v1.16.2",
			"github.com/spf13/cobra v1.10.2",
			"gopkg.in/yaml.v3 v3.0.1",
		}
		indirect := []string{
			"github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect",
			"github.com/inconshreveable/mousetrap v1.1.0 // indirect",
			"github.com/spf13/pflag v1.0.9 // indirect",
			"golang.org/x/image v0.0.0-20190910094157-69e4b8554b2a // indirect",
		}
		for _, pin := range direct {
			if !strings.Contains(mod, pin) {
				t.Errorf("go.mod is missing the direct pin %q", pin)
			}
			if strings.Contains(mod, pin+" // indirect") {
				t.Errorf("direct dependency %q is marked // indirect; go.mod is not tidy", pin)
			}
		}
		for _, pin := range indirect {
			if !strings.Contains(mod, pin) {
				t.Errorf("go.mod is missing the indirect pin %q", pin)
			}
		}
	})

	t.Run("PublishedContractPresent", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join("..", "..", "docs", "algorithm-contract.md"))
		if err != nil {
			t.Fatalf("published contract missing: %v", err)
		}
		doc := strings.ToLower(string(data))
		for _, topic := range []string{
			"union", "witness", "cycle", "tie-break", "complexity",
			"greedy", "exact", "schema", "counterexample",
		} {
			if !strings.Contains(doc, topic) {
				t.Errorf("contract doc does not cover %q", topic)
			}
		}
		for _, name := range []string{
			"FindCycle", "TopoOrder", "Closure", "MinMemorizeFor",
			"MinMemorizeSet", "CoverageCurve", "LeverageRanking",
		} {
			if !strings.Contains(string(data), name) {
				t.Errorf("contract doc does not document %s", name)
			}
		}
	})

	t.Run("CLIExecutableFixtures", func(t *testing.T) {
		root, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			t.Fatalf("repo root: %v", err)
		}
		bin := filepath.Join(t.TempDir(), "formulaforge")
		build := exec.Command("go", "build", "-o", bin, "./cmd/formulaforge")
		build.Dir = root
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build CLI: %v\n%s", err, out)
		}
		run := func(args ...string) (string, string, int) {
			cmd := exec.Command(bin, args...)
			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			code := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("run %v: %v", args, err)
			}
			return stdout.String(), stderr.String(), code
		}
		out, _, code := run("sheets")
		if code != 0 || !strings.Contains(out, "psy103") {
			t.Errorf("sheets: code=%d, output missing psy103:\n%s", code, out)
		}
		out, _, code = run("graph", "psy103")
		if code != 0 {
			t.Fatalf("graph psy103: exit %d", code)
		}
		for _, want := range []string{"formulas:   22", "derivations:26", "study waves:6 (longest chain: 5)", "roots: 5, leaves: 11", "minimum memorize set (5):"} {
			if !strings.Contains(out, want) {
				t.Errorf("graph psy103 output missing %q:\n%s", want, out)
			}
		}
		out, _, code = run("graph", "psy103", "--target", "regression-line,p-value")
		if code != 0 || !strings.Contains(out, "smallest set covering targets (1):") {
			t.Errorf("graph --target: code=%d, want a 1-formula covering set (mean):\n%s", code, out)
		}
		_, stderr, code := run("graph", "psy103", "--target", "regression_line")
		if code == 0 || !strings.Contains(stderr, "unknown formula id: regression_line") {
			t.Errorf("graph --target regression_line: code=%d stderr=%q, want the stable unknown-id error", code, stderr)
		}
		_, stderr, code = run("graph", "nosuchsheet")
		if code == 0 || !strings.Contains(stderr, `no sheet named "nosuchsheet"`) {
			t.Errorf("graph nosuchsheet: code=%d stderr=%q, want the stable unknown-sheet error", code, stderr)
		}
		out, _, code = run("study-order", "psy103")
		if code != 0 || !strings.HasPrefix(out, "Wave 1:") {
			t.Errorf("study-order psy103: code=%d, want output starting with Wave 1:\n%s", code, out)
		}
	})
}
