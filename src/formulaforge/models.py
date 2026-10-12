"""Data model: Formula and Sheet, loaded from YAML sheet files."""

from __future__ import annotations

import os
from dataclasses import dataclass, field

import yaml


@dataclass
class Formula:
    id: str
    name: str
    topic: str
    latex: str
    description: str = ""
    tags: list = field(default_factory=list)
    derives_from: list = field(default_factory=list)

    def text_blob(self) -> str:
        """All searchable text for this formula, lowercased."""
        parts = [self.name, self.topic, self.latex, self.description]
        parts.extend(self.tags)
        return " ".join(parts).lower()


@dataclass
class Sheet:
    course: str
    title: str
    description: str
    formulas: list  # list[Formula]

    def by_id(self, fid: str) -> Formula:
        for f in self.formulas:
            if f.id == fid:
                return f
        raise KeyError(f"unknown formula id: {fid}")

    def ids(self) -> list:
        return [f.id for f in self.formulas]


def load_sheet(path: str) -> Sheet:
    """Load and validate a sheet YAML file."""
    with open(path, encoding="utf-8") as fh:
        data = yaml.safe_load(fh)
    formulas = []
    seen = set()
    for raw in data.get("formulas", []):
        fid = raw["id"]
        if fid in seen:
            raise ValueError(f"duplicate formula id: {fid}")
        seen.add(fid)
        formulas.append(
            Formula(
                id=fid,
                name=raw["name"],
                topic=raw.get("topic", "General"),
                latex=raw["latex"],
                description=raw.get("description", ""),
                tags=list(raw.get("tags", [])),
                derives_from=list(raw.get("derives_from", [])),
            )
        )
    # validate references
    for f in formulas:
        if f.id in f.derives_from:
            raise ValueError(f"formula {f.id} derives from itself")
        for dep in f.derives_from:
            if dep not in seen:
                raise ValueError(f"formula {f.id} references unknown id: {dep}")
    return Sheet(
        course=data.get("course", ""),
        title=data.get("title", ""),
        description=data.get("description", ""),
        formulas=formulas,
    )


def sheets_dir() -> str:
    return os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "sheets")


def list_sheets() -> list:
    """Return sorted sheet names (without extension) in the sheets dir."""
    d = sheets_dir()
    names = []
    for fn in sorted(os.listdir(d)):
        if fn.endswith((".yaml", ".yml")):
            names.append(os.path.splitext(fn)[0])
    return names


def load_sheet_by_name(name: str) -> Sheet:
    d = sheets_dir()
    for ext in (".yaml", ".yml"):
        path = os.path.join(d, name + ext)
        if os.path.exists(path):
            return load_sheet(path)
    raise FileNotFoundError(f"no sheet named {name!r}")
