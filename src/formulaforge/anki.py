"""Export a formula sheet as an Anki deck (.apkg).

Two card types per formula:
1. "State the formula": name (+ description) on the front, the formula on the back.
2. "Name this formula": the formula on the front, the name on the back.

Math uses \\( \\) / \\[ \\] delimiters, which Anki renders with its built-in
MathJax \u2014 no LaTeX installation needed.
"""

from __future__ import annotations

import hashlib
import re

import genanki


def _to_mathjax(latex: str) -> str:
    """Wrap display math in \\[ \\] for Anki's MathJax."""
    return "\\[" + latex.strip() + "\\]"


def _stable_id(*parts: str) -> int:
    h = hashlib.sha256("|".join(parts).encode()).hexdigest()
    return int(h[:16], 16) % (2**63 - 1)


MODEL_NAME = "formulaforge basic"


def build_model() -> genanki.Model:
    return genanki.Model(
        _stable_id(MODEL_NAME),
        MODEL_NAME,
        fields=[{"name": "Front"}, {"name": "Back"}, {"name": "Topic"}],
        templates=[
            {
                "name": "State the formula",
                "qfmt": "{{Front}}<br><span style='color:#888'>{{Topic}}</span>",
                "afmt": "{{FrontSide}}<hr id='answer'>{{Back}}",
            },
            {
                "name": "Name this formula",
                "qfmt": "{{Back}}<br><span style='color:#888'>{{Topic}}</span>",
                "afmt": "{{FrontSide}}<hr id='answer'>{{Front}}",
            },
        ],
        css=".card { font-family: Georgia, serif; font-size: 20px; text-align: center; }",
    )


def export_deck(sheet, path: str) -> dict:
    """Write an .apkg deck. Returns {'cards', 'notes'}."""
    model = build_model()
    deck = genanki.Deck(
        _stable_id("formulaforge", sheet.course, sheet.title),
        f"formulaforge: {sheet.course} {sheet.title}",
    )
    name_of = {f.id: f.name for f in sheet.formulas}
    notes = 0
    for f in sheet.formulas:
        prereq = ""
        if f.derives_from:
            prereq = "<br><span style='color:#4a6fa5;font-size:14px'>Builds on: " + ", ".join(
                name_of[d] for d in f.derives_from
            ) + "</span>"
        front = f"<b>{f.name}</b><br>{f.description}{prereq}"
        back = _to_mathjax(f.latex)
        note = genanki.Note(
            model=model,
            fields=[front, back, f.topic],
            guid=_stable_id("note", sheet.course, f.id).to_bytes(8, "big").hex(),
        )
        deck.add_note(note)
        notes += 1
    genanki.Package(deck).write_to_file(path)
    return {"cards": notes * 2, "notes": notes}
