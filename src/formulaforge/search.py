"""Full-text search over formula sheets.

Tokenizes name, topic, tags, description, and LaTeX source; ranks by
weighted term hits (name > topic/tags > description > latex).
"""

from __future__ import annotations

import re

_TOKEN = re.compile(r"[a-z0-9]+")


def tokenize(text: str) -> list:
    return _TOKEN.findall(text.lower())


def score_formula(formula, terms: list) -> float:
    name_toks = set(tokenize(formula.name))
    topic_toks = set(tokenize(formula.topic)) | {t.lower() for t in formula.tags}
    desc_toks = set(tokenize(formula.description))
    latex_toks = set(tokenize(formula.latex))
    score = 0.0
    for term in terms:
        hit = False
        if term in name_toks:
            score += 3.0
            hit = True
        if term in topic_toks:
            score += 2.0
            hit = True
        if term in desc_toks:
            score += 1.0
            hit = True
        if not hit and term in latex_toks:
            score += 0.5
    return score


def search(sheets: dict, query: str, top_k: int = 10) -> list:
    """sheets: {name: Sheet}. Returns [(score, sheet_name, Formula)] ranked."""
    terms = tokenize(query)
    if not terms:
        return []
    hits = []
    for sname, sheet in sheets.items():
        for f in sheet.formulas:
            sc = score_formula(f, terms)
            if sc > 0:
                hits.append((sc, sname, f))
    hits.sort(key=lambda t: (-t[0], t[1], t[2].id))
    return hits[:top_k]
