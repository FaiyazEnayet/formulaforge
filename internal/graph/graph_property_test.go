package graph

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// FF-04 property harness: permutation, generalization and
// differential testing for the exact target solver.
//
// The gaps this harness closes, relative to the earlier coverage:
//
//   - TestMinMemorizeForPermutationRobust relabeled one hand-built
//     graph and asserted only the optimum size. A solver returning a
//     wrong witness of the right size, or one that only works for
//     particular ID labels, passed. Here every relabeled and shuffled
//     result is checked against an independent oracle: it must derive
//     all targets and its size must equal the independently computed
//     optimum.
//   - bruteMinFor checked candidate sets with the production
//     Graph.Closure, so a defect in the closure itself was invisible
//     to the oracle. Every check here runs through referenceClosure
//     and referenceBruteMinFor, implemented separately from the
//     production code (see reference_closure.go).
//
// All randomness is seeded and every failure message carries the
// seed that reproduces it, so a failure is a replayable case, not a
// flake. No test tunes to particular IDs: generators, relabelings
// and shuffles are all drawn from the seed.

const ff04BaseSeed = 20261213

// ff04GenConfig describes one family of generated DAGs.
type ff04GenConfig struct {
	seed       int64
	n          int
	primaryP   float64 // probability an earlier node joins the primary path
	altP       float64 // probability a node gains an alternative path
	altDensity float64 // probability an earlier node joins the alt path
	components int     // node partition; edges never cross components
	negIDs     bool    // ids are plain integers, including negatives
}

// ff04RandomDAG builds a small DAG with the requested AND/OR density
// and component structure. Node i may only derive from earlier nodes
// in its own component, so the graph is acyclic by construction and
// components stay disconnected.
func ff04RandomDAG(cfg ff04GenConfig) *Graph {
	rng := rand.New(rand.NewSource(cfg.seed))
	comp := make([]int, cfg.n)
	for i := range comp {
		if cfg.components > 1 {
			comp[i] = rng.Intn(cfg.components)
		}
	}
	idOf := func(i int) string {
		if cfg.negIDs {
			return fmt.Sprintf("%d", i-5)
		}
		return fmt.Sprintf("f%02d", i)
	}
	var fs []models.Formula
	for i := 0; i < cfg.n; i++ {
		f := models.Formula{ID: idOf(i), Name: idOf(i)}
		var prim []string
		for j := 0; j < i; j++ {
			if comp[j] != comp[i] {
				continue
			}
			if rng.Float64() < cfg.primaryP {
				prim = append(prim, idOf(j))
			}
		}
		f.DerivesFrom = prim
		if i > 1 && rng.Float64() < cfg.altP {
			var alt []string
			for j := 0; j < i; j++ {
				if comp[j] != comp[i] {
					continue
				}
				if rng.Float64() < cfg.altDensity {
					alt = append(alt, idOf(j))
				}
			}
			if len(alt) > 0 && fmt.Sprint(alt) != fmt.Sprint(prim) {
				f.AltDerivesFrom = [][]string{alt}
			}
		}
		fs = append(fs, f)
	}
	return New(&models.Sheet{Course: "T", Title: "FF04", Formulas: fs})
}

// ff04PickTargetsPrior draws a random target list (non-empty) and a
// random prior-knowledge list from the node ids.
func ff04PickTargetsPrior(rng *rand.Rand, ids []string) (targets, prior []string) {
	for _, id := range ids {
		if rng.Intn(2) == 0 {
			targets = append(targets, id)
		}
	}
	if len(targets) == 0 {
		targets = []string{ids[rng.Intn(len(ids))]}
	}
	for _, id := range ids {
		if rng.Intn(4) == 0 {
			prior = append(prior, id)
		}
	}
	return targets, prior
}

