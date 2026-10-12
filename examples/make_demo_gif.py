"""Render docs/demo.gif: the PHY 131 dependency graph assembling itself.

Reproducible: real sheet data, real graph algorithms. Phases:
1. title card, 2. waves appear in study order with edges, 3. the
minimum-memorize set lights up in gold with the coverage stat.
"""

import os

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
from matplotlib.animation import FuncAnimation

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.join(HERE, "..")
DOCS = os.path.join(ROOT, "docs")
SHEET = "phy131"
FPS = 10


def main():
    import sys

    sys.path.insert(0, os.path.join(ROOT, "src"))
    from formulaforge.graph import FormulaGraph
    from formulaforge.models import load_sheet_by_name

    sheet = load_sheet_by_name(SHEET)
    g = FormulaGraph(sheet)
    g.assert_acyclic()
    waves = g.waves()
    levels = g.levels()
    order = g.topo_order()
    name_of = {f.id: f.name for f in sheet.formulas}

    xs, ys = {}, {}
    for lv, wave in enumerate(waves):
        for j, nid in enumerate(wave):
            xs[nid] = (j - (len(wave) - 1) / 2) * 1.7
            ys[nid] = -lv

    minset = set(g.min_memorize_set())
    edges = [(p, n) for n in g.nodes for p in g.prereqs[n]]

    # frame plan
    TITLE_FRAMES = 12
    per_wave = 10
    wave_frames = [TITLE_FRAMES + i * per_wave for i in range(len(waves) + 1)]
    FINALE_START = wave_frames[-1]
    FINALE_FRAMES = 25
    total = FINALE_START + FINALE_FRAMES

    fig, ax = plt.subplots(figsize=(6.4, 4.2))
    fig.patch.set_facecolor("#0f1420")
    ax.set_facecolor("#0f1420")
    ax.set_xlim(-6.5, 6.5)
    ax.set_ylim(-len(waves) - 0.6, 1.2)
    ax.axis("off")

    def visible_at(f):
        if f < TITLE_FRAMES:
            return set()
        out = set()
        for i, start in enumerate(wave_frames[:-1]):
            if f >= start:
                out.update(waves[i])
        return out

    def frame(f):
        ax.clear()
        ax.set_xlim(-6.5, 6.5)
        ax.set_ylim(-len(waves) - 0.6, 1.2)
        ax.axis("off")
        if f < TITLE_FRAMES:
            ax.text(0, 0.1, "formulaforge", ha="center", va="center",
                    fontsize=30, color="#7fb3ff", weight="bold")
            ax.text(0, -0.7, "PHY 131 \u2014 every formula, mapped by what it builds on",
                    ha="center", va="center", fontsize=11, color="#9aa7c2")
            return
        vis = visible_at(f)
        finale = f >= FINALE_START
        # edges
        for p, n in edges:
            if p in vis and n in vis:
                ax.plot([xs[p], xs[n]], [ys[p], ys[n]], color="#3b507c",
                        lw=0.8, alpha=0.8, zorder=1)
        # nodes
        for nid in vis:
            lv = g.leverage(nid)
            size = 60 + 26 * np.sqrt(lv)
            if finale and nid in minset:
                color, ec, a = "#ffd166", "#ffffff", 1.0
            elif finale:
                color, ec, a = "#2c3a55", "#4a5a78", 0.55
            else:
                color, ec, a = "#7fb3ff", "#0f1420", 0.95
            ax.scatter([xs[nid]], [ys[nid]], s=size, c=color, ec=ec,
                       zorder=2, alpha=a)
        # current wave label
        if not finale:
            w = min(len(waves) - 1, max(i for i, s in enumerate(wave_frames[:-1]) if f >= s))
            ax.text(0, 0.75, f"study wave {w + 1} of {len(waves)}",
                    ha="center", fontsize=12, color="#e8ecf4", weight="bold")
        else:
            ax.text(0, 0.75, f"memorize {len(minset)}, derive the other {len(order) - len(minset)}",
                    ha="center", fontsize=12, color="#ffd166", weight="bold")
            ax.text(0, 0.35, "minimum-memorize set (greedy set cover)",
                    ha="center", fontsize=9, color="#9aa7c2")

    anim = FuncAnimation(fig, frame, frames=total, blit=False)
    os.makedirs(DOCS, exist_ok=True)
    out = os.path.join(DOCS, "demo.gif")
    anim.save(out, writer="pillow", fps=FPS)
    plt.close(fig)
    print(f"gif -> {out} ({os.path.getsize(out) / 1e6:.2f} MB, {total} frames)")


if __name__ == "__main__":
    main()
