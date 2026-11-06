# Changelog

## [Unreleased]
### Added
- Derivation witnesses for dependency closure (FO3). `Graph.Witnesses`
  returns one satisfied prerequisite set per derived formula, so each
  result in a closure carries the derivation that justified it instead
  of only the reachable set. The separate `Graph.CheckWitnesses`
  checker validates a witness DAG: it rejects a witness that is not
  exactly one whole derivation path (so a missing AND prerequisite or
  an extra member fails), rejects a witness for an already known or
  underivable formula, rejects a missing witness for any derived
  formula, and rejects a cyclic witness DAG. Regression test
  `TestStage3Formulaforge3` added.
- Catalog audit contract and oracle (FF-01 part 1). Formulas gain
  optional provenance fields (source, conditions, notation, units),
  documented in `docs/catalog-audit-contract.md`, and a new
  `internal/validate` package checks them together with structural
  LaTeX validity: balanced braces, paired environments, no doubled
  backslashes before commands, and a fixed command vocabulary. The
  audit pins the catalog as found in `internal/validate/baseline.json`
  (265 formulas, 284 edges): all 32 cse113 formulas carry the
  doubled-backslash defect, and every formula lacks provenance.
  An independent oracle (`internal/validate/testdata/oracle.yaml`)
  recomputes known answers for representative quantitative formulas
  from first principles against cited sources. Baseline drift fails
  the test suite until deliberately re-pinned. Content fixes and
  provenance population follow in part 2.
- Catalog audit content fix (FF-01 part 2). The cse113 sheet's
  doubled-backslash escaping is corrected, so all 32 of its formulas
  parse with single backslashes and pass LaTeX validation. Every one
  of the 265 formulas now carries hand-written provenance under the
  audit contract: a resolvable source with a section for each entry
  (OpenStax Calculus and University Physics for the calculus and
  physics sheets, Epp for cse113, Rosen for cse213, Lay for linear
  algebra, OpenStax Introductory Statistics for the statistics
  sheet), and stated conditions on the quantitative sheets,
  including the projectile entry's equal-heights and
  no-air-resistance assumptions behind its 45-degree optimum. The
  audit baseline is re-pinned at the clean end state (265 formulas,
  284 edges, zero defects, no provenance gaps), and new part-2 tests
  fail the suite if any defect or gap returns.
- Canonical showcase chart export (FF-00). `formulaforge export-showcase`
  regenerates scripts/showcase-data.json from the graph engine with
  deterministic ordering, a generator version, the source sheet hash, and
  the curve hash recorded in a provenance block. A drift check
  (`CheckShowcase`, covered by `TestStage3Formulaforge00`) fails on a
  stale file, so hand-maintained chart numbers cannot drift from the
  engine again. The demo GIF script reads the same file for design and
  sheet nodes and is unaffected.
- Published algorithm and schema contract (FO7). `docs/algorithm-contract.md`
  documents the sheet schema and its stable validation errors, the
  derivation semantics, and every graph algorithm with its input
  assumptions, worst-case complexity, invariants and tie-breaking
  rules, including the precise limits: union ancestors are not
  derivability, the exact memorize search has no internal step cap,
  greedy budget coverage is a heuristic (with a worked counterexample
  where greedy reaches 6 of 11 formulas at budget 2 against an exact
  optimum of 7), a cycle through a non-chosen alternative path is
  rejected by the ordering algorithms even though closure still
  derives the sheet, and topological order is readiness order rather
  than the lexicographically smallest order. The CLI examples in the
  contract are executable fixtures, and `go.mod` is tidied so direct
  dependencies are no longer marked indirect (no version changes;
  `go mod verify` passes). Covered by `TestStage3Formulaforge7`.
### Docs
- Centered the README header: uniform dark badge labels and three action buttons in the dashboard palette (Open Dashboard, Explore Method, Run Locally) with a black label side so they stand out on the page.

### Removed
- CSE 214 (Data Structures) formula sheet (29 formulas). The graph now
  totals 265 formulas and 284 derivation edges across the eight
  remaining sheets.
### Fixed
- PHY 131 showcase curve (FF-00). The committed curve overstated early
  coverage against the canonical graph closure: K=1 claimed 36 of 51
  formulas derivable where the engine gives 20, with smaller overstatements
  through K=6 and exact values from K=7 on. The curve is now generated
  from `CoverageCurve` by the new exporter, and the minimum-memorize set
  is listed in the engine canonical order (same 9 formulas, sorted).
  docs/dashboard.html still embeds the previous curve: it was audited by
  extraction in `TestStage3Formulaforge00HTMLAudit` and needs a dashboard
  regeneration to pick up the corrected numbers.
- Empty-sheet and invalid-query behavior (FO2). `models.Load` now rejects
  sheets with no formulas instead of silently producing an empty sheet.
  `CoverageCurve` no longer divides by the node count: a zero-node graph
  returns a single finite point (0, 1.0), so JSON export can never contain
  NaN from this path. `MinMemorizeFor` rejects unknown prior ids, and the
  derivation semantics are now an explicit documented contract (only ids
  mapped to true count as known in `Closure`). Regression test
  `TestStage3Formulaforge2` added.

## [1.2.0]
### Fixed
- Branch-and-bound for target memorize sets now picks residuals in topological
  order (prerequisites before dependents) instead of lexicographically. The old
  order could re-pick a target instead of its newly added prerequisites,
  missing the optimum when a shared intermediate (e.g. z for targets a,b,c)
  covers multiple targets. The counterexample now returns cardinality 1.


## [0.1.1] - 2026-10-15
### Fixed
- Minimum-memorize set now uses conjunctive derivation semantics. The old
  code treated coverage as pure graph reachability, so memorizing one
  prerequisite counted a formula as covered even when deriving it needs
  several. A formula is now known only when it is memorized, already known,
  or every prerequisite of one of its derivation paths is known.
### Added
- Alternative derivation paths: formulas can declare `alt_derives_from`,
  additional prerequisite sets that each suffice on their own.
- `graph --target`: exact smallest memorize set covering a list of target
  formulas (exam topics), solved with branch and bound.
- Prior knowledge: `MinMemorizeSet` accepts already-known formulas and
  leaves them out of the set.
### Changed
- `MinMemorizeSet` is now exact (the unknown roots) instead of greedy set
  cover. Reported set sizes are unchanged on the nine shipped sheets.
- The dashboard coverage curve now measures marginal gain under conjunctive
  closure; it remains a greedy heuristic.
