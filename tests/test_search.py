"""Tests for full-text formula search."""

from formulaforge.models import load_sheet_by_name
from formulaforge.search import search, tokenize


def sheets():
    return {n: load_sheet_by_name(n) for n in ["ams261", "phy131", "mat203", "che131"]}


def test_tokenize():
    assert tokenize("Green's Theorem!") == ["green", "s", "theorem"]


def test_finds_by_name():
    hits = search(sheets(), "stokes")
    assert hits and hits[0][2].id == "stokes"


def test_finds_across_sheets():
    hits = search(sheets(), "eigenvalues")
    ids = {f.id for _, _, f in hits}
    assert {"eigen-def", "char-poly", "diagonalization"} <= ids


def test_topic_weighted():
    hits = search(sheets(), "gas law")
    assert hits and hits[0][1] == "che131"


def test_empty_query_no_results():
    assert search(sheets(), "") == []
    assert search(sheets(), "zzzqqq") == []


def test_top_k_respected():
    hits = search(sheets(), "the", top_k=3)
    assert len(hits) <= 3
