"""Tests for HTML and PDF rendering."""

import os

import html as htmlmod

import pytest

from formulaforge.models import load_sheet_by_name
from formulaforge.render import render_html, render_pdf, sanitize_for_pdf


@pytest.fixture(params=["ams261", "phy131", "mat203", "che131"])
def sheet(request):
    return load_sheet_by_name(request.param)


def test_html_has_katex_and_formulas(sheet):
    out = render_html(sheet)
    assert "katex.min.css" in out
    assert "renderMathInElement" in out
    for f in sheet.formulas:
        assert htmlmod.escape(f.name) in out
        assert f.latex in out
    assert "@media print" in out


def test_html_shows_prereq_chips(sheet):
    out = render_html(sheet)
    assert "Builds on:" in out or "Foundation formula" in out


def test_sanitize_pmatrix():
    s = sanitize_for_pdf(r"\det \begin{pmatrix} a & b \\ c & d \end{pmatrix} = ad-bc")
    assert "pmatrix" not in s
    assert "begin" not in s


def test_sanitize_removes_unsupported():
    s = sanitize_for_pdf(r"\mathbb{R}^n, \boldsymbol{\tau}, \tfrac12, \lVert v \rVert")
    assert "mathbb" not in s and "boldsymbol" not in s and "tfrac" not in s


def test_pdf_renders_every_formula(sheet, tmp_path):
    from formulaforge.graph import FormulaGraph

    path = str(tmp_path / "sheet.pdf")
    info = render_pdf(sheet, path, FormulaGraph(sheet))
    assert os.path.exists(path) and os.path.getsize(path) > 5000
    assert info["pages"] >= 2
    assert info["fallbacks"] == [], f"mathtext fallbacks: {info['fallbacks']}"
