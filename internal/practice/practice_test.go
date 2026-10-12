package practice

import (
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/graph"
	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

func loadGraph(t *testing.T, name string) *graph.Graph {
	t.Helper()
	s, err := models.LoadSheetByName(name)
	if err != nil {
		t.Fatal(err)
	}
	return graph.New(s)
}

func TestGenDeriveCounts(t *testing.T) {
	g := loadGraph(t, "phy131")
	qs := GenDeriveQuestions(g, 10, 7)
	if len(qs) != 10 {
		t.Fatalf("got %d questions", len(qs))
	}
	for _, q := range qs {
		if q.Type != TypeDerive {
			t.Error("wrong type")
		}
		if len(g.Prereqs[q.Formula]) == 0 {
			t.Errorf("derive question for root formula %s", q.Formula)
		}
	}
}

func TestGenDeriveSeedDeterministic(t *testing.T) {
	g := loadGraph(t, "phy131")
	a := GenDeriveQuestions(g, 10, 7)
	b := GenDeriveQuestions(g, 10, 7)
	c := GenDeriveQuestions(g, 10, 99)
	for i := range a {
		if a[i].Formula != b[i].Formula {
			t.Fatal("same seed gave different questions")
		}
	}
	same := true
	for i := range a {
		if a[i].Formula != c[i].Formula {
			same = false
		}
	}
	if same {
		t.Error("different seeds gave identical questions (suspicious)")
	}
}

func TestGenDeriveMoreThanAvailable(t *testing.T) {
	g := loadGraph(t, "ams261")
	qs := GenDeriveQuestions(g, 1000, 7)
	// every question needs a prereq; count eligible
	eligible := 0
	for _, f := range g.Sheet.Formulas {
		if len(g.Prereqs[f.ID]) > 0 {
			eligible++
		}
	}
	if len(qs) != eligible {
		t.Errorf("got %d, want %d", len(qs), eligible)
	}
}

func TestGenOrderAnswerCorrect(t *testing.T) {
	g := loadGraph(t, "mat203")
	qs := GenOrderQuestions(g, 20, 7)
	if len(qs) != 20 {
		t.Fatalf("got %d", len(qs))
	}
	for _, q := range qs {
		if q.Type != TypeOrder {
			t.Error("wrong type")
		}
		// answer must be a strict ancestor of the other side
		other := q.First
		if other == q.Answer {
			other = q.Second
		}
		if !g.Descendants(q.Answer)[other] {
			t.Errorf("answer %s is not an ancestor of %s", q.Answer, other)
		}
	}
}

func TestGenStateCounts(t *testing.T) {
	g := loadGraph(t, "psy103")
	qs := GenStateQuestions(g, 5, 7)
	if len(qs) != 5 {
		t.Fatalf("got %d", len(qs))
	}
	seen := map[string]bool{}
	for _, q := range qs {
		if seen[q.Formula] {
			t.Error("duplicate question")
		}
		seen[q.Formula] = true
	}
}

func TestCheckDerivePerfect(t *testing.T) {
	g := loadGraph(t, "phy131")
	// newton2 is a root; use a formula with prereqs
	var fid string
	for _, f := range g.Sheet.Formulas {
		if len(g.Prereqs[f.ID]) > 0 {
			fid = f.ID
			break
		}
	}
	r := CheckDerive(g, fid, g.Prereqs[fid])
	if r.F1 != 1.0 || r.Precision != 1.0 || r.Recall != 1.0 {
		t.Errorf("perfect answer should score 1.0: %+v", r)
	}
}

func TestCheckDerivePartial(t *testing.T) {
	g := loadGraph(t, "phy131")
	// find a formula with >= 2 prereqs
	var fid string
	for _, f := range g.Sheet.Formulas {
		if len(g.Prereqs[f.ID]) >= 2 {
			fid = f.ID
			break
		}
	}
	if fid == "" {
		t.Skip("no multi-prereq formula")
	}
	r := CheckDerive(g, fid, []string{g.Prereqs[fid][0], "bogus-id"})
	if r.Precision != 0.5 {
		t.Errorf("precision = %f, want 0.5", r.Precision)
	}
	if r.F1 <= 0 || r.F1 >= 1 {
		t.Errorf("partial F1 = %f, want in (0,1)", r.F1)
	}
}

func TestCheckDeriveEmpty(t *testing.T) {
	g := loadGraph(t, "phy131")
	var fid string
	for _, f := range g.Sheet.Formulas {
		if len(g.Prereqs[f.ID]) > 0 {
			fid = f.ID
			break
		}
	}
	r := CheckDerive(g, fid, nil)
	if r.F1 != 0.0 || r.Precision != 0.0 || r.Recall != 0.0 {
		t.Errorf("empty answer should score 0: %+v", r)
	}
}

func TestCheckOrder(t *testing.T) {
	q := Question{Type: TypeOrder, First: "a", Second: "b", Answer: "a"}
	if !CheckOrder(q, "a") {
		t.Error("correct answer rejected")
	}
	if CheckOrder(q, "b") {
		t.Error("wrong answer accepted")
	}
	if !CheckOrder(q, "  a  ") {
		t.Error("whitespace-padded answer rejected")
	}
}

func TestRunSessionAllCorrect(t *testing.T) {
	g := loadGraph(t, "ams261")
	qs := GenDeriveQuestions(g, 5, 7)
	answerOf := func(q Question) []string { return g.Prereqs[q.Formula] }
	s := RunSession(g, qs, answerOf)
	if s.Correct != 5 || s.Total != 5 {
		t.Errorf("got %d/%d", s.Correct, s.Total)
	}
}

func TestRunSessionAllWrong(t *testing.T) {
	g := loadGraph(t, "ams261")
	qs := GenDeriveQuestions(g, 5, 7)
	answerOf := func(q Question) []string { return []string{"nope"} }
	s := RunSession(g, qs, answerOf)
	if s.Correct != 0 {
		t.Errorf("got %d correct, want 0", s.Correct)
	}
}

func TestRunSessionMixedModes(t *testing.T) {
	g := loadGraph(t, "phy131")
	qs := append(GenDeriveQuestions(g, 2, 7), GenOrderQuestions(g, 2, 7)...)
	qs = append(qs, GenStateQuestions(g, 2, 7)...)
	answerOf := func(q Question) []string {
		switch q.Type {
		case TypeDerive:
			return g.Prereqs[q.Formula]
		case TypeOrder:
			return []string{q.Answer}
		default:
			return []string{"y"}
		}
	}
	s := RunSession(g, qs, answerOf)
	if s.Correct != s.Total || s.Total != 6 {
		t.Errorf("got %d/%d", s.Correct, s.Total)
	}
	// "n" counts as wrong for state
	answerNo := func(q Question) []string { return []string{"n"} }
	s2 := RunSession(g, GenStateQuestions(g, 3, 7), answerNo)
	if s2.Correct != 0 {
		t.Error("'n' should not count as recalled")
	}
}
