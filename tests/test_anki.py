"""Tests for Anki deck export."""

import os
import zipfile

from formulaforge.anki import _to_mathjax, export_deck
from formulaforge.models import load_sheet_by_name


def test_mathjax_wrapping():
    assert _to_mathjax("x^2").startswith("\\[")


def test_apkg_is_valid(tmp_path):
    sheet = load_sheet_by_name("phy131")
    path = str(tmp_path / "phy131.apkg")
    info = export_deck(sheet, path)
    assert os.path.exists(path)
    assert info["notes"] == len(sheet.formulas)
    assert info["cards"] == 2 * len(sheet.formulas)
    with zipfile.ZipFile(path) as z:
        assert "collection.anki2" in z.namelist()
