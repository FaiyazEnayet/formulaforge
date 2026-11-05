# Catalog audit contract

`models.Load` checks that a sheet is structurally sound: unique ids,
resolvable derivation links, no self-reference. It says nothing about
whether the math is right, where an equation came from, what it
assumes, or whether its LaTeX is valid. This contract is the content
gate on top of that structural one. It is implemented by
`internal/validate` and enforced by the `TestStage3Formulaforge01`
tests, which run in CI with the rest of the Go suite.

## Provenance fields

Every formula carries four optional schema fields (`source`,
`conditions`, `notation`, `units` in the sheet YAML). Under this
contract a curated formula is complete only when:

- **source** names where the result comes from in a form a reviewer
  can resolve: a book or reference with a section (for example
  "OpenStax University Physics Volume 1, Section 4.3"), or a URL.
  A bare book title, an empty value, or a placeholder ("TBD", "n/a")
  does not satisfy the contract.
- **conditions** states the assumptions the result needs. Required
  for every formula on a quantitative sheet (ams151, ams161, ams210,
  ams261, phy131, psy103). The canonical failure this prevents: the
  projectile entry states that range is maximized at 45 degrees
  without saying that launch and landing heights are equal and air
  resistance is ignored. On the logic sheets (cse113, cse213), whose
  entries are definitions and identities, a source is required but
  conditions are not.
- **notation** keys the symbols when they are not self-evident.
- **units** gives the unit signature where relevant, as plain unit
  symbols (for example `m/s^2`). Prose in this field is rejected.

Provenance is written by hand for each formula. Automatically
generated filler is a contract violation, not a shortcut.

## LaTeX validation

`validate.CheckLatex` is a grammar smoke check, not a typesetter. It
rejects:

- unbalanced or unmatched braces;
- `\begin`/`\end` pairs that are missing, mismatched, or unclosed;
- alignment tabs (`&`) and line breaks (`\\`) outside an environment;
- doubled backslashes before a command (`\\frac`), the defect class
  that single-quoted YAML produces when content was escaped as if it
  were double-quoted;
- commands outside the catalog vocabulary (97 commands at the part-1
  baseline, derived from actual catalog usage). A new command is
  added to the vocabulary deliberately, in the same change that
  introduces the notation.

On YAML quoting: a single-quoted scalar keeps backslashes literal,
so single-quoted LaTeX takes single backslashes (`'\frac{a}{b}'`).
A double-quoted scalar processes escapes, so the file must double
each backslash (`"\\frac{a}{b}"`) to produce the same value. Mixing
the two conventions is how the cse113 defect happened.

## The oracle

`internal/validate/testdata/oracle.yaml` pins representative
quantitative formulas to known answers computed from first
principles, each with the authoritative source a reviewer can check
and the conditions the answer needs. The oracle test recomputes every
answer from the recorded inputs in plain Go; nothing is derived from
the catalog's own rendering, which is what makes it independent. New
spot-checks are added as cases with new inputs, never by editing a
recorded answer to match the code.

## Baseline and drift

`internal/validate/baseline.json` pins the audit state of the whole
catalog: the SHA-256 of the sheet sources, headline counts (265
formulas, 284 edges at part 1), a per-sheet inventory with finding
counts by class, and the exact defect list. The baseline test
re-audits the catalog and fails on any drift: a new defect, a fixed
defect, or a provenance change all require a deliberate re-pin
(`FF01_UPDATE_BASELINE=1 go test ./internal/validate -run
TestStage3Formulaforge01Part1`), reviewed like any other change.

Finding classes: `doubled-backslash` and `invalid-latex` are defects
(content that is wrong); `missing-provenance`, `invalid-provenance`
and `missing-conditions` are gaps (content that is incomplete).

## Typeset smoke

The Go tests check that the generated `.tex` for every sheet is a
complete document containing every formula verbatim.
`scripts/texsmoke.sh` goes further where a TeX toolchain is
installed: it exports every sheet and compiles each with pdflatex.
Without pdflatex it reports the skip and exits successfully; the
structural checks in the test suite are the always-on gate.

## Rollout

Part 1 (this contract) pinned the baseline as found: all 32 cse113
formulas carried the doubled-backslash defect (12 of them also
produced invalid-LaTeX findings as a consequence), and all 265
formulas lacked provenance (199 of them also lacked required
conditions). Part 2 fixed the content sheet by sheet and re-pinned
the baseline at the end state: the cse113 escaping is corrected, and
every formula now carries a hand-written source plus the conditions
its result needs on the quantitative sheets. The calculus and
physics entries cite OpenStax (Calculus Volumes 1-3, University
Physics Volumes 1-2), cse113 cites Epp's Discrete Mathematics with
Applications (5th edition), cse213 cites Rosen's Discrete
Mathematics and Its Applications (8th edition), ams210 cites Lay's
Linear Algebra and Its Applications, and psy103 cites OpenStax
Introductory Statistics. The part-2 tests hold the line: any new
defect or provenance gap fails the suite.
