// Package search implements full-text search over formula sheets.
// Tokenizes name, topic, tags, description, and LaTeX source; ranks by
// weighted term hits (name > topic/tags > description > latex).
package search

import (
	"regexp"
	"sort"
	"strings"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

var tokenRe = regexp.MustCompile(`[a-z0-9]+`)

// Tokenize lowercases text and extracts alphanumeric tokens.
func Tokenize(text string) []string {
	return tokenRe.FindAllString(strings.ToLower(text), -1)
}

func tokenSet(text string) map[string]bool {
	set := map[string]bool{}
	for _, t := range Tokenize(text) {
		set[t] = true
	}
	return set
}

// ScoreFormula ranks one formula against query terms.
func ScoreFormula(f *models.Formula, terms []string) float64 {
	nameToks := tokenSet(f.Name)
	topicToks := tokenSet(f.Topic)
	for _, t := range f.Tags {
		for _, tok := range Tokenize(t) {
			topicToks[tok] = true
		}
	}
	descToks := tokenSet(f.Description)
	latexToks := tokenSet(f.Latex)
	score := 0.0
	for _, term := range terms {
		hit := false
		if nameToks[term] {
			score += 3.0
			hit = true
		}
		if topicToks[term] {
			score += 2.0
			hit = true
		}
		if descToks[term] {
			score += 1.0
			hit = true
		}
		if !hit && latexToks[term] {
			score += 0.5
		}
	}
	return score
}

// Hit is one ranked search result.
type Hit struct {
	Score float64
	Sheet string
	Form  *models.Formula
}

// Search ranks formulas across sheets. Returns at most topK hits.
func Search(sheets map[string]*models.Sheet, query string, topK int) []Hit {
	terms := Tokenize(query)
	if len(terms) == 0 {
		return nil
	}
	var hits []Hit
	for sname, sheet := range sheets {
		for i := range sheet.Formulas {
			f := &sheet.Formulas[i]
			if sc := ScoreFormula(f, terms); sc > 0 {
				hits = append(hits, Hit{Score: sc, Sheet: sname, Form: f})
			}
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		if hits[i].Sheet != hits[j].Sheet {
			return hits[i].Sheet < hits[j].Sheet
		}
		return hits[i].Form.ID < hits[j].Form.ID
	})
	if topK > 0 && len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}
