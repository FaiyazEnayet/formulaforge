// Package graph implements dependency-graph algorithms over a formula
// sheet. The graph is a DAG: an edge A -> B means "B derives from A"
// (learn A first). Algorithms: cycle detection, topological study order
// grouped into waves, transitive dependents (leverage), longest-chain
// depth, derivation closure under conjunctive semantics, derivation
// witnesses for the closure, and exact minimum-memorize sets.
//
// Algorithm contract (full text: docs/algorithm-contract.md, pinned
// by TestStage3Formulaforge7):
//
//   - Two structures are kept distinct: Prereqs/Children are the
//     UNION of all derivation paths and drive ordering, cycle
//     detection, levels and leverage; derivability (Closure,
//     MinMemorizeSet, MinMemorizeFor) is computed on the paths
//     themselves under AND-within-a-path, OR-across-paths semantics.
//     Union membership is not a derivability claim.
//   - Cycles are union-graph cycles. A cycle that runs only through
//     a non-chosen alternative path is still rejected by TopoOrder,
//     Levels, Waves, Depth and Stats, even when Closure can derive
//     the whole sheet; sheet authors must keep the union acyclic.
//   - MinMemorizeFor is exact (branch and bound, no internal step
//     cap; worst case exponential in unresolved targets and path
//     choices). MinMemorizeForBounded adds context cancellation and
//     node/time budgets to the same search, returning a feasible
//     incumbent with lower and upper bounds when the budget runs out;
//     it never reports a stopped search as exact. CoverageCurve is
//     the budgeted alternative and is a greedy heuristic with no
//     optimality guarantee.
//   - Every output is deterministic for a fixed sheet. TopoOrder's
//     tie-breaking is a sorted initial ready queue, then readiness
//     (FIFO) order; it is not the lexicographically smallest
//     topological order. Rankings break ties by id ascending.
package graph

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/bits"
	"sort"
	"time"

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

// Derivation semantics: public contract.
//
// A formula is known when it is memorized, already known (prior
// knowledge), or derivable: every member of at least one of its
// derivation paths is known. Deriving C from A and B needs both A and
// B; knowing A alone is not enough. A formula with no derivation paths
// is a root: it can only be memorized or already known.
//
// Prior and known ids: MinMemorizeSet and MinMemorizeFor take prior
// knowledge as a list of formula ids. MinMemorizeFor rejects unknown
// prior ids with an error, exactly as it rejects unknown target ids.
// MinMemorizeSet has no error return, so unknown prior ids are silently
// ignored instead: they can contribute nothing to the closure. This is
// the documented, stable behavior, not an accident.
//
// Bool maps: Closure takes the known set as a map. A formula counts as
// known if and only if it maps to true. Ids mapped to false and ids
// absent from the map are both not known; unknown ids in the map are
// ignored. Presence alone does not count; only an explicit true does.
//
// Closure(known) is the fixed point of that rule: the starting set plus
// everything derivable from it. MinMemorizeSet(prior) is the exact
// smallest set covering the whole sheet: every unknown root must be
// memorized, and the unknown roots derive everything else, so the answer
// is exact rather than an approximation. MinMemorizeFor(targets, prior)
// is the exact smallest set covering a target list (exam topics), found
// with branch and bound.

