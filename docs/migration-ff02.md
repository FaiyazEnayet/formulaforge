# Migration report: typed relationships (FF-02)

When FormulaForge started typing relationships, every derivation
edge in the shipped sheets was a plain `derives_from` or
`alt_derives_from` entry with no rule attached. FF-02 keeps that
meaning: a derivation path with no rule is certified by default, so
an untyped sheet behaves exactly as it did before. This report
records what the typing changed across the shipped catalog, computed
by `graph.MigrationReportFor`, which compares the graph each sheet
produced before typing (every path treated as certified) with the
graph it produces now (only certified paths), and enumerates every
changed root, edge, and coverage-curve point.

## Result: no changes

No shipped formula uses `conceptual_prerequisites` or
`derivation_rules`, so no root, edge, or coverage-curve point
changed on any sheet. The full inventory:

| Sheet | Formulas | Certified edges | Conceptual edges | Unreviewed paths | Changes |
| :--- | ---: | ---: | ---: | ---: | ---: |
| ams151 | 25 | 29 | 0 | 0 | 0 |
| ams161 | 26 | 21 | 0 | 0 | 0 |
| ams210 | 30 | 37 | 0 | 0 | 0 |
| ams261 | 45 | 53 | 0 | 0 | 0 |
| cse113 | 32 | 30 | 0 | 0 | 0 |
| cse213 | 34 | 22 | 0 | 0 | 0 |
| phy131 | 51 | 66 | 0 | 0 | 0 |
| psy103 | 22 | 26 | 0 | 0 | 0 |
| **Total** | **265** | **284** | **0** | **0** | **0** |

The totals match the pinned catalog baseline (265 formulas, 284
union edges), and the dashboard showcase file for PHY 131 was
regenerated from the engine with the same curve.

## What a change looks like

The report is not a formality: on a sheet that does use the new
fields it lists each effect. The FF-02 regression fixture
(`TestStage3Formulaforge02`) carries one: a formula whose only
derivation is unreviewed becomes a new root, its two derivation
edges are removed, and the coverage curve shifts (K=1 moves from
0.4444 to 0.3333, and the curve gains a point, length 6 to 7,
because the newly rooted formula must be memorized). The test pins
that enumeration, and the shipped-catalog check above
(`TestStage3Formulaforge02/ShippedCatalogUnchanged`) pins the zero
change result, so any future sheet edit that introduces a
conceptual link or an unreviewed rule will show up here as a listed
change rather than a silent one.

## Scope note

This migration changes the engine, not the sheet content. No YAML
file was edited: in particular the known ams261 line-scalar and
line-vector entries were left exactly as they are, pending a
separate owner decision on that content.
