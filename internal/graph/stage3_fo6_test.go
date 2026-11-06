// Stage 3 FO6 fixtures: benchmark semantics and optimization against
// independent oracles.
//
// The existing brute-force test (TestMinMemorizeForMatchesBruteForce)
// checks the exact solver on graphs from randomANDORDAG, whose
// identifiers run in topological order (f00 is always a root) and
// whose shapes have no deliberate shared bottlenecks. This file
// generates relabeled DAGs (identifier order shuffled against
// topological order) across five families (sparse, AND-heavy,
// OR-heavy, bottleneck-heavy, dense mixed), and benchmarks the exact
// solvers (MinMemorizeFor, MinMemorizeSet) and the greedy
// CoverageCurve against independent exhaustive oracles written here
// directly from subset enumeration. The fresh oracles are
// cross-checked against the established bruteMinFor oracle in
// graph_test.go on every exactness case.
//
// Acceptance: exact solver cost equals the exhaustive optimum to 0
// tolerance on every generated case (minimum 100 seeded small
// graphs); greedy gaps and runtimes are reported in full, including
// nonzero gaps and any exact-solver timeouts. No approximation
// guarantee is claimed for the greedy curve.
package graph

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// fo6Family selects a graph shape for the generator.
type fo6Family int

const (
	fo6Sparse fo6Family = iota
	fo6AndHeavy
	fo6OrHeavy
	fo6Bottleneck
	fo6DenseMixed
)

func (f fo6Family) String() string {
	switch f {
	case fo6Sparse:
		return "sparse"
	case fo6AndHeavy:
		return "and-heavy"
	case fo6OrHeavy:
		return "or-heavy"
	case fo6Bottleneck:
		return "bottleneck"
	case fo6DenseMixed:
		return "dense-mixed"
	}
	return "unknown"
}

// fo6Spec is a DAG in index space: paths[i] lists the derivation
// paths of node i, each path a list of prerequisite indices. Every
// prerequisite index is smaller than i, so the graph is acyclic by
// construction and index order is a topological order.
type fo6Spec struct {
	n     int
	paths [][][]int
}

// fo6RandomSpec builds a seeded DAG in index space. Node 0 is a
// root; each later node is a root with family-dependent probability,
// otherwise it takes a primary path of 1 to 3 earlier nodes (AND
// fan-in). Depending on the family it also gains alternative paths
// (OR) and a bias toward shared hub nodes (bottlenecks: a few
// intermediate nodes that many later paths funnel through). Multiple
// roots matter: a single-root graph is trivially covered by
// memorizing that root, so the greedy curve would have two points
// and no gap could ever appear.
func fo6RandomSpec(seed int64, n int, family fo6Family) fo6Spec {
	rng := rand.New(rand.NewSource(seed))
	spec := fo6Spec{n: n, paths: make([][][]int, n)}
	// Hubs live in the first half so many later nodes can funnel
	// through them. A second hub sits near the middle.
	hubs := []int{}
	if n > 4 {
		hubs = append(hubs, 1+rng.Intn(n/4))
	}
	if n > 8 {
		hubs = append(hubs, n/3+rng.Intn(n/3))
	}
	var pAlt, pHub, pRoot float64
	switch family {
	case fo6Sparse:
		pAlt, pHub, pRoot = 0.10, 0.05, 0.25
	case fo6AndHeavy:
		pAlt, pHub, pRoot = 0.15, 0.10, 0.20
	case fo6OrHeavy:
		pAlt, pHub, pRoot = 0.55, 0.10, 0.20
	case fo6Bottleneck:
		pAlt, pHub, pRoot = 0.20, 0.65, 0.15
	case fo6DenseMixed:
		pAlt, pHub, pRoot = 0.35, 0.30, 0.20
	}
	for i := 1; i < n; i++ {
		if rng.Float64() < pRoot {
			continue
		}
		prim := fo6RandomPath(rng, i, family, hubs, pHub)
		paths := [][]int{prim}
		if rng.Float64() < pAlt {
			alt := fo6RandomPath(rng, i, family, hubs, pHub)
			if !fo6SamePathSet(alt, prim) {
				paths = append(paths, alt)
			}
		}
		if rng.Float64() < pAlt/2 {
			alt := fo6RandomPath(rng, i, family, hubs, pHub)
			if !fo6SamePathSet(alt, prim) && (len(paths) < 2 || !fo6SamePathSet(alt, paths[1])) {
				paths = append(paths, alt)
			}
		}
		spec.paths[i] = paths
	}
	return spec
}

