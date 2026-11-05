// Package validate implements the catalog audit contract defined in
// docs/catalog-audit-contract.md: per-formula provenance (source,
// conditions, notation, units), structural LaTeX validation, and a
// whole-catalog audit whose results can be pinned to a baseline.
//
// The audit separates two kinds of findings. Defects are content that
// is wrong today (invalid LaTeX, doubled backslashes). Provenance gaps
// are content that is incomplete today (missing source or conditions).
// models.Load intentionally stays structural; this package is the
// content gate the curated catalog is held to in CI.
package validate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// Finding classes reported by the audit.
const (
	ClassDoubledBackslash  = "doubled-backslash"
	ClassInvalidLatex      = "invalid-latex"
	ClassMissingProvenance = "missing-provenance"
	ClassInvalidProvenance = "invalid-provenance"
	ClassMissingConditions = "missing-conditions"
)

// Provenance is the audit contract for one formula: where the result
// comes from, the conditions it holds under, what its symbols mean,
// and its unit signature where relevant. It mirrors the optional
// provenance fields on models.Formula and can also be parsed from
// standalone YAML (fixtures, review submissions).
type Provenance struct {
	Source     string `yaml:"source"`
	Conditions string `yaml:"conditions"`
	Notation   string `yaml:"notation"`
	Units      string `yaml:"units"`
}

// ParseProvenance parses provenance YAML strictly: an unknown field is
// an error, so a misspelled field cannot silently drop provenance.
func ParseProvenance(data []byte) (Provenance, error) {
	var p Provenance
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return Provenance{}, fmt.Errorf("parse provenance: %w", err)
	}
	return p, nil
}

var (
	sectionRe = regexp.MustCompile(`(?i)(§|\bsection\b|\bsec\.\b|\bchapter\b|\bch\.\b|\btheorem\b|\bexample\b|\bexercise\b)`)
	unitsRe   = regexp.MustCompile(`^[A-Za-z0-9 /·^()\-]+$`)
)

var placeholderWords = map[string]bool{
	"tbd": true, "todo": true, "n/a": true, "none": true,
	"lorem": true, "placeholder": true, "unknown": true,
}

