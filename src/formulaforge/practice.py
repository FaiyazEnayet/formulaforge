"""Practice mode: quiz yourself on derivations.

Three modes:
- derive: given a formula, name the formulas it directly builds on.
- order:  given two formulas, say which must be learned first.
- state:  given a name, recall the formula (self-graded).

Question generators take a seed for reproducibility; answer providers are
callables so tests can script them and the CLI can wire up stdin.
"""

from __future__ import annotations

import random


def _rng(seed):
    return random.Random(seed)


def gen_derive_questions(graph, n: int, seed: int = 7) -> list:
    """Formulas that have at least one prerequisite."""
    rng = _rng(seed)
    cands = [f.id for f in graph.sheet.formulas if graph.prereqs[f.id]]
    rng.shuffle(cands)
    return [{"type": "derive", "formula": fid} for fid in cands[:n]]


def gen_order_questions(graph, n: int, seed: int = 7) -> list:
    """Pairs (a, b) where a is a strict ancestor of b: answer is a."""
    rng = _rng(seed)
    pairs = []
    nodes = graph.nodes
    for a in nodes:
        for b in graph.descendants(a):
            pairs.append((a, b))
    rng.shuffle(pairs)
    out = []
    for a, b in pairs[:n]:
        first, second = (a, b) if rng.random() < 0.5 else (b, a)
        out.append({"type": "order", "first": first, "second": second, "answer": a})
    return out


def gen_state_questions(graph, n: int, seed: int = 7) -> list:
    rng = _rng(seed)
    ids = list(graph.nodes)
    rng.shuffle(ids)
    return [{"type": "state", "formula": fid} for fid in ids[:n]]


def check_derive(graph, fid: str, given: list) -> dict:
    """Score a derive answer with precision/recall/F1 over prerequisite ids."""
    truth = set(graph.prereqs[fid])
    got = {g.strip() for g in given if g.strip()}
    tp = len(truth & got)
    precision = tp / len(got) if got else 0.0
    recall = tp / len(truth) if truth else 1.0
    f1 = 2 * precision * recall / (precision + recall) if (precision + recall) else 0.0
    return {
        "truth": sorted(truth),
        "given": sorted(got),
        "precision": precision,
        "recall": recall,
        "f1": f1,
    }


def check_order(question: dict, answer: str) -> bool:
    return answer.strip() == question["answer"]


def run_session(graph, questions: list, answer_of) -> dict:
    """answer_of(question) -> answer. Returns per-question results + summary."""
    results = []
    for q in questions:
        if q["type"] == "derive":
            r = check_derive(graph, q["formula"], answer_of(q))
            ok = r["f1"] == 1.0
            results.append({"question": q, "ok": ok, "detail": r})
        elif q["type"] == "order":
            ans = answer_of(q)
            ok = check_order(q, ans)
            results.append({"question": q, "ok": ok, "answer": ans})
        elif q["type"] == "state":
            ans = str(answer_of(q)).strip().lower()
            ok = ans in ("y", "yes")
            results.append({"question": q, "ok": ok, "self_report": ans})
    correct = sum(1 for r in results if r["ok"])
    return {"results": results, "correct": correct, "total": len(results)}