// Closure returns the starting set plus everything derivable from it.
// Only ids mapped to true count as known; false or unknown ids are
// ignored.
func (g *Graph) Closure(known map[string]bool) map[string]bool {
	out := make(map[string]bool, len(known))
	for k := range known {
		if known[k] {
			out[k] = true
		}
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

// Witnesses returns a derivation witness DAG: for every formula in
// Closure(known) that is not already in the known set, one satisfied
// prerequisite set (a whole derivation path). Roots and already known
// formulas get no entry: they need no derivation. The chosen path is
// the first satisfied path in the deterministic path order (shortest,
// then lexicographic), so the result is deterministic for a fixed
// sheet. A witness DAG explains which derivation justified each
// result; a reachable set alone cannot.
func (g *Graph) Witnesses(known map[string]bool) map[string][]string {
	closed := g.Closure(known)
	out := make(map[string][]string)
	for _, n := range g.Nodes {
		if !closed[n] || known[n] {
			continue
		}
		for _, path := range g.Paths[n] {
			ok := true
			for _, p := range path {
				if !closed[p] {
					ok = false
					break
				}
			}
			if ok {
				out[n] = append([]string{}, path...)
				break
			}
		}
	}
	return out
}

// CheckWitnesses validates a derivation witness DAG against the known
// set. It rejects a witness for a formula that is not derivable from
// the known set, a witness for an already known formula, a witness
// that is not exactly one whole derivation path (so a missing AND
// prerequisite or an extra member is rejected), a witness citing a
// prerequisite outside the closure, a missing witness for any derived
// formula, and a cyclic witness DAG. A nil witness map is valid only
// when nothing is derived.
func (g *Graph) CheckWitnesses(known map[string]bool, witnesses map[string][]string) error {
	closed := g.Closure(known)
	for id, prereqs := range witnesses {
		if !closed[id] {
			return fmt.Errorf("witness for formula %s which is not derivable from the known set", id)
		}
		if known[id] {
			return fmt.Errorf("witness for already known formula %s", id)
		}
		if len(g.Paths[id]) == 0 {
			return fmt.Errorf("root formula %s cannot have a derivation witness", id)
		}
		if !matchesWholePath(prereqs, g.Paths[id]) {
			return fmt.Errorf("witness for %s is not a whole derivation path: %v", id, prereqs)
		}
		for _, p := range prereqs {
			if !closed[p] {
				return fmt.Errorf("witness for %s cites prerequisite %s which is not known", id, p)
			}
		}
	}
	for _, n := range g.Nodes {
		if closed[n] && !known[n] {
			if _, ok := witnesses[n]; !ok {
				return fmt.Errorf("missing witness for derived formula %s", n)
			}
		}
	}
	// Acyclicity: witness edges run from a derived formula to its
	// witnessed prerequisites; a cycle means no derivation in it is
	// grounded in the known set.
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var visit func(u string) error
	visit = func(u string) error {
		color[u] = gray
		for _, p := range witnesses[u] {
			if _, ok := witnesses[p]; !ok {
				continue
			}
			if color[p] == gray {
				return fmt.Errorf("witness cycle: %s depends on %s", u, p)
			}
			if color[p] == white {
				if err := visit(p); err != nil {
					return err
				}
			}
		}
		color[u] = black
		return nil
	}
	for id := range witnesses {
		if color[id] == white {
			if err := visit(id); err != nil {
				return err
			}
		}
	}
	return nil
}

// matchesWholePath reports whether prereqs is exactly one of the
// derivation paths: same members, no more and no fewer. Order does not
// matter; paths are stored sorted.
func matchesWholePath(prereqs []string, paths [][]string) bool {
	for _, path := range paths {
		if len(path) != len(prereqs) {
			continue
		}
		set := make(map[string]bool, len(prereqs))
		for _, p := range prereqs {
			set[p] = true
		}
		ok := true
		for _, p := range path {
			if !set[p] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// MinMemorizeSet returns the exact smallest set of formulas to memorize
// so that the whole sheet is derivable, given prior knowledge.
// Unknown prior ids are ignored (they cannot contribute to the
// closure); see the derivation semantics contract above.
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
// sheets. Unknown target ids are an error, as are unknown prior ids.
//
// It runs the bounded search (MinMemorizeForBounded) with no budget;
// callers that need a node or time cap, or cancellation, use that
// variant directly.
func (g *Graph) MinMemorizeFor(targets []string, prior []string) ([]string, error) {
	res, err := g.MinMemorizeForBounded(context.Background(), targets, prior, SearchBudget{})
	if err != nil {
		return nil, err
	}
	if !res.Exact {
		return nil, fmt.Errorf("unbounded search ended without an exact result")
	}
	return res.Set, nil
}

// SearchBudget caps a bounded exact search. A zero MaxNodes means no
// node cap and a zero MaxDuration means no time cap; negative values
// are treated the same as zero.
type SearchBudget struct {
	MaxNodes    int
	MaxDuration time.Duration
}

// BoundedSearchResult is the outcome of MinMemorizeForBounded.
//
// Set is always a feasible memorize set: Closure(prior + Set) derives
// every target. When Exact is true the search ran to completion and
// Set is an optimum, so LowerBound == UpperBound == len(Set). When the
// search stopped early, Set is the best incumbent found so far,
// Exact is false, and LowerBound <= optimum <= UpperBound with
// Gap() = UpperBound - LowerBound. A stopped search is never reported
// exact: check Exact (or Status()) rather than the gap.
type BoundedSearchResult struct {
	Set             []string
	Exact           bool
	LowerBound      int
	UpperBound      int
	NodesVisited    int
	BudgetExhausted bool
	Canceled        bool
}

// Gap returns UpperBound - LowerBound, the width of the honest
// interval holding the optimum.
func (r BoundedSearchResult) Gap() int {
	return r.UpperBound - r.LowerBound
}

// Status reports the proof status of the result: "optimal" when the
// search ran to completion, "budget-exhausted" when a node or time
// budget stopped it, and "canceled" when the context was canceled.
func (r BoundedSearchResult) Status() string {
	if r.Exact {
		return "optimal"
	}
	if r.Canceled {
		return "canceled"
	}
	return "budget-exhausted"
}

// MinMemorizeForBounded is the bounded form of MinMemorizeFor: the
// same exact branch-and-bound search, with context cancellation and
// node and time budgets. Closures inside the search run on memoized
// bitsets instead of fresh maps, and residuals are picked in
// deterministic topological order (lowest rank, ties by id).
//
// On budget exhaustion it returns the best incumbent found so far
// with an honest lower and upper bound, Exact false, and
// BudgetExhausted true. On context cancellation it returns the
// incumbent found so far, Canceled true, and the context error. The
// incumbent is always feasible, and the bounds always bracket the
// optimum. With no budget the search runs to completion and the
// result is exact, identical to MinMemorizeFor.
func (g *Graph) MinMemorizeForBounded(ctx context.Context, targets []string, prior []string, budget SearchBudget) (BoundedSearchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ids := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		ids[n] = true
	}
	for _, t := range targets {
		if !ids[t] {
			return BoundedSearchResult{}, fmt.Errorf("unknown formula id: %s", t)
		}
	}
	for _, p := range prior {
		if !ids[p] {
			return BoundedSearchResult{}, fmt.Errorf("unknown prior id: %s", p)
		}
	}
	s := g.newBoundedSearch(ctx, budget)
	priorSet := toSet(prior)
	base := g.Closure(priorSet)
	need := map[string]bool{}
	for _, t := range targets {
		if !base[t] {
			need[t] = true
		}
	}
	// Memorizing every unknown root always derives everything, so it is
	// a valid starting upper bound.
	upper := g.MinMemorizeSet(prior)
	s.best = &bbBest{set: upper, size: len(upper)}
	if len(need) == 0 {
		return BoundedSearchResult{Set: []string{}, Exact: true}, nil
	}
	s.branch(s.bitsOf(sortedKeys(need)), newBitSet(len(g.Nodes)), s.bitsOf(sortedKeys(priorSet)))
	res := BoundedSearchResult{
		Set:          s.best.set,
		UpperBound:   s.best.size,
		NodesVisited: s.nodes,
		Canceled:     s.canceled,
	}
	if s.canceled {
		res.LowerBound = s.lowerBound()
		return res, ctx.Err()
	}
	if s.exhausted {
		res.BudgetExhausted = true
		res.LowerBound = s.lowerBound()
		return res, nil
	}
	res.Exact = true
	res.LowerBound = s.best.size
	return res, nil
}

// topoRankFromPaths returns a topological rank (lower = closer to roots)
// derived from the Paths derivation graph. Nodes with no paths are roots.
func (g *Graph) topoRankFromPaths() map[string]int {
	// indeg = number of distinct prerequisites across all paths
	indeg := make(map[string]int, len(g.Nodes))
	children := make(map[string][]string)
	for _, n := range g.Nodes {
		prereqs := map[string]bool{}
		for _, path := range g.Paths[n] {
			for _, p := range path {
				prereqs[p] = true
			}
		}
		indeg[n] = len(prereqs)
		for p := range prereqs {
			children[p] = append(children[p], n)
		}
	}
	var queue []string
	for _, n := range g.Nodes {
		if indeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	sort.Strings(queue)
	rank := make(map[string]int, len(g.Nodes))
	pos := 0
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		rank[u] = pos
		pos++
		for _, v := range children[u] {
			indeg[v]--
			if indeg[v] == 0 {
				queue = append(queue, v)
			}
		}
		sort.Strings(queue)
	}
	// Any nodes not reached (cycles) get a high rank; they sort last.
	for _, n := range g.Nodes {
		if _, ok := rank[n]; !ok {
			rank[n] = pos
			pos++
		}
	}
	return rank
}

type bbBest struct {
	set  []string
	size int
}

// bitSet is a fixed-width set of node indices, used for memoized
// closures inside the bounded search.
type bitSet []uint64

func newBitSet(n int) bitSet {
	return make(bitSet, (n+63)/64)
}

func (b bitSet) clone() bitSet {
	out := make(bitSet, len(b))
	copy(out, b)
	return out
}

func (b bitSet) has(i int) bool {
	return b[i/64]&(1<<uint(i%64)) != 0
}

func (b bitSet) set(i int) {
	b[i/64] |= 1 << uint(i%64)
}

func (b bitSet) count() int {
	n := 0
	for _, w := range b {
		n += bits.OnesCount64(w)
	}
	return n
}

func (b bitSet) empty() bool {
	for _, w := range b {
		if w != 0 {
			return false
		}
	}
	return true
}

// subsetOf reports whether every member of b is a member of other.
func (b bitSet) subsetOf(other bitSet) bool {
	for i, w := range b {
		if w&^other[i] != 0 {
			return false
		}
	}
	return true
}

// key encodes the set as a string for memo maps.
func (b bitSet) key() string {
	buf := make([]byte, 0, len(b)*8)
	var tmp [8]byte
	for _, w := range b {
		binary.LittleEndian.PutUint64(tmp[:], w)
		buf = append(buf, tmp[:]...)
	}
	return string(buf)
}

func unionBitSets(a, b bitSet) bitSet {
	out := a.clone()
	for i, w := range b {
		out[i] |= w
	}
	return out
}

// bbSearch is the mutable state of one bounded branch-and-bound run.
// The graph itself is never mutated; all search state lives here.
type bbSearch struct {
	g           *Graph
	ids         []string // index -> node id
	idx         map[string]int
	pathBits    [][]bitSet // per node index: one bitset per derivation path
	hasPaths    []bool
	topoRank    map[string]int
	ctx         context.Context
	budget      SearchBudget
	start       time.Time
	best        *bbBest
	memo        map[string]int
	closureMemo map[string]bitSet
	closureRuns int
	closureHits int
	nodes       int
	exhausted   bool
	canceled    bool
	frontierLB  int // minimum lower bound over budget-cut nodes; -1 when none recorded
}

// newBoundedSearch builds the search state for g, precomputing the
// node index and the per-path bitsets the memoized closure runs on.
func (g *Graph) newBoundedSearch(ctx context.Context, budget SearchBudget) *bbSearch {
	s := &bbSearch{
		g:           g,
		ids:         append([]string{}, g.Nodes...),
		idx:         make(map[string]int, len(g.Nodes)),
		topoRank:    g.topoRankFromPaths(),
		ctx:         ctx,
		budget:      budget,
		start:       time.Now(),
		memo:        map[string]int{},
		closureMemo: map[string]bitSet{},
		frontierLB:  -1,
	}
	for i, n := range g.Nodes {
		s.idx[n] = i
	}
	s.pathBits = make([][]bitSet, len(g.Nodes))
	s.hasPaths = make([]bool, len(g.Nodes))
	for i, n := range g.Nodes {
		paths := g.Paths[n]
		s.hasPaths[i] = len(paths) > 0
		s.pathBits[i] = make([]bitSet, len(paths))
		for j, p := range paths {
			bs := newBitSet(len(g.Nodes))
			for _, m := range p {
				bs.set(s.idx[m])
			}
			s.pathBits[i][j] = bs
		}
	}
	return s
}

// bitsOf converts an id list to its index bitset.
func (s *bbSearch) bitsOf(ids []string) bitSet {
	bs := newBitSet(len(s.ids))
	for _, id := range ids {
		if i, ok := s.idx[id]; ok {
			bs.set(i)
		}
	}
	return bs
}

// closureBits returns the derivation closure of known as a bitset,
// memoized on the known set so repeated states share one fixpoint
// run. The returned bitset is owned by the memo and must not be
// mutated by callers.
func (s *bbSearch) closureBits(known bitSet) bitSet {
	key := known.key()
	if c, ok := s.closureMemo[key]; ok {
		s.closureHits++
		return c
	}
	s.closureRuns++
	out := known.clone()
	for changed := true; changed; {
		changed = false
		for i := range s.ids {
			if out.has(i) {
				continue
			}
			for _, p := range s.pathBits[i] {
				if p.subsetOf(out) {
					out.set(i)
					changed = true
					break
				}
			}
		}
	}
	s.closureMemo[key] = out
	return out
}

// branch searches memorize sets covering need. mem only grows and
// need only grows, so the search terminates. It mirrors the
// unbounded search exactly: same seeding, same pruning, same
// memo rule, same child order, so an unlimited budget returns the
// same set as MinMemorizeFor.
//
// The node and time budgets and the context are checked at every
// node. A cut node records a lower bound for its subtree instead of
// expanding, and a node visited after the budget is spent records
// its bound and returns without expanding, so the global lower
// bound stays valid while the remaining work stays small.
func (s *bbSearch) branch(need, mem, prior bitSet) {
	if s.canceled {
		return
	}
	known := s.closureBits(unionBitSets(prior, mem))
	resid := need.clone()
	for i, w := range known {
		resid[i] &^= w
	}
	if resid.empty() {
		if mem.count() < s.best.size {
			s.best.size = mem.count()
			s.best.set = s.idsOf(mem)
		}
		return
	}
	if mem.count() >= s.best.size {
		return
	}
	// Same residual need and known set reached with no more
	// memorizations cannot lead to a better solution.
	key := resid.key() + "|" + known.key()
	if prev, ok := s.memo[key]; ok && prev <= mem.count() {
		return
	}
	s.memo[key] = mem.count()
	if s.exhausted {
		s.recordFrontier(mem, resid)
		return
	}
	if s.budget.MaxNodes > 0 && s.nodes >= s.budget.MaxNodes {
		s.exhausted = true
		s.recordFrontier(mem, resid)
		return
	}
	if s.budget.MaxDuration > 0 && time.Since(s.start) >= s.budget.MaxDuration {
		s.exhausted = true
		s.recordFrontier(mem, resid)
		return
	}
	if err := s.ctx.Err(); err != nil {
		s.canceled = true
		return
	}
	s.nodes++
	r := s.pickResidual(resid)
	// Derive r through each of its paths: every path member must be known.
	for _, path := range s.g.Paths[s.ids[r]] {
		need2 := need.clone()
		grew := false
		for _, p := range path {
			pi := s.idx[p]
			if !need2.has(pi) {
				need2.set(pi)
				grew = true
			}
		}
		if grew {
			s.branch(need2, mem, prior)
			if s.canceled {
				return
			}
		}
	}
	// Or memorize r outright.
	mem2 := mem.clone()
	mem2.set(r)
	s.branch(need, mem2, prior)
}

// pickResidual returns the index of the topologically earliest
// residual node, ties broken by id: the deterministic topological
// queue order for the search.
func (s *bbSearch) pickResidual(resid bitSet) int {
	best := -1
	for i := range s.ids {
		if !resid.has(i) {
			continue
		}
		if best < 0 || s.topoRank[s.ids[i]] < s.topoRank[s.ids[best]] ||
			(s.topoRank[s.ids[i]] == s.topoRank[s.ids[best]] && s.ids[i] < s.ids[best]) {
			best = i
		}
	}
	return best
}

// recordFrontier records the lower bound of one budget-cut subtree:
// the memorizations already made, plus one per residual root, since
// a root has no derivation path and can only become known by being
// memorized. The global lower bound is the minimum over all cut
// subtrees, clamped to the incumbent upper bound.
func (s *bbSearch) recordFrontier(mem, resid bitSet) {
	lb := mem.count()
	for i := range s.ids {
		if resid.has(i) && !s.hasPaths[i] {
			lb++
		}
	}
	if s.frontierLB < 0 || lb < s.frontierLB {
		s.frontierLB = lb
	}
}

// lowerBound returns the global lower bound: the minimum frontier
// bound, clamped to the incumbent upper bound, or 0 when no node
// was ever cut.
func (s *bbSearch) lowerBound() int {
	if s.frontierLB < 0 {
		return 0
	}
	if s.frontierLB > s.best.size {
		return s.best.size
	}
	return s.frontierLB
}

// idsOf returns the sorted ids in a bitset.
func (s *bbSearch) idsOf(bs bitSet) []string {
	out := []string{}
	for i := range s.ids {
		if bs.has(i) {
			out = append(out, s.ids[i])
		}
	}
	sort.Strings(out)
	return out
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
//
// Zero-node contract: an empty graph is vacuously fully covered, so the
// curve is a single point (0, 1.0). Older code divided by the node count
// here and produced NaN; every fraction is now finite and JSON-safe.
func (g *Graph) CoverageCurve() []CoveragePoint {
	if len(g.Nodes) == 0 {
		return []CoveragePoint{{K: 0, Fraction: 1.0}}
	}
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