func isPlaceholder(s string) bool {
	return placeholderWords[strings.ToLower(strings.TrimSpace(s))]
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// Problems returns the contract violations in p. requireConditions is
// set for quantitative sheets, where a result stated without its
// conditions is incomplete (the projectile optimum is the canonical
// example: maximized at 45 degrees only for equal launch and landing
// heights with no air resistance).
func (p Provenance) Problems(requireConditions bool) []string {
	var out []string
	src := strings.TrimSpace(p.Source)
	switch {
	case src == "" || isPlaceholder(src):
		out = append(out, "missing source")
	case !isURL(src) && !sectionRe.MatchString(src):
		out = append(out, "source has no section or URL")
	}
	if requireConditions {
		cond := strings.TrimSpace(p.Conditions)
		if cond == "" || isPlaceholder(cond) {
			out = append(out, "missing conditions")
		}
	}
	if u := strings.TrimSpace(p.Units); u != "" && !unitsRe.MatchString(u) {
		out = append(out, "units is not a unit signature")
	}
	return out
}

// knownCommands is the TeX command vocabulary the catalog is allowed
// to use. It was derived from the commands actually used at the
// part-1 baseline (97 commands across the 8 sheets at HEAD, counting
// the cse113 commands that appear only in doubled form there); a new
// command is added here deliberately, in review, when new notation is
// introduced. \begin and \end are handled structurally, not via this
// set.
var knownCommands = map[string]bool{
	"Delta": true, "Rightarrow": true, "Sigma": true, "alpha": true,
	"approx": true, "bar": true, "beta": true, "binom": true,
	"boldsymbol": true, "cap": true, "cdot": true, "cdots": true,
	"chi": true, "circ": true, "cong": true, "cos": true, "cup": true,
	"deg": true, "delta": true, "det": true, "dim": true, "dots": true,
	"downarrow": true, "emptyset": true, "equiv": true, "exists": true,
	"forall": true, "frac": true, "ge": true, "hat": true,
	"hline": true, "iff": true, "iiint": true, "iint": true,
	"implies": true, "in": true, "infty": true, "int": true,
	"lVert": true, "lambda": true, "land": true, "langle": true,
	"lceil": true, "le": true, "left": true, "leftrightarrow": true,
	"lfloor": true, "lim": true, "limsup": true, "ln": true,
	"lnot": true, "log": true, "lor": true, "mathbb": true,
	"mathbf": true, "mathrm": true, "max": true, "mid": true,
	"min": true, "mp": true, "mu": true, "nabla": true, "ne": true,
	"not": true, "oint": true, "omega": true, "overline": true,
	"partial": true, "perp": true, "phi": true, "pi": true, "pm": true,
	"propto": true, "quad": true, "rVert": true, "rangle": true,
	"rceil": true, "rfloor": true, "rho": true, "right": true,
	"sec": true, "sigma": true, "sin": true, "sqrt": true,
	"subseteq": true, "sum": true, "tan": true, "tau": true,
	"text": true, "tfrac": true, "theta": true, "times": true,
	"to": true, "varepsilon": true, "vdash": true,
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// parseEnvArg reads "{name}" starting at s[i]; it returns the name and
// the index just past the closing brace.
func parseEnvArg(s string, i int) (string, int, bool) {
	if i >= len(s) || s[i] != '{' {
		return "", i, false
	}
	j := strings.IndexByte(s[i:], '}')
	if j < 0 {
		return "", i, false
	}
	name := s[i+1 : i+j]
	if name == "" {
		return "", i, false
	}
	for k := 0; k < len(name); k++ {
		if !isLetter(name[k]) && name[k] != '*' {
			return "", i, false
		}
	}
	return name, i + j + 1, true
}

// CheckLatex structurally validates one formula's LaTeX source. It is
// a grammar smoke check, not a typesetter: balanced braces, paired
// \begin/\end environments, alignment tabs only inside environments,
// line breaks only inside environments, no doubled backslashes before
// commands, and no commands outside the catalog vocabulary. It
// returns the list of problems found; an empty list means the source
// passed every check.
func CheckLatex(s string) []string {
	var problems []string
	if strings.TrimSpace(s) == "" {
		return []string{"empty latex"}
	}
	depth := 0
	var envs []string
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '{':
			depth++
			i++
		case c == '}':
			depth--
			if depth < 0 {
				problems = append(problems, "unmatched closing brace")
				depth = 0
			}
			i++
		case c == '&':
			if len(envs) == 0 {
				problems = append(problems, "alignment tab & outside an environment")
			}
			i++
		case c == '\\':
			j := i
			for j < len(s) && s[j] == '\\' {
				j++
			}
			run := j - i
			if j < len(s) && isLetter(s[j]) && run >= 2 {
				k := j
				for k < len(s) && isLetter(s[k]) {
					k++
				}
				problems = append(problems, fmt.Sprintf("doubled backslash before command \\%s", s[j:k]))
				i = k
				continue
			}
			pairs := run / 2
			for p := 0; p < pairs; p++ {
				if len(envs) == 0 {
					problems = append(problems, `line break \\ outside an environment`)
				}
			}
			if run%2 == 0 {
				i = j
				continue
			}
			if j >= len(s) {
				problems = append(problems, "trailing backslash")
				i = j
				continue
			}
			nc := s[j]
			if !isLetter(nc) {
				// Single-character escape: \{ \} \% \, \; and friends.
				i = j + 1
				continue
			}
			k := j
			for k < len(s) && isLetter(s[k]) {
				k++
			}
			name := s[j:k]
			switch name {
			case "begin", "end":
				env, next, ok := parseEnvArg(s, k)
				if !ok {
					problems = append(problems, "\\"+name+" without {environment}")
					i = k
					continue
				}
				if name == "begin" {
					envs = append(envs, env)
				} else if len(envs) == 0 {
					problems = append(problems, fmt.Sprintf("\\end{%s} without \\begin", env))
				} else if envs[len(envs)-1] != env {
					problems = append(problems, fmt.Sprintf("\\end{%s} does not match \\begin{%s}", env, envs[len(envs)-1]))
				} else {
					envs = envs[:len(envs)-1]
				}
				i = next
			default:
				if !knownCommands[name] {
					problems = append(problems, fmt.Sprintf("unknown command \\%s", name))
				}
				i = k
			}
		default:
			i++
		}
	}
	if depth != 0 {
		problems = append(problems, "unbalanced braces")
	}
	if len(envs) > 0 {
		problems = append(problems, fmt.Sprintf("unclosed environment %s", envs[len(envs)-1]))
	}
	return problems
}

