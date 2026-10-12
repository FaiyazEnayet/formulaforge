"""formulaforge command line interface."""

from __future__ import annotations

import argparse
import json
import os
import sys

from .models import list_sheets, load_sheet_by_name, load_sheet


def _all_sheets() -> dict:
    return {name: load_sheet_by_name(name) for name in list_sheets()}


def _graph_of(sheet):
    from .graph import FormulaGraph

    return FormulaGraph(sheet)


def cmd_sheets(_args):
    for name in list_sheets():
        s = load_sheet_by_name(name)
        print(f"{name:10} {s.course:8} {s.title} ({len(s.formulas)} formulas)")


def cmd_build(args):
    sheet = load_sheet_by_name(args.sheet)
    from .render import render_html

    out = args.output or f"{args.sheet}.html"
    with open(out, "w", encoding="utf-8") as fh:
        fh.write(render_html(sheet))
    print(f"built {out} ({len(sheet.formulas)} formulas)")


def cmd_graph(args):
    sheet = load_sheet_by_name(args.sheet)
    g = _graph_of(sheet)
    g.assert_acyclic()
    st = g.stats()
    if args.json:
        payload = {
            "sheet": args.sheet,
            "nodes": [{"id": f.id, "name": f.name, "topic": f.topic,
                       "level": g.levels()[f.id],
                       "leverage": g.leverage(f.id)} for f in sheet.formulas],
            "edges": [{"from": p, "to": n} for n in g.nodes for p in g.prereqs[n]],
            "stats": {k: v for k, v in st.items() if k != "top_leverage"},
            "top_leverage": [{"id": n, "unlocks": u} for n, u in st["top_leverage"]],
        }
        out = args.output or f"{args.sheet}.graph.json"
        with open(out, "w", encoding="utf-8") as fh:
            json.dump(payload, fh, indent=2)
        print(f"wrote {out}")
        return
    print(f"{sheet.course} {sheet.title}")
    print(f"  formulas:   {st['nodes']}")
    print(f"  derivations:{st['edges']}")
    print(f"  study waves:{st['waves']} (longest chain: {st['depth']})")
    print(f"  roots: {st['roots']}, leaves: {st['leaves']}")
    print("  highest leverage (unlocks the most):")
    for fid, n in st["top_leverage"]:
        print(f"    {sheet.by_id(fid).name}: {n}")
    print(f"  minimum memorize set ({st['min_memorize_size']}):")
    for fid in st["min_memorize_set"]:
        print(f"    - {sheet.by_id(fid).name}")


def cmd_study_order(args):
    sheet = load_sheet_by_name(args.sheet)
    g = _graph_of(sheet)
    g.assert_acyclic()
    for i, wave in enumerate(g.waves(), 1):
        print(f"Wave {i}:")
        for fid in wave:
            print(f"  - {sheet.by_id(fid).name}")


def cmd_export(args):
    sheet = load_sheet_by_name(args.sheet)
    fmt = args.format
    out = args.output or f"{args.sheet}.{fmt if fmt != 'anki' else 'apkg'}"
    if fmt == "html":
        from .render import render_html

        with open(out, "w", encoding="utf-8") as fh:
            fh.write(render_html(sheet))
    elif fmt == "pdf":
        from .render import render_pdf

        info = render_pdf(sheet, out)
        print(f"  pages: {info['pages']}, math fallbacks: {len(info['fallbacks'])}")
    elif fmt == "anki":
        from .anki import export_deck

        info = export_deck(sheet, out)
        print(f"  notes: {info['notes']}, cards: {info['cards']}")
    else:  # pragma: no cover
        raise SystemExit(f"unknown format: {fmt}")
    print(f"exported {out}")


def cmd_search(args):
    from .search import search

    sheets = _all_sheets()
    if args.sheet:
        sheets = {args.sheet: sheets[args.sheet]}
    hits = search(sheets, args.query, top_k=args.top)
    if not hits:
        print("no matches")
        return
    for score, sname, f in hits:
        print(f"[{sname}] {f.name} ({f.topic})  score={score:.1f}")


def cmd_practice(args):
    from .practice import (
        gen_derive_questions,
        gen_order_questions,
        gen_state_questions,
        run_session,
    )

    sheet = load_sheet_by_name(args.sheet)
    g = _graph_of(sheet)
    g.assert_acyclic()
    gen = {"derive": gen_derive_questions, "order": gen_order_questions,
           "state": gen_state_questions}[args.mode]
    questions = gen(g, args.n, seed=args.seed)
    if not questions:
        print("no questions available for this mode")
        return
    name_of = {f.id: f.name for f in sheet.formulas}

    def ask(q):
        if q["type"] == "derive":
            print(f"\n{q['type'].upper()}: '{name_of[q['formula']]}' builds on which formulas?")
            print("  (comma-separated ids; available: " +
                  ", ".join(sorted(g.nodes)) + ")")
            return input("> ").split(",")
        if q["type"] == "order":
            a, b = name_of[q["first"]], name_of[q["second"]]
            print(f"\nORDER: which must be learned first?\n  1) {a}\n  2) {b}")
            pick = input("1 or 2 > ").strip()
            return q["first"] if pick == "1" else q["second"]
        print(f"\nSTATE: {name_of[q['formula']]}")
        print(f"  {sheet.by_id(q['formula']).description}")
        return input("did you recall it? (y/n) > ")

    summary = run_session(g, questions, ask)
    print(f"\nscore: {summary['correct']}/{summary['total']}")
    for r in summary["results"]:
        if not r["ok"] and r["question"]["type"] == "derive":
            truth = [name_of[t] for t in r["detail"]["truth"]]
            print(f"  miss: {name_of[r['question']['formula']]} -> {', '.join(truth)}")


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="formulaforge",
                                description="Formula sheet studio for STEM students.")
    sub = p.add_subparsers(dest="cmd", required=True)

    s = sub.add_parser("sheets", help="list available sheets")
    s.set_defaults(fn=cmd_sheets)

    s = sub.add_parser("build", help="render a sheet to printable HTML")
    s.add_argument("sheet")
    s.add_argument("-o", "--output")
    s.set_defaults(fn=cmd_build)

    s = sub.add_parser("graph", help="dependency graph stats and centrality")
    s.add_argument("sheet")
    s.add_argument("--json", action="store_true")
    s.add_argument("-o", "--output")
    s.set_defaults(fn=cmd_graph)

    s = sub.add_parser("study-order", help="topological study order in waves")
    s.add_argument("sheet")
    s.set_defaults(fn=cmd_study_order)

    s = sub.add_parser("export", help="export a sheet")
    s.add_argument("sheet")
    s.add_argument("--format", choices=["html", "pdf", "anki"], required=True)
    s.add_argument("-o", "--output")
    s.set_defaults(fn=cmd_export)

    s = sub.add_parser("search", help="full-text search across sheets")
    s.add_argument("query")
    s.add_argument("--sheet")
    s.add_argument("--top", type=int, default=10)
    s.set_defaults(fn=cmd_search)

    s = sub.add_parser("practice", help="quiz yourself on derivations")
    s.add_argument("sheet")
    s.add_argument("--mode", choices=["derive", "order", "state"], default="derive")
    s.add_argument("--n", type=int, default=10)
    s.add_argument("--seed", type=int, default=7)
    s.set_defaults(fn=cmd_practice)
    return p


def main(argv=None):
    args = build_parser().parse_args(argv)
    args.fn(args)


if __name__ == "__main__":
    main()
