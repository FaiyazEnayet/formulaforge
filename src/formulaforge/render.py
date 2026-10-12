"""Render formula sheets to beautiful output.

- HTML: printable page with KaTeX-rendered math, topic sections, and
  "builds on" chips linking each formula to its prerequisites.
- PDF: paginated PDF via matplotlib mathtext (no browser or LaTeX install
  needed). A sanitizer maps LaTeX constructs mathtext cannot handle.
"""

from __future__ import annotations

import html as htmlmod
import re

KATEX_CSS = "https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/katex.min.css"
KATEX_JS = "https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/katex.min.js"
KATEX_AUTOR = "https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/contrib/auto-render.min.js"

CSS = """
:root { --ink: #1a1a2e; --accent: #4a6fa5; --paper: #ffffff; --wash: #f4f6fa; }
* { box-sizing: border-box; }
body { font-family: Georgia, 'Times New Roman', serif; color: var(--ink);
       max-width: 860px; margin: 0 auto; padding: 2rem 1.5rem; background: var(--paper); }
header.sheet-head { border-bottom: 3px solid var(--accent); margin-bottom: 1.5rem; padding-bottom: 1rem; }
header.sheet-head h1 { margin: 0 0 .25rem; font-size: 2rem; }
header.sheet-head .course { color: var(--accent); font-weight: bold; letter-spacing: .08em;
  text-transform: uppercase; font-size: .85rem; font-family: system-ui, sans-serif; }
header.sheet-head p.desc { color: #444; font-style: italic; }
.stats { display: flex; gap: .6rem; flex-wrap: wrap; margin: .8rem 0; font-family: system-ui, sans-serif; }
.stat { background: var(--wash); border-radius: 8px; padding: .35rem .7rem; font-size: .85rem; }
.stat b { color: var(--accent); }
h2.topic { font-family: system-ui, sans-serif; color: var(--accent); margin: 2rem 0 .8rem;
  font-size: 1.15rem; border-bottom: 1px solid #dde3ee; padding-bottom: .3rem; }
.card { background: var(--wash); border-radius: 10px; padding: .9rem 1.1rem; margin: 0 0 .9rem; }
.card h3 { margin: 0 0 .4rem; font-size: 1.05rem; font-family: system-ui, sans-serif; }
.card .math { font-size: 1.15rem; overflow-x: auto; padding: .3rem 0; }
.card p.desc { margin: .4rem 0; color: #333; font-size: .95rem; }
.card .meta { font-family: system-ui, sans-serif; font-size: .78rem; color: #666; margin-top: .5rem; }
.chip { display: inline-block; background: #e2e9f5; color: #2c4a73; border-radius: 20px;
  padding: .1rem .55rem; margin: .1rem .15rem .1rem 0; font-size: .75rem; }
footer { margin-top: 2.5rem; padding-top: 1rem; border-top: 1px solid #dde3ee;
  font-family: system-ui, sans-serif; font-size: .8rem; color: #888; }
@media print {
  body { max-width: 100%; padding: 0; }
  .card { break-inside: avoid; background: #fff; border: 1px solid #ddd; }
}
"""


def _topics_in_order(sheet) -> list:
    seen, order = set(), []
    for f in sheet.formulas:
        if f.topic not in seen:
            seen.add(f.topic)
            order.append(f.topic)
    return order


def render_html(sheet, graph=None) -> str:
    """Full standalone HTML page for a sheet."""
    if graph is None:
        from .graph import FormulaGraph

        graph = FormulaGraph(sheet)
    stats = graph.stats()
    parts = []
    parts.append("<!DOCTYPE html><html lang='en'><head><meta charset='utf-8'>")
    parts.append("<meta name='viewport' content='width=device-width, initial-scale=1'>")
    parts.append(f"<title>{htmlmod.escape(sheet.title)} \u2014 formulaforge</title>")
    parts.append(f"<link rel='stylesheet' href='{KATEX_CSS}'>")
    parts.append(f"<script defer src='{KATEX_JS}'></script>")
    parts.append(f"<script defer src='{KATEX_AUTOR}'></script>")
    parts.append(
        "<script>document.addEventListener('DOMContentLoaded',function(){"
        "renderMathInElement(document.body,{delimiters:["
        "{left:'$$',right:'$$',display:true},{left:'\\\\(',right:'\\\\)',display:false}"
        "]});});</script>"
    )
    parts.append(f"<style>{CSS}</style></head><body>")
    parts.append("<header class='sheet-head'>")
    parts.append(f"<div class='course'>{htmlmod.escape(sheet.course)}</div>")
    parts.append(f"<h1>{htmlmod.escape(sheet.title)}</h1>")
    parts.append(f"<p class='desc'>{htmlmod.escape(sheet.description)}</p>")
    parts.append("<div class='stats'>")
    parts.append(f"<span class='stat'><b>{stats['nodes']}</b> formulas</span>")
    parts.append(f"<span class='stat'><b>{stats['edges']}</b> derivations</span>")
    parts.append(f"<span class='stat'><b>{stats['waves']}</b> study waves</span>")
    parts.append(
        f"<span class='stat'>memorize <b>{stats['min_memorize_size']}</b>, derive the rest</span>"
    )
    parts.append("</div></header>")

    by_topic = {}
    for f in sheet.formulas:
        by_topic.setdefault(f.topic, []).append(f)
    name_of = {f.id: f.name for f in sheet.formulas}

    for topic in _topics_in_order(sheet):
        parts.append(f"<h2 class='topic'>{htmlmod.escape(topic)}</h2>")
        for f in by_topic[topic]:
            parts.append(f"<div class='card' id='{f.id}'>")
            parts.append(f"<h3>{htmlmod.escape(f.name)}</h3>")
            parts.append(f"<div class='math'>$${f.latex}$$</div>")
            if f.description:
                parts.append(f"<p class='desc'>{htmlmod.escape(f.description)}</p>")
            parts.append("<div class='meta'>")
            if f.derives_from:
                chips = "".join(
                    f"<span class='chip'>{htmlmod.escape(name_of[d])}</span>"
                    for d in f.derives_from
                )
                parts.append(f"Builds on: {chips}")
            else:
                parts.append("Foundation formula \u2014 learn first")
            for t in f.tags:
                parts.append(f"<span class='chip'>#{htmlmod.escape(t)}</span>")
            parts.append("</div></div>")
    parts.append(
        "<footer>Built with formulaforge \u2014 write sheets in Markdown + LaTeX, "
        "study the dependency graph.</footer>"
    )
    parts.append("</body></html>")
    return "\n".join(parts)


