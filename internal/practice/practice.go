// Package practice implements derivation quizzes.
//
// Three modes:
//   - derive: given a formula, name the formulas it directly builds on.
//   - order:  given two formulas, say which must be learned first.
//   - state:  given a name, recall the formula (self-graded).
//
// Question generators take a seed for reproducibility; answer providers
// are functions so tests can script them and the CLI can wire up stdin.
package practice

import (
	"math/rand"
	"sort"
	"strings"

	"github.com/FaiyazEnayet/formulaforge/internal/graph"
)

// Question types.
const (
	TypeDerive = "derive"
	TypeOrder  = "order"
	TypeState  = "state"
)

// Question is one quiz prompt.
type Question struct {
	Type     string
	Formula  string // derive + state: formula id
	First    string // order: one side
	Second   string // order: other side
	Answer   string // order: the id that must be learned first
}

// GenDeriveQuestions picks formulas that have at least one prerequisite.
func GenDeriveQuestions(g *graph.Graph, n int, seed int64) []Question {
	rng := rand.New(rand.NewSource(seed))
	var cands []string
	for _, f := range g.Sheet.Formulas {
		if len(g.Prereqs[f.ID]) > 0 {
			cands = append(cands, f.ID)
		}
	}
	rng.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })
	if n > len(cands) {
		n = len(cands)
	}
	out := make([]Question, 0, n)
	for _, id := range cands[:n] {
		out = append(out, Question{Type: TypeDerive, Formula: id})
	}
	return out
}

// GenOrderQuestions picks pairs (a, b) where a is a strict ancestor of b.
func GenOrderQuestions(g *graph.Graph, n int, seed int64) []Question {
	rng := rand.New(rand.NewSource(seed))
	type pair struct{ a, b string }
	var pairs []pair
	for _, a := range g.Nodes {
		for b := range g.Descendants(a) {
			pairs = append(pairs, pair{a, b})
		}
	}
	rng.Shuffle(len(pairs), func(i, j int) { pairs[i], pairs[j] = pairs[j], pairs[i] })
	if n > len(pairs) {
		n = len(pairs)
	}
	out := make([]Question, 0, n)
	for _, p := range pairs[:n] {
		first, second := p.a, p.b
		if rng.Float64() >= 0.5 {
			first, second = p.b, p.a
		}
		out = append(out, Question{Type: TypeOrder, First: first, Second: second, Answer: p.a})
	}
	return out
}

// GenStateQuestions picks random formulas to recall.
func GenStateQuestions(g *graph.Graph, n int, seed int64) []Question {
	rng := rand.New(rand.NewSource(seed))
	ids := append([]string{}, g.Nodes...)
	rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	if n > len(ids) {
		n = len(ids)
	}
	out := make([]Question, 0, n)
	for _, id := range ids[:n] {
		out = append(out, Question{Type: TypeState, Formula: id})
	}
	return out
}

// DeriveResult scores a derive answer with precision/recall/F1.
type DeriveResult struct {
	Truth     []string
	Given     []string
	Precision float64
	Recall    float64
	F1        float64
}

// CheckDerive scores the given prerequisite ids against the truth.
func CheckDerive(g *graph.Graph, fid string, given []string) DeriveResult {
	truth := map[string]bool{}
	for _, p := range g.Prereqs[fid] {
		truth[p] = true
	}
	got := map[string]bool{}
	for _, s := range given {
		if t := strings.TrimSpace(s); t != "" {
			got[t] = true
		}
	}
	tp := 0
	for id := range got {
		if truth[id] {
			tp++
		}
	}
	precision := 0.0
	if len(got) > 0 {
		precision = float64(tp) / float64(len(got))
	}
	recall := 1.0
	if len(truth) > 0 {
		recall = float64(tp) / float64(len(truth))
	}
	f1 := 0.0
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}
	truthList := make([]string, 0, len(truth))
	for id := range truth {
		truthList = append(truthList, id)
	}
	sort.Strings(truthList)
	givenList := make([]string, 0, len(got))
	for id := range got {
		givenList = append(givenList, id)
	}
	sort.Strings(givenList)
	return DeriveResult{Truth: truthList, Given: givenList, Precision: precision, Recall: recall, F1: f1}
}

// CheckOrder reports whether the answer matches the expected first-learned id.
func CheckOrder(q Question, answer string) bool {
	return strings.TrimSpace(answer) == q.Answer
}

// Result is the graded outcome of one question.
type Result struct {
	Question   Question
	OK         bool
	Derive     DeriveResult // set for derive questions
	Answer     string       // order: raw answer given
	SelfReport string       // state: normalized y/n
}

// Summary aggregates a whole session.
type Summary struct {
	Results []Result
	Correct int
	Total   int
}

// AnswerOf provides an answer for a question (stdin in the CLI, scripted in tests).
type AnswerOf func(q Question) []string

// RunSession grades every question with the given answer provider.
func RunSession(g *graph.Graph, questions []Question, answerOf AnswerOf) Summary {
	var results []Result
	for _, q := range questions {
		switch q.Type {
		case TypeDerive:
			r := CheckDerive(g, q.Formula, answerOf(q))
			results = append(results, Result{Question: q, OK: r.F1 == 1.0, Derive: r})
		case TypeOrder:
			ans := answerOf(q)
			a := ""
			if len(ans) > 0 {
				a = ans[0]
			}
			results = append(results, Result{Question: q, OK: CheckOrder(q, a), Answer: a})
		case TypeState:
			ans := ""
			if a := answerOf(q); len(a) > 0 {
				ans = strings.ToLower(strings.TrimSpace(a[0]))
			}
			results = append(results, Result{Question: q, OK: ans == "y" || ans == "yes", SelfReport: ans})
		}
	}
	correct := 0
	for _, r := range results {
		if r.OK {
			correct++
		}
	}
	return Summary{Results: results, Correct: correct, Total: len(results)}
}
