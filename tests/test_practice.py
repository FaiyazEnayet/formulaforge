"""Tests for practice mode."""

from formulaforge.graph import FormulaGraph
from formulaforge.models import load_sheet_by_name
from formulaforge.practice import (
    check_derive,
    check_order,
    gen_derive_questions,
    gen_order_questions,
    gen_state_questions,
    run_session,
)


def graph():
    return FormulaGraph(load_sheet_by_name("ams261"))


def test_derive_questions_have_prereqs():
    g = graph()
    qs = gen_derive_questions(g, 5, seed=1)
    assert qs and all(g.prereqs[q["formula"]] for q in qs)


def test_derive_scoring():
    g = graph()
    r = check_derive(g, "gradient", ["partial-def"])
    assert r["f1"] == 1.0
    r = check_derive(g, "gradient", ["wrong-id"])
    assert r["f1"] == 0.0
    r = check_derive(g, "gradient", ["partial-def", "wrong-id"])
    assert 0.0 < r["f1"] < 1.0


def test_order_questions_answer_is_ancestor():
    g = graph()
    qs = gen_order_questions(g, 5, seed=2)
    assert qs
    for q in qs:
        assert q["answer"] in g.ancestors(q["first"]) | g.ancestors(q["second"]) | {q["first"], q["second"]}
        assert check_order(q, q["answer"])
        other = q["second"] if q["answer"] == q["first"] else q["first"]
        assert not check_order(q, other)


def test_state_questions():
    g = graph()
    qs = gen_state_questions(g, 3, seed=3)
    assert len(qs) == 3


def test_run_session_all_correct():
    g = graph()
    qs = gen_derive_questions(g, 4, seed=4) + gen_order_questions(g, 4, seed=4)

    def answer(q):
        if q["type"] == "derive":
            return list(g.prereqs[q["formula"]])
        return q["answer"]

    summary = run_session(g, qs, answer)
    assert summary["correct"] == summary["total"] == 8


def test_run_session_all_wrong():
    g = graph()
    qs = gen_derive_questions(g, 3, seed=5)
    summary = run_session(g, qs, lambda q: ["nope"])
    assert summary["correct"] == 0
