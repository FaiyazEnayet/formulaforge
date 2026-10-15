package graph

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

func loadGraph(t *testing.T, name string) *Graph {
	t.Helper()
	s, err := models.LoadSheetByName(name)
	if err != nil {
		t.Fatal(err)
	}
	return New(s)
}

func TestAllSheetsAcyclic(t *testing.T) {
	for _, name := range models.ListSheets() {
		g := loadGraph(t, name)
		if err := g.AssertAcyclic(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if c := g.FindCycle(); c != nil {
			t.Errorf("%s: unexpected cycle %v", name, c)
		}
	}
}

func TestTotalEdges(t *testing.T) {
	total := 0
	for _, name := range models.ListSheets() {
		g := loadGraph(t, name)
		st, err := g.Stats()
		if err != nil {
			t.Fatal(err)
		}
		total += st.Edges
	}
	if total != 303 {
		t.Errorf("total edges = %d, want 303", total)
	}
}

func TestTopoOrderRespectsPrereqs(t *testing.T) {
	for _, name := range models.ListSheets() {
		g := loadGraph(t, name)
		order, err := g.TopoOrder()
		if err != nil {
			t.Fatal(err)
		}
		if len(order) != len(g.Nodes) {
			t.Fatalf("%s: order length %d != %d nodes", name, len(order), len(g.Nodes))
		}
		pos := map[string]int{}
		for i, n := range order {
			pos[n] = i
		}
		for _, n := range g.Nodes {
			for _, p := range g.Prereqs[n] {
				if pos[p] >= pos[n] {
					t.Errorf("%s: prereq %s not before %s", name, p, n)
				}
			}
		}
	}
}

func TestTopoOrderDeterministic(t *testing.T) {
	g := loadGraph(t, "phy131")
	a, _ := g.TopoOrder()
	b, _ := g.TopoOrder()
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("topo order not deterministic")
		}
	}
}

func TestWavesCoverAllNodes(t *testing.T) {
	want := map[string]int{"ams151": 5, "ams161": 5, "ams210": 5, "ams261": 6, "phy131": 7, "cse113": 5, "cse213": 5, "cse214": 4, "psy103": 6}
	for _, name := range models.ListSheets() {
		g := loadGraph(t, name)
		waves, err := g.Waves()
		if err != nil {
			t.Fatal(err)
		}
		if len(waves) != want[name] {
			t.Errorf("%s: %d waves, want %d", name, len(waves), want[name])
		}
		seen := map[string]bool{}
		for i, w := range waves {
			for _, n := range w {
				if seen[n] {
					t.Errorf("%s: node %s in two waves", name, n)
				}
				seen[n] = true
				for _, p := range g.Prereqs[n] {
					// prereq must be in a strictly earlier wave
					pl := -1
					for j, w2 := range waves {
						for _, m := range w2 {
							if m == p {
								pl = j
							}
						}
					}
					if pl >= i {
						t.Errorf("%s: prereq %s not in earlier wave than %s", name, p, n)
					}
				}
			}
		}
		if len(seen) != len(g.Nodes) {
			t.Errorf("%s: waves cover %d of %d nodes", name, len(seen), len(g.Nodes))
		}
	}
}

func TestLeverageKnownValues(t *testing.T) {
	cases := []struct {
		sheet string
		id    string
		want  int
	}{
		{"phy131", "newton2", 35},
		{"ams261", "partial-def", 11},
		{"psy103", "mean", 17},
		{"cse213", "add-rule", 5},
	}
	for _, c := range cases {
		g := loadGraph(t, c.sheet)
		if got := g.Leverage(c.id); got != c.want {
			t.Errorf("%s/%s: leverage = %d, want %d", c.sheet, c.id, got, c.want)
		}
	}
}

func TestLeverageRankingSorted(t *testing.T) {
	g := loadGraph(t, "cse213")
	r := g.LeverageRanking()
	for i := 1; i < len(r); i++ {
		if r[i].Leverage > r[i-1].Leverage {
			t.Fatal("ranking not sorted desc")
		}
		if r[i].Leverage == r[i-1].Leverage && r[i].ID < r[i-1].ID {
			t.Fatal("ties not broken by id")
		}
	}
	if r[0].ID != "add-rule" {
		t.Errorf("top leverage = %s, want add-rule", r[0].ID)
	}
}

