// Stage 3 FF-05 fixtures: profile and accelerate closure
// computation without changing answers.
//
// The production Closure used to rescan every node and every path
// until a fixpoint was reached. The compiled closure in compiled.go
// compiles each conjunctive derivation path once into reverse
// incidence lists and runs forward chaining: each path counts its
// unmet prerequisites, each prerequisite points at the paths
// waiting on it, and a consequent fires the moment one of its
// paths reaches zero.
//
// These tests are the equivalence oracle for that change:
//
//   - Closure matches the independent FF-04 reference closure on
//     every shipped sheet, on generated small graphs across all
//     five FO6 families, and on hand-built edge cases (duplicate
//     path members, a self-referential path, a path naming an
//     unknown id).
//   - The compiled structure itself is checked: one compiled path
//     per derivation path, reverse incidence counts that match the
//     path memberships, and a compiled closure that agrees with
//     the reference on every probed known set.
//   - Solver and curve answers are pinned to the values the
//     pre-change code produced on every shipped sheet
//     (CoverageCurve, MinMemorizeSet, MinMemorizeFor on fixed
//     targets, Stats), so any answer change fails loudly.
//   - Witnesses produced from the compiled closure still validate.
//
// Benchmarks at the bottom measure the same fixed instances the
// pre-change code was measured on; see the PR body for the
// before/after numbers.
package graph

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// ff05EdgeSheet is a hand-built fixture for the closure edge cases
// the shipped sheets do not exercise.
//
//   - Chain a -> b -> c, plus a diamond: d derives from {b, c} or
//     from {a} alone.
//   - e carries a path with a duplicated member {a, a}: it must
//     fire exactly when a is known, not need a twice.
//   - s carries a self-referential path {s} and an alternative
//     {a}: it must not derive from nothing, and must derive once
//     a is known.
//   - u derives only from {ghost}, an id that is not a graph node:
//     it must never derive. v derives from {ghost} or from {a}:
//     only the second path can ever fire.
func ff05EdgeSheet() *Graph {
	fs := []models.Formula{
		{ID: "a", Name: "a"},
		{ID: "b", Name: "b", DerivesFrom: []string{"a"}},
		{ID: "c", Name: "c", DerivesFrom: []string{"b"}},
		{ID: "d", Name: "d", DerivesFrom: []string{"b", "c"}, AltDerivesFrom: [][]string{{"a"}}},
		{ID: "e", Name: "e", DerivesFrom: []string{"a", "a"}},
		{ID: "s", Name: "s", DerivesFrom: []string{"s"}, AltDerivesFrom: [][]string{{"a"}}},
		{ID: "u", Name: "u", DerivesFrom: []string{"ghost"}},
		{ID: "v", Name: "v", DerivesFrom: []string{"ghost"}, AltDerivesFrom: [][]string{{"a"}}},
	}
	return New(&models.Sheet{Course: "T", Title: "FF05", Formulas: fs})
}

// ff05SeedSlice converts a known map to the compiled closure's
// indexed seed: position i holds whether Nodes[i] is known.
func ff05SeedSlice(g *Graph, known map[string]bool) []bool {
	seed := make([]bool, len(g.Nodes))
	for i, n := range g.Nodes {
		seed[i] = known[n]
	}
	return seed
}

// ff05SliceSet converts an indexed closure result back to a set.
func ff05SliceSet(g *Graph, done []bool) map[string]bool {
	out := map[string]bool{}
	for i, n := range g.Nodes {
		if done[i] {
			out[n] = true
		}
	}
	return out
}

// ff05CheckClosure asserts that the production closure, the
// compiled closure, and the independent reference closure all agree
// on known, and that the result holds graph nodes only.
func ff05CheckClosure(t *testing.T, g *Graph, known map[string]bool, where string) {
	t.Helper()
	want := referenceClosure(g, known)
	if got := g.Closure(known); fmt.Sprint(sortedKeys(got)) != fmt.Sprint(sortedKeys(want)) {
		t.Errorf("%s: Closure = %v, reference = %v", where, sortedKeys(got), sortedKeys(want))
	}
	compiled := g.compiledForm().closure(ff05SeedSlice(g, known))
	if got := ff05SliceSet(g, compiled); fmt.Sprint(sortedKeys(got)) != fmt.Sprint(sortedKeys(want)) {
		t.Errorf("%s: compiled closure = %v, reference = %v", where, sortedKeys(got), sortedKeys(want))
	}
	for id := range want {
		found := false
		for _, n := range g.Nodes {
			if n == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: closure holds non-node id %q", where, id)
		}
	}
}

