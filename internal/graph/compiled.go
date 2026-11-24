package graph

// Compiled closure (FF-05): forward chaining over reverse
// incidence lists.
//
// The original Closure rescanned every node and every derivation
// path until a fixpoint was reached, so a closure cost grew with
// the number of fixpoint passes times the whole sheet. The branch
// search calls a closure at each recursive state and CoverageCurve
// tries a fresh closure for every candidate at each step, so that
// rescanning was the hot path for the solver and the curve.
//
// A compiledGraph compiles each conjunctive derivation path once:
//
//   - pathNode / pathMembers hold the consequent and the deduped
//     prerequisite indices of each path (integer-indexed nodes, in
//     Graph.Nodes order);
//   - waiting maps a prerequisite index to the compiled paths that
//     need it to become known (reverse incidence lists);
//   - emptyPaths holds the paths with no prerequisites, which fire
//     at once.
//
// A closure run then tracks the remaining unmet prerequisites per
// path. When a node becomes known, only the paths waiting on it
// are touched; a consequent fires the moment one of its paths
// reaches zero. Each path fires at most once and each incidence
// edge is visited at most once, so a run costs O(paths + incidence
// edges) instead of O(passes x nodes x paths). This is forward
// chaining for acyclic Horn-style rules: AND within a path,
// OR across paths, exactly the semantics in the package contract.
//
// The compiled form is built once per Graph in newGraph and kept
// on the Graph; compiledForm also compiles lazily for a Graph
// built by hand, which only the tests do. Compilation only reads
// the Graph, and a compiled closure never mutates it, so sharing
// one compiled form across calls is safe.
//
// Edge cases, matching the original Closure exactly:
//
//   - A path naming a prerequisite that is not a graph node can
//     never fire, so it is left out of the incidence lists.
//   - A path with a duplicated member counts that member once.
//   - A path with no members fires at once.
//   - A self-referential path can only fire after its own node
//     became known another way, which the done check enforces.

// compiledGraph is the compiled form of a Graph's derivation paths.
type compiledGraph struct {
	n int // node count; index i is Graph.Nodes[i]

	// pathNode[pi] is the consequent node index of compiled path pi.
	pathNode []int
	// pathMembers[pi] is the deduped prerequisite indices of path pi.
	pathMembers [][]int
	// waiting[p] lists the compiled paths that need node p to
	// become known.
	waiting [][]int
	// emptyPaths lists the compiled paths with no prerequisites.
	emptyPaths []int
}

// compileGraph compiles g's derivation paths into reverse
// incidence lists. Paths are visited in Graph.Nodes order, and
// within a node in the stored path order, so compilation is
// deterministic for a fixed sheet.
func compileGraph(g *Graph) *compiledGraph {
	c := &compiledGraph{
		n:       len(g.Nodes),
		waiting: make([][]int, len(g.Nodes)),
	}
	idx := make(map[string]int, len(g.Nodes))
	for i, n := range g.Nodes {
		idx[n] = i
	}
	for _, n := range g.Nodes {
		node, ok := idx[n]
		if !ok {
			continue
		}
		for _, path := range g.Paths[n] {
			// Dedupe members (paths are stored sorted, so
			// duplicates sit side by side) and resolve indices.
			// A member that is not a graph node can never become
			// known, so a path naming one is dead: it is left
			// out of the incidence lists entirely.
			members := make([]int, 0, len(path))
			dead := false
			for _, m := range path {
				i, ok := idx[m]
				if !ok {
					dead = true
					break
				}
				if len(members) > 0 && members[len(members)-1] == i {
					continue
				}
				members = append(members, i)
			}
			if dead {
				continue
			}
			pi := len(c.pathNode)
			c.pathNode = append(c.pathNode, node)
			c.pathMembers = append(c.pathMembers, members)
			if len(members) == 0 {
				c.emptyPaths = append(c.emptyPaths, pi)
				continue
			}
			for _, m := range members {
				c.waiting[m] = append(c.waiting[m], pi)
			}
		}
	}
	return c
}

// compiledForm returns the compiled form of g, compiling it on
// first use when the Graph was not built by newGraph.
func (g *Graph) compiledForm() *compiledGraph {
	if g.compiled == nil {
		g.compiled = compileGraph(g)
	}
	return g.compiled
}

// closure returns the derivation closure of seed by forward
// chaining: the seed plus everything derivable from it. seed is
// indexed like Graph.Nodes; entries past the node count are
// ignored and a short seed reads as false past its end. The call
// takes ownership of seed and reuses it as the result buffer, so
// callers must pass a freshly built slice. The returned slice is
// the same backing array as seed.
func (c *compiledGraph) closure(seed []bool) []bool {
	done := seed
	if len(done) < c.n {
		grown := make([]bool, c.n)
		copy(grown, done)
		done = grown
	}
	// One backing array holds the per-path remaining counts and
	// the work queue, keeping the run to a single scratch
	// allocation besides the result buffer.
	scratch := make([]int, len(c.pathNode), len(c.pathNode)+c.n)
	remaining := scratch
	queue := scratch[len(scratch):len(scratch):cap(scratch)]
	for i := 0; i < c.n; i++ {
		if done[i] {
			queue = append(queue, i)
		}
	}
	for pi, members := range c.pathMembers {
		remaining[pi] = len(members)
	}
	for _, pi := range c.emptyPaths {
		if node := c.pathNode[pi]; !done[node] {
			done[node] = true
			queue = append(queue, node)
		}
	}
	for head := 0; head < len(queue); head++ {
		id := queue[head]
		for _, pi := range c.waiting[id] {
			node := c.pathNode[pi]
			if done[node] {
				continue
			}
			remaining[pi]--
			if remaining[pi] == 0 {
				done[node] = true
				queue = append(queue, node)
			}
		}
	}
	return done
}

// closureBits is the bitset form of closure, for the branch search:
// the derivation closure of known, with indices in Graph.Nodes
// order. The returned bitset is freshly allocated on every call.
func (c *compiledGraph) closureBits(known bitSet) bitSet {
	done := known.clone()
	// One backing array holds the per-path remaining counts and
	// the work queue, keeping the run to a single scratch
	// allocation besides the result bitset.
	scratch := make([]int, len(c.pathNode), len(c.pathNode)+c.n)
	remaining := scratch
	queue := scratch[len(scratch):len(scratch):cap(scratch)]
	for i := 0; i < c.n; i++ {
		if done.has(i) {
			queue = append(queue, i)
		}
	}
	for pi, members := range c.pathMembers {
		remaining[pi] = len(members)
	}
	for _, pi := range c.emptyPaths {
		if node := c.pathNode[pi]; !done.has(node) {
			done.set(node)
			queue = append(queue, node)
		}
	}
	for head := 0; head < len(queue); head++ {
		id := queue[head]
		for _, pi := range c.waiting[id] {
			node := c.pathNode[pi]
			if done.has(node) {
				continue
			}
			remaining[pi]--
			if remaining[pi] == 0 {
				done.set(node)
				queue = append(queue, node)
			}
		}
	}
	return done
}