// ff04Relabel returns the graph with every id replaced by a fresh
// name under a seeded random permutation, plus the old-to-new map.
// Structure is untouched: only labels change.
func ff04Relabel(g *Graph, rng *rand.Rand) (*Graph, map[string]string) {
	perm := rng.Perm(len(g.Nodes))
	rename := make(map[string]string, len(g.Nodes))
	for i, n := range g.Nodes {
		rename[n] = fmt.Sprintf("g%03d", perm[i])
	}
	var fs []models.Formula
	for _, n := range g.Nodes {
		f := models.Formula{ID: rename[n], Name: rename[n]}
		for pi, p := range g.Paths[n] {
			rp := make([]string, len(p))
			for k, m := range p {
				rp[k] = rename[m]
			}
			if pi == 0 {
				f.DerivesFrom = rp
			} else {
				f.AltDerivesFrom = append(f.AltDerivesFrom, rp)
			}
		}
		fs = append(fs, f)
	}
	return New(&models.Sheet{Course: "T", Title: "FF04-relabeled", Formulas: fs}), rename
}

// ff04RelabelSheet relabels a whole sheet the same way, renaming
// every formula id and every derivation reference.
func ff04RelabelSheet(s *models.Sheet, rng *rand.Rand) *models.Sheet {
	perm := rng.Perm(len(s.Formulas))
	rename := make(map[string]string, len(s.Formulas))
	for i, f := range s.Formulas {
		rename[f.ID] = fmt.Sprintf("s%03d", perm[i])
	}
	out := &models.Sheet{Course: s.Course, Title: s.Title + " relabeled"}
	for _, f := range s.Formulas {
		nf := f
		nf.ID = rename[f.ID]
		nf.Name = rename[f.ID]
		remap := func(ids []string) []string {
			if ids == nil {
				return nil
			}
			r := make([]string, len(ids))
			for i, id := range ids {
				r[i] = rename[id]
			}
			return r
		}
		nf.DerivesFrom = remap(f.DerivesFrom)
		nf.AltDerivesFrom = nil
		for _, alt := range f.AltDerivesFrom {
			nf.AltDerivesFrom = append(nf.AltDerivesFrom, remap(alt))
		}
		out.Formulas = append(out.Formulas, nf)
	}
	return out
}

// ff04Shuffle rebuilds the graph from the same formulas with the
// formula order and the order of members inside every derivation
// path shuffled. The graph builder sorts paths and prerequisites,
// so the rebuilt graph must behave identically.
func ff04Shuffle(g *Graph, rng *rand.Rand) *Graph {
	var fs []models.Formula
	for _, idx := range rng.Perm(len(g.Nodes)) {
		n := g.Nodes[idx]
		f := models.Formula{ID: n, Name: n}
		for k, pidx := range rng.Perm(len(g.Paths[n])) {
			p := append([]string{}, g.Paths[n][pidx]...)
			rng.Shuffle(len(p), func(a, b int) { p[a], p[b] = p[b], p[a] })
			if k == 0 {
				f.DerivesFrom = p
			} else {
				f.AltDerivesFrom = append(f.AltDerivesFrom, p)
			}
		}
		fs = append(fs, f)
	}
	return New(&models.Sheet{Course: "T", Title: "FF04-shuffled", Formulas: fs})
}

// ff04VerifyOptimum checks a memorize set against the independent
// reference oracle only: it must derive every target under the
// reference closure, and its size must equal the independently
// computed optimum. It never consults the production closure or
// solver, so it catches defects in either.
func ff04VerifyOptimum(g *Graph, targets, prior, got []string) error {
	ids := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		ids[n] = true
	}
	for _, m := range got {
		if !ids[m] {
			return fmt.Errorf("memorize set contains unknown id %q", m)
		}
	}
	known := toSet(prior)
	for _, m := range got {
		known[m] = true
	}
	closed := referenceClosure(g, known)
	for _, t := range targets {
		if !closed[t] {
			return fmt.Errorf("target %q not derivable from %v under the reference closure", t, got)
		}
	}
	want := referenceBruteMinFor(g, targets, prior)
	if want == nil {
		return fmt.Errorf("reference oracle found no feasible set for targets %v", targets)
	}
	if len(got) != len(want) {
		return fmt.Errorf("size %d, reference optimum size %d (%v)", len(got), len(want), want)
	}
	return nil
}

// ff04Densities sweeps sparse to dense AND/OR mixes.
var ff04Densities = []struct {
	p, alt, altD float64
}{
	{0.15, 0.10, 0.30},
	{0.33, 0.25, 0.25},
	{0.50, 0.40, 0.40},
	{0.70, 0.50, 0.50},
}

