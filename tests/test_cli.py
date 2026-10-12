"""End-to-end CLI tests."""

import json
import os
import subprocess
import sys

import pytest

BIN = [sys.executable, "-m", "formulaforge.cli"]
ENV = dict(os.environ, PYTHONPATH="src")


def run(*args):
    r = subprocess.run(BIN + list(args), capture_output=True, text=True,
                       env=ENV, cwd=os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    return r


def test_sheets_lists_four():
    r = run("sheets")
    assert r.returncode == 0
    for name in ["ams261", "phy131", "mat203", "che131"]:
        assert name in r.stdout


def test_build_html(tmp_path):
    out = str(tmp_path / "s.html")
    r = run("build", "mat203", "-o", out)
    assert r.returncode == 0 and os.path.exists(out)
    assert "katex" in open(out).read()


def test_graph_stats_and_json(tmp_path):
    r = run("graph", "phy131")
    assert r.returncode == 0
    assert "minimum memorize set" in r.stdout
    out = str(tmp_path / "g.json")
    r = run("graph", "phy131", "--json", "-o", out)
    assert r.returncode == 0
    payload = json.load(open(out))
    assert payload["stats"]["nodes"] == 27
    assert len(payload["edges"]) == payload["stats"]["edges"]
    assert payload["top_leverage"][0]["id"] == "newton2"


def test_study_order(tmp_path):
    r = run("study-order", "ams261")
    assert r.returncode == 0
    assert "Wave 1:" in r.stdout
    # partial-def (root) appears before gradient (dependent)
    assert r.stdout.index("Partial derivative") < r.stdout.index("Gradient vector")


def test_export_pdf_and_anki(tmp_path):
    pdf = str(tmp_path / "s.pdf")
    r = run("export", "che131", "--format", "pdf", "-o", pdf)
    assert r.returncode == 0 and os.path.getsize(pdf) > 5000
    apkg = str(tmp_path / "s.apkg")
    r = run("export", "ams261", "--format", "anki", "-o", apkg)
    assert r.returncode == 0 and os.path.getsize(apkg) > 1000


def test_search_cli():
    r = run("search", "eigenvalue")
    assert r.returncode == 0 and "mat203" in r.stdout
    r = run("search", "zzzqqq")
    assert "no matches" in r.stdout