func TestStage3Formulaforge05(t *testing.T) {
	t.Run("CompiledStructureCountsPaths", func(t *testing.T) {
		for _, name := range models.ListSheets() {
			g := loadGraph(t, name)
			c := g.compiledForm()
			totalPaths := 0
			for _, paths := range g.Paths {
				totalPaths += len(paths)
			}
			if len(c.pathNode) != totalPaths {
				t.Errorf("sheet %s: compiled paths = %d, want %d", name, len(c.pathNode), totalPaths)
			}
			if c.n != len(g.Nodes) {
				t.Errorf("sheet %s: compiled node count = %d, want %d", name, c.n, len(g.Nodes))
			}
			// Reverse incidence: each prerequisite must list
			// exactly the compiled paths that name it.
			idx := map[string]int{}
			for i, n := range g.Nodes {
				idx[n] = i
			}
			wantWaiting := make([]int, len(g.Nodes))
			for _, paths := range g.Paths {
				for _, p := range paths {
					seen := map[string]bool{}
					for _, m := range p {
						if seen[m] {
							continue
						}
						seen[m] = true
						if i, ok := idx[m]; ok {
							wantWaiting[i]++
						}
					}
				}
			}
			for i, n := range g.Nodes {
				if len(c.waiting[i]) != wantWaiting[i] {
					t.Errorf("sheet %s: waiting[%s] = %d paths, want %d", name, n, len(c.waiting[i]), wantWaiting[i])
				}
			}
		}
	})

	t.Run("ClosureMatchesReferenceOnShippedSheets", func(t *testing.T) {
		for _, name := range models.ListSheets() {
			g := loadGraph(t, name)
			rng := rand.New(rand.NewSource(505))
			knownSets := []map[string]bool{
				{},
				toSet(g.MinMemorizeSet(nil)),
				{"zzz": true}, // unknown id: ignored
			}
			for k := 0; k < 4; k++ {
				known := map[string]bool{}
				for _, id := range g.Nodes {
					if rng.Intn(3) == 0 {
						known[id] = true
					}
				}
				// A false entry must not count as known.
				known[g.Nodes[k%len(g.Nodes)]] = false
				knownSets = append(knownSets, known)
			}
			for i, known := range knownSets {
				ff05CheckClosure(t, g, known, fmt.Sprintf("sheet %s known-set %d", name, i))
			}
		}
	})

	t.Run("ClosureMatchesReferenceOnGeneratedGraphs", func(t *testing.T) {
		families := []fo6Family{fo6Sparse, fo6AndHeavy, fo6OrHeavy, fo6Bottleneck, fo6DenseMixed}
		for _, family := range families {
			for seed := int64(0); seed < 4; seed++ {
				spec := fo6RandomSpec(seed, 12, family)
				g, _ := fo6Graph(spec, 7000+seed)
				rng := rand.New(rand.NewSource(505 + seed))
				for k := 0; k < 5; k++ {
					known := map[string]bool{}
					for _, id := range g.Nodes {
						if rng.Intn(2) == 0 {
							known[id] = true
						}
					}
					ff05CheckClosure(t, g, known, fmt.Sprintf("%s seed %d set %d", family, seed, k))
				}
			}
		}
	})

	t.Run("EdgeCases", func(t *testing.T) {
		g := ff05EdgeSheet()
		cases := []struct {
			known map[string]bool
			want  []string
		}{
			{map[string]bool{}, []string{}},
			{map[string]bool{"a": true}, []string{"a", "b", "c", "d", "e", "s", "v"}},
			{map[string]bool{"b": true}, []string{"b", "c", "d"}},
			{map[string]bool{"u": true}, []string{"u"}},
			{map[string]bool{"a": false}, []string{}},
			{map[string]bool{"ghost": true}, []string{}},
		}
		for i, tc := range cases {
			got := g.Closure(tc.known)
			if fmt.Sprint(sortedKeys(got)) != fmt.Sprint(tc.want) {
				t.Errorf("case %d: Closure(%v) = %v, want %v", i, tc.known, sortedKeys(got), tc.want)
			}
			ff05CheckClosure(t, g, tc.known, fmt.Sprintf("edge case %d", i))
		}
		// The compiled structure must hold no live path for u
		// (its only path names a non-node, so it is compiled
		// out and can never fire) and exactly one live path
		// for v (only the {a} alternative survives).
		c := g.compiledForm()
		counts := map[string]int{}
		for _, node := range c.pathNode {
			counts[g.Nodes[node]]++
		}
		if counts["u"] != 0 {
			t.Errorf("compiled live paths for u = %d, want 0 (dead path compiled out)", counts["u"])
		}
		if counts["v"] != 1 {
			t.Errorf("compiled live paths for v = %d, want 1 (only the {a} alternative)", counts["v"])
		}
	})

	t.Run("CompiledClosureOnHandBuiltGraph", func(t *testing.T) {
		// compileGraph on a Graph built by hand, including an empty
		// path (fires at once) and a path naming a non-node.
		g := &Graph{
			Nodes: []string{"a", "b", "c"},
			Paths: map[string][][]string{
				"a": {},
				"b": {{}},
				"c": {{"ghost"}, {"b"}},
			},
		}
		c := compileGraph(g)
		done := c.closure([]bool{false, false, false})
		if got := ff05SliceSet(g, done); fmt.Sprint(sortedKeys(got)) != "[b c]" {
			t.Errorf("hand-built compiled closure = %v, want [b c]", sortedKeys(got))
		}
		done = c.closure([]bool{true, false, false})
		if got := ff05SliceSet(g, done); fmt.Sprint(sortedKeys(got)) != "[a b c]" {
			t.Errorf("hand-built compiled closure from {a} = %v, want [a b c]", sortedKeys(got))
		}
		// The bitset form must agree with the slice form.
		bits := c.closureBits(newBitSet(3))
		if got := ff05SliceSet(g, bitsToBools(bits, 3)); fmt.Sprint(sortedKeys(got)) != "[b c]" {
			t.Errorf("hand-built compiled bitset closure = %v, want [b c]", sortedKeys(got))
		}
	})

	t.Run("SolverAndCurveAnswersUnchanged", func(t *testing.T) {
		// Pinned to the values the pre-change code produced on
		// every shipped sheet (captured 2026-11-24).
		wantCurve := map[string]string{
			"ams151": "[{0 0} {1 0.8} {2 1}]",
			"ams161": "[{0 0} {1 0.38461538461538464} {2 0.6153846153846154} {3 0.7307692307692307} {4 0.8461538461538461} {5 0.9230769230769231} {6 0.9615384615384616} {7 1}]",
			"ams210": "[{0 0} {1 0.2} {2 0.4} {3 0.5} {4 0.6333333333333333} {5 0.7333333333333333} {6 0.8666666666666667} {7 0.9666666666666667} {8 1}]",
			"ams261": "[{0 0} {1 0.37777777777777777} {2 0.5777777777777777} {3 0.7333333333333333} {4 0.8888888888888888} {5 1}]",
			"cse113": "[{0 0} {1 0.3125} {2 0.4375} {3 0.5625} {4 0.6875} {5 0.8125} {6 0.875} {7 0.90625} {8 0.9375} {9 0.96875} {10 1}]",
			"cse213": "[{0 0} {1 0.17647058823529413} {2 0.35294117647058826} {3 0.5294117647058824} {4 0.6176470588235294} {5 0.6764705882352942} {6 0.7352941176470589} {7 0.7941176470588235} {8 0.8529411764705882} {9 0.8823529411764706} {10 0.9117647058823529} {11 0.9411764705882353} {12 0.9705882352941176} {13 1}]",
			"phy131": "[{0 0} {1 0.39215686274509803} {2 0.5490196078431373} {3 0.7058823529411765} {4 0.7843137254901961} {5 0.8627450980392157} {6 0.9215686274509803} {7 0.9607843137254902} {8 0.9803921568627451} {9 1}]",
			"psy103": "[{0 0} {1 0.8181818181818182} {2 0.8636363636363636} {3 0.9090909090909091} {4 0.9545454545454546} {5 1}]",
		}
		wantMinMem := map[string]string{
			"ams151": "[antiderivative limit-def]",
			"ams161": "[improper-def parametric partial-fractions parts polar seq-limit trig-integrals]",
			"ams210": "[det-2x2 dot lin-indep matrix-add matrix-mult rref span transpose]",
			"ams261": "[double-rect line-scalar partial-def seq-limit vec-ops]",
			"cse113": "[cases element-method one-to-one onto quantifiers rational sequences truth-table]",
			"cse213": "[add-rule adj-matrix demorgan fsa-def graph-coloring handshaking implication independent-set injective mult-rule pigeonhole regex-def weak-induction]",
			"phy131": "[cm continuity density gravitation ke moment-inertia newton2 temp-scales vel-def]",
			"psy103": "[chi-square mean median mode range]",
		}
		wantMinFor := map[string]string{
			"ams151": "[antiderivative limit-def]",
			"ams161": "[parts polar-arclen seq-limit]",
			"ams210": "[matrix-add span symmetric-eigen]",
			"ams261": "[partial-def seq-limit vec-ops]",
			"cse113": "[composition truth-table]",
			"cse213": "[add-rule demorgan regex-def]",
			"phy131": "[ke newton2 vel-def]",
			"psy103": "[chi-square mean]",
		}
		wantStats := map[string][6]int{
			"ams151": {25, 29, 4, 5, 2, 12},
			"ams161": {26, 21, 4, 5, 7, 12},
			"ams210": {30, 37, 4, 5, 8, 13},
			"ams261": {45, 53, 5, 6, 5, 20},
			"cse113": {32, 30, 4, 5, 8, 12},
			"cse213": {34, 22, 4, 5, 13, 19},
			"phy131": {51, 66, 6, 7, 9, 23},
			"psy103": {22, 26, 5, 6, 5, 11},
		}
		for _, name := range models.ListSheets() {
			g := loadGraph(t, name)
			if got := fmt.Sprint(g.CoverageCurve()); got != wantCurve[name] {
				t.Errorf("sheet %s: CoverageCurve = %s, want %s", name, got, wantCurve[name])
			}
			if got := fmt.Sprint(g.MinMemorizeSet(nil)); got != wantMinMem[name] {
				t.Errorf("sheet %s: MinMemorizeSet = %s, want %s", name, got, wantMinMem[name])
			}
			targets := []string{g.Nodes[0], g.Nodes[len(g.Nodes)/2], g.Nodes[len(g.Nodes)-1]}
			ms, err := g.MinMemorizeFor(targets, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprint(ms); got != wantMinFor[name] {
				t.Errorf("sheet %s: MinMemorizeFor(%v) = %s, want %s", name, targets, got, wantMinFor[name])
			}
			st, err := g.Stats()
			if err != nil {
				t.Fatal(err)
			}
			gotStats := [6]int{st.Nodes, st.Edges, st.Depth, st.Waves, st.Roots, st.Leaves}
			if gotStats != wantStats[name] {
				t.Errorf("sheet %s: Stats = %v, want %v", name, gotStats, wantStats[name])
			}
		}
	})

	t.Run("SolverOptimumMatchesBruteForceOnGeneratedGraphs", func(t *testing.T) {
		families := []fo6Family{fo6Sparse, fo6AndHeavy, fo6OrHeavy, fo6Bottleneck, fo6DenseMixed}
		for _, family := range families {
			for seed := int64(0); seed < 3; seed++ {
				spec := fo6RandomSpec(seed, 12, family)
				g, idOf := fo6Graph(spec, 8000+seed)
				targets := fo6IDs(idOf, []int{9, 10, 11})
				oracle := referenceBruteMinFor(g, targets, nil)
				got, err := g.MinMemorizeFor(targets, nil)
				if err != nil {
					t.Fatalf("%s seed %d: %v", family, seed, err)
				}
				if len(got) != len(oracle) {
					t.Errorf("%s seed %d: optimum size %d, oracle %d (got %v, oracle %v)", family, seed, len(got), len(oracle), got, oracle)
				}
				closed := referenceClosure(g, toSet(got))
				for _, tg := range targets {
					if !closed[tg] {
						t.Errorf("%s seed %d: solver set %v does not derive %s under the reference closure", family, seed, got, tg)
					}
				}
			}
		}
	})

	t.Run("WitnessesStillValidate", func(t *testing.T) {
		for _, name := range models.ListSheets() {
			g := loadGraph(t, name)
			known := toSet(g.MinMemorizeSet(nil)[:1])
			w := g.Witnesses(known)
			if err := g.CheckWitnesses(known, w); err != nil {
				t.Errorf("sheet %s: witnesses from compiled closure fail validation: %v", name, err)
			}
			// Witnesses must be whole derivation paths.
			for id, path := range w {
				if !matchesWholePath(path, g.Paths[id]) {
					t.Errorf("sheet %s: witness for %s is not a whole path: %v", name, id, path)
				}
			}
		}
	})
}

// bitsToBools expands a bitset into a bool slice of length n.
func bitsToBools(b bitSet, n int) []bool {
	out := make([]bool, n)
	for i := range out {
		out[i] = b.has(i)
	}
	return out
}

// ff05ClosureBenchGraphs returns the fixed benchmark instances:
// one known set per shipped sheet plus generated dense graphs.
func ff05ClosureBenchGraphs() []struct {
	name  string
	g     *Graph
	known map[string]bool
} {
	var out []struct {
		name  string
		g     *Graph
		known map[string]bool
	}
	for _, name := range models.ListSheets() {
		s, err := models.LoadSheetByName(name)
		if err != nil {
			panic(err)
		}
		g := New(s)
		rng := rand.New(rand.NewSource(505))
		known := map[string]bool{}
		for _, id := range g.Nodes {
			if rng.Intn(3) == 0 {
				known[id] = true
			}
		}
		out = append(out, struct {
			name  string
			g     *Graph
			known map[string]bool
		}{name, g, known})
	}
	for _, n := range []int{28, 80} {
		spec := fo6RandomSpec(11, n, fo6DenseMixed)
		g, _ := fo6Graph(spec, 9000+int64(n))
		rng := rand.New(rand.NewSource(505))
		known := map[string]bool{}
		for _, id := range g.Nodes {
			if rng.Intn(3) == 0 {
				known[id] = true
			}
		}
		out = append(out, struct {
			name  string
			g     *Graph
			known map[string]bool
		}{fmt.Sprintf("generated-dense-n%d", n), g, known})
	}
	return out
}

// BenchmarkStage3Formulaforge05ClosureFixed benchmarks the closure
// on fixed instances: every shipped sheet from a fixed one-third
// known set, and generated dense-mixed graphs at n = 28 and n = 80.
// The same instances were measured on the pre-change code; see the
// PR body for the before/after table.
func BenchmarkStage3Formulaforge05ClosureFixed(b *testing.B) {
	for _, tc := range ff05ClosureBenchGraphs() {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = tc.g.Closure(tc.known)
			}
		})
	}
}