func TestMinMemorizeSetIsExactRoots(t *testing.T) {
	for _, name := range models.ListSheets() {
		g := loadGraph(t, name)
		ms := g.MinMemorizeSet(nil)
		// The exact minimum is the set of roots: anything without a
		// derivation path must be memorized, and the roots derive the rest.
		var roots []string
		for _, n := range g.Nodes {
			if len(g.Paths[n]) == 0 {
				roots = append(roots, n)
			}
		}
		sort.Strings(roots)
		if fmt.Sprint(ms) != fmt.Sprint(roots) {
			t.Errorf("%s: min-memorize = %v, want roots %v", name, ms, roots)
		}
		// full coverage from the set
		if got := g.Closure(toSet(ms)); len(got) != len(g.Nodes) {
			t.Errorf("%s: min-memorize covers %d of %d", name, len(got), len(g.Nodes))
		}
		// minimal: dropping any single root breaks full coverage
		for _, drop := range ms {
			rest := make([]string, 0, len(ms)-1)
			for _, m := range ms {
				if m != drop {
					rest = append(rest, m)
				}
			}
			if got := g.Closure(toSet(rest)); len(got) == len(g.Nodes) {
				t.Errorf("%s: set without %s still covers all: not minimal", name, drop)
			}
		}
	}
}

func TestMinMemorizeSetPriorKnowledge(t *testing.T) {
	g := loadGraph(t, "phy131")
	var roots []string
	for _, n := range g.Nodes {
		if len(g.Paths[n]) == 0 {
			roots = append(roots, n)
		}
	}
	sort.Strings(roots)
	prior := roots[:2]
	ms := g.MinMemorizeSet(prior)
	for _, p := range prior {
		for _, m := range ms {
			if m == p {
				t.Errorf("already-known root %s must not be in the memorize set", p)
			}
		}
	}
	if len(ms) != len(roots)-2 {
		t.Errorf("min-memorize with 2 known roots = %d, want %d", len(ms), len(roots)-2)
	}
	if got := g.Closure(toSet(append(append([]string{}, prior...), ms...))); len(got) != len(g.Nodes) {
		t.Error("prior + memorize set does not cover the sheet")
	}
}

func TestMinMemorizeSetDeterministic(t *testing.T) {
	g := loadGraph(t, "psy103")
	a := g.MinMemorizeSet(nil)
	b := g.MinMemorizeSet(nil)
	if len(a) != len(b) {
		t.Fatal("length mismatch")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("min-memorize set not deterministic")
		}
	}
}

// TestClosureIsConjunctive is the regression test for the coverage bug:
// under the old reachability semantics, memorizing A alone counted C as
// covered even when C derives from A and B. Conjunctive semantics must
// require every prerequisite of a derivation path.
func TestClosureIsConjunctive(t *testing.T) {
	s := &models.Sheet{
		Course: "T", Title: "Conjunctive",
		Formulas: []models.Formula{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B"},
			{ID: "c", Name: "C", DerivesFrom: []string{"a", "b"}},
			{ID: "d", Name: "D", DerivesFrom: []string{"a"}, AltDerivesFrom: [][]string{{"b"}}},
		},
	}
	g := New(s)
	if got := g.Closure(toSet([]string{"a"})); got["c"] {
		t.Error("C must not be derivable from A alone; it needs A and B")
	}
	if got := g.Closure(toSet([]string{"a", "b"})); !got["c"] {
		t.Error("C must be derivable from A and B together")
	}
	if got := g.Closure(toSet([]string{"b"})); !got["d"] {
		t.Error("D must be derivable from B alone via its alternative path")
	}
	if got := g.Closure(toSet([]string{})); len(got) != 0 {
		t.Errorf("empty start must derive nothing, got %v", got)
	}
}