// Finding is one audit result for one formula.
type Finding struct {
	Sheet  string `json:"sheet"`
	ID     string `json:"id"`
	Class  string `json:"class"`
	Detail string `json:"detail"`
}

// quantitativeSheets require stated conditions for every formula.
// The logic sheets state definitions and identities, where a source
// is still required but conditions are not.
var quantitativeSheets = map[string]bool{
	"ams151": true, "ams161": true, "ams210": true,
	"ams261": true, "phy131": true, "psy103": true,
}

// RequiresConditions reports whether formulas on the sheet must state
// their conditions under the audit contract.
func RequiresConditions(sheet string) bool {
	return quantitativeSheets[sheet]
}

// AuditFormula audits one formula: LaTeX validity first, then
// provenance. A formula can produce several findings.
func AuditFormula(sheet string, f *models.Formula) []Finding {
	var out []Finding
	add := func(class, detail string) {
		out = append(out, Finding{Sheet: sheet, ID: f.ID, Class: class, Detail: detail})
	}
	doubled := 0
	invalid := 0
	var firstDoubled, firstInvalid string
	for _, p := range CheckLatex(f.Latex) {
		if strings.HasPrefix(p, "doubled backslash") {
			doubled++
			if firstDoubled == "" {
				firstDoubled = p
			}
		} else {
			invalid++
			if firstInvalid == "" {
				firstInvalid = p
			}
		}
	}
	if doubled > 0 {
		add(ClassDoubledBackslash, fmt.Sprintf("%d doubled-backslash commands, first: %s", doubled, firstDoubled))
	}
	if invalid > 0 {
		add(ClassInvalidLatex, fmt.Sprintf("%d problems, first: %s", invalid, firstInvalid))
	}
	prov := Provenance{Source: f.Source, Conditions: f.Conditions, Notation: f.Notation, Units: f.Units}
	for _, p := range prov.Problems(RequiresConditions(sheet)) {
		switch {
		case strings.HasPrefix(p, "missing conditions"):
			add(ClassMissingConditions, p)
		case strings.HasPrefix(p, "units"):
			add(ClassInvalidProvenance, p)
		default:
			add(ClassMissingProvenance, p)
		}
	}
	return out
}

// AuditSheet audits every formula on one loaded sheet.
func AuditSheet(name string, s *models.Sheet) []Finding {
	var out []Finding
	for i := range s.Formulas {
		out = append(out, AuditFormula(name, &s.Formulas[i])...)
	}
	return out
}

// AuditCatalog loads every embedded sheet and audits it. Findings are
// sorted by sheet, formula id and class so results are deterministic.
func AuditCatalog() ([]Finding, error) {
	all, err := models.LoadAll()
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, name := range models.ListSheets() {
		out = append(out, AuditSheet(name, all[name])...)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Sheet != out[b].Sheet {
			return out[a].Sheet < out[b].Sheet
		}
		if out[a].ID != out[b].ID {
			return out[a].ID < out[b].ID
		}
		return out[a].Class < out[b].Class
	})
	return out, nil
}

// Defects returns only the content-defect findings (invalid LaTeX and
// doubled backslashes), dropping provenance gaps.
func Defects(findings []Finding) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Class == ClassDoubledBackslash || f.Class == ClassInvalidLatex {
			out = append(out, f)
		}
	}
	return out
}

// ClassCounts tallies findings per class.
func ClassCounts(findings []Finding) map[string]int {
	out := map[string]int{}
	for _, f := range findings {
		out[f.Class]++
	}
	return out
}

