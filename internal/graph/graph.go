// Package graph implements dependency-graph algorithms over a formula
// sheet. The graph is a DAG: an edge A -> B means "B derives from A"
// (learn A first). Algorithms: cycle detection, topological study order
// grouped into waves, transitive dependents (leverage), longest-chain
// depth, and a greedy minimum-memorize set cover.
package graph

import (
	"fmt"
	"sort"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// Graph is the dependency DAG of a sheet.
type Graph struct {
	Sheet    *models.Sheet
	Nodes    []string
	Prereqs  map[string][]string // node -> direct prerequisites
	Children map[string][]string // node -> direct dependents
}

// New builds the graph for a sheet.
func New(sheet *models.Sheet) *Graph {
	g := &Graph{
		Sheet:    sheet,
		Prereqs:  map[string][]string{},
		Children: map[string][]string{},
	}
	for _, f := range sheet.Formulas {
		g.Nodes = append(g.Nodes, f.ID)
		deps := append([]string{}, f.DerivesFrom...)
		sort.Strings(deps)
		g.Prereqs[f.ID] = deps
		if _, ok := g.Children[f.ID]; !ok {
			g.Children[f.ID] = []string{}
		}
	}
	for _, n := range g.Nodes {
		for _, p := range g.Prereqs[n] {
			g.Children[p] = append(g.Children[p], n)
		}
	}
	for n := range g.Children {
		sort.Strings(g.Children[n])
	}
	return g
}

// FindCycle returns node ids forming a cycle, or nil if the graph is acyclic.
func (g *Graph) FindCycle() []string {
	const ( // colors
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	for _, n := range g.Nodes {
		color[n] = white
	}
	var stack []string
	var visit func(u string) []string
	visit = func(u string) []string {
		color[u] = gray
		stack = append(stack, u)
		for _, v := range g.Children[u] {
			if color[v] == gray {
				idx := 0
				for i, s := range stack {
					if s == v {
						idx = i
						break
					}
				}
				cycle := append([]string{}, stack[idx:]...)
				return append(cycle, v)
			}
			if color[v] == white {
				if hit := visit(v); hit != nil {
					return hit
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[u] = black
		return nil
	}
	for _, n := range g.Nodes {
		if color[n] == white {
			if hit := visit(n); hit != nil {
				return hit
			}
		}
	}
	return nil
}

// AssertAcyclic returns an error if the graph contains a cycle.
func (g *Graph) AssertAcyclic() error {
	if c := g.FindCycle(); c != nil {
		return fmt.Errorf("dependency cycle: %v", c)
	}
	return nil
}

// TopoOrder returns a topological ordering via Kahn's algorithm.
// Deterministic: ties broken by node id, matching the reference build.
func (g *Graph) TopoOrder() ([]string, error) {
	indeg := map[string]int{}
	for _, n := range g.Nodes {
		indeg[n] = len(g.Prereqs[n])
	}
	var queue []string
	for _, n := range g.Nodes {
		if indeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	sort.Strings(queue)
	var order []string
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		order = append(order, u)
		for _, v := range g.Children[u] { // children pre-sorted
			indeg[v]--
			if indeg[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
	if len(order) != len(g.Nodes) {
		return nil, fmt.Errorf("dependency cycle detected")
	}
	return order, nil
}

// Levels returns the longest-path layering: level 0 = roots.
func (g *Graph) Levels() (map[string]int, error) {
	order, err := g.TopoOrder()
	if err != nil {
		return nil, err
	}
	level := map[string]int{}
	for _, n := range order {
		if len(g.Prereqs[n]) == 0 {
			level[n] = 0
		} else {
			mx := 0
			for _, p := range g.Prereqs[n] {
				if level[p]+1 > mx {
					mx = level[p] + 1
				}
			}
			level[n] = mx
		}
	}
	return level, nil
}

// Waves groups nodes by level; each wave is in topological order.
func (g *Graph) Waves() ([][]string, error) {
	level, err := g.Levels()
	if err != nil {
		return nil, err
	}
	order, err := g.TopoOrder()
	if err != nil {
		return nil, err
	}
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	maxLevel := 0
	for _, l := range level {
		if l > maxLevel {
			maxLevel = l
		}
	}
	waves := make([][]string, maxLevel+1)
	for _, n := range g.Nodes {
		waves[level[n]] = append(waves[level[n]], n)
	}
	for i := range waves {
		sort.Slice(waves[i], func(a, b int) bool { return pos[waves[i][a]] < pos[waves[i][b]] })
	}
	return waves, nil
}

// Depth is the longest dependency chain length.
func (g *Graph) Depth() (int, error) {
	level, err := g.Levels()
	if err != nil {
		return 0, err
	}
	mx := 0
	for _, l := range level {
		if l > mx {
			mx = l
		}
	}
	return mx, nil
}

// Descendants returns every formula transitively derived from node.
func (g *Graph) Descendants(node string) map[string]bool {
	seen := map[string]bool{}
	stack := []string{node}
	for len(stack) > 0 {
		u := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, v := range g.Children[u] {
			if v != node && !seen[v] {
				seen[v] = true
				stack = append(stack, v)
			}
		}
	}
	return seen
}

// Ancestors returns every formula node transitively builds on.
func (g *Graph) Ancestors(node string) map[string]bool {
	seen := map[string]bool{}
	stack := []string{node}
	for len(stack) > 0 {
		u := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, p := range g.Prereqs[u] {
			if p != node && !seen[p] {
				seen[p] = true
				stack = append(stack, p)
			}
		}
	}
	return seen
}

// Leverage is the number of formulas transitively unlocked by learning node.
func (g *Graph) Leverage(node string) int {
	return len(g.Descendants(node))
}

// LeverageEntry pairs a node with its leverage.
type LeverageEntry struct {
	ID       string
	Leverage int
}

// LeverageRanking sorts nodes by leverage desc, ties by id (deterministic).
func (g *Graph) LeverageRanking() []LeverageEntry {
	out := make([]LeverageEntry, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		out = append(out, LeverageEntry{ID: n, Leverage: g.Leverage(n)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Leverage != out[j].Leverage {
			return out[i].Leverage > out[j].Leverage
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// FoundationRanking sorts nodes by ancestor count desc (deepest formulas).
func (g *Graph) FoundationRanking() []LeverageEntry {
	out := make([]LeverageEntry, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		out = append(out, LeverageEntry{ID: n, Leverage: len(g.Ancestors(n))})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Leverage != out[j].Leverage {
			return out[i].Leverage > out[j].Leverage
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// CoverageOf returns the formulas gained by memorizing node:
// itself plus everything derivable from it.
func (g *Graph) CoverageOf(node string) map[string]bool {
	cov := g.Descendants(node)
	cov[node] = true
	return cov
}

// bestGain picks the node with max (new coverage, id) — ties go to the
// larger id, matching the reference implementation exactly.
func (g *Graph) bestGain(uncovered map[string]bool) (string, int) {
	best, bestGain := "", -1
	for _, n := range g.Nodes {
		cov := g.CoverageOf(n)
		gain := 0
		for id := range cov {
			if uncovered[id] {
				gain++
			}
		}
		if gain > bestGain || (gain == bestGain && n > best) {
			best, bestGain = n, gain
		}
	}
	return best, bestGain
}

// MinMemorizeSet runs greedy set cover: a small set of formulas whose
// coverage is the whole sheet.
func (g *Graph) MinMemorizeSet() []string {
	uncovered := map[string]bool{}
	for _, n := range g.Nodes {
		uncovered[n] = true
	}
	var chosen []string
	for len(uncovered) > 0 {
		best, gain := g.bestGain(uncovered)
		if gain <= 0 {
			break // unreachable on a DAG
		}
		chosen = append(chosen, best)
		for id := range g.CoverageOf(best) {
			delete(uncovered, id)
		}
	}
	return chosen
}

// CoveragePoint is one (formulas memorized, fraction of sheet derivable).
type CoveragePoint struct {
	K        int
	Fraction float64
}

// CoverageCurve replays the greedy set cover for the dashboard.
func (g *Graph) CoverageCurve() []CoveragePoint {
	uncovered := map[string]bool{}
	for _, n := range g.Nodes {
		uncovered[n] = true
	}
	total := float64(len(g.Nodes))
	curve := []CoveragePoint{{K: 0, Fraction: 0.0}}
	for len(uncovered) > 0 {
		best, gain := g.bestGain(uncovered)
		if gain <= 0 {
			break
		}
		for id := range g.CoverageOf(best) {
			delete(uncovered, id)
		}
		curve = append(curve, CoveragePoint{
			K:        len(curve),
			Fraction: 1.0 - float64(len(uncovered))/total,
		})
	}
	return curve
}

// Stats bundles the headline numbers for a sheet.
type Stats struct {
	Nodes          int
	Edges          int
	Depth          int
	Waves          int
	Roots          int
	Leaves         int
	TopLeverage    []LeverageEntry
	MinMemorizeSet []string
}

// Stats computes headline numbers for the sheet graph.
func (g *Graph) Stats() (*Stats, error) {
	if err := g.AssertAcyclic(); err != nil {
		return nil, err
	}
	waves, err := g.Waves()
	if err != nil {
		return nil, err
	}
	depth, err := g.Depth()
	if err != nil {
		return nil, err
	}
	edges := 0
	roots := 0
	for _, n := range g.Nodes {
		edges += len(g.Prereqs[n])
		if len(g.Prereqs[n]) == 0 {
			roots++
		}
	}
	leaves := 0
	for _, n := range g.Nodes {
		if len(g.Children[n]) == 0 {
			leaves++
		}
	}
	ranking := g.LeverageRanking()
	top := ranking
	if len(top) > 5 {
		top = top[:5]
	}
	return &Stats{
		Nodes:          len(g.Nodes),
		Edges:          edges,
		Depth:          depth,
		Waves:          len(waves),
		Roots:          roots,
		Leaves:         leaves,
		TopLeverage:    top,
		MinMemorizeSet: g.MinMemorizeSet(),
	}, nil
}
