<p align="center"><img src="docs/readme-banner.svg?v=type-20261017b" alt="FormulaForge: Mathematical dependencies, made inspectable" width="1200"></p>

[![Tests](https://github.com/FaiyazEnayet/formulaforge/actions/workflows/ci.yml/badge.svg)](https://github.com/FaiyazEnayet/formulaforge/actions/workflows/ci.yml) [![License: MIT](https://img.shields.io/badge/license-MIT-007EC6)](LICENSE) ![Go 1.24+](https://img.shields.io/badge/Go-1.24%2B-00ADD8) ![Built with Go + graph algorithms](https://img.shields.io/badge/built%20with-Go%20%2B%20graph%20algorithms-555555) [![Interactive demo](https://img.shields.io/badge/demo-interactive-4C9D39)](https://faiyazenayet.github.io/formulaforge/docs/dashboard.html)

<p align="center"><a href="https://faiyazenayet.github.io/formulaforge/docs/dashboard.html"><b>Open interactive demo</b></a> · <a href="#method">Method</a> · <a href="#run-locally">Run locally</a></p>

# FormulaForge

FormulaForge turns a formula sheet into a searchable dependency model. It shows which prerequisites support a result, what is already derivable and which concepts are useful to learn next.


[![FormulaForge demo](docs/demo.gif?v=showcase-20261017b)](https://faiyazenayet.github.io/formulaforge/docs/dashboard.html)


A dependency view is useful for planning revision only when its relationships are interpreted correctly. Selecting a formula reveals the concepts that support it, while search and topic filters keep a large sheet navigable. Structural reach, valid derivability and an optimal memorization strategy answer different questions and should be assessed separately.

## Explore

- Search and filter eight dashboard sheets, then select a formula to inspect its prerequisites.
- Explore structural leverage separately from valid derivability.
- Use the coverage view as a greedy study heuristic, not an exact optimal learning plan.

## Method

Sheets declare formulas and derivation paths in YAML. A path requires all its prerequisites; alternative paths are OR choices. The graph supports structural leverage, derivation closure, study order and target-set search.

## Evidence at a glance

| Scope | Recorded result or contract |
| :--- | :--- |
| Dashboard scope | Eight sheets; CSE 214 excluded from the redesigned dashboard |
| Derivation semantics | AND within each path; OR across alternative paths |
| Coverage curve | Greedy heuristic |
| Target-set solver | Exact branch and bound; exponential worst-case complexity |

## Updating this project

Dashboard source language: **Go**. Generator: `internal/dashboard/dashboard.go via the Go CLI`. To keep the interactive page, figures, data animation, and README consistent, follow [the coding-agent update guide](AGENT-DASHBOARD-GUIDE.md). The local interactive output is `docs/dashboard.html`.

## Run locally


```sh
go build -o formulaforge ./cmd/formulaforge
go test ./...
./formulaforge graph phy131
./formulaforge dashboard -o docs/dashboard.html
```

## Technical design

A formula can declare one or more derivation paths. Every prerequisite in a path is required; satisfying any alternative path is sufficient. The graph also stores the union of prerequisites for structural topology and leverage. This distinction prevents a reachable descendant from being mislabeled as immediately derivable.

```mermaid
flowchart LR
  N0["YAML and LaTeX sheets"] --> N1["Validated derivation paths"]
  N1["Validated derivation paths"] --> N2["Structural DAG"]
  N1["Validated derivation paths"] --> N3["Derivation closure"]
  N2["Structural DAG"] --> N4["Leverage and study order"]
  N3["Derivation closure"] --> N5["Target search and coverage"]
```

## Evaluation protocol

Full-sheet and target-set questions are different. A full-sheet memorize set is based on roots outside prior knowledge. The target-set solver uses exact branch and bound. The dashboard coverage curve uses a greedy sequence and should be described as a heuristic, even though its closure calculation respects derivation semantics.

![The eight dashboard source sheets](docs/results.png)

The dashboard is an exploration tool for authored mathematical relationships. It does not infer new theorems or certify the completeness of a course syllabus. CSE 214 is excluded from the redesigned dashboard; the eight remaining source sheets retain their actual formulas and dependencies.

## Code and data

`internal/models/` loads embedded sheets. `internal/graph/` owns dependency semantics and graph algorithms. `internal/search/` handles search. `internal/dashboard/` generates the HTML output.


## Limitations

- Structural descendants are not automatically derivable from one memorized node.
- Dense target searches need a time or node budget for predictable response times.
- The PDF export is a reading copy; use the LaTeX export for typeset mathematics.

## License

[MIT](LICENSE). Third-party datasets, libraries and fonts retain their own terms.
