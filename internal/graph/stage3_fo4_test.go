// Stage 3 FO4 fixtures: bounded exact search with an honest
// incomplete-result contract.
//
// The unbounded solver (MinMemorizeFor) has no step, time, or node
// cap. This file pins the bounded variant (MinMemorizeForBounded):
// budget exhaustion returns a feasible incumbent with a lower and
// upper bound and is never reported exact; cancellation stops the
// search; a generous budget matches the unbounded solver and the
// independent exhaustive oracle; and the search is deterministic
// under relabeling. The bounded search reuses the FO6 relabeled-DAG
// generator and exhaustive oracle in stage3_fo6_test.go.
package graph

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// fo4BoundedSheet is a hand-built fixture with a known optimum.
//
// Roots: a, b, c, d. Targets t1 and t2 each derive through {c} or
// through a two-member path sharing a. The optimum is {c} (size 1);
// the all-roots upper bound is {a, b, c, d} (size 4).
func fo4BoundedSheet() *Graph {
	fs := []models.Formula{
		{ID: "a", Name: "a"},
		{ID: "b", Name: "b"},
		{ID: "c", Name: "c"},
		{ID: "d", Name: "d"},
		{ID: "t1", Name: "t1", DerivesFrom: []string{"c"}, AltDerivesFrom: [][]string{{"a", "b"}}},
		{ID: "t2", Name: "t2", DerivesFrom: []string{"c"}, AltDerivesFrom: [][]string{{"a", "d"}}},
	}
	return New(&models.Sheet{Course: "T", Title: "FO4", Formulas: fs})
}

// fo4Feasible reports whether memorizing set (plus prior) derives
// every target: the witness replay for a bounded result.
func fo4Feasible(g *Graph, set, targets, prior []string) bool {
	known := map[string]bool{}
	for _, p := range prior {
		known[p] = true
	}
	for _, m := range set {
		known[m] = true
	}
	closed := g.Closure(known)
	for _, t := range targets {
		if !closed[t] {
			return false
		}
	}
	return true
}

