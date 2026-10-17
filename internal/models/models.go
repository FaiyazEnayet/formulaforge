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
type Formula struct {
	ID             string     `yaml:"id"`
	Name           string     `yaml:"name"`
	Topic          string     `yaml:"topic"`
	Latex          string     `yaml:"latex"`
	Description    string     `yaml:"description"`
	Tags           []string   `yaml:"tags"`
	DerivesFrom    []string   `yaml:"derives_from"`
	AltDerivesFrom [][]string `yaml:"alt_derives_from"`
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
		// normalize tags
		for j, t := range f.Tags {
			f.Tags[j] = strings.TrimSpace(t)
		}
	}
	return s, nil
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