func TestMinMemorizeFor(t *testing.T) {
	s := &models.Sheet{
		Course: "T", Title: "Targets",
		Formulas: []models.Formula{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B"},
			{ID: "c", Name: "C"},
			{ID: "t1", Name: "T1", DerivesFrom: []string{"a", "b"}},
			{ID: "t2", Name: "T2", DerivesFrom: []string{"b", "c"}},
			{ID: "u", Name: "U", DerivesFrom: []string{"a"}, AltDerivesFrom: [][]string{{"c"}}},
		},
	}
	g := New(s)
	// Memorizing t1 and t2 directly (2) beats learning a, b, c (3).
	ms, err := g.MinMemorizeFor([]string{"t1", "t2"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(ms) != "[t1 t2]" {
		t.Errorf("MinMemorizeFor(t1,t2) = %v, want [t1 t2]", ms)
	}
	// With B already known, covering t1 takes one memorization: either A
	// or t1 itself. Deterministic search picks A.
	ms, err = g.MinMemorizeFor([]string{"t1"}, []string{"b"})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(ms) != "[a]" {
		t.Errorf("MinMemorizeFor(t1, prior=b) = %v, want [a]", ms)
	}
	// Alternative paths: u via [a] or [c]; search order settles on [a].
	ms, err = g.MinMemorizeFor([]string{"u"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(ms) != "[a]" {
		t.Errorf("MinMemorizeFor(u) = %v, want [a]", ms)
	}
	// Already-known targets need nothing.
	ms, err = g.MinMemorizeFor([]string{"a"}, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 0 {
		t.Errorf("known target needs no memorization, got %v", ms)
	}
	// Unknown target ids are an error.
	if _, err := g.MinMemorizeFor([]string{"nope"}, nil); err == nil {
		t.Error("expected error for unknown target id")
	}
}

// bruteMinFor is the exact reference: smallest subset (ties broken
// lexicographically) whose closure covers the targets.
func bruteMinFor(g *Graph, targets []string, prior []string) []string {
	ids := append([]string{}, g.Nodes...)
	sort.Strings(ids)
	priorSet := toSet(prior)
	var best []string
	bestSize := len(ids) + 1
	for mask := 0; mask < (1 << len(ids)); mask++ {
		var mem []string
		for i, id := range ids {
			if mask&(1<<i) != 0 {
				mem = append(mem, id)
			}
		}
		if len(mem) > bestSize {
			continue
		}
		km := make(map[string]bool, len(priorSet)+len(mem))
		for k := range priorSet {
			km[k] = true
		}
		for k := range toSet(mem) {
			km[k] = true
		}
		known := g.Closure(km)
		ok := true
		for _, tg := range targets {
			if !known[tg] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if len(mem) < bestSize || (len(mem) == bestSize && fmt.Sprint(mem) < fmt.Sprint(best)) {
			bestSize, best = len(mem), mem
		}
	}
	return best
}

// randomANDORDAG builds a small DAG with occasional alternative
// derivation paths, seeded for reproducibility.
func randomANDORDAG(seed int64, n int) *Graph {
	rng := rand.New(rand.NewSource(seed))
	var fs []models.Formula
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("f%02d", i)
		f := models.Formula{ID: id, Name: id}
		var prim []string
		for j := 0; j < i; j++ {
			if rng.Intn(3) == 0 {
				prim = append(prim, fmt.Sprintf("f%02d", j))
			}
		}
		f.DerivesFrom = prim
		if i > 1 && rng.Intn(4) == 0 {
			var alt []string
			for j := 0; j < i; j++ {
				if rng.Intn(4) == 0 {
					alt = append(alt, fmt.Sprintf("f%02d", j))
				}
			}
			if len(alt) > 0 && fmt.Sprint(alt) != fmt.Sprint(prim) {
				f.AltDerivesFrom = [][]string{alt}
			}
		}
		fs = append(fs, f)
	}
	return New(&models.Sheet{Course: "T", Title: "Random", Formulas: fs})
}

func TestMinMemorizeForMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 30; trial++ {
		g := randomANDORDAG(int64(trial), 6+rng.Intn(5))
		ids := append([]string{}, g.Nodes...)
		sort.Strings(ids)
		var targets []string
		for _, id := range ids {
			if rng.Intn(2) == 0 {
				targets = append(targets, id)
			}
		}
		if len(targets) == 0 {
			targets = []string{ids[0]}
		}
		var prior []string
		for _, id := range ids {
			if rng.Intn(4) == 0 {
				prior = append(prior, id)
			}
		}
		got, err := g.MinMemorizeFor(targets, prior)
		if err != nil {
			t.Fatal(err)
		}
		if want := bruteMinFor(g, targets, prior); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("trial %d targets=%v prior=%v: got %v, brute force %v", trial, targets, prior, got, want)
		}
		// Full-sheet exactness too.
		if got, want := g.MinMemorizeSet(prior), bruteMinFor(g, ids, prior); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("trial %d full cover prior=%v: got %v, brute force %v", trial, prior, got, want)
		}
	}
}

// TestMinMemorizeForRealSheet checks the solver terminates fast and
// returns a valid set on a real sheet (exactness is covered by the
// brute-force test on small graphs).
func TestMinMemorizeForRealSheet(t *testing.T) {
	g := loadGraph(t, "phy131")
	targets := []string{"projectile", "newton2", "work-energy"}
	ms, err := g.MinMemorizeFor(targets, nil)
	if err != nil {
		t.Fatal(err)
	}
	known := g.Closure(toSet(ms))
	for _, tg := range targets {
		if !known[tg] {
			t.Errorf("target %s not derivable from %v", tg, ms)
		}
	}
	if len(ms) > len(g.MinMemorizeSet(nil)) {
		t.Errorf("target set larger than full-sheet set: %v", ms)
	}
	again, _ := g.MinMemorizeFor(targets, nil)
	if fmt.Sprint(ms) != fmt.Sprint(again) {
		t.Error("MinMemorizeFor not deterministic")
	}
}

func TestCoverageCurve(t *testing.T) {
	g := loadGraph(t, "ams261")
	curve := g.CoverageCurve()
	if len(curve) == 0 || curve[0].K != 0 || curve[0].Fraction != 0.0 {
		t.Error("curve must start at (0, 0)")
	}
	last := curve[len(curve)-1]
	if last.Fraction != 1.0 {
		t.Errorf("curve must end at 1.0, got %f", last.Fraction)
	}
	for i := 1; i < len(curve); i++ {
		if curve[i].Fraction < curve[i-1].Fraction {
			t.Error("curve must be non-decreasing")
		}
		if curve[i].K != curve[i-1].K+1 {
			t.Error("curve K must increment by 1")
		}
	}
	// the curve is deterministic
	again := g.CoverageCurve()
	if fmt.Sprint(curve) != fmt.Sprint(again) {
		t.Error("curve not deterministic")
	}
}

func TestDescendantsAncestors(t *testing.T) {
	g := loadGraph(t, "phy131")
	d := g.Descendants("newton2")
	if len(d) != 35 {
		t.Errorf("descendants(newton2) = %d, want 35", len(d))
	}
	if d["newton2"] {
		t.Error("descendants must not include the node itself")
	}
	a := g.Ancestors("newton2")
	if len(a) != 0 {
		t.Errorf("ancestors(newton2) = %d, want 0 (it is a root)", len(a))
	}
	// a deep node has both
	g2 := loadGraph(t, "psy103")
	da := g2.Descendants("mean")
	if len(da) == 0 {
		t.Error("mean should unlock formulas")
	}
	aa := g2.Ancestors("ci-95")
	if len(aa) == 0 {
		t.Error("ci-95 should have ancestors")
	}
}

func TestStats(t *testing.T) {
	g := loadGraph(t, "phy131")
	st, err := g.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Nodes != 51 || st.Edges != 66 || st.Waves != 7 || st.Depth != 6 {
		t.Errorf("unexpected stats: %+v", st)
	}
	if st.Roots+st.Leaves == 0 {
		t.Error("roots/leaves not counted")
	}
	if len(st.TopLeverage) != 5 {
		t.Errorf("top leverage has %d entries, want 5", len(st.TopLeverage))
	}
	if len(st.MinMemorizeSet) != 9 {
		t.Errorf("min set size %d, want 9", len(st.MinMemorizeSet))
	}
}

func TestCycleDetection(t *testing.T) {
	s := &models.Sheet{
		Course: "T", Title: "Cycle",
		Formulas: []models.Formula{
			{ID: "a", Name: "A", DerivesFrom: []string{"c"}},
			{ID: "b", Name: "B", DerivesFrom: []string{"a"}},
			{ID: "c", Name: "C", DerivesFrom: []string{"b"}},
		},
	}
	g := New(s)
	if c := g.FindCycle(); c == nil {
		t.Fatal("expected a cycle")
	}
	if err := g.AssertAcyclic(); err == nil {
		t.Fatal("expected AssertAcyclic error")
	}
	if _, err := g.TopoOrder(); err == nil {
		t.Fatal("expected TopoOrder error on cycle")
	}
	if _, err := g.Stats(); err == nil {
		t.Fatal("expected Stats error on cycle")
	}
}

func TestFoundationRanking(t *testing.T) {
	g := loadGraph(t, "ams261")
	r := g.FoundationRanking()
	if len(r) != len(g.Nodes) {
		t.Fatal("ranking length mismatch")
	}
	for i := 1; i < len(r); i++ {
		if r[i].Leverage > r[i-1].Leverage {
			t.Fatal("not sorted desc")
		}
	}
}
