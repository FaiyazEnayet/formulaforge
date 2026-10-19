package graph

import "testing"

func TestMinMemorizeForSharedPrereq(t *testing.T) {
    // Counterexample: roots x,y; z derives from x,y; a,b,c derive from z.
    // Targets a,b,c. Optimum is memorizing z alone (size 1).
    g := &Graph{
        Nodes: []string{"x", "y", "z", "a", "b", "c"},
        Paths: map[string][][]string{
            "x": {},
            "y": {},
            "z": {{"x", "y"}},
            "a": {{"z"}},
            "b": {{"z"}},
            "c": {{"z"}},
        },
    }
    ms, err := g.MinMemorizeFor([]string{"a", "b", "c"}, nil)
    if err != nil {
        t.Fatal(err)
    }
    if len(ms) != 1 {
        t.Errorf("got %v (size %d), want size 1 (memorize z)", ms, len(ms))
    }
}

func TestMinMemorizeForPermutationRobust(t *testing.T) {
    // The optimum must not depend on node ID labels. Relabel the
    // counterexample's nodes and verify cardinality 1 is still found.
    relabels := map[string]string{
        "x": "n1", "y": "n2", "z": "n3",
        "a": "n4", "b": "n5", "c": "n6",
    }
    // Also try a relabeling that reverses lexicographic vs topological order.
    relabels2 := map[string]string{
        "x": "z9", "y": "z8", "z": "a1",
        "a": "z7", "b": "z6", "c": "z5",
    }
    for _, rl := range []map[string]string{relabels, relabels2} {
        g := &Graph{Nodes: []string{}}
        paths := map[string][][]string{}
        for _, n := range []string{"x", "y", "z", "a", "b", "c"} {
            rn := rl[n]
            g.Nodes = append(g.Nodes, rn)
            var rpaths [][]string
            for _, p := range map[string][][]string{
                "x": {}, "y": {}, "z": {{"x", "y"}},
                "a": {{"z"}}, "b": {{"z"}}, "c": {{"z"}},
            }[n] {
                rp := make([]string, len(p))
                for i, q := range p {
                    rp[i] = rl[q]
                }
                rpaths = append(rpaths, rp)
            }
            paths[rn] = rpaths
        }
        g.Paths = paths
        targets := []string{rl["a"], rl["b"], rl["c"]}
        ms, err := g.MinMemorizeFor(targets, nil)
        if err != nil {
            t.Fatal(err)
        }
        if len(ms) != 1 {
            t.Errorf("relabel %v: got %v (size %d), want size 1", rl, ms, len(ms))
        }
    }
}