func TestStage3Formulaforge04ReferenceClosureAgrees(t *testing.T) {
	for ci, d := range ff04Densities {
		for comps := 1; comps <= 3; comps++ {
			for trial := 0; trial < 10; trial++ {
				seed := int64(ff04BaseSeed + ci*1000 + comps*100 + trial)
				g := ff04RandomDAG(ff04GenConfig{seed: seed, n: 12, primaryP: d.p, altP: d.alt, altDensity: d.altD, components: comps})
				rng := rand.New(rand.NewSource(seed + 7))
				for k := 0; k < 5; k++ {
					known := map[string]bool{}
					for _, id := range g.Nodes {
						if r := rng.Intn(3); r == 0 {
							known[id] = true
						} else if r == 1 {
							known[id] = false
						}
					}
					known["no-such-id"] = true
					prod := sortedKeys(g.Closure(known))
					ref := sortedKeys(referenceClosure(g, known))
					if fmt.Sprint(prod) != fmt.Sprint(ref) {
						t.Errorf("seed %d density %d comps %d known-set %d: production closure %v, reference closure %v", seed, ci, comps, k, prod, ref)
					}
				}
			}
		}
	}
}

func TestStage3Formulaforge04MetamorphicRelabeling(t *testing.T) {
	for ci, d := range ff04Densities {
		for comps := 1; comps <= 3; comps++ {
			for trial := 0; trial < 8; trial++ {
				seed := int64(ff04BaseSeed + 5000 + ci*1000 + comps*100 + trial)
				g := ff04RandomDAG(ff04GenConfig{seed: seed, n: 10, primaryP: d.p, altP: d.alt, altDensity: d.altD, components: comps})
				rng := rand.New(rand.NewSource(seed + 11))
				targets, prior := ff04PickTargetsPrior(rng, g.Nodes)
				got, err := g.MinMemorizeFor(targets, prior)
				if err != nil {
					t.Fatalf("seed %d: solver error: %v", seed, err)
				}
				again, err := g.MinMemorizeFor(targets, prior)
				if err != nil {
					t.Fatalf("seed %d: solver error on repeat: %v", seed, err)
				}
				if fmt.Sprint(got) != fmt.Sprint(again) {
					t.Errorf("seed %d: solver not deterministic: %v vs %v", seed, got, again)
				}
				if err := ff04VerifyOptimum(g, targets, prior, got); err != nil {
					t.Errorf("seed %d targets=%v prior=%v: original result fails independent verification: %v", seed, targets, prior, err)
				}
				rg, rename := ff04Relabel(g, rng)
				rtargets := make([]string, len(targets))
				for i, tg := range targets {
					rtargets[i] = rename[tg]
				}
				rprior := make([]string, len(prior))
				for i, p := range prior {
					rprior[i] = rename[p]
				}
				rgot, err := rg.MinMemorizeFor(rtargets, rprior)
				if err != nil {
					t.Fatalf("seed %d: relabeled solver error: %v", seed, err)
				}
				if len(rgot) != len(got) {
					t.Errorf("seed %d targets=%v prior=%v: relabeling changed optimum size %d to %d (got %v, relabeled %v)", seed, targets, prior, len(got), len(rgot), got, rgot)
				}
				// Map the relabeled witness back and verify it is an
				// optimum of the original graph under the reference
				// oracle. Tie-breaking is by id by design, so the
				// witness itself may differ across labelings; what
				// must not change is optimality.
				back := make([]string, len(rgot))
				rev := make(map[string]string, len(rename))
				for old, new := range rename {
					rev[new] = old
				}
				for i, id := range rgot {
					back[i] = rev[id]
				}
				if err := ff04VerifyOptimum(g, targets, prior, back); err != nil {
					t.Errorf("seed %d targets=%v prior=%v: relabeled witness mapped back %v fails independent verification: %v", seed, targets, prior, back, err)
				}
			}
		}
	}
}

