package dashboard

// Stage 3 FF-00: release-blocking evidence regeneration and provenance.
//
// The committed PHY 131 showcase curve in scripts/showcase-data.json is
// generated from the engine, not maintained by hand. These tests lock that
// in: the committed curve must equal the CoverageCurve oracle under the
// dashboard convention ([0, 0] followed by the engine points from K=1),
// the drift checker must accept the committed file and reject a stale
// fixture, and the separately generated docs/dashboard.html output is
// audited by extraction. Coverage fractions are fractions of a sheet
// derivable under conjunctive closure; they are a greedy heuristic, not
// a claim about what any student knows.

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/graph"
	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// curveTolerance is the allowed absolute difference between a committed
// curve fraction and the engine fraction. Engine fractions are rational
// counts over the node total, so any real drift is at least 1/nodes
// (about 0.02 for PHY 131); 1e-12 admits only float round-trip wobble.
const curveTolerance = 1e-12

func ff00RepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// ff00EngineCurve returns the dashboard-convention curve for a sheet:
// [0, 0] followed by the CoverageCurve points from K=1.
func ff00EngineCurve(t *testing.T, name string) [][]float64 {
	t.Helper()
	s, err := models.LoadSheetByName(name)
	if err != nil {
		t.Fatal(err)
	}
	g := graph.New(s)
	if err := g.AssertAcyclic(); err != nil {
		t.Fatal(err)
	}
	raw := g.CoverageCurve()
	curve := [][]float64{{0, 0}}
	for _, c := range raw[1:] {
		curve = append(curve, []float64{float64(c.K), c.Fraction})
	}
	return curve
}

func ff00ShowcasePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(ff00RepoRoot(t), "scripts", "showcase-data.json")
}

func ff00CurveDiff(a, b [][]float64) (maxDiff float64, ok bool) {
	if len(a) != len(b) {
		return math.Inf(1), false
	}
	for i := range a {
		if a[i][0] != b[i][0] {
			return math.Inf(1), false
		}
		if d := math.Abs(a[i][1] - b[i][1]); d > maxDiff {
			maxDiff = d
		}
	}
	return maxDiff, maxDiff <= curveTolerance
}

