// Package graph implements dependency-graph algorithms over a formula
// sheet. The graph is a DAG: an edge A -> B means "B derives from A"
// (learn A first). Algorithms: cycle detection, topological study order
// grouped into waves, transitive dependents (leverage), longest-chain
// depth, derivation closure under conjunctive semantics, and exact
// minimum-memorize sets.
package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// Graph is the dependency DAG of a sheet.
type Graph struct {
	Sheet    *models.Sheet
	Nodes    []string
	Prereqs  map[string][]string   // node -> direct prerequisites (union over paths)
	Children map[string][]string   // node -> direct dependents
	Paths    map[string][][]string // node -> derivation paths; empty means root
}

// New builds the graph for a sheet.
func New(sheet *models.Sheet) *Graph {
	g := &Graph{
		Sheet:    sheet,
		Prereqs:  map[string][]string{},
		Children: map[string][]string{},
		Paths:    map[string][][]string{},
	}
	for _, f := range sheet.Formulas {
		g.Nodes = append(g.Nodes, f.ID)
		var paths [][]string
		if len(f.DerivesFrom) > 0 {
			d := append([]string{}, f.DerivesFrom...)
			sort.Strings(d)
			paths = append(paths, d)
		}
		for _, alt := range f.AltDerivesFrom {
			a := append([]string{}, alt...)
			sort.Strings(a)
			paths = append(paths, a)
		}
		sort.Slice(paths, func(i, j int) bool {
			if len(paths[i]) != len(paths[j]) {
				return len(paths[i]) < len(paths[j])
			}
			for k := range paths[i] {
				if paths[i][k] != paths[j][k] {
					return paths[i][k] < paths[j][k]
				}
			}
			return false
		})
		g.Paths[f.ID] = paths
		// Prereqs is the union over paths: anything that might have to
		// be learned first. Topo order and cycle checks use this.
		union := map[string]bool{}
		for _, p := range paths {
			for _, m := range p {
				union[m] = true
			}
		}
		deps := make([]string, 0, len(union))
		for m := range union {
			deps = append(deps, m)
		}
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

// Leverage is the number of formulas that list the node as a (transitive)
// prerequisite: a structural count of dependents, not a derivability claim.
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

// Derivation semantics.
//
// A formula is known when it is memorized, already known (prior
// knowledge), or derivable: every member of at least one of its
// derivation paths is known. Deriving C from A and B needs both A and
// B; knowing A alone is not enough. A formula with no derivation paths
// is a root: it can only be memorized or already known.
//
// Closure(known) is the fixed point of that rule: the starting set plus
// everything derivable from it. MinMemorizeSet(prior) is the exact
// smallest set covering the whole sheet: every unknown root must be
// memorized, and the unknown roots derive everything else, so the answer
// is exact rather than an approximation. MinMemorizeFor(targets, prior)
// is the exact smallest set covering a target list (exam topics), found
// with branch and bound.

// Closure returns the starting set plus everything derivable from it.
func (g *Graph) Closure(known map[string]bool) map[string]bool {
	out := make(map[string]bool, len(known))
	for k := range known {
		out[k] = true
	}
	for changed := true; changed; {
		changed = false
		for _, n := range g.Nodes {
			if out[n] {
				continue
			}
			for _, path := range g.Paths[n] {
				ok := true
				for _, p := range path {
					if !out[p] {
						ok = false
						break
					}
				}
				if ok {
					out[n] = true
					changed = true
					break
				}
			}
		}
	}
	return out
}

// MinMemorizeSet returns the exact smallest set of formulas to memorize
// so that the whole sheet is derivable, given prior knowledge.
func (g *Graph) MinMemorizeSet(prior []string) []string {
	known := g.Closure(toSet(prior))
	var out []string
	for _, n := range g.Nodes {
		if len(g.Paths[n]) == 0 && !known[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// MinMemorizeFor returns the exact smallest set of formulas to memorize
// so that every target is derivable, given prior knowledge. Each target
// is either memorized directly or derived through one of its derivation
// paths; branch and bound tries both, pruning against the best solution
// found so far. Exact, exponential in the worst case, fast on real
// sheets. Unknown target ids are an error.
func (g *Graph) MinMemorizeFor(targets []string, prior []string) ([]string, error) {
	ids := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		ids[n] = true
	}
	for _, t := range targets {
		if !ids[t] {
			return nil, fmt.Errorf("unknown formula id: %s", t)
		}
	}
	priorSet := toSet(prior)
	base := g.Closure(priorSet)
	need := map[string]bool{}
	for _, t := range targets {
		if !base[t] {
			need[t] = true
		}
	}
	if len(need) == 0 {
		return []string{}, nil
	}
	// Memorizing every unknown root always derives everything, so it is
	// a valid starting upper bound.
	upper := g.MinMemorizeSet(prior)
	best := &bbBest{set: upper, size: len(upper)}
	memo := map[string]int{}
	g.branch(need, map[string]bool{}, priorSet, best, memo)
	return best.set, nil
}

type bbBest struct {
	set  []string
	size int
}

// branch searches memorize sets covering need. mem only grows, need only
// grows, so the search terminates.
func (g *Graph) branch(need, mem, prior map[string]bool, best *bbBest, memo map[string]int) {
	km := make(map[string]bool, len(prior)+len(mem))
	for k := range prior {
		km[k] = true
	}
	for k := range mem {
		km[k] = true
	}
	known := g.Closure(km)
	var resid []string
	for r := range need {
		if !known[r] {
			resid = append(resid, r)
		}
	}
	if len(resid) == 0 {
		if len(mem) < best.size {
			best.size = len(mem)
			best.set = sortedKeys(mem)
		}
		return
	}
	if len(mem) >= best.size {
		return
	}
	// Same residual need and known set reached with no more memorizations
	// cannot lead to a better solution.
	key := memoKey(resid, known)
	if s, ok := memo[key]; ok && s <= len(mem) {
		return
	}
	memo[key] = len(mem)
	sort.Strings(resid)
	r := resid[0]
	// Derive r through each of its paths: every path member must be known.
	for _, path := range g.Paths[r] {
		need2 := make(map[string]bool, len(need)+len(path))
		for k := range need {
			need2[k] = true
		}
		grew := false
		for _, p := range path {
			if !need2[p] {
				need2[p] = true
				grew = true
			}
		}
		if grew {
			g.branch(need2, mem, prior, best, memo)
		}
	}
	// Or memorize r outright.
	mem2 := make(map[string]bool, len(mem)+1)
	for k := range mem {
		mem2[k] = true
	}
	mem2[r] = true
	g.branch(need, mem2, prior, best, memo)
}

func toSet(ids []string) map[string]bool {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func memoKey(resid []string, known map[string]bool) string {
	rs := append([]string{}, resid...)
	sort.Strings(rs)
	ks := make([]string, 0, len(known))
	for k := range known {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(rs, ",") + "|" + strings.Join(ks, ",")
}

// CoveragePoint is one (formulas memorized, fraction of sheet derivable).
type CoveragePoint struct {
	K        int
	Fraction float64
}

// CoverageCurve replays greedy budgeted memorization for the dashboard:
// each step memorizes the formula with the largest marginal gain in
// derivable formulas under conjunctive closure, ties broken by id.
// Greedy is a heuristic for the budgeted-coverage problem, not an exact
// solution.
func (g *Graph) CoverageCurve() []CoveragePoint {
	mem := map[string]bool{}
	known := g.Closure(mem)
	total := float64(len(g.Nodes))
	curve := []CoveragePoint{{K: 0, Fraction: float64(len(known)) / total}}
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
		curve = append(curve, CoveragePoint{
			K:        len(curve),
			Fraction: float64(len(known)) / total,
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
		MinMemorizeSet: g.MinMemorizeSet(nil),
	}, nil
}
