package search

import (
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

func allSheets(t *testing.T) map[string]*models.Sheet {
	t.Helper()
	s, err := models.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTokenize(t *testing.T) {
	got := Tokenize("Green's Theorem!")
	want := []string{"green", "s", "theorem"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestTokenizeEmpty(t *testing.T) {
	if len(Tokenize("")) != 0 {
		t.Error("empty string should give no tokens")
	}
	if len(Tokenize("!!!")) != 0 {
		t.Error("punctuation should give no tokens")
	}
}

func TestFindsByName(t *testing.T) {
	hits := Search(allSheets(t), "stokes", 10)
	if len(hits) == 0 {
		t.Fatal("no hits for 'stokes'")
	}
	if hits[0].Form.ID != "stokes" {
		t.Errorf("top hit = %s, want stokes", hits[0].Form.ID)
	}
}

func TestFindsAcrossSheets(t *testing.T) {
	hits := Search(allSheets(t), "induction", 20)
	ids := map[string]bool{}
	for _, h := range hits {
		ids[h.Form.ID] = true
	}
	for _, want := range []string{"weak-induction"} {
		if !ids[want] {
			t.Errorf("missing %s in results", want)
		}
	}
}

func TestTopicWeighted(t *testing.T) {
	hits := Search(allSheets(t), "z-score", 10)
	if len(hits) == 0 {
		t.Fatal("no hits")
	}
	if hits[0].Sheet != "psy103" {
		t.Errorf("top sheet = %s, want psy103", hits[0].Sheet)
	}
}

func TestEmptyQueryNoResults(t *testing.T) {
	if len(Search(allSheets(t), "", 10)) != 0 {
		t.Error("empty query should return nothing")
	}
	if len(Search(allSheets(t), "zzzqqq", 10)) != 0 {
		t.Error("nonsense query should return nothing")
	}
}

func TestTopKRespected(t *testing.T) {
	hits := Search(allSheets(t), "the", 3)
	if len(hits) > 3 {
		t.Errorf("got %d hits, want <= 3", len(hits))
	}
}

func TestNameBeatsLatex(t *testing.T) {
	// "z-score" appears in the NAME of psy103's z-score formula; latex-only
	// matches score lower.
	hits := Search(allSheets(t), "z-score", 10)
	if len(hits) == 0 {
		t.Fatal("no hits")
	}
	if hits[0].Form.ID != "z-score" || hits[0].Sheet != "psy103" {
		t.Errorf("top hit = %s/%s, want psy103/z-score", hits[0].Sheet, hits[0].Form.ID)
	}
}

func TestScoreOrdering(t *testing.T) {
	hits := Search(allSheets(t), "correlation", 10)
	if len(hits) < 2 {
		t.Fatal("expected multiple hits")
	}
	for i := 1; i < len(hits); i++ {
		if hits[i].Score > hits[i-1].Score {
			t.Fatal("hits not sorted by score desc")
		}
	}
}

func TestCaseInsensitive(t *testing.T) {
	a := Search(allSheets(t), "NEWTON", 10)
	b := Search(allSheets(t), "newton", 10)
	if len(a) != len(b) {
		t.Error("search should be case-insensitive")
	}
}

func TestSheetFilterViaMap(t *testing.T) {
	sheets := allSheets(t)
	one := map[string]*models.Sheet{"psy103": sheets["psy103"]}
	hits := Search(one, "correlation", 10)
	for _, h := range hits {
		if h.Sheet != "psy103" {
			t.Error("sheet filter leaked")
		}
	}
}