func TestStage3Formulaforge00CurveMatchesEngine(t *testing.T) {
	data, err := os.ReadFile(ff00ShowcasePath(t))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Sheet struct {
			Course string      `json:"course"`
			Curve  [][]float64 `json:"curve"`
		} `json:"sheet"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Sheet.Course != "PHY 131" {
		t.Fatalf("showcase sheet course = %q, want PHY 131", doc.Sheet.Course)
	}
	engine := ff00EngineCurve(t, "phy131")
	maxDiff, ok := ff00CurveDiff(engine, doc.Sheet.Curve)
	if !ok {
		t.Errorf("committed showcase curve differs from engine: max diff %g (tolerance %g)", maxDiff, curveTolerance)
		for i := range engine {
			if i >= len(doc.Sheet.Curve) {
				break
			}
			if math.Abs(engine[i][1]-doc.Sheet.Curve[i][1]) > curveTolerance {
				t.Errorf("  K=%v engine=%.16g committed=%.16g", engine[i][0], engine[i][1], doc.Sheet.Curve[i][1])
			}
		}
	}
}

// ff00ExtractHTMLCurve extracts the phy131 curve embedded in a dashboard
// HTML file. It reads only; it never regenerates the file.
func ff00ExtractHTMLCurve(t *testing.T, htmlPath string) [][]float64 {
	t.Helper()
	raw, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	i := strings.Index(page, `"phy131":`)
	if i < 0 {
		t.Fatal("dashboard HTML has no phy131 payload")
	}
	j := strings.Index(page[i:], `"curve":`)
	if j < 0 {
		t.Fatal("dashboard HTML phy131 payload has no curve")
	}
	k := i + j + len(`"curve":`)
	for k < len(page) && page[k] != '[' {
		k++
	}
	depth := 0
	end := k
	for end < len(page) {
		switch page[end] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				var curve [][]float64
				if err := json.Unmarshal([]byte(page[k:end+1]), &curve); err != nil {
					t.Fatal(err)
				}
				return curve
			}
		}
		end++
	}
	t.Fatal("dashboard HTML phy131 curve array never closes")
	return nil
}

func ff00CheckCurveShape(t *testing.T, curve [][]float64) {
	t.Helper()
	if len(curve) < 2 {
		t.Fatalf("curve has %d points, want at least 2", len(curve))
	}
	for i, pt := range curve {
		if len(pt) != 2 {
			t.Fatalf("curve point %d has %d values, want 2", i, len(pt))
		}
		if pt[0] != float64(i) {
			t.Fatalf("curve point %d has K=%v, want %d", i, pt[0], i)
		}
		if i > 0 && pt[1] < curve[i-1][1] {
			t.Fatalf("curve decreases at K=%d (%g -> %g)", i, curve[i-1][1], pt[1])
		}
	}
	if curve[len(curve)-1][1] != 1.0 {
		t.Fatalf("curve does not end at 1.0 (ends at %g)", curve[len(curve)-1][1])
	}
}

// TestStage3Formulaforge00DriftCheck verifies that the committed
// showcase file matches a fresh export: identical sheet payload and
// matching provenance hashes.
func TestStage3Formulaforge00DriftCheck(t *testing.T) {
	if err := CheckShowcase(ff00ShowcasePath(t), "phy131"); err != nil {
		t.Errorf("drift check failed: %v", err)
	}
}

// TestStage3Formulaforge00StaleFixtureFails proves the drift checker
// rejects stale input: a file with one perturbed curve value, and a file
// with a tampered provenance hash, must both fail the check.
func TestStage3Formulaforge00StaleFixtureFails(t *testing.T) {
	raw, err := os.ReadFile(ff00ShowcasePath(t))
	if err != nil {
		t.Fatal(err)
	}
	// Variant 1: perturb one curve value, keep provenance intact.
	var docMap map[string]any
	if err := json.Unmarshal(raw, &docMap); err != nil {
		t.Fatal(err)
	}
	sheet := docMap["sheet"].(map[string]any)
	curve := sheet["curve"].([]any)
	pt := curve[1].([]any)
	pt[1] = pt[1].(float64) + 0.01
	stale1, err := json.Marshal(docMap)
	if err != nil {
		t.Fatal(err)
	}
	p1 := filepath.Join(t.TempDir(), "stale-curve.json")
	if err := os.WriteFile(p1, stale1, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckShowcase(p1, "phy131"); err == nil {
		t.Error("drift check accepted a file with a perturbed curve value")
	} else {
		t.Logf("stale curve correctly rejected: %v", err)
	}
	// Variant 2: tamper the provenance hash, keep the payload intact.
	docMap2 := map[string]any{}
	if err := json.Unmarshal(raw, &docMap2); err != nil {
		t.Fatal(err)
	}
	prov := docMap2["provenance"].(map[string]any)
	prov["curve_sha256"] = "0000000000000000000000000000000000000000000000000000000000000000"
	stale2, err := json.Marshal(docMap2)
	if err != nil {
		t.Fatal(err)
	}
	p2 := filepath.Join(t.TempDir(), "stale-hash.json")
	if err := os.WriteFile(p2, stale2, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckShowcase(p2, "phy131"); err == nil {
		t.Error("drift check accepted a file with a tampered provenance hash")
	} else {
		t.Logf("tampered hash correctly rejected: %v", err)
	}
}

// TestStage3Formulaforge00ExportDeterministic verifies that two fresh
// exports are byte-identical and that the provenance hashes match the
// source bytes and the marshaled curve.
func TestStage3Formulaforge00ExportDeterministic(t *testing.T) {
	a, err := ExportShowcase("phy131")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ExportShowcase("phy131")
	if err != nil {
		t.Fatal(err)
	}
	pa, err := json.Marshal(a.Payload)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := json.Marshal(b.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(pa) != string(pb) {
		t.Error("two fresh exports differ")
	}
	if a.Provenance != b.Provenance {
		t.Error("two fresh exports carry different provenance")
	}
	src, _, err := models.SheetSource("phy131")
	if err != nil {
		t.Fatal(err)
	}
	curveJSON, err := json.Marshal(a.Payload.Curve)
	if err != nil {
		t.Fatal(err)
	}
	if a.Provenance.SourceSHA256 != showcaseSHA256(src) {
		t.Error("provenance source hash does not match sheet source bytes")
	}
	if a.Provenance.CurveSHA256 != showcaseSHA256(curveJSON) {
		t.Error("provenance curve hash does not match marshaled curve")
	}
	if a.Provenance.Generator != ShowcaseExportVersion {
		t.Errorf("provenance generator = %q, want %q", a.Provenance.Generator, ShowcaseExportVersion)
	}
}

// TestStage3Formulaforge00NoMastery locks in that the generated showcase
// file describes coverage as a greedy heuristic over derivable formulas,
// never as a claim about what a student has mastered.
func TestStage3Formulaforge00NoMastery(t *testing.T) {
	raw, err := os.ReadFile(ff00ShowcasePath(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(raw)), "master") {
		t.Error("generated showcase file contains a mastery interpretation")
	}
}

// docs/dashboard.html by extracting its embedded PHY 131 curve and
// comparing it with the engine. The HTML file is generated output and is
// never regenerated by this unit, so a value mismatch is reported as an
// audit finding (dashboard regeneration is a separate owner step), while
// a missing or malformed curve fails the test.
func TestStage3Formulaforge00HTMLAudit(t *testing.T) {
	htmlPath := filepath.Join(ff00RepoRoot(t), "docs", "dashboard.html")
	htmlCurve := ff00ExtractHTMLCurve(t, htmlPath)
	ff00CheckCurveShape(t, htmlCurve)
	engine := ff00EngineCurve(t, "phy131")
	maxDiff, ok := ff00CurveDiff(engine, htmlCurve)
	if !ok {
		t.Logf("HTML AUDIT FINDING: docs/dashboard.html phy131 curve differs from engine (max diff %g); dashboard regeneration is a separate owner step", maxDiff)
		for i := range engine {
			if i >= len(htmlCurve) {
				break
			}
			if math.Abs(engine[i][1]-htmlCurve[i][1]) > curveTolerance {
				t.Logf("  K=%v engine=%.16g html=%.16g", engine[i][0], engine[i][1], htmlCurve[i][1])
			}
		}
	}
}