# ---- PDF -------------------------------------------------------------------

_PMATH_SUBS = [
    (re.compile(r"\\tfrac"), r"\\frac"),
    (re.compile(r"\\lVert"), r"\\Vert"),
    (re.compile(r"\\rVert"), r"\\Vert"),
    (re.compile(r"\\boldsymbol\{([^{}]+)\}"), r"\1"),
    (re.compile(r"\\mathbb\{([A-Za-z])\}"), r"\\mathbf{\1}"),
    (re.compile(r"\\iint"), r"\\int\\int"),
    (re.compile(r"\\iiint"), r"\\int\\int\\int"),
    (re.compile(r"\\oiint"), r"\\oint"),
    (re.compile(r"\\%"), r"\\%"),
    (re.compile(r"\\le(?![a-zA-Z])"), r"\\leq"),
    (re.compile(r"\\ge(?![a-zA-Z])"), r"\\geq"),
    (re.compile(r"\\iff(?![a-zA-Z])"), r"\\Leftrightarrow"),
    (re.compile(r"\\implies(?![a-zA-Z])"), r"\\Rightarrow"),
]


def _pmatrix_to_brackets(latex: str) -> str:
    """Replace pmatrix environments with bracket notation mathtext can parse."""

    def repl(m):
        body = m.group(1).replace(r"\\", "; ").replace("&", ", ")
        body = re.sub(r"\s+", " ", body).strip()
        return r"\left[ " + body + r" \right]"

    return re.sub(
        r"\\begin\{pmatrix\}(.*?)\\end\{pmatrix\}", repl, latex, flags=re.DOTALL
    )


def sanitize_for_pdf(latex: str) -> str:
    out = _pmatrix_to_brackets(latex)
    for pat, rep in _PMATH_SUBS:
        out = pat.sub(rep, out)
    return out


def render_pdf(sheet, path: str, graph=None) -> dict:
    """Paginated PDF via matplotlib mathtext. Returns {'pages', 'fallbacks'}."""
    import textwrap

    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    from matplotlib.backends.backend_pdf import PdfPages

    if graph is None:
        from .graph import FormulaGraph

        graph = FormulaGraph(sheet)
    name_of = {f.id: f.name for f in sheet.formulas}
    fallbacks = []
    pages = 0

    with PdfPages(path) as pdf:
        # title page
        fig = plt.figure(figsize=(8.5, 11))
        fig.text(0.5, 0.62, sheet.course, ha="center", fontsize=16, color="#4a6fa5",
                 weight="bold")
        fig.text(0.5, 0.55, sheet.title, ha="center", fontsize=26, weight="bold")
        fig.text(0.5, 0.48, sheet.description, ha="center", fontsize=11,
                 color="#555555", wrap=True)
        st = graph.stats()
        fig.text(
            0.5, 0.38,
            f"{st['nodes']} formulas  |  {st['edges']} derivations  |  "
            f"{st['waves']} study waves  |  memorize {st['min_memorize_size']}, derive the rest",
            ha="center", fontsize=10, color="#333333",
        )
        fig.text(0.5, 0.06, "Built with formulaforge", ha="center",
                 fontsize=9, color="#888888")
        pdf.savefig(fig)
        plt.close(fig)
        pages += 1

        for topic in _topics_in_order(sheet):
            formulas = [f for f in sheet.formulas if f.topic == topic]
            # ~4 formulas per page
            for i in range(0, len(formulas), 4):
                chunk = formulas[i : i + 4]
                fig = plt.figure(figsize=(8.5, 11))
                fig.text(0.08, 0.94, topic, fontsize=15, weight="bold",
                         color="#4a6fa5")
                y = 0.86
                for f in chunk:
                    fig.text(0.08, y, f.name, fontsize=12, weight="bold")
                    y -= 0.035
                    math = "$" + sanitize_for_pdf(f.latex) + "$"
                    try:
                        fig.text(0.10, y, math, fontsize=11)
                    except Exception:
                        fallbacks.append(f.id)
                        fig.text(0.10, y, "LaTeX: " + f.latex[:90],
                                 fontsize=9, family="monospace")
                    y -= 0.055
                    for line in textwrap.wrap(f.description, 92):
                        fig.text(0.08, y, line, fontsize=9.5, color="#333333")
                        y -= 0.028
                    if f.derives_from:
                        deps = ", ".join(name_of[d] for d in f.derives_from)
                        fig.text(0.08, y, "Builds on: " + deps, fontsize=8.5,
                                 color="#4a6fa5", style="italic")
                        y -= 0.030
                    y -= 0.045
                fig.text(0.5, 0.04, "formulaforge", ha="center", fontsize=8,
                         color="#aaaaaa")
                pdf.savefig(fig)
                plt.close(fig)
                pages += 1
    return {"pages": pages, "fallbacks": fallbacks}
