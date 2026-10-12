"""Build docs/dashboard.html: self-contained interactive Plotly dashboard.

Panels: dependency-graph explorer (click a formula to highlight its
derivation chain), study-order timeline, leverage ranking, coverage curve,
and stat cards. One <select> switches between the four seeded sheets.
plotly.js is inlined so the file works offline; KaTeX (CDN) renders the
selected formula.
"""

import json
import os

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.join(HERE, "..")
DOCS = os.path.join(ROOT, "docs")

PALETTE = [
    "#4a6fa5", "#e07a5f", "#81b29a", "#f2cc8f", "#9d7bff",
    "#5fc9c9", "#e58bb1", "#a3b18a", "#f4845f", "#6d9dc5",
]

with open("/tmp/plotly.min.js", encoding="utf-8") as fh:
    PLOTLY_JS = fh.read()


def sheet_payload(name, sheet, graph):
    levels = graph.levels()
    order = graph.topo_order()
    pos_in_order = {n: i for i, n in enumerate(order)}
    waves = graph.waves()
    stats = graph.stats()
    topics = []
    for f in sheet.formulas:
        if f.topic not in topics:
            topics.append(f.topic)
    tcolor = {t: PALETTE[i % len(PALETTE)] for i, t in enumerate(topics)}

    # layered layout: x spread within level, y = -level
    xs, ys = {}, {}
    for lv, wave in enumerate(waves):
        for j, nid in enumerate(wave):
            xs[nid] = (j - (len(wave) - 1) / 2) * 1.6
            ys[nid] = -lv

    nodes = {}
    for f in sheet.formulas:
        nodes[f.id] = {
            "name": f.name,
            "topic": f.topic,
            "color": tcolor[f.topic],
            "x": xs[f.id],
            "y": ys[f.id],
            "level": levels[f.id],
            "leverage": graph.leverage(f.id),
            "n_prereq": len(graph.prereqs[f.id]),
            "latex": f.latex,
            "desc": f.description,
            "prereqs": sorted(graph.prereqs[f.id]),
            "children": sorted(graph.children[f.id]),
            "order": pos_in_order[f.id],
        }
    edges = [{"from": p, "to": n} for n in graph.nodes for p in graph.prereqs[n]]
    curve = graph.coverage_curve()
    leverage_top = [
        {"id": nid, "name": sheet.by_id(nid).name, "unlocks": u}
        for nid, u in graph.leverage_ranking()[:12]
    ]
    timeline = [
        {"id": nid, "name": sheet.by_id(nid).name, "wave": levels[nid],
         "leverage": graph.leverage(nid), "color": tcolor[sheet.by_id(nid).topic]}
        for nid in order
    ]
    return {
        "course": sheet.course,
        "title": sheet.title,
        "description": sheet.description,
        "topics": topics,
        "nodes": nodes,
        "edges": edges,
        "stats": {
            "nodes": stats["nodes"],
            "edges": stats["edges"],
            "waves": stats["waves"],
            "depth": stats["depth"],
            "min_size": stats["min_memorize_size"],
            "min_set": [
                {"id": nid, "name": sheet.by_id(nid).name}
                for nid in stats["min_memorize_set"]
            ],
            "top_unlocker": {
                "name": sheet.by_id(stats["top_leverage"][0][0]).name,
                "unlocks": stats["top_leverage"][0][1],
            },
        },
        "curve": curve,
        "leverage_top": leverage_top,
        "timeline": timeline,
    }


def main():
    import sys

    sys.path.insert(0, os.path.join(ROOT, "src"))
    from formulaforge.graph import FormulaGraph
    from formulaforge.models import list_sheets, load_sheet_by_name

    data = {}
    order_names = []
    for name in list_sheets():
        sheet = load_sheet_by_name(name)
        graph = FormulaGraph(sheet)
        graph.assert_acyclic()
        data[name] = sheet_payload(name, sheet, graph)
        order_names.append(name)

    payload_json = json.dumps(data)
    names_json = json.dumps(order_names)

    page = """<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>formulaforge \u2014 dependency graph explorer</title>
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
<h1><span class="ff">formulaforge</span> \u2014 dependency graph explorer</h1>
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
<footer>Built with formulaforge \u2014 numbers computed live from the sheet graphs.</footer>
</div>
<script>""" + PLOTLY_JS + """</script>
<script>
const DATA = """ + payload_json + """;
const ORDER = """ + names_json + """;
const DARK = {paper_bgcolor: '#141b2c', plot_bgcolor: '#141b2c',
  font: {color: '#e8ecf4'}, xaxis: {gridcolor: '#232f4a'}, yaxis: {gridcolor: '#232f4a'}};

const sheetSel = document.getElementById('sheet');
ORDER.forEach(n => { const o = document.createElement('option'); o.value = n;
  o.textContent = DATA[n].course + ' \\u2014 ' + DATA[n].title; sheetSel.appendChild(o); });

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
    Object.assign({margin: {l: 20, r: 20, t: 10, b: 20},
      xaxis: {visible: false}, yaxis: {visible: false}, hovermode: 'closest'}, DARK),
    {responsive: true, displayModeBar: false});
  document.getElementById('graph').on('plotly_click', onNodeClick);
}
function onNodeClick(ev) {
  const d = DATA[cur];
  if (!ev || !ev.points || !ev.points.length) { resetHighlight(); return; }
  const idx = ev.points[0].pointIndex, id = nodeIds[idx];
  const up = new Set(), down = new Set(), stack = [id];
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
    Object.assign({margin: {l: 230, r: 20, t: 10, b: 30},
      xaxis: {title: 'formulas unlocked'}, yaxis: {autorange: 'reversed', tickfont: {size: 10}}}, DARK),
    {responsive: true, displayModeBar: false});
}
function renderLeverage() {
  const d = DATA[cur], L = d.leverage_top;
  Plotly.newPlot('leverage',
    [{y: L.map(r => r.name), x: L.map(r => r.unlocks), type: 'bar', orientation: 'h',
      marker: {color: '#7fb3ff'}, hovertemplate: '<b>%{y}</b><br>unlocks %{x} formulas<extra></extra>', showlegend: false}],
    Object.assign({margin: {l: 230, r: 20, t: 10, b: 30},
      xaxis: {title: 'formulas unlocked'}, yaxis: {autorange: 'reversed', tickfont: {size: 10}}}, DARK),
    {responsive: true, displayModeBar: false});
}
function renderCurve() {
  const d = DATA[cur], c = d.curve;
  Plotly.newPlot('curve',
    [{x: c.map(p => p[0]), y: c.map(p => p[1] * 100), type: 'scatter', mode: 'lines+markers',
      line: {color: '#81b29a', width: 3}, marker: {size: 7},
      hovertemplate: 'memorize %{x}: %{y:.0f}% derivable<extra></extra>', showlegend: false}],
    Object.assign({margin: {l: 50, r: 20, t: 10, b: 40},
      xaxis: {title: 'formulas memorized'}, yaxis: {title: '% of sheet derivable', range: [0, 105]}}, DARK),
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
</script>
</body></html>"""

    os.makedirs(DOCS, exist_ok=True)
    out = os.path.join(DOCS, "dashboard.html")
    with open(out, "w", encoding="utf-8") as fh:
        fh.write(page)
    print(f"dashboard -> {out} ({os.path.getsize(out) / 1e6:.1f} MB)")


if __name__ == "__main__":
    main()
