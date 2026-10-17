// Package dashboard builds docs/dashboard.html: a self-contained
// interactive Plotly page.
//
// Panels: dependency-graph explorer (click a formula to highlight its
// derivation chain), study-order timeline, leverage ranking, coverage
// curve, and stat cards. A <select> switches between the four seeded
// sheets. plotly.js is inlined so the file works offline; KaTeX (CDN)
// renders the selected formula.
package dashboard

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/FaiyazEnayet/formulaforge/internal/graph"
	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

//go:embed plotly.min.js
var plotlyJS string

var palette = []string{
	"#4a6fa5", "#e07a5f", "#81b29a", "#f2cc8f", "#9d7bff",
	"#5fc9c9", "#e58bb1", "#a3b18a", "#f4845f", "#6d9dc5",
}

type nodePayload struct {
	Name     string   `json:"name"`
	Topic    string   `json:"topic"`
	Color    string   `json:"color"`
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	Level    int      `json:"level"`
	Leverage int      `json:"leverage"`
	NPrereq  int      `json:"n_prereq"`
	Latex    string   `json:"latex"`
	Desc     string   `json:"desc"`
	Prereqs  []string `json:"prereqs"`
	Children []string `json:"children"`
	Order    int      `json:"order"`
}

type edgePayload struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type nameID struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type unlocker struct {
	Name    string `json:"name"`
	Unlocks int    `json:"unlocks"`
}

type statsPayload struct {
	Nodes       int      `json:"nodes"`
	Edges       int      `json:"edges"`
	Waves       int      `json:"waves"`
	Depth       int      `json:"depth"`
	MinSize     int      `json:"min_size"`
	MinSet      []nameID `json:"min_set"`
	TopUnlocker unlocker `json:"top_unlocker"`
}

type timelineRow struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Wave     int    `json:"wave"`
	Leverage int    `json:"leverage"`
	Color    string `json:"color"`
}

type leverageRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Unlocks int    `json:"unlocks"`
}

type sheetPayload struct {
	Course     string                `json:"course"`
	Title      string                `json:"title"`
	Description string               `json:"description"`
	Topics     []string              `json:"topics"`
	Nodes      map[string]nodePayload `json:"nodes"`
	Edges      []edgePayload         `json:"edges"`
	Stats      statsPayload          `json:"stats"`
	Curve      [][]float64           `json:"curve"`
	LeverageTop []leverageRow        `json:"leverage_top"`
	Timeline   []timelineRow         `json:"timeline"`
}

func payloadFor(sheet *models.Sheet) (*sheetPayload, error) {
	g := graph.New(sheet)
	if err := g.AssertAcyclic(); err != nil {
		return nil, err
	}
	levels, err := g.Levels()
	if err != nil {
		return nil, err
	}
	order, err := g.TopoOrder()
	if err != nil {
		return nil, err
	}
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	waves, err := g.Waves()
	if err != nil {
		return nil, err
	}
	st, err := g.Stats()
	if err != nil {
		return nil, err
	}
	topics := sheet.Topics()
	tcolor := map[string]string{}
	for i, t := range topics {
		tcolor[t] = palette[i%len(palette)]
	}
	xs, ys := map[string]float64{}, map[string]float64{}
	for lv, wave := range waves {
		for j, nid := range wave {
			xs[nid] = (float64(j) - float64(len(wave)-1)/2) * 1.6
			ys[nid] = float64(-lv)
		}
	}
	nodes := map[string]nodePayload{}
	for i := range sheet.Formulas {
		f := &sheet.Formulas[i]
		pre := append([]string{}, g.Prereqs[f.ID]...)
		sort.Strings(pre)
		ch := append([]string{}, g.Children[f.ID]...)
		sort.Strings(ch)
		nodes[f.ID] = nodePayload{
			Name: f.Name, Topic: f.Topic, Color: tcolor[f.Topic],
			X: xs[f.ID], Y: ys[f.ID], Level: levels[f.ID],
			Leverage: g.Leverage(f.ID), NPrereq: len(g.Prereqs[f.ID]),
			Latex: f.Latex, Desc: f.Description,
			Prereqs: pre, Children: ch, Order: pos[f.ID],
		}
	}
	var edges []edgePayload
	for _, n := range g.Nodes {
		for _, p := range g.Prereqs[n] {
			edges = append(edges, edgePayload{From: p, To: n})
		}
	}
	if edges == nil {
		edges = []edgePayload{}
	}
	curve := [][]float64{{0, 0}}
	for _, c := range g.CoverageCurve()[1:] {
		curve = append(curve, []float64{float64(c.K), c.Fraction})
	}
	var levTop []leverageRow
	for _, e := range g.LeverageRanking() {
		f, _ := sheet.ByID(e.ID)
		levTop = append(levTop, leverageRow{ID: e.ID, Name: f.Name, Unlocks: e.Leverage})
		if len(levTop) == 12 {
			break
		}
	}
	var timeline []timelineRow
	for _, nid := range order {
		f, _ := sheet.ByID(nid)
		timeline = append(timeline, timelineRow{
			ID: nid, Name: f.Name, Wave: levels[nid],
			Leverage: g.Leverage(nid), Color: tcolor[f.Topic],
		})
	}
	minSet := []nameID{}
	for _, nid := range st.MinMemorizeSet {
		f, _ := sheet.ByID(nid)
		minSet = append(minSet, nameID{ID: nid, Name: f.Name})
	}
	top0 := st.TopLeverage[0]
	topName, _ := sheet.ByID(top0.ID)
	return &sheetPayload{
		Course: sheet.Course, Title: sheet.Title, Description: sheet.Description,
		Topics: topics, Nodes: nodes, Edges: edges,
		Stats: statsPayload{
			Nodes: st.Nodes, Edges: st.Edges, Waves: st.Waves, Depth: st.Depth,
			MinSize: len(st.MinMemorizeSet), MinSet: minSet,
			TopUnlocker: unlocker{Name: topName.Name, Unlocks: top0.Leverage},
		},
		Curve: curve, LeverageTop: levTop, Timeline: timeline,
	}, nil
}

//go:embed portfolio-head.html
var pageHead string

//go:embed portfolio-app.js
var pageJS string

// Build generates the dashboard HTML and writes it to out.
func Build(out string) error {
	sheets, err := models.LoadAll()
	if err != nil {
		return err
	}
	allNames := models.ListSheets()
	names := make([]string, 0, len(allNames))
	for _, name := range allNames {
		if name != "cse214" { names = append(names, name) }
	}
	data := map[string]*sheetPayload{}
	for _, name := range names {
		p, err := payloadFor(sheets[name])
		if err != nil {
			return fmt.Errorf("sheet %s: %w", name, err)
		}
		data[name] = p
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}
	namesJSON, err := json.Marshal(names)
	if err != nil {
		return err
	}
	js := strings.Replace(pageJS, "__DATA__", string(dataJSON), 1)
	js = strings.Replace(js, "__ORDER__", string(namesJSON), 1)

	var b strings.Builder
	b.WriteString(pageHead)
	b.WriteString("<script>\n")
	b.WriteString(plotlyJS)
	b.WriteString("\n</script>\n<script>\n")
	b.WriteString(js)
	b.WriteString("\n</script>\n</body></html>\n")
	if err := os.MkdirAll(dirOf(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, []byte(b.String()), 0o644)
}

func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}
