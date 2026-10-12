package graph

import (
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
	if total != 244 {
		t.Errorf("total edges = %d, want 244", total)
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
	want := map[string]int{"ams151": 5, "ams161": 5, "ams210": 5, "ams261": 3, "phy131": 5, "cse113": 5, "cse213": 5, "cse214": 4, "psy103": 6}
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
		{"phy131", "newton2", 17},
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

func TestMinMemorizeSetSizes(t *testing.T) {
	want := map[string]int{"ams151": 2, "ams161": 7, "ams210": 8, "ams261": 3, "phy131": 6, "cse113": 8, "cse213": 13, "cse214": 12, "psy103": 5}
	for _, name := range models.ListSheets() {
		g := loadGraph(t, name)
		ms := g.MinMemorizeSet()
		if len(ms) != want[name] {
			t.Errorf("%s: min-memorize = %d, want %d", name, len(ms), want[name])
		}
		// coverage must be complete
		covered := map[string]bool{}
		for _, id := range ms {
			for c := range g.CoverageOf(id) {
				covered[c] = true
			}
		}
		if len(covered) != len(g.Nodes) {
			t.Errorf("%s: min-memorize set covers %d of %d", name, len(covered), len(g.Nodes))
		}
	}
}

func TestMinMemorizeSetDeterministic(t *testing.T) {
	g := loadGraph(t, "psy103")
	a := g.MinMemorizeSet()
	b := g.MinMemorizeSet()
	if len(a) != len(b) {
		t.Fatal("length mismatch")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("min-memorize set not deterministic")
		}
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
	// greedy set size matches MinMemorizeSet
	if len(curve)-1 != len(g.MinMemorizeSet()) {
		t.Error("curve length mismatch with min-memorize set")
	}
}

func TestDescendantsAncestors(t *testing.T) {
	g := loadGraph(t, "phy131")
	d := g.Descendants("newton2")
	if len(d) != 17 {
		t.Errorf("descendants(newton2) = %d, want 17", len(d))
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
	if st.Nodes != 27 || st.Edges != 34 || st.Waves != 5 || st.Depth != 4 {
		t.Errorf("unexpected stats: %+v", st)
	}
	if st.Roots+st.Leaves == 0 {
		t.Error("roots/leaves not counted")
	}
	if len(st.TopLeverage) != 5 {
		t.Errorf("top leverage has %d entries, want 5", len(st.TopLeverage))
	}
	if len(st.MinMemorizeSet) != 6 {
		t.Errorf("min set size %d, want 6", len(st.MinMemorizeSet))
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