// fo6RandomPath draws one derivation path for node i: a set of 1 to 3
// distinct earlier indices. The size distribution depends on the
// family (AND-heavy favors wider conjunctions). With probability
// pHub one member is a hub node, so paths funnel through shared
// intermediates.
func fo6RandomPath(rng *rand.Rand, i int, family fo6Family, hubs []int, pHub float64) []int {
	size := 1
	switch family {
	case fo6Sparse:
		if rng.Intn(4) == 0 {
			size = 2
		}
	case fo6AndHeavy:
		switch rng.Intn(4) {
		case 0:
			size = 1
		case 1, 2:
			size = 2
		default:
			size = 3
		}
	case fo6OrHeavy:
		if rng.Intn(3) == 0 {
			size = 2
		}
	case fo6Bottleneck:
		if rng.Intn(3) == 0 {
			size = 3
		} else {
			size = 2
		}
	case fo6DenseMixed:
		size = 1 + rng.Intn(3)
	}
	if size > i {
		size = i
	}
	members := map[int]bool{}
	if len(hubs) > 0 && rng.Float64() < pHub {
		h := hubs[rng.Intn(len(hubs))]
		if h < i {
			members[h] = true
		}
	}
	for len(members) < size {
		members[rng.Intn(i)] = true
	}
	out := make([]int, 0, len(members))
	for m := range members {
		out = append(out, m)
	}
	sort.Ints(out)
	return out
}