func TestStage3Formulaforge04InputShuffles(t *testing.T) {
	for ci, d := range ff04Densities {
		for comps := 1; comps <= 3; comps++ {
			for trial := 0; trial < 8; trial++ {
				seed := int64(ff04BaseSeed + 9000 + ci*1000 + comps*100 + trial)
				g := ff04RandomDAG(ff04GenConfig{seed: seed, n: 10, primaryP: d.p, altP: d.alt, altDensity: d.altD, components: comps})
				rng := rand.New(rand.NewSource(seed + 13))
				targets, prior := ff04PickTargetsPrior(rng, g.Nodes)
				got, err := g.MinMemorizeFor(targets, prior)
				if err != nil {
					t.Fatalf("seed %d: solver error: %v", seed, err)
				}
				sg := ff04Shuffle(g, rng)
				sgot, err := sg.MinMemorizeFor(targets, prior)
				if err != nil {
					t.Fatalf("seed %d: shuffled solver error: %v", seed, err)
				}
				if len(sgot) != len(got) {
					t.Errorf("seed %d targets=%v prior=%v: input shuffle changed optimum size %d to %d (got %v, shuffled %v)", seed, targets, prior, len(got), len(sgot), got, sgot)
				}
				if err := ff04VerifyOptimum(sg, targets, prior, sgot); err != nil {
					t.Errorf("seed %d targets=%v prior=%v: shuffled result fails independent verification: %v", seed, targets, prior, err)
				}
				if a, b := g.MinMemorizeSet(prior), sg.MinMemorizeSet(prior); fmt.Sprint(a) != fmt.Sprint(b) {
					t.Errorf("seed %d prior=%v: input shuffle changed full-sheet set %v to %v", seed, prior, a, b)
				}
			}
		}
	}
}

func TestStage3Formulaforge04BruteForceExact(t *testing.T) {
	for _, n := range []int{6, 8, 10, 12} {
		for ci, d := range ff04Densities {
			for comps := 1; comps <= 3; comps++ {
				for trial := 0; trial < 6; trial++ {
					seed := int64(ff04BaseSeed + 20000 + n*1000 + ci*100 + comps*10 + trial)
					g := ff04RandomDAG(ff04GenConfig{seed: seed, n: n, primaryP: d.p, altP: d.alt, altDensity: d.altD, components: comps})
					rng := rand.New(rand.NewSource(seed + 17))
					targets, prior := ff04PickTargetsPrior(rng, g.Nodes)
					got, err := g.MinMemorizeFor(targets, prior)
					if err != nil {
						t.Fatalf("seed %d: solver error: %v", seed, err)
					}
					if err := ff04VerifyOptimum(g, targets, prior, got); err != nil {
						t.Errorf("seed %d n=%d targets=%v prior=%v: solver result %v fails independent verification: %v", seed, n, targets, prior, got, err)
					}
					// When the optimum is unique, the solver must
					// return exactly that set, not just its size.
					if optima := referenceAllOptima(g, targets, prior); len(optima) == 1 {
						if fmt.Sprint(got) != fmt.Sprint(optima[0]) {
							t.Errorf("seed %d n=%d targets=%v prior=%v: unique optimum is %v, solver returned %v", seed, n, targets, prior, optima[0], got)
						}
					}
					// Full-sheet exactness through the same oracle.
					ids := append([]string{}, g.Nodes...)
					sort.Strings(ids)
					full := g.MinMemorizeSet(prior)
					if err := ff04VerifyOptimum(g, ids, prior, full); err != nil {
						t.Errorf("seed %d n=%d prior=%v: full-sheet set %v fails independent verification: %v", seed, n, prior, full, err)
					}
				}
			}
		}
	}
}

