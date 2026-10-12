package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWritesSelfContainedHTML(t *testing.T) {
	out := filepath.Join(t.TempDir(), "dashboard.html")
	if err := Build(out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	if len(page) < 1_000_000 {
		t.Errorf("dashboard too small (%d bytes) — plotly.js may not be inlined", len(page))
	}
	for _, want := range []string{
		"plotly", "PSY 103", "Calculus III", "Classical Physics I", "Foundations of Computer Science I", "Foundations of Computer Science II",
		"dependency graph explorer", "katex", "min_set", "Study order timeline",
		"ams261", "phy131", "cse113", "cse213", "psy103",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("dashboard missing %q", want)
		}
	}
}

func TestPayloadForStats(t *testing.T) {
	sheets := map[string]string{"ams261": "AMS 261", "psy103": "PSY 103"}
	for name, course := range sheets {
		s := mustLoad(t, name)
		p, err := payloadFor(s)
		if err != nil {
			t.Fatal(err)
		}
		if p.Course != course {
			t.Errorf("course = %q", p.Course)
		}
		if len(p.Nodes) == 0 || len(p.Edges) == 0 {
			t.Error("empty nodes/edges")
		}
		if len(p.Timeline) != len(p.Nodes) {
			t.Error("timeline length mismatch")
		}
		if p.Curve[len(p.Curve)-1][1] != 1.0 {
			t.Error("coverage curve does not end at 1.0")
		}
	}
}

func TestPayloadMinSetSizes(t *testing.T) {
	want := map[string]int{"ams261": 5, "phy131": 9, "cse113": 8, "cse213": 13, "psy103": 5}
	for name, n := range want {
		p, err := payloadFor(mustLoad(t, name))
		if err != nil {
			t.Fatal(err)
		}
		if p.Stats.MinSize != n {
			t.Errorf("%s: min_size = %d, want %d", name, p.Stats.MinSize, n)
		}
	}
}