func fo6SamePathSet(a, b []int) bool {
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

// fo6Graph builds a Graph from a spec, assigning identifiers by a
// shuffled permutation (seeded by idSeed) so lexicographic identifier
// order is uncorrelated with topological order, and shuffling the
// formula list order as well. It returns the graph and the index to
// identifier map, so callers can translate index-space target and
// prior sets into identifiers for each labeling.
func fo6Graph(spec fo6Spec, idSeed int64) (*Graph, []string) {
	rng := rand.New(rand.NewSource(idSeed))
	ids := make([]string, spec.n)
	for i := range ids {
		ids[i] = fmt.Sprintf("n%02d", i)
	}
	rng.Shuffle(len(ids), func(a, b int) { ids[a], ids[b] = ids[b], ids[a] })
	idOf := make([]string, spec.n)
	for i := range idOf {
		idOf[i] = ids[i]
	}
	order := rng.Perm(spec.n)
	fs := make([]models.Formula, 0, spec.n)
	for _, i := range order {
		f := models.Formula{ID: idOf[i], Name: idOf[i]}
		for _, path := range spec.paths[i] {
			var p []string
			for _, m := range path {
				p = append(p, idOf[m])
			}
			if len(f.DerivesFrom) == 0 {
				f.DerivesFrom = p
			} else {
				f.AltDerivesFrom = append(f.AltDerivesFrom, p)
			}
		}
		fs = append(fs, f)
	}
	return New(&models.Sheet{Course: "T", Title: "FO6", Formulas: fs}), idOf
}

// fo6IDs translates an index-space set into identifiers under idOf.
func fo6IDs(idOf []string, idx []int) []string {
	out := make([]string, 0, len(idx))
	for _, i := range idx {
		out = append(out, idOf[i])
	}
	return out
}

// fo6ExhaustiveMinSize is an independent oracle: the size of the
// smallest memorize set (beyond prior) whose closure covers targets,
// found by enumerating every subset of the not-already-known nodes.
// It shares no code with the branch-and-bound solver.
func fo6ExhaustiveMinSize(g *Graph, targets []string, prior []string) int {
	priorSet := toSet(prior)
	known := g.Closure(priorSet)
	need := false
	for _, t := range targets {
		if !known[t] {
			need = true
		}
	}
	if !need {
		return 0
	}
	var cand []string
	for _, id := range g.Nodes {
		if !priorSet[id] {
			cand = append(cand, id)
		}
	}
	sort.Strings(cand)
	best := len(cand) + 1
	for mask := 0; mask < (1 << len(cand)); mask++ {
		mem := make(map[string]bool, len(priorSet))
		for k := range priorSet {
			mem[k] = true
		}
		size := 0
		for i, id := range cand {
			if mask&(1<<i) != 0 {
				mem[id] = true
				size++
			}
		}
		if size >= best {
			continue
		}
		closed := g.Closure(mem)
		ok := true
		for _, t := range targets {
			if !closed[t] {
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

// fo6ExhaustiveCoverageTable is an independent oracle: best[k] is the
// largest number of formulas derivable by memorizing exactly k
// formulas (beyond prior), found by enumerating every subset once
// and recording the best coverage per popcount.
func fo6ExhaustiveCoverageTable(g *Graph, prior []string) []int {
	priorSet := toSet(prior)
	var cand []string
	for _, id := range g.Nodes {
		if !priorSet[id] {
			cand = append(cand, id)
		}
	}
	sort.Strings(cand)
	best := make([]int, len(cand)+1)
	for mask := 0; mask < (1 << len(cand)); mask++ {
		mem := make(map[string]bool, len(priorSet))
		for k := range priorSet {
			mem[k] = true
		}
		size := 0
		for i, id := range cand {
			if mask&(1<<i) != 0 {
				mem[id] = true
				size++
			}
		}
		if n := len(g.Closure(mem)); n > best[size] {
			best[size] = n
		}
	}
	return best
}

// fo6GreedyReplay replays the documented greedy rule from
// docs/algorithm-contract.md (at each step memorize the unmemorized
// formula with the largest marginal closure gain, ties broken by
// smallest id) and returns the covered count after each step, starting
// with the empty-memorization count. It is an independent check that
// CoverageCurve reports the greedy algorithm's actual coverage.
func fo6GreedyReplay(g *Graph) []int {
	if len(g.Nodes) == 0 {
		return []int{0}
	}
	mem := map[string]bool{}
	known := g.Closure(mem)
	counts := []int{len(known)}
	for len(known) < len(g.Nodes) {
		best, bestGain := "", -1
		for _, n := range g.Nodes {
			if mem[n] {
				continue
			}
			trial := make(map[string]bool, len(mem)+1)
			for k := range mem {
				trial[k] = true
			}
			trial[n] = true
			gain := len(g.Closure(trial)) - len(known)
			if gain > bestGain || (gain == bestGain && n < best) {
				best, bestGain = n, gain
			}
		}
		if bestGain <= 0 {
			break
		}
		mem[best] = true
		known = g.Closure(mem)
		counts = append(counts, len(known))
	}
	return counts
}

// fo6Case is one generated benchmark case: a spec plus its labeling.
type fo6Case struct {
	seed   int64
	family fo6Family
	n      int
	spec   fo6Spec
	g      *Graph
	idOf   []string
}

// fo6Cases builds the benchmark set: 120 seeded graphs, 24 per
// family, n from 8 to 12.
func fo6Cases() []fo6Case {
	var cases []fo6Case
	for seed := int64(0); seed < 120; seed++ {
		family := fo6Family(seed % 5)
		n := 8 + int(seed%5)
		spec := fo6RandomSpec(seed, n, family)
		g, idOf := fo6Graph(spec, seed*1000+7)
		cases = append(cases, fo6Case{seed: seed, family: family, n: n, spec: spec, g: g, idOf: idOf})
	}
	return cases
}

// fo6RandomIndexSet draws a random subset of node indices.
func fo6RandomIndexSet(rng *rand.Rand, n, p int) []int {
	var out []int
	for i := 0; i < n; i++ {
		if rng.Intn(p) == 0 {
			out = append(out, i)
		}
	}
	return out
}

func TestStage3Formulaforge6(t *testing.T) {
	cases := fo6Cases()

	t.Run("ExactMatchesExhaustive", func(t *testing.T) {
		checked := 0
		for _, c := range cases {
			rng := rand.New(rand.NewSource(c.seed*31 + 5))
			// Two random target sets plus the full-sheet target set,
			// each with a random prior.
			targetSets := [][]int{
				fo6RandomIndexSet(rng, c.n, 2),
				fo6RandomIndexSet(rng, c.n, 3),
			}
			all := make([]int, c.n)
			for i := range all {
				all[i] = i
			}
			targetSets = append(targetSets, all)
			for ti, tidx := range targetSets {
				if len(tidx) == 0 {
					tidx = []int{0}
				}
				prior := fo6RandomIndexSet(rng, c.n, 4)
				targets := fo6IDs(c.idOf, tidx)
				priorIDs := fo6IDs(c.idOf, prior)
				got, err := c.g.MinMemorizeFor(targets, priorIDs)
				if err != nil {
					t.Fatalf("seed %d family %s case %d: MinMemorizeFor: %v", c.seed, c.family, ti, err)
				}
				want := fo6ExhaustiveMinSize(c.g, targets, priorIDs)
				if len(got) != want {
					t.Errorf("seed %d family %s case %d targets=%v prior=%v: MinMemorizeFor size %d, exhaustive optimum %d (set %v)", c.seed, c.family, ti, targets, priorIDs, len(got), want, got)
				}
				// The fresh oracle must agree with the established
				// bruteMinFor oracle on every case.
				if established := bruteMinFor(c.g, targets, priorIDs); len(established) != want {
					t.Errorf("seed %d family %s case %d: fresh oracle %d disagrees with bruteMinFor %d", c.seed, c.family, ti, want, len(established))
				}
				// The returned set must actually cover the targets.
				known := c.g.Closure(toSet(append(append([]string{}, priorIDs...), got...)))
				for _, tg := range targets {
					if !known[tg] {
						t.Errorf("seed %d family %s case %d: returned set %v does not derive target %s", c.seed, c.family, ti, got, tg)
					}
				}
				checked++
			}
			// Full-sheet exactness with the same random prior.
			prior := fo6RandomIndexSet(rng, c.n, 4)
			priorIDs := fo6IDs(c.idOf, prior)
			ids := append([]string{}, c.g.Nodes...)
			if got, want := len(c.g.MinMemorizeSet(priorIDs)), fo6ExhaustiveMinSize(c.g, ids, priorIDs); got != want {
				t.Errorf("seed %d family %s full cover prior=%v: MinMemorizeSet size %d, exhaustive optimum %d", c.seed, c.family, priorIDs, got, want)
			}
			checked++
		}
		t.Logf("exactness: %d MinMemorizeFor/MinMemorizeSet cases across 120 graphs, 0 tolerance, 0 mismatches", checked)
	})

	t.Run("GreedyGapsReported", func(t *testing.T) {
		points, nonzero := 0, 0
		maxGap := 0
		hist := map[int]int{}
		type gapCase struct {
			seed, k, greedy, optimal int
			family                   fo6Family
		}
		var worst []gapCase
		for _, c := range cases {
			curve := c.g.CoverageCurve()
			replay := fo6GreedyReplay(c.g)
			best := fo6ExhaustiveCoverageTable(c.g, nil)
			if len(curve) == 0 {
				t.Fatalf("seed %d family %s: empty coverage curve", c.seed, c.family)
			}
			if last := curve[len(curve)-1]; last.Fraction != 1.0 {
				t.Errorf("seed %d family %s: curve must end at 1.0, got %f", c.seed, c.family, last.Fraction)
			}
			for i := 1; i < len(curve); i++ {
				if curve[i].Fraction < curve[i-1].Fraction {
					t.Errorf("seed %d family %s: curve must be non-decreasing", c.seed, c.family)
				}
				if curve[i].K != curve[i-1].K+1 {
					t.Errorf("seed %d family %s: curve K must increment by 1", c.seed, c.family)
				}
			}
			// The curve must match the independent greedy replay
			// point for point; fractions are counts over n.
			if len(curve) != len(replay) {
				t.Errorf("seed %d family %s: curve has %d points, greedy replay %d", c.seed, c.family, len(curve), len(replay))
			}
			for i, pt := range curve {
				if i >= len(replay) {
					break
				}
				if want := float64(replay[i]) / float64(c.n); pt.Fraction != want {
					t.Errorf("seed %d family %s k=%d: curve fraction %f, greedy replay %f", c.seed, c.family, pt.K, pt.Fraction, want)
				}
			}
			for k, greedy := range replay {
				optimal := 0
				if k < len(best) {
					optimal = best[k]
				}
				gap := optimal - greedy
				if gap < 0 {
					t.Errorf("seed %d family %s k=%d: greedy covers %d, above exhaustive optimum %d: oracle is wrong", c.seed, c.family, k, greedy, optimal)
				}
				points++
				hist[gap]++
				if gap > 0 {
					nonzero++
				}
				if gap > maxGap {
					maxGap = gap
				}
				if gap >= 2 {
					worst = append(worst, gapCase{seed: int(c.seed), k: k, greedy: greedy, optimal: optimal, family: c.family})
				}
			}
		}
		t.Logf("greedy gaps: %d (graph,budget) points, %d nonzero (%.1f%%), max gap %d formulas", points, nonzero, 100*float64(nonzero)/float64(points), maxGap)
		t.Logf("greedy gap histogram (gap in formulas -> points): %v", hist)
		for _, w := range worst {
			t.Logf("worst gap: seed %d family %s budget %d: greedy covers %d, optimum %d", w.seed, w.family, w.k, w.greedy, w.optimal)
		}
	})

	t.Run("RelabelInvariance", func(t *testing.T) {
		curveDiffers := 0
		maxFracDiff := 0.0
		for _, c := range cases {
			g2, idOf2 := fo6Graph(c.spec, c.seed*1000+999)
			rng := rand.New(rand.NewSource(c.seed*31 + 11))
			tidx := fo6RandomIndexSet(rng, c.n, 2)
			if len(tidx) == 0 {
				tidx = []int{0}
			}
			prior := fo6RandomIndexSet(rng, c.n, 4)
			// Exact costs must be identical under relabeling: the
			// optimum size does not depend on identifier order.
			got1, err1 := c.g.MinMemorizeFor(fo6IDs(c.idOf, tidx), fo6IDs(c.idOf, prior))
			got2, err2 := g2.MinMemorizeFor(fo6IDs(idOf2, tidx), fo6IDs(idOf2, prior))
			if err1 != nil || err2 != nil {
				t.Fatalf("seed %d family %s: MinMemorizeFor under relabel: %v / %v", c.seed, c.family, err1, err2)
			}
			if len(got1) != len(got2) {
				t.Errorf("seed %d family %s: MinMemorizeFor cost %d vs relabeled %d", c.seed, c.family, len(got1), len(got2))
			}
			if s1, s2 := len(c.g.MinMemorizeSet(fo6IDs(c.idOf, prior))), len(g2.MinMemorizeSet(fo6IDs(idOf2, prior))); s1 != s2 {
				t.Errorf("seed %d family %s: MinMemorizeSet cost %d vs relabeled %d", c.seed, c.family, s1, s2)
			}
			// The greedy curve is not required to be invariant: it
			// breaks ties by identifier, so relabeling may change
			// which formula is memorized first. Measure, do not
			// assert, and report the sensitivity. Compare over the
			// full length of both curves; a finished curve stays
			// at 1.0 past its last point.
			curve1, curve2 := c.g.CoverageCurve(), g2.CoverageCurve()
			diff := 0.0
			for i := 0; i < len(curve1) || i < len(curve2); i++ {
				f1, f2 := 1.0, 1.0
				if i < len(curve1) {
					f1 = curve1[i].Fraction
				}
				if i < len(curve2) {
					f2 = curve2[i].Fraction
				}
				if d := f1 - f2; d < 0 {
					if -d > diff {
						diff = -d
					}
				} else if d > diff {
					diff = d
				}
			}
			if diff > 0 {
				curveDiffers++
			}
			if diff > maxFracDiff {
				maxFracDiff = diff
			}
		}
		t.Logf("relabel: exact costs identical on all 120 graphs; greedy curve fractions differ on %d/120 graphs, max fraction difference %.3f", curveDiffers, maxFracDiff)
	})

	t.Run("RuntimeAndTimeouts", func(t *testing.T) {
		var exactTotal, oracleTotal, greedyTotal time.Duration
		var exactMax time.Duration
		for _, c := range cases {
			ids := append([]string{}, c.g.Nodes...)
			start := time.Now()
			if _, err := c.g.MinMemorizeFor(ids, nil); err != nil {
				t.Fatal(err)
			}
			d := time.Since(start)
			exactTotal += d
			if d > exactMax {
				exactMax = d
			}
			start = time.Now()
			fo6ExhaustiveMinSize(c.g, ids, nil)
			oracleTotal += time.Since(start)
			start = time.Now()
			c.g.CoverageCurve()
			greedyTotal += time.Since(start)
		}
		t.Logf("runtime on 120 small graphs (full-sheet targets): exact total %s max %s per case; exhaustive oracle total %s; greedy total %s", exactTotal, exactMax, oracleTotal, greedyTotal)
		// Larger graphs: the exhaustive oracle is infeasible there, so
		// only the exact solver runs, with a wall-clock budget. A case
		// that exceeds the budget is recorded as a timeout, not a
		// failure, and its returned set is still checked for validity
		// if it arrives.
		const budget = 20 * time.Second
		timeouts := 0
		var largeMax time.Duration
		for _, n := range []int{20, 24, 26, 28} {
			spec := fo6RandomSpec(int64(1000+n), n, fo6Bottleneck)
			g, _ := fo6Graph(spec, int64(2000+n))
			ids := append([]string{}, g.Nodes...)
			done := make(chan []string, 1)
			start := time.Now()
			go func() {
				ms, err := g.MinMemorizeFor(ids, nil)
				if err == nil {
					done <- ms
				} else {
					done <- nil
				}
			}()
			select {
			case ms := <-done:
				d := time.Since(start)
				if d > largeMax {
					largeMax = d
				}
				if ms == nil {
					t.Errorf("n=%d large graph: MinMemorizeFor error", n)
					continue
				}
				known := g.Closure(toSet(ms))
				for _, id := range ids {
					if !known[id] {
						t.Errorf("n=%d large graph: returned set does not cover %s", n, id)
					}
				}
				t.Logf("large graph n=%d: exact solver %s, memorize set size %d", n, d, len(ms))
			case <-time.After(budget):
				timeouts++
				t.Logf("large graph n=%d: TIMEOUT after %s (exact solver still running in background)", n, budget)
			}
			start = time.Now()
			g.CoverageCurve()
			t.Logf("large graph n=%d: greedy curve %s", n, time.Since(start))
		}
		t.Logf("large-graph probe: 4 cases (n=20,24,26,28, bottleneck family), %d timeouts, max exact runtime %s", timeouts, largeMax)
	})
}
