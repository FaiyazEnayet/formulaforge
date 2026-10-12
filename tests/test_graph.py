"""Tests for the dependency-graph algorithms."""

import pytest

from formulaforge.graph import FormulaGraph
from formulaforge.models import Formula, Sheet, list_sheets, load_sheet_by_name


def tiny_sheet():
    # a -> b -> c, a -> c, d (isolated)
    fs = [
        Formula("a", "A", "T", "a", derives_from=[]),
        Formula("b", "B", "T", "b", derives_from=["a"]),
        Formula("c", "C", "T", "c", derives_from=["a", "b"]),
        Formula("d", "D", "T", "d", derives_from=[]),
    ]
    return Sheet("X", "Tiny", "", fs)


def test_topo_order_respects_edges():
    g = FormulaGraph(tiny_sheet())
    order = g.topo_order()
    pos = {n: i for i, n in enumerate(order)}
    assert pos["a"] < pos["b"] < pos["c"]
    assert set(order) == {"a", "b", "c", "d"}


def test_cycle_detection():
    fs = [
        Formula("a", "A", "T", "a", derives_from=["c"]),
        Formula("b", "B", "T", "b", derives_from=["a"]),
        Formula("c", "C", "T", "c", derives_from=["b"]),
    ]
    g = FormulaGraph(Sheet("X", "Cyc", "", fs))
    cycle = g.find_cycle()
    assert cycle is not None and set(cycle) >= {"a", "b", "c"}
    with pytest.raises(ValueError):
        g.topo_order()
    with pytest.raises(ValueError):
        g.assert_acyclic()


def test_no_cycle_in_tiny():
    assert FormulaGraph(tiny_sheet()).find_cycle() is None


def test_levels_and_waves():
    g = FormulaGraph(tiny_sheet())
    assert g.levels() == {"a": 0, "d": 0, "b": 1, "c": 2}
    waves = g.waves()
    assert [set(w) for w in waves] == [{"a", "d"}, {"b"}, {"c"}]


def test_descendants_ancestors():
    g = FormulaGraph(tiny_sheet())
    assert g.descendants("a") == {"b", "c"}
    assert g.descendants("d") == set()
    assert g.ancestors("c") == {"a", "b"}
    assert g.leverage("a") == 2
    assert g.leverage("d") == 0


def test_leverage_ranking_order():
    g = FormulaGraph(tiny_sheet())
    ranking = g.leverage_ranking()
    assert ranking[0] == ("a", 2)
    assert ranking[-1] == ("d", 0)


def test_min_memorize_set_covers_all():
    g = FormulaGraph(tiny_sheet())
    chosen = g.min_memorize_set()
    covered = set()
    for n in chosen:
        covered |= g.coverage_of(n)
    assert covered == set(g.nodes)
    # optimal here is {a, d}
    assert set(chosen) == {"a", "d"}


def test_coverage_curve_monotone_and_complete():
    g = FormulaGraph(tiny_sheet())
    curve = g.coverage_curve()
    assert curve[0] == (0, 0.0)
    assert curve[-1][1] == pytest.approx(1.0)
    fracs = [f for _, f in curve]
    assert all(b >= a for a, b in zip(fracs, fracs[1:]))


@pytest.mark.parametrize("name", list_sheets())
def test_seeded_sheets_acyclic(name):
    s = load_sheet_by_name(name)
    g = FormulaGraph(s)
    g.assert_acyclic()
    # every edge respected by topo order
    pos = {n: i for i, n in enumerate(g.topo_order())}
    for n in g.nodes:
        for p in g.prereqs[n]:
            assert pos[p] < pos[n], f"{p} must precede {n} in {name}"
    # waves partition all nodes
    assert sum(len(w) for w in g.waves()) == len(s.formulas)
    # min set covers everything
    covered = set()
    for n in g.min_memorize_set():
        covered |= g.coverage_of(n)
    assert covered == set(g.nodes)
