// Package models defines the Formula and Sheet data model and loads
// sheet YAML files (embedded into the binary via go:embed).
package models

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed sheets/*.yaml
var sheetsFS embed.FS

// Formula is a single named mathematical result with LaTeX source and
// derivation links to other formulas. DerivesFrom is the primary way to
// derive it: every listed prerequisite is needed. Each AltDerivesFrom
// entry is an alternative prerequisite set that also suffices on its own.
//
// Relationships are typed. ConceptualPrerequisites are pedagogical
// links ("worth learning first"): they stay navigable, but they never
// grant derivability, so a conceptual edge alone can never put a
// formula in Closure. DerivationRules certify the derivation paths:
// each rule states the derivation, cites a reference, and records
// its assumptions. A rule with status "unreviewed" is not a proof
// claim: its path never enters Closure or optimization, and
// renderers must not present it as proven. Rules align positionally
// with the paths: rule 0 covers DerivesFrom, rule 1 covers the first
// AltDerivesFrom entry, and so on. A path with no rule is certified
// by default, so sheets written before typed relationships keep
// their existing semantics.
//
// The provenance fields (Source, Conditions, Notation, Units) carry the
// catalog audit contract defined in docs/catalog-audit-contract.md and
// checked by internal/validate: where the result comes from, the
// conditions it holds under, what its symbols mean, and its unit
// signature where relevant. They are optional in the schema while the
// audit rolls out; the audit reports every formula that lacks them.
type Formula struct {
	ID                      string           `yaml:"id"`
	Name                    string           `yaml:"name"`
	Topic                   string           `yaml:"topic"`
	Latex                   string           `yaml:"latex"`
	Description             string           `yaml:"description"`
	Tags                    []string         `yaml:"tags"`
	DerivesFrom             []string         `yaml:"derives_from"`
	AltDerivesFrom          [][]string       `yaml:"alt_derives_from"`
	ConceptualPrerequisites []string         `yaml:"conceptual_prerequisites"`
	DerivationRules         []DerivationRule `yaml:"derivation_rules"`
	Source                  string           `yaml:"source,omitempty"`
	Conditions              string           `yaml:"conditions,omitempty"`
	Notation                string           `yaml:"notation,omitempty"`
	Units                   string           `yaml:"units,omitempty"`
}

// DerivationRule is the certified statement of one derivation path:
// what the rule says, where it comes from, what it assumes, and
// whether it has been independently reviewed.
type DerivationRule struct {
	Statement   string `yaml:"statement,omitempty"`
	Reference   string `yaml:"reference,omitempty"`
	Assumptions string `yaml:"assumptions,omitempty"`
	Status      string `yaml:"status,omitempty"`
}

// Derivation rule status values.
const (
	// StatusReviewed marks a derivation rule as independently
	// reviewed: its path is a certified sufficient derivation.
	StatusReviewed = "reviewed"
	// StatusUnreviewed marks a derivation rule as not yet reviewed:
	// its path is not a proof claim and never enters Closure or
	// optimization.
	StatusUnreviewed = "unreviewed"
)

// Certified reports whether the rule counts as a reviewed sufficient
// derivation. Load rejects unknown statuses, so in practice a
// certified rule has status "" or "reviewed".
func (r DerivationRule) Certified() bool {
	return r.Status != StatusUnreviewed
}

// AllPaths returns every derivation path in order: the primary path
// when non-empty, then each alternative path. Each path is a copy.
func (f *Formula) AllPaths() [][]string {
	var paths [][]string
	if len(f.DerivesFrom) > 0 {
		paths = append(paths, append([]string{}, f.DerivesFrom...))
	}
	for _, alt := range f.AltDerivesFrom {
		paths = append(paths, append([]string{}, alt...))
	}
	return paths
}

// RuleFor returns the derivation rule for the i-th derivation path
// (0 is the primary path, 1 and up are the alternatives in order). A
// path with no rule carries the zero rule, which is certified.
func (f *Formula) RuleFor(i int) DerivationRule {
	if i < 0 || i >= len(f.DerivationRules) {
		return DerivationRule{}
	}
	return f.DerivationRules[i]
}

// CertifiedPaths returns the derivation paths whose rule is certified
// (reviewed), primary first, then alternatives in order. Each path is
// a copy; callers may sort or mutate it freely.
func (f *Formula) CertifiedPaths() [][]string {
	var out [][]string
	for i, p := range f.AllPaths() {
		if f.RuleFor(i).Certified() {
			out = append(out, p)
		}
	}
	return out
}

// UncertifiedPaths returns the derivation paths whose rule status is
// "unreviewed", primary first, then alternatives in order.
func (f *Formula) UncertifiedPaths() [][]string {
	var out [][]string
	for i, p := range f.AllPaths() {
		if !f.RuleFor(i).Certified() {
			out = append(out, p)
		}
	}
	return out
}

// HasUnreviewedDerivation reports whether any derivation path of the
// formula is unreviewed.
func (f *Formula) HasUnreviewedDerivation() bool {
	return len(f.UncertifiedPaths()) > 0
}

// CertifiedPrimary returns the primary derivation path when it is
// certified, or nil when the primary path is absent or unreviewed.
func (f *Formula) CertifiedPrimary() []string {
	if len(f.DerivesFrom) == 0 || !f.RuleFor(0).Certified() {
		return nil
	}
	return append([]string{}, f.DerivesFrom...)
}

// UncertifiedMembers returns the sorted union of the members of the
// formula's unreviewed derivation paths.
func (f *Formula) UncertifiedMembers() []string {
	union := map[string]bool{}
	for _, p := range f.UncertifiedPaths() {
		for _, m := range p {
			union[m] = true
		}
	}
	out := make([]string, 0, len(union))
	for m := range union {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// Sheet is a course's full formula collection.
type Sheet struct {
	Course      string    `yaml:"course"`
	Title       string    `yaml:"title"`
	Description string    `yaml:"description"`
	Formulas    []Formula `yaml:"formulas"`

	byID map[string]*Formula
}

// ByID returns the formula with the given id, or an error.
func (s *Sheet) ByID(id string) (*Formula, error) {
	f, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("unknown formula id: %s", id)
	}
	return f, nil
}

// DerivationDisplay splits a formula's derivation into display
// names: the first certified path (the "Builds on" line) and the
// members of its unreviewed paths (the "under review" line). Names
// come from the sheet; an id with no formula on the sheet (impossible
// on a loaded sheet) falls back to the id itself.
func (s *Sheet) DerivationDisplay(f *Formula) (certified, unreviewed []string) {
	name := func(id string) string {
		if other, err := s.ByID(id); err == nil {
			return other.Name
		}
		return id
	}
	if paths := f.CertifiedPaths(); len(paths) > 0 {
		for _, d := range paths[0] {
			certified = append(certified, name(d))
		}
	}
	for _, d := range f.UncertifiedMembers() {
		unreviewed = append(unreviewed, name(d))
	}
	return certified, unreviewed
}

// IDs returns all formula ids in sheet order.
func (s *Sheet) IDs() []string {
	ids := make([]string, 0, len(s.Formulas))
	for i := range s.Formulas {
		ids = append(ids, s.Formulas[i].ID)
	}
	return ids
}

// Topics returns topics in first-appearance order.
func (s *Sheet) Topics() []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range s.Formulas {
		t := f.Topic
		if t == "" {
			t = "General"
		}
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

type rawSheet struct {
	Course      string    `yaml:"course"`
	Title       string    `yaml:"title"`
	Description string    `yaml:"description"`
	Formulas    []Formula `yaml:"formulas"`
}

// Load parses and validates sheet YAML data.
//
// Validation contract: malformed YAML is a parse error; a sheet with no
// formulas is rejected ("sheet has no formulas"); every formula needs a
// non-empty, unique id; every dependency id must exist on the sheet; a
// formula may not derive from itself; alternative derivation paths must
// be non-empty. Conceptual prerequisites must also exist on the sheet,
// a formula may not list itself as one, and the conceptual links must
// be acyclic. Every derivation rule needs a matching derivation path
// and a known status ("reviewed" or "unreviewed"; empty means
// reviewed). Valid input produces a deterministic error or a usable
// sheet, never a partial one.
func Load(data []byte) (*Sheet, error) {
	var raw rawSheet
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse sheet: %w", err)
	}
	s := &Sheet{
		Course:      raw.Course,
		Title:       raw.Title,
		Description: raw.Description,
		Formulas:    raw.Formulas,
		byID:        map[string]*Formula{},
	}
	if len(s.Formulas) == 0 {
		return nil, fmt.Errorf("sheet has no formulas")
	}
	for i := range s.Formulas {
		f := &s.Formulas[i]
		if f.ID == "" {
			return nil, fmt.Errorf("formula %d has no id", i)
		}
		if _, dup := s.byID[f.ID]; dup {
			return nil, fmt.Errorf("duplicate formula id: %s", f.ID)
		}
		if f.Topic == "" {
			f.Topic = "General"
		}
		s.byID[f.ID] = f
	}
	for i := range s.Formulas {
		f := &s.Formulas[i]
		checkDep := func(dep string) error {
			if dep == f.ID {
				return fmt.Errorf("formula %s derives from itself", f.ID)
			}
			if _, ok := s.byID[dep]; !ok {
				return fmt.Errorf("formula %s references unknown id: %s", f.ID, dep)
			}
			return nil
		}
		for _, dep := range f.DerivesFrom {
			if err := checkDep(dep); err != nil {
				return nil, err
			}
		}
		for _, alt := range f.AltDerivesFrom {
			if len(alt) == 0 {
				return nil, fmt.Errorf("formula %s has an empty alternative derivation path", f.ID)
			}
			for _, dep := range alt {
				if err := checkDep(dep); err != nil {
					return nil, err
				}
			}
		}
		for _, dep := range f.ConceptualPrerequisites {
			if dep == f.ID {
				return nil, fmt.Errorf("formula %s lists itself as a conceptual prerequisite", f.ID)
			}
			if _, ok := s.byID[dep]; !ok {
				return nil, fmt.Errorf("formula %s references unknown id: %s", f.ID, dep)
			}
		}
		if n := len(f.AllPaths()); len(f.DerivationRules) > n {
			return nil, fmt.Errorf("formula %s has derivation rule %d with no matching derivation path", f.ID, n)
		}
		for _, rule := range f.DerivationRules {
			switch rule.Status {
			case "", StatusReviewed, StatusUnreviewed:
			default:
				return nil, fmt.Errorf("formula %s has invalid derivation rule status: %q", f.ID, rule.Status)
			}
		}
		// normalize tags
		for j, t := range f.Tags {
			f.Tags[j] = strings.TrimSpace(t)
		}
	}
	if err := checkConceptualCycle(s); err != nil {
		return nil, err
	}
	return s, nil
}

// checkConceptualCycle rejects cycles in the conceptual-prerequisite
// links. Derivation cycles are a graph-layer concern (FindCycle,
// AssertAcyclic); conceptual links live only in the model, so their
// cycles are caught here at load time. Nodes are visited in sheet
// order and links in listed order, so the reported cycle is
// deterministic for a fixed sheet.
func checkConceptualCycle(s *Sheet) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string
	var visit func(id string) error
	visit = func(id string) error {
		color[id] = gray
		stack = append(stack, id)
		f, _ := s.ByID(id)
		for _, dep := range f.ConceptualPrerequisites {
			if color[dep] == gray {
				idx := 0
				for i, n := range stack {
					if n == dep {
						idx = i
						break
					}
				}
				cycle := append([]string{}, stack[idx:]...)
				return fmt.Errorf("conceptual prerequisite cycle: %v", append(cycle, dep))
			}
			if color[dep] == white {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[id] = black
		return nil
	}
	for _, f := range s.Formulas {
		if color[f.ID] == white {
			if err := visit(f.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// ListSheets returns the sorted names of embedded sheets (no extension).
func ListSheets() []string {
	entries, err := sheetsFS.ReadDir("sheets")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".yaml") || strings.HasSuffix(n, ".yml") {
			base := n[:strings.LastIndex(n, ".")]
			names = append(names, base)
		}
	}
	sort.Strings(names)
	return names
}

// LoadSheetByName loads one of the embedded sheets.
func LoadSheetByName(name string) (*Sheet, error) {
	for _, ext := range []string{".yaml", ".yml"} {
		data, err := sheetsFS.ReadFile("sheets/" + name + ext)
		if err == nil {
			return Load(data)
		}
	}
	return nil, fmt.Errorf("no sheet named %q", name)
}

// SheetSource returns the embedded YAML bytes for a sheet and the embedded
// path they were read from. Chart exporters hash these bytes so a
// generated file records exactly which source revision it came from.
func SheetSource(name string) ([]byte, string, error) {
	for _, ext := range []string{".yaml", ".yml"} {
		p := "sheets/" + name + ext
		data, err := sheetsFS.ReadFile(p)
		if err == nil {
			return data, p, nil
		}
	}
	return nil, "", fmt.Errorf("no sheet named %q", name)
}

// LoadAll loads every embedded sheet keyed by name.
func LoadAll() (map[string]*Sheet, error) {
	out := map[string]*Sheet{}
	for _, name := range ListSheets() {
		s, err := LoadSheetByName(name)
		if err != nil {
			return nil, err
		}
		out[name] = s
	}
	return out, nil
}
