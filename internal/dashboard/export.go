package dashboard

// Canonical chart export for the showcase file.
//
// scripts/showcase-data.json used to be maintained by hand, and its PHY 131
// curve drifted from the engine: the committed values overstated early
// coverage by up to 16 formulas. The export in this file regenerates the
// sheet payload from the graph engine (the same payloadFor output the
// dashboard embeds) and records provenance: the generator version, the
// source sheet revision hash, and the curve hash. CheckShowcase compares a
// committed file against a fresh export, so a stale file fails loudly
// instead of shipping wrong numbers again.
//
// The export is deterministic: no timestamps, sorted JSON keys, and the
// existing repo/design/source blocks are carried over byte-for-byte, so
// regenerating an unchanged engine produces identical bytes.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/FaiyazEnayet/formulaforge/internal/graph"
	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

// ShowcaseExportVersion identifies the chart export format. It is recorded
// in every generated showcase file so a drift check can tell whether the
// file predates the current exporter.
const ShowcaseExportVersion = "ff00-chart-export/1"

// ShowcaseProvenance records how a generated showcase file was produced:
// which source sheet revision and which curve it contains.
type ShowcaseProvenance struct {
	Generator    string `json:"generator"`
	Sheet        string `json:"sheet"`
	SourceFile   string `json:"source_file"`
	SourceSHA256 string `json:"source_sha256"`
	CurveSHA256  string `json:"curve_sha256"`
}

// ShowcaseExport is the generated content for one showcase sheet.
type ShowcaseExport struct {
	Payload    *sheetPayload
	Provenance ShowcaseProvenance
}

func showcaseSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ExportShowcase builds the canonical chart payload for a sheet from the
// engine and records its provenance. The payload is the same payloadFor
// output the dashboard embeds, so the showcase file and a freshly built
// dashboard agree by construction.
func ExportShowcase(sheetName string) (*ShowcaseExport, error) {
	s, err := models.LoadSheetByName(sheetName)
	if err != nil {
		return nil, err
	}
	src, srcPath, err := models.SheetSource(sheetName)
	if err != nil {
		return nil, err
	}
	g := graph.New(s)
	if err := g.AssertAcyclic(); err != nil {
		return nil, err
	}
	p, err := payloadFor(s)
	if err != nil {
		return nil, err
	}
	curveJSON, err := json.Marshal(p.Curve)
	if err != nil {
		return nil, err
	}
	return &ShowcaseExport{
		Payload: p,
		Provenance: ShowcaseProvenance{
			Generator:    ShowcaseExportVersion,
			Sheet:        sheetName,
			SourceFile:   "internal/models/" + srcPath,
			SourceSHA256: showcaseSHA256(src),
			CurveSHA256:  showcaseSHA256(curveJSON),
		},
	}, nil
}

// showcaseDoc is the on-disk shape of scripts/showcase-data.json. Repo,
// Design, and Source are carried over from the existing file; only Sheet
// and Provenance are generated. The field order matches the committed
// file, and the output is compact JSON with no trailing newline.
type showcaseDoc struct {
	Repo       json.RawMessage    `json:"repo"`
	Design     json.RawMessage    `json:"design"`
	Source     json.RawMessage    `json:"source"`
	Sheet      *sheetPayload      `json:"sheet"`
	Provenance ShowcaseProvenance `json:"provenance"`
}

// WriteShowcase renders exp into the showcase file at out, preserving the
// existing repo/design/source blocks. It errors when the existing file is
// missing or lacks those blocks, so it can never silently drop the design
// the demo GIF script reads.
func WriteShowcase(existingPath, out string, exp *ShowcaseExport) error {
	raw, err := os.ReadFile(existingPath)
	if err != nil {
		return err
	}
	var doc showcaseDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if len(doc.Repo) == 0 || len(doc.Design) == 0 || len(doc.Source) == 0 {
		return fmt.Errorf("existing showcase file %s lacks repo/design/source blocks", existingPath)
	}
	doc.Sheet = exp.Payload
	doc.Provenance = exp.Provenance
	encoded, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(out, encoded, 0o644)
}

// CheckShowcase verifies that the showcase file at path matches a fresh
// export of sheetName: the sheet payload must be identical and the
// provenance hashes must match the recomputed values. It returns an error
// naming the first mismatch, so a stale file fails loudly.
func CheckShowcase(path, sheetName string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc showcaseDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	exp, err := ExportShowcase(sheetName)
	if err != nil {
		return err
	}
	if doc.Provenance.Generator != ShowcaseExportVersion {
		return fmt.Errorf("showcase provenance generator = %q, want %q", doc.Provenance.Generator, ShowcaseExportVersion)
	}
	if doc.Provenance.Sheet != sheetName {
		return fmt.Errorf("showcase provenance sheet = %q, want %q", doc.Provenance.Sheet, sheetName)
	}
	if doc.Provenance.SourceSHA256 != exp.Provenance.SourceSHA256 {
		return fmt.Errorf("showcase source hash %q does not match fresh export %q: regenerate the showcase file", doc.Provenance.SourceSHA256, exp.Provenance.SourceSHA256)
	}
	if doc.Provenance.CurveSHA256 != exp.Provenance.CurveSHA256 {
		return fmt.Errorf("showcase curve hash %q does not match fresh export %q: regenerate the showcase file", doc.Provenance.CurveSHA256, exp.Provenance.CurveSHA256)
	}
	if doc.Sheet == nil {
		return fmt.Errorf("showcase file %s has no sheet payload", path)
	}
	wantSheet, err := json.Marshal(exp.Payload)
	if err != nil {
		return err
	}
	gotSheet, err := json.Marshal(doc.Sheet)
	if err != nil {
		return err
	}
	if !bytes.Equal(wantSheet, gotSheet) {
		return fmt.Errorf("showcase sheet payload differs from fresh export: regenerate the showcase file")
	}
	return nil
}
