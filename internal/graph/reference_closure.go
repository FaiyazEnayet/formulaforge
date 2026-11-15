package graph

import "sort"

// Reference oracle for the derivation semantics (FF-04).
//
// This module is an independent implementation of the closure and
// exact-optimum computations, written from the semantics stated in
// the package documentation, not from the production code. It exists
// so the differential tests can catch a defect in the production
// closure or solver: the earlier brute-force oracle checked
// candidates with the production Graph.Closure, so a closure defect
// was invisible to it.
//
// Independence is structural, not just textual:
//
//   - Production Closure rescans every node and every path
//     repeatedly until a fixpoint is reached. referenceClosure runs
//     forward chaining over reverse incidence lists: each derivation
//     path counts its unmet prerequisites, each prerequisite points
//     at the paths waiting on it, and a path fires the moment its
//     count reaches zero.
//   - Production MinMemorizeFor is branch and bound with memoized
//     bitset closures. referenceBruteMinFor enumerates subsets by
//     increasing size and checks each one with the reference
//     closure only, so it shares no search code and no closure code
//     with the solver.
//
// The reference functions are test oracles. No production code path
// calls them; they are exercised only by the property tests in
// graph_property_test.go. They are deliberately exponential where
// the solver is clever: referenceBruteMinFor is only ever run on
// bounded graphs (n <= 12 in the tests).

// referenceClosure returns the derivation closure of known: the
// starting set plus everything derivable from it under
// AND-within-a-path, OR-across-paths semantics.
//
// Only ids mapped to true count as known; ids mapped to false and
// ids absent from the map are not known, and unknown ids in the map
// are ignored. A formula with no derivation paths is a root and is
// never derived. A derivation path with no members is satisfied at
// once. A path naming a prerequisite that is not a graph node can
// never fire.
func referenceClosure(g *Graph, known map[string]bool) map[string]bool {
	isNode := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		isNode[n] = true
	}
	out := make(map[string]bool, len(known))
	var queue []string
	enqueue := func(id string) {
		if !out[id] {
			out[id] = true
			queue = append(queue, id)
		}
	}
	for id, v := range known {
		if v && isNode[id] {
			enqueue(id)
		}
	}

	// One entry per derivation path, with its still-unmet
	// prerequisite count. waiting maps a prerequisite id to the
	// paths that need it to become known.
	type refPath struct {
		node      string
		remaining int
	}
	waiting := make(map[string][]*refPath)
	for _, n := range g.Nodes {
		for _, path := range g.Paths[n] {
			seen := make(map[string]bool, len(path))
			entry := &refPath{node: n}
			for _, m := range path {
				if seen[m] {
					continue
				}
				seen[m] = true
				entry.remaining++
				waiting[m] = append(waiting[m], entry)
			}
			if entry.remaining == 0 {
				enqueue(n)
			}
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, entry := range waiting[id] {
			if out[entry.node] {
				continue
			}
			entry.remaining--
			if entry.remaining == 0 {
				enqueue(entry.node)
			}
		}
	}
	return out
}

// referenceDerives reports whether every target is derivable from
// the known set under the reference closure.
func referenceDerives(g *Graph, known map[string]bool, targets []string) bool {
	closed := referenceClosure(g, known)
	for _, t := range targets {
		if !closed[t] {
			return false
		}
	}
	return true
}

// referenceBruteMinFor returns the exact smallest memorize set
// covering targets given prior knowledge, computed by exhaustive
// subset enumeration through the reference closure only. Ties are
// broken lexicographically (smallest sorted id list first). It
// returns nil when no feasible set exists, which happens only for
// unknown target or prior ids.
func referenceBruteMinFor(g *Graph, targets, prior []string) []string {
	optima := referenceAllOptima(g, targets, prior)
	if len(optima) == 0 {
		return nil
	}
	return optima[0]
}

// referenceAllOptima returns every minimum-size memorize set
// covering targets given prior knowledge, in lexicographic order.
// It returns an empty slice when no feasible set exists.
func referenceAllOptima(g *Graph, targets, prior []string) [][]string {
	ids := append([]string{}, g.Nodes...)
	sort.Strings(ids)
	isNode := make(map[string]bool, len(ids))
	for _, n := range ids {
		isNode[n] = true
	}
	for _, t := range targets {
		if !isNode[t] {
			return nil
		}
	}
	priorSet := toSet(prior)
	for p := range priorSet {
		if !isNode[p] {
			return nil
		}
	}
	// Subsets of increasing size, in lexicographic order of the
	// sorted id list, so the first feasible size yields the
	// lexicographically smallest optimum first.
	for size := 0; size <= len(ids); size++ {
		var optima [][]string
		comb := make([]int, size)
		for i := range comb {
			comb[i] = i
		}
		for {
			mem := make([]string, size)
			for i, idx := range comb {
				mem[i] = ids[idx]
			}
			known := make(map[string]bool, len(priorSet)+len(mem))
			for k := range priorSet {
				known[k] = true
			}
			for _, m := range mem {
				known[m] = true
			}
			if referenceDerives(g, known, targets) {
				optima = append(optima, mem)
			}
			if !nextCombination(comb, len(ids)) {
				break
			}
		}
		if len(optima) > 0 {
			return optima
		}
	}
	return nil
}

// nextCombination advances comb to the next size-combination of
// [0, n) in lexicographic order, reporting false when exhausted.
func nextCombination(comb []int, n int) bool {
	k := len(comb)
	for i := k - 1; i >= 0; i-- {
		if comb[i] < n-k+i {
			comb[i]++
			for j := i + 1; j < k; j++ {
				comb[j] = comb[j-1] + 1
			}
			return true
		}
	}
	return false
}
