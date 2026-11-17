package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// MigrationChange is one enumerated difference between the derivation
// graph a sheet produced before typed relationships (every path
// treated as certified) and the graph it produces now (only certified
// paths enter Closure and optimization). Every change is listed;
// nothing is summarized away.
//
// Kind is one of "root-added", "root-removed", "edge-added",
// "edge-removed", or "curve-changed". Detail names the affected
// item: a root id, an edge as "from->to", or a coverage-curve point
// as "K=<k>: <before> -> <after>".
type MigrationChange struct {
	Kind   string
	Detail string
}

// MigrationReport is the full typed-relationship migration effect for
// one sheet. An empty Changes list means the sheet is unaffected:
// its roots, edges, and coverage curve are exactly what they were
// before typing.
type MigrationReport struct {
	Sheet   string
	Changes []MigrationChange
}

// Changed reports whether the migration changed anything at all for
// the sheet.
func (r *MigrationReport) Changed() bool {
	return len(r.Changes) > 0
}

// String renders the report as a readable multi-line summary.
func (r *MigrationReport) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "migration report for sheet %s: %d change(s)\n", r.Sheet, len(r.Changes))
	for _, ch := range r.Changes {
		fmt.Fprintf(&b, "  %s %s\n", ch.Kind, ch.Detail)
	}
	return b.String()
}

func migrationRoots(g *Graph) map[string]bool {
	roots := map[string]bool{}
	for _, n := range g.Nodes {
		if len(g.Paths[n]) == 0 {
			roots[n] = true
		}
	}
	return roots
}

func migrationEdges(g *Graph) map[string]bool {
	edges := map[string]bool{}
	for _, n := range g.Nodes {
		for _, p := range g.Prereqs[n] {
			edges[p+"->"+n] = true
		}
	}
	return edges
}

// MigrationReportFor compares the derivation graph a sheet produced
// before typed relationships with the graph it produces now, and
// enumerates every changed root, edge, and coverage-curve point. The
// comparison is exact: roots and edges are sets, and curve fractions
// are compared bit-for-bit (the engine is deterministic, so any real
// difference is a change worth listing).
func MigrationReportFor(sheet *models.Sheet) *MigrationReport {
	before := newGraph(sheet, false)
	after := newGraph(sheet, true)
	rep := &MigrationReport{Sheet: sheet.Course}

	beforeRoots, afterRoots := migrationRoots(before), migrationRoots(after)
	var added, removed []string
	for n := range afterRoots {
		if !beforeRoots[n] {
			added = append(added, n)
		}
	}
	for n := range beforeRoots {
		if !afterRoots[n] {
			removed = append(removed, n)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	for _, n := range added {
		rep.Changes = append(rep.Changes, MigrationChange{Kind: "root-added", Detail: n})
	}
	for _, n := range removed {
		rep.Changes = append(rep.Changes, MigrationChange{Kind: "root-removed", Detail: n})
	}

	beforeEdges, afterEdges := migrationEdges(before), migrationEdges(after)
	var addedEdges, removedEdges []string
	for e := range afterEdges {
		if !beforeEdges[e] {
			addedEdges = append(addedEdges, e)
		}
	}
	for e := range beforeEdges {
		if !afterEdges[e] {
			removedEdges = append(removedEdges, e)
		}
	}
	sort.Strings(addedEdges)
	sort.Strings(removedEdges)
	for _, e := range addedEdges {
		rep.Changes = append(rep.Changes, MigrationChange{Kind: "edge-added", Detail: e})
	}
	for _, e := range removedEdges {
		rep.Changes = append(rep.Changes, MigrationChange{Kind: "edge-removed", Detail: e})
	}

	beforeCurve, afterCurve := before.CoverageCurve(), after.CoverageCurve()
	n := len(beforeCurve)
	if len(afterCurve) < n {
		n = len(afterCurve)
	}
	for i := 0; i < n; i++ {
		if beforeCurve[i].K != afterCurve[i].K || beforeCurve[i].Fraction != afterCurve[i].Fraction {
			rep.Changes = append(rep.Changes, MigrationChange{
				Kind:   "curve-changed",
				Detail: fmt.Sprintf("K=%d: %.4f -> %.4f", beforeCurve[i].K, beforeCurve[i].Fraction, afterCurve[i].Fraction),
			})
		}
	}
	if len(beforeCurve) != len(afterCurve) {
		rep.Changes = append(rep.Changes, MigrationChange{
			Kind:   "curve-changed",
			Detail: fmt.Sprintf("length %d -> %d", len(beforeCurve), len(afterCurve)),
		})
	}
	return rep
}