func TestStage3Formulaforge4(t *testing.T) {
	t.Run("BudgetExhaustionReturnsFeasibleIncumbentWithHonestBound", func(t *testing.T) {
		g := fo4BoundedSheet()
		targets := []string{"t1", "t2"}
		optimum, err := g.MinMemorizeFor(targets, nil)
		if err != nil {
			t.Fatalf("MinMemorizeFor: %v", err)
		}
		if len(optimum) != 1 || optimum[0] != "c" {
			t.Fatalf("unbounded optimum = %v, want [c]", optimum)
		}
		// A one-node budget lets the root call expand but cuts every
		// child: the incumbent stays the all-roots upper bound.
		res, err := g.MinMemorizeForBounded(context.Background(), targets, nil, SearchBudget{MaxNodes: 1})
		if err != nil {
			t.Fatalf("MinMemorizeForBounded: %v", err)
		}
		if res.Exact {
			t.Error("budget-exhausted result reports Exact, want false")
		}
		if !res.BudgetExhausted {
			t.Error("BudgetExhausted is false, want true")
		}
		if res.Status() != "budget-exhausted" {
			t.Errorf("Status() = %q, want %q", res.Status(), "budget-exhausted")
		}
		if !fo4Feasible(g, res.Set, targets, nil) {
			t.Errorf("incumbent %v does not derive targets, want a feasible set", res.Set)
		}
		if fmt.Sprint(res.Set) != "[a b c d]" {
			t.Errorf("incumbent = %v, want [a b c d]", res.Set)
		}
		if res.UpperBound != 4 {
			t.Errorf("UpperBound = %d, want 4", res.UpperBound)
		}
		// Lower bound: the cut children are (need {t1,t2,c}) with one
		// root residual c, (need {t1,t2,a,b}) with two root residuals
		// a and b, and (mem {t1}, need {t2}) with none; the minimum
		// is 1. It must sit at or below the optimum.
		if res.LowerBound != 1 {
			t.Errorf("LowerBound = %d, want 1", res.LowerBound)
		}
		if res.LowerBound > len(optimum) || len(optimum) > res.UpperBound {
			t.Errorf("bound %d <= optimum %d <= %d does not hold", res.LowerBound, len(optimum), res.UpperBound)
		}
		if res.Gap() != res.UpperBound-res.LowerBound || res.Gap() != 3 {
			t.Errorf("Gap() = %d, want 3", res.Gap())
		}
		if res.NodesVisited != 1 {
			t.Errorf("NodesVisited = %d, want 1", res.NodesVisited)
		}
	})

	t.Run("GenerousBudgetMatchesUnboundedOptimum", func(t *testing.T) {
		g := fo4BoundedSheet()
		targets := []string{"t1", "t2"}
		want, err := g.MinMemorizeFor(targets, nil)
		if err != nil {
			t.Fatalf("MinMemorizeFor: %v", err)
		}
		res, err := g.MinMemorizeForBounded(context.Background(), targets, nil, SearchBudget{MaxNodes: 10000, MaxDuration: 10 * time.Second})
		if err != nil {
			t.Fatalf("MinMemorizeForBounded: %v", err)
		}
		if !res.Exact {
			t.Error("generous budget did not finish exact")
		}
		if res.Status() != "optimal" {
			t.Errorf("Status() = %q, want %q", res.Status(), "optimal")
		}
		if fmt.Sprint(res.Set) != fmt.Sprint(want) {
			t.Errorf("bounded set = %v, unbounded = %v", res.Set, want)
		}
		if res.LowerBound != len(want) || res.UpperBound != len(want) || res.Gap() != 0 {
			t.Errorf("bounds = [%d, %d] gap %d, want [%d, %d] gap 0", res.LowerBound, res.UpperBound, res.Gap(), len(want), len(want))
		}
		if res.BudgetExhausted || res.Canceled {
			t.Error("generous run reports exhaustion or cancellation, want neither")
		}
	})

	t.Run("ContextCancellationStopsSearch", func(t *testing.T) {
		g := fo4BoundedSheet()
		targets := []string{"t1", "t2"}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		res, err := g.MinMemorizeForBounded(ctx, targets, nil, SearchBudget{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if !res.Canceled {
			t.Error("Canceled is false, want true")
		}
		if res.Exact {
			t.Error("canceled result reports Exact, want false")
		}
		if res.Status() != "canceled" {
			t.Errorf("Status() = %q, want %q", res.Status(), "canceled")
		}
		if !fo4Feasible(g, res.Set, targets, nil) {
			t.Errorf("canceled incumbent %v does not derive targets", res.Set)
		}
		if res.NodesVisited != 0 {
			t.Errorf("NodesVisited = %d, want 0 (search stopped before expanding)", res.NodesVisited)
		}
	})

	t.Run("TimeBudgetExhaustsHonestly", func(t *testing.T) {
		g := fo4BoundedSheet()
		targets := []string{"t1", "t2"}
		res, err := g.MinMemorizeForBounded(context.Background(), targets, nil, SearchBudget{MaxDuration: time.Nanosecond})
		if err != nil {
			t.Fatalf("MinMemorizeForBounded: %v", err)
		}
		if res.Exact {
			t.Error("time-exhausted result reports Exact, want false")
		}
		if !res.BudgetExhausted {
			t.Error("BudgetExhausted is false, want true")
		}
		if !fo4Feasible(g, res.Set, targets, nil) {
			t.Errorf("incumbent %v does not derive targets", res.Set)
		}
		if res.LowerBound > 1 || 1 > res.UpperBound {
			t.Errorf("bound %d <= optimum 1 <= %d does not hold", res.LowerBound, res.UpperBound)
		}
		if res.NodesVisited != 0 {
			t.Errorf("NodesVisited = %d, want 0 (time expired before any expansion)", res.NodesVisited)
		}
	})

	t.Run("MemoizedClosureBitsetsMatchClosure", func(t *testing.T) {
		g := fo4BoundedSheet()
		s := g.newBoundedSearch(context.Background(), SearchBudget{})
		knownSets := [][]string{
			{},
			{"c"},
			{"a"},
			{"a", "b"},
			{"t1"},
			{"a", "b", "c", "d"},
		}
		for _, ks := range knownSets {
			bits := s.bitsOf(ks)
			got := s.closureBits(bits)
			want := g.Closure(toSet(ks))
			for _, n := range g.Nodes {
				if got.has(s.idx[n]) != want[n] {
					t.Errorf("closureBits(%v)[%s] = %v, Closure = %v", ks, n, got.has(s.idx[n]), want[n])
				}
			}
		}
		runsBefore, hitsBefore := s.closureRuns, s.closureHits
		again := s.closureBits(s.bitsOf([]string{"c"}))
		if !again.has(s.idx["t1"]) || !again.has(s.idx["t2"]) {
			t.Error("repeated closureBits({c}) no longer derives t1 and t2")
		}
		if s.closureHits <= hitsBefore {
			t.Error("repeated closureBits call did not hit the memo")
		}
		if s.closureRuns != runsBefore {
			t.Error("repeated closureBits call recomputed instead of hitting the memo")
		}
	})

	t.Run("DeterministicTopologicalQueueOrdering", func(t *testing.T) {
		g := fo4BoundedSheet()
		targets := []string{"t1", "t2"}
		first, err := g.MinMemorizeForBounded(context.Background(), targets, nil, SearchBudget{MaxNodes: 1})
		if err != nil {
			t.Fatalf("first run: %v", err)
		}
		second, err := g.MinMemorizeForBounded(context.Background(), targets, nil, SearchBudget{MaxNodes: 1})
		if err != nil {
			t.Fatalf("second run: %v", err)
		}
		if fmt.Sprint(first.Set) != fmt.Sprint(second.Set) || first.LowerBound != second.LowerBound ||
			first.UpperBound != second.UpperBound || first.NodesVisited != second.NodesVisited {
			t.Errorf("same budget, different results: %+v vs %+v", first, second)
		}
		// TopoOrder itself is unchanged by this item (FO7 pin): roots
		// a and b, with aa deriving from a, order a, b, aa.
		fo7 := New(&models.Sheet{Course: "T", Title: "FO7", Formulas: []models.Formula{
			{ID: "a", Name: "a"},
			{ID: "b", Name: "b"},
			{ID: "aa", Name: "aa", DerivesFrom: []string{"a"}},
		}})
		order, err := fo7.TopoOrder()
		if err != nil {
			t.Fatalf("TopoOrder: %v", err)
		}
		if fmt.Sprint(order) != "[a b aa]" {
			t.Errorf("TopoOrder = %v, want [a b aa]", order)
		}
		// Relabeled graphs keep the same optimum size and valid
		// bounds: identifier order must not change the answer.
		spec := fo6RandomSpec(7, 12, fo6Bottleneck)
		g1, idOf1 := fo6Graph(spec, 101)
		g2, idOf2 := fo6Graph(spec, 202)
		tg1 := fo6IDs(idOf1, []int{9, 10, 11})
		tg2 := fo6IDs(idOf2, []int{9, 10, 11})
		opt1 := fo6ExhaustiveMinSize(g1, tg1, nil)
		opt2 := fo6ExhaustiveMinSize(g2, tg2, nil)
		if opt1 != opt2 {
			t.Fatalf("relabeling changed the optimum: %d vs %d", opt1, opt2)
		}
		for i, tc := range []struct {
			g  *Graph
			tg []string
		}{{g1, tg1}, {g2, tg2}} {
			res, err := tc.g.MinMemorizeForBounded(context.Background(), tc.tg, nil, SearchBudget{MaxNodes: 30})
			if err != nil {
				t.Fatalf("labeling %d: %v", i, err)
			}
			if !fo4Feasible(tc.g, res.Set, tc.tg, nil) {
				t.Errorf("labeling %d: incumbent %v does not derive targets", i, res.Set)
			}
			if res.LowerBound > opt1 || opt1 > res.UpperBound {
				t.Errorf("labeling %d: bound %d <= optimum %d <= %d does not hold", i, res.LowerBound, opt1, res.UpperBound)
			}
			if res.Exact && len(res.Set) != opt1 {
				t.Errorf("labeling %d: exact set size %d, optimum %d", i, len(res.Set), opt1)
			}
		}
	})

	t.Run("GeneratedDAGMatrixBoundsHold", func(t *testing.T) {
		families := []fo6Family{fo6Sparse, fo6AndHeavy, fo6OrHeavy, fo6Bottleneck, fo6DenseMixed}
		exactRuns, exhaustedRuns := 0, 0
		for _, family := range families {
			for seed := int64(0); seed < 3; seed++ {
				spec := fo6RandomSpec(seed, 12, family)
				g, idOf := fo6Graph(spec, 1000+seed)
				targets := fo6IDs(idOf, []int{9, 10, 11})
				oracle := fo6ExhaustiveMinSize(g, targets, nil)
				bounded, err := g.MinMemorizeForBounded(context.Background(), targets, nil, SearchBudget{MaxNodes: 8})
				if err != nil {
					t.Fatalf("%s seed %d bounded: %v", family, seed, err)
				}
				if bounded.Exact {
					exactRuns++
				} else {
					exhaustedRuns++
				}
				if !fo4Feasible(g, bounded.Set, targets, nil) {
					t.Errorf("%s seed %d: incumbent %v does not derive targets", family, seed, bounded.Set)
				}
				if bounded.LowerBound > oracle || oracle > bounded.UpperBound {
					t.Errorf("%s seed %d: bound %d <= oracle %d <= %d does not hold", family, seed, bounded.LowerBound, oracle, bounded.UpperBound)
				}
				if bounded.Exact && len(bounded.Set) != oracle {
					t.Errorf("%s seed %d: exact size %d, oracle %d", family, seed, len(bounded.Set), oracle)
				}
				generous, err := g.MinMemorizeForBounded(context.Background(), targets, nil, SearchBudget{})
				if err != nil {
					t.Fatalf("%s seed %d generous: %v", family, seed, err)
				}
				if !generous.Exact || len(generous.Set) != oracle {
					t.Errorf("%s seed %d: generous run set %v exact=%v, oracle %d", family, seed, generous.Set, generous.Exact, oracle)
				}
				unbounded, err := g.MinMemorizeFor(targets, nil)
				if err != nil {
					t.Fatalf("%s seed %d unbounded: %v", family, seed, err)
				}
				if len(unbounded) != oracle {
					t.Errorf("%s seed %d: unbounded size %d, oracle %d", family, seed, len(unbounded), oracle)
				}
				// Identifier order must not leak into the optimum:
				// the bounded set is sorted by id by construction.
				if !slices.IsSorted(bounded.Set) {
					t.Errorf("%s seed %d: bounded set %v is not sorted", family, seed, bounded.Set)
				}
			}
		}
		t.Logf("matrix: %d bounded runs finished exact, %d exhausted (15 graphs)", exactRuns, exhaustedRuns)
	})
}

// BenchmarkStage3Formulaforge4BoundedSearch benchmarks the bounded
// search on relabeled DAGs by size, density, and alternative count:
// one seeded graph per family at n = 12, 20, and 28, targets the
// last three nodes, at a generous 500-node budget (the search
// finishes exact) and a tight 5-node budget (the bound and gap are
// exercised). The reported nodes/op is the branch-and-bound node
// count at that budget, and gap/op is the honest bound gap (0 when
// the search finishes exact).
func BenchmarkStage3Formulaforge4BoundedSearch(b *testing.B) {
	families := []fo6Family{fo6Sparse, fo6AndHeavy, fo6OrHeavy, fo6Bottleneck, fo6DenseMixed}
	for _, budget := range []int{500, 5} {
		for _, n := range []int{12, 20, 28} {
			for _, family := range families {
				spec := fo6RandomSpec(11, n, family)
				g, idOf := fo6Graph(spec, 5000+int64(n))
				targets := fo6IDs(idOf, []int{n - 3, n - 2, n - 1})
				b.Run(fmt.Sprintf("b%d/n%d/%s", budget, n, family), func(b *testing.B) {
					var res BoundedSearchResult
					var err error
					for i := 0; i < b.N; i++ {
						res, err = g.MinMemorizeForBounded(context.Background(), targets, nil, SearchBudget{MaxNodes: budget})
						if err != nil {
							b.Fatalf("MinMemorizeForBounded: %v", err)
						}
					}
					b.ReportMetric(float64(res.NodesVisited), "nodes/op")
					b.ReportMetric(float64(res.Gap()), "gap/op")
					b.ReportMetric(float64(res.UpperBound), "upper/op")
					b.ReportMetric(float64(res.LowerBound), "lower/op")
				})
			}
		}
	}
}