// ff04Components derives the disconnected components of a graph with
// union-find over the union of derivation paths, independently of
// how the graph was generated.
func ff04Components(g *Graph) [][]string {
	parent := make(map[string]string, len(g.Nodes))
	for _, n := range g.Nodes {
		parent[n] = n
	}
	var find func(string) string
	find = func(x string) string {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	for _, n := range g.Nodes {
		for _, p := range g.Paths[n] {
			for _, m := range p {
				if _, ok := parent[m]; ok {
					union(n, m)
				}
			}
		}
	}
	groups := map[string][]string{}
	for _, n := range g.Nodes {
		r := find(n)
		groups[r] = append(groups[r], n)
	}
	var out [][]string
	for _, grp := range groups {
		sort.Strings(grp)
		out = append(out, grp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// ff04Induced returns the subgraph on exactly the given nodes,
// keeping only derivation paths whose members are all inside.
func ff04Induced(g *Graph, nodes []string) *Graph {
	inside := toSet(nodes)
	var fs []models.Formula
	for _, n := range nodes {
		f := models.Formula{ID: n, Name: n}
		for pi, p := range g.Paths[n] {
			keep := true
			for _, m := range p {
				if !inside[m] {
					keep = false
					break
				}
			}
			if !keep {
				continue
			}
			if pi == 0 {
				f.DerivesFrom = append([]string{}, p...)
			} else {
				f.AltDerivesFrom = append(f.AltDerivesFrom, append([]string{}, p...))
			}
		}
		fs = append(fs, f)
	}
	return New(&models.Sheet{Course: "T", Title: "FF04-induced", Formulas: fs})
}

func TestStage3Formulaforge04DisconnectedComponents(t *testing.T) {
	for trial := 0; trial < 20; trial++ {
		seed := int64(ff04BaseSeed + 40000 + trial)
		g := ff04RandomDAG(ff04GenConfig{seed: seed, n: 12, primaryP: 0.4, altP: 0.3, altDensity: 0.35, components: 3})
		comps := ff04Components(g)
		if len(comps) < 2 {
			t.Fatalf("seed %d: generator produced %d components, want at least 2", seed, len(comps))
		}
		rng := rand.New(rand.NewSource(seed + 19))
		targets, prior := ff04PickTargetsPrior(rng, g.Nodes)
		got, err := g.MinMemorizeFor(targets, prior)
		if err != nil {
			t.Fatalf("seed %d: solver error: %v", seed, err)
		}
		if err := ff04VerifyOptimum(g, targets, prior, got); err != nil {
			t.Errorf("seed %d targets=%v prior=%v: solver result %v fails independent verification: %v", seed, targets, prior, got, err)
		}
		// The whole-graph optimum is the sum of the per-component
		// optima: components share no derivation paths, so no
		// memorization can serve two components at once.
		sum := 0
		for _, c := range comps {
			sub := ff04Induced(g, c)
			ct, cp := []string{}, []string{}
			in := toSet(c)
			for _, tg := range targets {
				if in[tg] {
					ct = append(ct, tg)
				}
			}
			for _, p := range prior {
				if in[p] {
					cp = append(cp, p)
				}
			}
			opt := referenceBruteMinFor(sub, ct, cp)
			if opt == nil {
				t.Fatalf("seed %d: reference oracle found no feasible set for component %v", seed, c)
			}
			sum += len(opt)
		}
		if len(got) != sum {
			t.Errorf("seed %d targets=%v prior=%v: whole-graph optimum size %d, sum of per-component optima %d", seed, targets, prior, len(got), sum)
		}
	}
}

func TestStage3Formulaforge04Ties(t *testing.T) {
	// Hand-built tie graph: t1 derives from a or b, t2 derives from
	// c or d. Nine distinct optima of size 2: each target can be
	// covered by memorizing the target itself or either of its
	// single prerequisites, so 3 ways to cover t1 times 3 ways to
	// cover t2, and no smaller set works.
	g := &Graph{
		Nodes: []string{"a", "b", "c", "d", "t1", "t2"},
		Paths: map[string][][]string{
			"a": {}, "b": {}, "c": {}, "d": {},
			"t1": {{"a"}, {"b"}},
			"t2": {{"c"}, {"d"}},
		},
	}
	optima := referenceAllOptima(g, []string{"t1", "t2"}, nil)
	if len(optima) != 9 {
		t.Fatalf("tie fixture: want 9 optima, reference oracle found %d: %v", len(optima), optima)
	}
	got, err := g.MinMemorizeFor([]string{"t1", "t2"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range optima {
		if fmt.Sprint(got) == fmt.Sprint(o) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tie fixture: solver returned %v, which is not one of the 9 optima %v", got, optima)
	}
	// Generated sweep: whenever several optima exist, the solver
	// must return one of them. The sweep must actually hit ties,
	// or the check is vacuous.
	tied := 0
	for trial := 0; trial < 60; trial++ {
		seed := int64(ff04BaseSeed + 60000 + trial)
		gg := ff04RandomDAG(ff04GenConfig{seed: seed, n: 10, primaryP: 0.4, altP: 0.5, altDensity: 0.4, components: 1})
		rng := rand.New(rand.NewSource(seed + 23))
		targets, prior := ff04PickTargetsPrior(rng, gg.Nodes)
		optima := referenceAllOptima(gg, targets, prior)
		if len(optima) == 0 {
			t.Fatalf("seed %d: reference oracle found no optimum", seed)
		}
		if len(optima) > 1 {
			tied++
		}
		got, err := gg.MinMemorizeFor(targets, prior)
		if err != nil {
			t.Fatalf("seed %d: solver error: %v", seed, err)
		}
		found := false
		for _, o := range optima {
			if fmt.Sprint(got) == fmt.Sprint(o) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("seed %d targets=%v prior=%v: solver returned %v, not one of the %d optima %v", seed, targets, prior, got, len(optima), optima)
		}
	}
	if tied < 5 {
		t.Errorf("tie sweep hit ties on only %d of 60 graphs, want at least 5; the tie check would be vacuous", tied)
	}
}

func TestStage3Formulaforge04NegativeAndUnknownIDs(t *testing.T) {
	// Plain integer ids, including negatives. 0 derives from -2 and
	// -1; 1 derives from 0; 2 derives from 0 or 1. Optimum for
	// targets 1 and 2 is size 1: memorize 0 or memorize 1.
	g := &Graph{
		Nodes: []string{"-2", "-1", "0", "1", "2"},
		Paths: map[string][][]string{
			"-2": {}, "-1": {},
			"0": {{"-2", "-1"}},
			"1": {{"0"}},
			"2": {{"0"}, {"1"}},
		},
	}
	got, err := g.MinMemorizeFor([]string{"1", "2"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ff04VerifyOptimum(g, []string{"1", "2"}, nil, got); err != nil {
		t.Errorf("negative ids: solver result %v fails independent verification: %v", got, err)
	}
	optima := referenceAllOptima(g, []string{"1", "2"}, nil)
	if len(optima) != 2 {
		t.Errorf("negative ids: want 2 optima ({0} and {1}), reference oracle found %v", optima)
	}
	// Generated graphs with negative ids too.
	for trial := 0; trial < 10; trial++ {
		seed := int64(ff04BaseSeed + 70000 + trial)
		gg := ff04RandomDAG(ff04GenConfig{seed: seed, n: 10, primaryP: 0.4, altP: 0.3, altDensity: 0.35, components: 2, negIDs: true})
		rng := rand.New(rand.NewSource(seed + 29))
		targets, prior := ff04PickTargetsPrior(rng, gg.Nodes)
		got, err := gg.MinMemorizeFor(targets, prior)
		if err != nil {
			t.Fatalf("seed %d: solver error with negative ids: %v", seed, err)
		}
		if err := ff04VerifyOptimum(gg, targets, prior, got); err != nil {
			t.Errorf("seed %d targets=%v prior=%v: negative-id result %v fails independent verification: %v", seed, targets, prior, got, err)
		}
	}
	// Unknown ids: unknown targets and unknown prior ids are an
	// error for the solver; unknown prior ids are ignored by the
	// full-sheet solver; unknown and false ids never count as known
	// in either closure.
	if _, err := g.MinMemorizeFor([]string{"99"}, nil); err == nil {
		t.Error("unknown target id must be an error")
	}
	if _, err := g.MinMemorizeFor([]string{"1"}, []string{"99"}); err == nil {
		t.Error("unknown prior id must be an error")
	}
	full := g.MinMemorizeSet([]string{"99", "-2"})
	if fmt.Sprint(full) != fmt.Sprint([]string{"-1"}) {
		t.Errorf("MinMemorizeSet with unknown prior: got %v, want [-1]", full)
	}
	known := map[string]bool{"-2": true, "-1": false, "99": true}
	if a, b := g.Closure(known), referenceClosure(g, known); fmt.Sprint(sortedKeys(a)) != fmt.Sprint(sortedKeys(b)) {
		t.Errorf("closure with false/unknown ids: production %v, reference %v", sortedKeys(a), sortedKeys(b))
	}
	if got := referenceClosure(g, known); got["-1"] || got["99"] || !got["-2"] {
		t.Errorf("reference closure must ignore false and unknown ids, got %v", sortedKeys(got))
	}
}

// ff04Mutant is a deliberately corrupted solver: it returns a set
// that a correct solver must never return.
type ff04Mutant struct {
	name  string
	solve func(g *Graph, targets, prior []string) ([]string, error)
}

func TestStage3Formulaforge04CorruptedSolversFail(t *testing.T) {
	mutants := []ff04Mutant{
		{
			name: "union-semantics",
			solve: func(g *Graph, targets, prior []string) ([]string, error) {
				// Treats one prerequisite as enough for an AND
				// path: memorizes only the first member of the
				// first path of each uncovered target.
				out := map[string]bool{}
				known := g.Closure(toSet(prior))
				for _, tg := range targets {
					if known[tg] {
						continue
					}
					if len(g.Paths[tg]) > 0 && len(g.Paths[tg][0]) > 0 {
						out[g.Paths[tg][0][0]] = true
					} else {
						out[tg] = true
					}
				}
				return sortedKeys(out), nil
			},
		},
		{
			name: "superset",
			solve: func(g *Graph, targets, prior []string) ([]string, error) {
				got, err := g.MinMemorizeFor(targets, prior)
				if err != nil {
					return nil, err
				}
				for _, n := range g.Nodes {
					if len(g.Paths[n]) == 0 {
						in := false
						for _, m := range got {
							if m == n {
								in = true
								break
							}
						}
						if !in {
							return append(append([]string{}, got...), n), nil
						}
					}
				}
				return got, nil
			},
		},
		{
			name: "drop-one",
			solve: func(g *Graph, targets, prior []string) ([]string, error) {
				got, err := g.MinMemorizeFor(targets, prior)
				if err != nil {
					return nil, err
				}
				if len(got) == 0 {
					return got, nil
				}
				return got[1:], nil
			},
		},
		{
			name: "empty",
			solve: func(g *Graph, targets, prior []string) ([]string, error) {
				return nil, nil
			},
		},
	}
	for _, m := range mutants {
		caught := 0
		checked := 0
		for trial := 0; trial < 40; trial++ {
			seed := int64(ff04BaseSeed + 80000 + trial)
			g := ff04RandomDAG(ff04GenConfig{seed: seed, n: 10, primaryP: 0.45, altP: 0.35, altDensity: 0.4, components: 1})
			rng := rand.New(rand.NewSource(seed + 31))
			targets, prior := ff04PickTargetsPrior(rng, g.Nodes)
			mut, err := m.solve(g, targets, prior)
			if err != nil {
				t.Fatalf("seed %d mutant %s: solver error: %v", seed, m.name, err)
			}
			// Positive control on the same graph: the real solver
			// must pass the verifier, or the verifier itself is
			// broken rather than the mutant.
			real, err := g.MinMemorizeFor(targets, prior)
			if err != nil {
				t.Fatalf("seed %d: solver error: %v", seed, err)
			}
			if err := ff04VerifyOptimum(g, targets, prior, real); err != nil {
				t.Fatalf("seed %d: real solver fails verification on the mutant control graph: %v", seed, err)
			}
			if err := ff04VerifyOptimum(g, targets, prior, mut); err == nil {
				// Mutant accidentally produced a valid optimum
				// on this graph; not a catch, not a failure.
				continue
			}
			checked++
			caught++
		}
		if caught < 5 {
			t.Errorf("mutant %s was caught on only %d of 40 graphs (genuinely wrong on %d), want at least 5; the mutation check would be vacuous", m.name, caught, checked)
		}
	}
}

func TestStage3Formulaforge04RealSheets(t *testing.T) {
	for _, name := range models.ListSheets() {
		g := loadGraph(t, name)
		rng := rand.New(rand.NewSource(ff04BaseSeed))
		knownSets := []map[string]bool{
			{},
			toSet(g.MinMemorizeSet(nil)),
		}
		for k := 0; k < 3; k++ {
			known := map[string]bool{}
			for _, id := range g.Nodes {
				if rng.Intn(3) == 0 {
					known[id] = true
				}
			}
			knownSets = append(knownSets, known)
		}
		for i, known := range knownSets {
			if a, b := g.Closure(known), referenceClosure(g, known); fmt.Sprint(sortedKeys(a)) != fmt.Sprint(sortedKeys(b)) {
				t.Errorf("sheet %s known-set %d: production closure disagrees with reference closure", name, i)
			}
		}
		// Full-sheet set: reference closure of prior plus the set
		// must cover the sheet, and its size must equal the number
		// of roots the prior does not already derive.
		set := g.MinMemorizeSet(nil)
		closed := referenceClosure(g, toSet(set))
		if len(closed) != len(g.Nodes) {
			t.Errorf("sheet %s: reference closure of MinMemorizeSet covers %d of %d nodes", name, len(closed), len(g.Nodes))
		}
		roots := 0
		for _, n := range g.Nodes {
			if len(g.Paths[n]) == 0 {
				roots++
			}
		}
		if len(set) != roots {
			t.Errorf("sheet %s: MinMemorizeSet size %d, want %d unknown roots", name, len(set), roots)
		}
	}
	// Exactness spot checks on real sheets: the returned set must
	// derive the targets under the reference closure, be
	// deterministic, and be no larger than the full-sheet set.
	// (Cardinality optimality on real sheets is covered by the
	// bounded-graph brute-force comparison above.)
	for _, name := range []string{"phy131", "ams261"} {
		g := loadGraph(t, name)
		targets := []string{g.Nodes[0], g.Nodes[len(g.Nodes)/2], g.Nodes[len(g.Nodes)-1]}
		ms, err := g.MinMemorizeFor(targets, nil)
		if err != nil {
			t.Fatal(err)
		}
		known := toSet(ms)
		closed := referenceClosure(g, known)
		for _, tg := range targets {
			if !closed[tg] {
				t.Errorf("sheet %s: target %s not derivable from %v under the reference closure", name, tg, ms)
			}
		}
		again, _ := g.MinMemorizeFor(targets, nil)
		if fmt.Sprint(ms) != fmt.Sprint(again) {
			t.Errorf("sheet %s: MinMemorizeFor not deterministic", name)
		}
		if len(ms) > len(g.MinMemorizeSet(nil)) {
			t.Errorf("sheet %s: target set %v larger than full-sheet set", name, ms)
		}
	}
	// Metamorphic relabeling on a real sheet: renaming every id
	// must not change the optimum size, and the mapped-back witness
	// must still derive the targets under the reference closure.
	s, err := models.LoadSheetByName("phy131")
	if err != nil {
		t.Fatal(err)
	}
	g := New(s)
	targets := []string{"projectile", "newton2", "work-energy"}
	got, err := g.MinMemorizeFor(targets, nil)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(ff04BaseSeed + 99))
	rs := ff04RelabelSheet(s, rng)
	rg := New(rs)
	rename := map[string]string{}
	for i, f := range s.Formulas {
		rename[f.ID] = rs.Formulas[i].ID
	}
	rtargets := []string{rename["projectile"], rename["newton2"], rename["work-energy"]}
	rgot, err := rg.MinMemorizeFor(rtargets, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rgot) != len(got) {
		t.Errorf("phy131 relabeling changed optimum size %d to %d (got %v, relabeled %v)", len(got), len(rgot), got, rgot)
	}
	rev := map[string]string{}
	for old, new := range rename {
		rev[new] = old
	}
	back := make([]string, len(rgot))
	for i, id := range rgot {
		back[i] = rev[id]
	}
	// Brute force over a 100-plus node sheet is infeasible, so verify
	// derivation only on the mapped-back witness.
	known := toSet(back)
	closed := referenceClosure(g, known)
	for _, tg := range targets {
		if !closed[tg] {
			t.Errorf("phy131 relabeled witness mapped back %v does not derive target %s under the reference closure", back, tg)
		}
	}
}
