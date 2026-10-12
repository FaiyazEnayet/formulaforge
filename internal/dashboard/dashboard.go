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

const pageHead = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>formulaforge — dependency graph explorer</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/katex.min.css">
<script src="https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/katex.min.js"></script>
<style>
  body { font-family: system-ui, -apple-system, sans-serif; margin: 0; background: #0f1420; color: #e8ecf4; }
  .wrap { max-width: 1180px; margin: 0 auto; padding: 1.4rem 1.2rem 3rem; }
  h1 { margin: 0; font-size: 1.7rem; letter-spacing: .01em; }
  h1 .ff { color: #7fb3ff; }
  .sub { color: #9aa7c2; margin: .3rem 0 1rem; }
  .bar { display: flex; gap: .8rem; align-items: center; margin-bottom: 1rem; flex-wrap: wrap; }
  select { background: #1a2334; color: #e8ecf4; border: 1px solid #2c3a55; border-radius: 8px;
           padding: .5rem .8rem; font-size: 1rem; }
  .cards { display: flex; gap: .7rem; flex-wrap: wrap; margin-bottom: 1rem; }
  .card { background: #1a2334; border: 1px solid #2c3a55; border-radius: 10px; padding: .6rem .9rem; min-width: 130px; }
  .card .v { font-size: 1.5rem; font-weight: 700; color: #7fb3ff; }
  .card .k { font-size: .78rem; color: #9aa7c2; }
  .panel { background: #141b2c; border: 1px solid #2c3a55; border-radius: 12px; padding: .8rem; margin-bottom: 1rem; }
  .panel h2 { margin: .2rem 0 .6rem; font-size: 1.05rem; color: #c9d6ef; }
  .hint { font-size: .82rem; color: #9aa7c2; margin: .4rem 0 0; }
  #detail { background: #1a2334; border: 1px solid #2c3a55; border-radius: 10px; padding: .9rem 1.1rem; min-height: 90px; }
  #detail h3 { margin: 0 0 .3rem; font-size: 1.05rem; }
  #detail .math { font-size: 1.25rem; overflow-x: auto; color: #fff; }
  #detail p { color: #9aa7c2; font-size: .9rem; margin: .4rem 0 0; }
  .minset { display: flex; flex-wrap: wrap; gap: .4rem; margin-top: .5rem; }
  .pill { background: #243354; border: 1px solid #3b507c; border-radius: 20px; padding: .25rem .7rem; font-size: .82rem; }
  footer { color: #66708a; font-size: .8rem; margin-top: 1.5rem; }
</style>
</head><body><div class="wrap">
<h1><span class="ff">formulaforge</span> — dependency graph explorer</h1>
<p class="sub">Every formula is a node. An edge means "derive this from that". Click any node to trace its derivation chain.</p>
<div class="bar">
  <label for="sheet">Sheet:</label>
  <select id="sheet"></select>
</div>
<div class="cards" id="cards"></div>
<div class="panel"><h2>Dependency graph</h2><div id="graph" style="height:520px"></div>
<p class="hint">Click a formula to highlight what it builds on (gold) and what it unlocks (blue). Click empty space to reset.</p></div>
<div class="panel"><h2>Formula detail</h2><div id="detail"><p>Click a node in the graph to inspect it.</p></div></div>
<div class="panel"><h2>Study order timeline</h2><div id="timeline" style="height:420px"></div>
<p class="hint">Topological order, top = learn first. Bar length = leverage (formulas unlocked).</p></div>
<div class="panel"><h2>Highest-leverage formulas</h2><div id="leverage" style="height:380px"></div>
<p class="hint">Learn these first: each unlocks the most downstream formulas.</p></div>
<div class="panel"><h2>Minimum-memorize coverage</h2><div id="curve" style="height:320px"></div>
<div class="minset" id="minset"></div>
<p class="hint">Greedy set cover: memorize the highlighted set and derive everything else.</p></div>
<footer>Built with formulaforge — numbers computed live from the sheet graphs.</footer>
</div>
`

const pageJS = `
const DATA = __DATA__;
const ORDER = __ORDER__;
function dark(extra) {
  // Deep-merge helper: per-panel axis options must survive alongside the
  // dark theme (a flat Object.assign with DARK wiped them). Bar panels also
  // declare yaxis type explicitly: Plotly's category auto-detection fails
  // for plots rendered after a markers+text scatter, leaving bars blank.
  extra = extra || {};
  return {
    paper_bgcolor: '#141b2c', plot_bgcolor: '#141b2c',
    font: {color: '#e8ecf4'},
    margin: extra.margin, hovermode: extra.hovermode,
    xaxis: Object.assign({gridcolor: '#232f4a'}, extra.xaxis),
    yaxis: Object.assign({gridcolor: '#232f4a'}, extra.yaxis)
  };
}

const sheetSel = document.getElementById('sheet');
ORDER.forEach(n => { const o = document.createElement('option'); o.value = n;
  o.textContent = DATA[n].course + ' \u2014 ' + DATA[n].title; sheetSel.appendChild(o); });

let cur = ORDER[0], nodeIds = [], adj = {};

function edgeTraces(d) {
  const xs = [], ys = [];
  d.edges.forEach(e => { const a = d.nodes[e.from], b = d.nodes[e.to];
    xs.push(a.x, b.x, null); ys.push(a.y, b.y, null); });
  return {x: xs, y: ys, mode: 'lines', type: 'scatter',
    line: {color: 'rgba(140,160,200,0.35)', width: 1}, hoverinfo: 'skip', showlegend: false};
}
function nodeTrace(d) {
  nodeIds = Object.keys(d.nodes);
  const x = [], y = [], text = [], color = [], size = [], custom = [];
  nodeIds.forEach(id => { const n = d.nodes[id];
    x.push(n.x); y.push(n.y); text.push(n.name); color.push(n.color);
    size.push(10 + 4 * Math.sqrt(n.leverage));
    custom.push([n.name, n.topic, n.leverage, n.n_prereq, n.level]); });
  return {x, y, text, mode: 'markers', type: 'scatter', customdata: custom,
    marker: {color, size, line: {color: '#0f1420', width: 1}, opacity: 0.95},
    hovertemplate: '<b>%{text}</b><br>topic: %{customdata[1]}<br>unlocks: %{customdata[2]} formulas<br>builds on: %{customdata[3]}<br>wave %{customdata[4]}<extra></extra>',
    showlegend: false};
}

function renderGraph() {
  const d = DATA[cur];
  adj = {};
  Object.keys(d.nodes).forEach(id => adj[id] = {p: d.nodes[id].prereqs, c: d.nodes[id].children});
  Plotly.newPlot('graph', [edgeTraces(d), nodeTrace(d)],
    dark({margin: {l: 20, r: 20, t: 10, b: 20},
      xaxis: {visible: false}, yaxis: {visible: false}, hovermode: 'closest'}),
    {responsive: true, displayModeBar: false});
  document.getElementById('graph').on('plotly_click', onNodeClick);
}
function onNodeClick(ev) {
  const d = DATA[cur];
  if (!ev || !ev.points || !ev.points.length) { resetHighlight(); return; }
  const idx = ev.points[0].pointIndex, id = nodeIds[idx];
  const up = new Set(), down = new Set();
  const seen = new Set([id]);
  const walk = (nid, key, acc) => { (adj[nid][key] || []).forEach(m => {
    if (!seen.has(m)) { seen.add(m); acc.add(m); walk(m, key, acc); } }); };
  walk(id, 'p', up); walk(id, 'c', down);
  const op = nodeIds.map(nid => nid === id ? 1 : (up.has(nid) || down.has(nid) ? 0.95 : 0.12));
  const col = nodeIds.map(nid => nid === id ? '#ffd166' : (up.has(nid) ? '#f4a259' : (down.has(nid) ? '#7fb3ff' : d.nodes[nid].color)));
  Plotly.restyle('graph', {'marker.opacity': [op], 'marker.color': [col]}, [1]);
  showDetail(id);
}
function resetHighlight() {
  const d = DATA[cur];
  Plotly.restyle('graph', {'marker.opacity': [[0.95]], 'marker.color': [[nodeIds.map(id => d.nodes[id].color)]]}, [1]);
}
function showDetail(id) {
  const d = DATA[cur], n = d.nodes[id], el = document.getElementById('detail');
  const pre = n.prereqs.map(p => d.nodes[p].name).join(', ') || 'none \u2014 foundation';
  el.innerHTML = '<h3>' + n.name + ' <span style="color:#9aa7c2;font-weight:normal">(' + n.topic + ')</span></h3>'
    + '<div class="math" id="dmath"></div>'
    + '<p>' + n.desc + '</p>'
    + '<p>Builds on: ' + pre + ' \u00b7 Unlocks ' + n.leverage + ' formulas \u00b7 Wave ' + n.level + '</p>';
  try { katex.render(n.latex, document.getElementById('dmath'), {displayMode: true, throwOnError: false}); }
  catch (e) { document.getElementById('dmath').textContent = n.latex; }
}

function renderTimeline() {
  const d = DATA[cur], t = d.timeline;
  Plotly.newPlot('timeline',
    [{y: t.map(r => r.name), x: t.map(r => Math.max(r.leverage, 0.4)), type: 'bar', orientation: 'h',
      marker: {color: t.map(r => r.color)}, hovertemplate: '<b>%{y}</b><br>wave %{customdata}<extra></extra>',
      customdata: t.map(r => r.wave), showlegend: false}],
    dark({margin: {l: 230, r: 20, t: 10, b: 30},
      xaxis: {title: 'formulas unlocked'},
      yaxis: {type: 'category', autorange: 'reversed', tickfont: {size: 10}}}),
    {responsive: true, displayModeBar: false});
}
function renderLeverage() {
  const d = DATA[cur], L = d.leverage_top;
  Plotly.newPlot('leverage',
    [{y: L.map(r => r.name), x: L.map(r => r.unlocks), type: 'bar', orientation: 'h',
      marker: {color: '#7fb3ff'}, hovertemplate: '<b>%{y}</b><br>unlocks %{x} formulas<extra></extra>', showlegend: false}],
    dark({margin: {l: 230, r: 20, t: 10, b: 30},
      xaxis: {title: 'formulas unlocked'},
      yaxis: {type: 'category', autorange: 'reversed', tickfont: {size: 10}}}),
    {responsive: true, displayModeBar: false});
}
function renderCurve() {
  const d = DATA[cur], c = d.curve;
  Plotly.newPlot('curve',
    [{x: c.map(p => p[0]), y: c.map(p => p[1] * 100), type: 'scatter', mode: 'lines+markers',
      line: {color: '#81b29a', width: 3}, marker: {size: 7},
      hovertemplate: 'memorize %{x}: %{y:.0f}% derivable<extra></extra>', showlegend: false}],
    dark({margin: {l: 50, r: 20, t: 10, b: 40},
      xaxis: {title: 'formulas memorized'}, yaxis: {title: '% of sheet derivable', range: [0, 105]}}),
    {responsive: true, displayModeBar: false});
  document.getElementById('minset').innerHTML =
    d.stats.min_set.map(m => '<span class="pill">' + m.name + '</span>').join('');
}
function renderCards() {
  const s = DATA[cur].stats;
  document.getElementById('cards').innerHTML =
    '<div class="card"><div class="v">' + s.nodes + '</div><div class="k">formulas</div></div>'
    + '<div class="card"><div class="v">' + s.edges + '</div><div class="k">derivations</div></div>'
    + '<div class="card"><div class="v">' + s.waves + '</div><div class="k">study waves</div></div>'
    + '<div class="card"><div class="v">' + s.min_size + '</div><div class="k">to memorize</div></div>'
    + '<div class="card"><div class="v">' + s.top_unlocker.unlocks + '</div><div class="k">unlocked by<br>' + s.top_unlocker.name + '</div></div>';
}
function renderAll() {
  renderCards(); renderGraph(); renderTimeline(); renderLeverage(); renderCurve();
  document.getElementById('detail').innerHTML = '<p>Click a node in the graph to inspect it.</p>';
}
sheetSel.addEventListener('change', () => { cur = sheetSel.value; renderAll(); });
renderAll();
`

// Build generates the dashboard HTML and writes it to out.
func Build(out string) error {
	sheets, err := models.LoadAll()
	if err != nil {
		return err
	}
	names := models.ListSheets()
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
