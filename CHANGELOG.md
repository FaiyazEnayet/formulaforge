# Changelog

## [Unreleased]
### Added
- Canonical showcase chart export (FF-00). `formulaforge export-showcase`
  regenerates scripts/showcase-data.json from the graph engine with
  deterministic ordering, a generator version, the source sheet hash, and
  the curve hash recorded in a provenance block. A drift check
  (`CheckShowcase`, covered by `TestStage3Formulaforge00`) fails on a
  stale file, so hand-maintained chart numbers cannot drift from the
  engine again. The demo GIF script reads the same file for design and
  sheet nodes and is unaffected.
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