// BenchmarkStage3Formulaforge05CoverageCurve benchmarks the greedy
// coverage curve on the two largest shipped sheets.
func BenchmarkStage3Formulaforge05CoverageCurve(b *testing.B) {
	for _, name := range []string{"phy131", "ams261"} {
		s, err := models.LoadSheetByName(name)
		if err != nil {
			b.Fatal(err)
		}
		g := New(s)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = g.CoverageCurve()
			}
		})
	}
}

// BenchmarkStage3Formulaforge05MinMemorizeFor benchmarks the exact
// solver on fixed targets: the largest shipped sheet and a
// generated bottleneck graph, both exercising the branch search's
// memoized closures.
func BenchmarkStage3Formulaforge05MinMemorizeFor(b *testing.B) {
	s, err := models.LoadSheetByName("phy131")
	if err != nil {
		b.Fatal(err)
	}
	g := New(s)
	targets := []string{g.Nodes[0], g.Nodes[len(g.Nodes)/2], g.Nodes[len(g.Nodes)-1]}
	b.Run("phy131", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := g.MinMemorizeFor(targets, nil); err != nil {
				b.Fatal(err)
			}
		}
	})
	spec := fo6RandomSpec(11, 28, fo6Bottleneck)
	gg, idOf := fo6Graph(spec, 9028)
	gtargets := fo6IDs(idOf, []int{25, 26, 27})
	b.Run("generated-bottleneck-n28", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := gg.MinMemorizeFor(gtargets, nil); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkStage3Formulaforge05BoundedSearch benchmarks the bounded
// search (closure-heavy branch and bound) on a generated dense
// graph at a generous budget.
func BenchmarkStage3Formulaforge05BoundedSearch(b *testing.B) {
	spec := fo6RandomSpec(11, 28, fo6DenseMixed)
	g, idOf := fo6Graph(spec, 9028)
	targets := fo6IDs(idOf, []int{25, 26, 27})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := g.MinMemorizeForBounded(context.Background(), targets, nil, SearchBudget{MaxNodes: 500}); err != nil {
			b.Fatal(err)
		}
	}
}