// CatalogHash hashes the embedded sheet sources in sheet order. Any
// content edit to any sheet changes it, which is what makes the
// pinned baseline a drift check rather than a comment.
func CatalogHash() (string, error) {
	h := sha256.New()
	for _, name := range models.ListSheets() {
		data, _, err := models.SheetSource(name)
		if err != nil {
			return "", err
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SheetBaseline is the pinned per-sheet audit state.
type SheetBaseline struct {
	Name            string         `json:"name"`
	Formulas        int            `json:"formulas"`
	Edges           int            `json:"edges"`
	FindingsByClass map[string]int `json:"findings_by_class"`
}

// Baseline is the pinned catalog state at a point in time: headline
// counts, per-sheet inventory, the exact defect list, and provenance
// gap counts. Part 1 of the audit pins the current state; part 2
// re-pins it as content is fixed, ending at an empty defect list.
type Baseline struct {
	CatalogHash   string          `json:"catalog_hash"`
	TotalFormulas int             `json:"total_formulas"`
	TotalEdges    int             `json:"total_edges"`
	Sheets        []SheetBaseline `json:"sheets"`
	Defects       []Finding       `json:"defects"`
}

// sheetEdges counts derivation edges the way the graph engine does:
// the size of each formula's union of prerequisites across its
// primary and alternative derivation paths.
func sheetEdges(s *models.Sheet) int {
	edges := 0
	for i := range s.Formulas {
		f := &s.Formulas[i]
		union := map[string]bool{}
		for _, d := range f.DerivesFrom {
			union[d] = true
		}
		for _, alt := range f.AltDerivesFrom {
			for _, d := range alt {
				union[d] = true
			}
		}
		edges += len(union)
	}
	return edges
}

// CaptureBaseline audits the catalog as it stands and returns the
// pinned representation.
func CaptureBaseline() (*Baseline, error) {
	all, err := models.LoadAll()
	if err != nil {
		return nil, err
	}
	findings, err := AuditCatalog()
	if err != nil {
		return nil, err
	}
	hash, err := CatalogHash()
	if err != nil {
		return nil, err
	}
	b := &Baseline{CatalogHash: hash, Defects: Defects(findings)}
	if b.Defects == nil {
		b.Defects = []Finding{}
	}
	for _, name := range models.ListSheets() {
		s := all[name]
		edges := sheetEdges(s)
		b.TotalFormulas += len(s.Formulas)
		b.TotalEdges += edges
		counts := map[string]int{}
		for _, f := range findings {
			if f.Sheet == name {
				counts[f.Class]++
			}
		}
		b.Sheets = append(b.Sheets, SheetBaseline{
			Name: name, Formulas: len(s.Formulas), Edges: edges, FindingsByClass: counts,
		})
	}
	return b, nil
}

// CompareBaseline diffs a fresh capture against a pinned baseline and
// describes every difference; an empty result means the catalog still
// matches the pin exactly.
func CompareBaseline(pinned, fresh *Baseline) []string {
	var diffs []string
	if pinned.CatalogHash != fresh.CatalogHash {
		diffs = append(diffs, "catalog hash changed (sheet content edited since the baseline was pinned)")
	}
	if pinned.TotalFormulas != fresh.TotalFormulas {
		diffs = append(diffs, fmt.Sprintf("total formulas: pinned %d, now %d", pinned.TotalFormulas, fresh.TotalFormulas))
	}
	if pinned.TotalEdges != fresh.TotalEdges {
		diffs = append(diffs, fmt.Sprintf("total edges: pinned %d, now %d", pinned.TotalEdges, fresh.TotalEdges))
	}
	key := func(f Finding) string { return f.Sheet + "/" + f.ID + "/" + f.Class }
	pinnedDefects := map[string]bool{}
	for _, d := range pinned.Defects {
		pinnedDefects[key(d)] = true
	}
	freshDefects := map[string]bool{}
	for _, d := range fresh.Defects {
		freshDefects[key(d)] = true
		if !pinnedDefects[key(d)] {
			diffs = append(diffs, "new defect not in baseline: "+key(d))
		}
	}
	for _, d := range pinned.Defects {
		if !freshDefects[key(d)] {
			diffs = append(diffs, "baseline defect no longer present (fix landed; re-pin the baseline): "+key(d))
		}
	}
	return diffs
}
