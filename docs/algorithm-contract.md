# Algorithm and schema contract

This document is the public contract for the FormulaForge dependency
graph engine (`internal/graph`, built on the sheet schema in
`internal/models`). It states what each algorithm assumes, what it
guarantees, how it breaks ties, what it costs, and where its limits
are. The behaviors stated here are pinned by executable fixtures in
`TestStage3Formulaforge7` (`internal/graph/stage3_fo7_test.go`): if a
behavior described below changes, that test is expected to fail until
the contract is updated with it.

Notation: `V` is the number of formulas on a sheet, `E` is the number
of union prerequisite edges (see below), and `P` is the total number
of members across all derivation paths of all formulas.

## Sheet schema contract

A sheet is a YAML document:

| Field | Type | Meaning |
| --- | --- | --- |
| `course` | string | Course code, for example `PSY 103`. |
| `title` | string | Sheet title. |
| `description` | string | Free-text summary. |
| `formulas` | list | The formulas, in sheet order. |

Each formula has: `id` (unique, non-empty), `name`, `topic` (empty
means `General`), `latex`, `description`, `tags` (trimmed on load),
`derives_from` (the primary derivation path), and `alt_derives_from`
(a list of alternative derivation paths).

Validation is fail-fast and total: a sheet either loads completely or
returns one stable error, never a partial sheet. The error strings are
part of the contract:

- malformed YAML: `parse sheet: ...`
- no formulas: `sheet has no formulas`
- a formula without an id: `formula <n> has no id`
- a repeated id: `duplicate formula id: <id>`
- a formula listing itself as a prerequisite: `formula <id> derives from itself`
- a prerequisite that is not on the sheet: `formula <id> references unknown id: <dep>`
- an empty alternative path: `formula <id> has an empty alternative derivation path`

Sheets are embedded in the binary and addressed by name (`sheets`,
`graph <sheet>`, and the other CLI commands below). An unknown sheet
name is an error: `no sheet named "<name>"`, exit status 1.

## Derivation semantics

Each formula carries a set of *derivation paths*. The primary path is
`derives_from`; every `alt_derives_from` entry is one more path. A
formula with no paths is a *root*.

- **Within a path, prerequisites are conjunctive (AND):** deriving a
  formula through a path requires every member of that path to be
  known.
- **Across paths, alternatives are disjunctive (OR):** any one fully
  known path suffices.
- A formula is *known* when it is memorized, given as prior knowledge,
  or derivable under the two rules above. `Closure` computes the
  fixed point of this rule.

Two derived structures sit on top of the paths, and confusing them is
the most common misreading of this engine:

- **Union prerequisites** (`Prereqs`, and `Children`, its inverse):
  the union of every member of every path. Topological order, cycle
  detection, levels, and leverage are computed on this union graph.
  Union membership is a structural fact ("this formula can appear
  upstream"), not a derivability claim.
- **Derivability** (`Closure`, `MinMemorizeSet`, `MinMemorizeFor`):
  computed on the paths themselves, under the AND/OR rule.

Concretely (fixture `fo7OrSheet` in the FO7 test): `t` derives from
`[a, b]` or from `[r]`. Its union ancestors are `{a, b, r}`, yet
knowing only `r` derives `t`, and knowing only `a` does not (the AND
path also needs `b`). `Ancestors(t)` reporting `a` and `b` does not
mean either is required.

Prior knowledge and known sets:

- `MinMemorizeFor` rejects unknown target ids (`unknown formula id:
  <id>`) and unknown prior ids (`unknown prior id: <id>`).
- `MinMemorizeSet` has no error return; unknown prior ids are
  silently ignored, because they can contribute nothing to a closure.
- `Closure` takes a `map[string]bool`. Only ids mapped to `true`
  count as known; ids mapped to `false` and absent ids are both not
  known, and unknown ids in the map are ignored.

## Exact targets vs heuristic budget coverage

Two different questions are easy to conflate:

- **Exact:** "What is the smallest set to memorize so that these
  targets (or the whole sheet) become derivable?" `MinMemorizeFor`
  and `MinMemorizeSet` answer this exactly; see the search limits
  below for the cost.
- **Heuristic:** "If I memorize formulas one at a time, how much of
  the sheet becomes derivable after each step?" `CoverageCurve`
  answers this greedily: at each step it memorizes the formula with
  the largest marginal closure gain, ties broken by id. Greedy is a
  heuristic for the budgeted-coverage problem, not an exact
  optimizer, and no approximation guarantee is claimed.

Counterexample (fixture `fo7GreedySheet`, 11 formulas): root `x` has
three direct dependents, so greedy memorizes `x` first (gain 4) and
reaches 6 of 11 formulas at budget 2. The exact budget-2 optimum is
the pair `{a, b}` (7 of 11), because together they unlock the joint
node `j` and its dependents. `CoverageCurve` is the right tool for a
study-progress display; it must not be quoted as optimal coverage.

## Algorithm reference

The graph value is immutable after `New`; no method mutates it, and
every output is deterministic for a fixed sheet (all iteration that
could observe Go map order is sorted or driven by sheet order).

### New (construction)

Builds `Paths`, `Prereqs`, `Children` from a loaded sheet. Each
node's paths are sorted canonically: by length, then
lexicographically. `Prereqs` and `Children` lists are sorted by id.
Cost: dominated by sorting, `O(P log P)` worst case. Assumption: the
sheet has already passed `models.Load` validation; `New` performs no
validation of its own.

### FindCycle / AssertAcyclic

Depth-first search over the union graph (`Children`), with nodes
visited in sheet order and children in sorted id order. Returns the
first cycle found as a list of ids that starts and ends with the
same node, or `nil` when the union graph is acyclic. Cost:
`O(V + E)`. `AssertAcyclic` wraps it, returning `dependency cycle:
<ids>` on failure. See "Cycles" below for exactly what a "cycle"
means here.

### TopoOrder

Kahn's algorithm over union prerequisites. Tie-breaking, precisely:
the initial ready queue (zero-prerequisite nodes) is sorted by id;
after that, nodes are emitted in the order they become ready
(FIFO), and each node's dependents are examined in sorted id order.
The result is deterministic but is *not* claimed to be the
lexicographically smallest topological order. Counterexample
(fixture `fo7FIFOSheet`): roots `a` and `b`, with `aa` deriving from
`a`, produce `a, b, aa`; the lexicographically smallest valid order
would be `a, aa, b`, but `aa` only becomes ready once `a` is
emitted, landing behind the already-queued `b`. Cost: `O(V + E)`
plus the initial `O(V log V)` sort. Returns `dependency cycle
detected` when the union graph is cyclic.

### Levels / Waves / Depth

`Levels` assigns each node the length of the longest union
prerequisite chain ending at it (roots are 0), in `O(V + E)` after
the topological order. `Waves` groups nodes by level; within a wave,
nodes appear in topological order. `Depth` is the maximum level.
All three propagate the `TopoOrder` error on cyclic input.

### Descendants / Ancestors / Leverage / rankings

`Descendants` and `Ancestors` are transitive closures over the
union graph (`Children` resp. `Prereqs`), each `O(V + E)`. They
answer "what is upstream/downstream of this formula", not "what is
needed to derive it" (see the union warning above). `Leverage(node)`
is the descendant count. `LeverageRanking` sorts by leverage
descending, ties by id ascending; `FoundationRanking` does the same
with ancestor counts. A ranking costs `O(V * (V + E))`.

### Closure

Repeated passes over all nodes until no pass derives anything new.
Each pass costs `O(P)`; there are at most `V` productive passes, so
the worst case is `O(V * P)`. The result is a superset of the
true-valued input ids, is monotone in the input, and is idempotent.
`Closure` does not reject cyclic path structures: the fixed point
is well-defined for any set of paths (see "Cycles").

### MinMemorizeSet (exact, whole sheet)

Every root that is not already known must be memorized, and the
roots together derive everything else, so the exact answer is: the
unknown roots, sorted by id. Cost: one `Closure` plus a scan,
`O(V * P)`. There is no search involved; the answer is a theorem
about the semantics, not an optimization result.

### MinMemorizeFor (exact, per target list)

Branch and bound over "memorize this node" vs "derive it through
one of its paths", seeded with `MinMemorizeSet` as an upper bound,
pruned against the best solution found, and memoized on
(residual need, known set) states. Residuals are expanded in
topological-rank order (prerequisites before dependents), where the
rank comes from a Kahn pass over the path graph whose ready queue is
re-sorted by id at every step. The returned set is sorted by id and
its size equals the exhaustive-search optimum; the FO7 fixtures
verify this against a brute-force oracle on every fixture case, and
verify the call is deterministic.

Search limits: the worst case is exponential in the number of
unresolved targets and their combined path choices, and there is
**no internal step, time, or node cap**: the search always runs to
exact completion. Each search node costs one `Closure`
(`O(V * P)`). This is a deliberate contract choice (exact answers,
honest cost) with a practical consequence: `MinMemorizeFor` is
intended for sheet-scale inputs (the eight shipped sheets have 22
to 51 formulas and answer instantly), and callers pointing it at
much larger or pathologically branching graphs own the wait. For
budgeted questions on large inputs, `MinMemorizeForBounded` is the
supported exact tool (below) and `CoverageCurve` is the supported
heuristic tool, with the caveat above.

### MinMemorizeForBounded (exact with budgets, per target list)

The same branch-and-bound search, with context cancellation and
node and time budgets. Closures inside the search run on memoized
bitsets instead of fresh maps, and residuals are picked in
deterministic topological order (lowest rank, ties by id), so a
fixed sheet and budget always give the same result. The result
carries a proof status: `optimal` when the search ran to
completion, `budget-exhausted` when a node or time budget stopped
it, and `canceled` when the context was canceled. On a stopped
search the returned set is the best feasible incumbent found so
far, and the lower and upper bounds always bracket the optimum:
the lower bound is the minimum over budget-cut subtrees of
memorizations made plus one per residual root (a root has no
derivation path and can only be memorized), clamped to the
incumbent. A stopped search is never reported exact, even when
the gap is zero. With no budget the result is exact and identical
to `MinMemorizeFor`. The `graph --target` command exposes the
budgets as `--search-nodes` and `--search-timeout`.

### CoverageCurve (heuristic)

At each step, evaluates every unmemorized formula's marginal
closure gain and memorizes the best (ties by smallest id). At most
`V` steps; each step runs up to `V` closures, so the worst case is
`O(V^3 * P)`: the most expensive routine in this package relative
to `V`, and another reason it is a display tool for sheet-scale
inputs. Invariants: `K` increases by 1 per point, fractions are
finite, lie in `[0, 1]`, and never decrease. Zero-node contract: an
empty graph returns the single point `(0, 1.0)` (vacuously covered)
instead of dividing by the node count.

### Stats

Bundles node/edge counts (edges = union prerequisite count), depth,
wave count, root/leaf counts, the top-5 leverage ranking, and
`MinMemorizeSet`. Rejects cyclic union graphs via `AssertAcyclic`.

## Witness interpretation

A *witness* for a target is the evidence that a memorize set covers
it. Read a result of `MinMemorizeFor` (memorize set `M`, prior `Pr`)
as follows: a formula `n` is witnessed when

- `n` is in `M` or `Pr` (a leaf witness), or
- one of `n`'s derivation paths has every member witnessed
  (recursively).

The witness is therefore a tree whose internal nodes pick one path
each and whose leaves are exactly the memorized or prior formulas
relied on. A claimed witness that bottoms out anywhere else is
invalid: in particular, witnessing a node through a path requires
witnesses for *all* members of that path (the AND rule), and one
fully witnessed path is enough (the OR rule). The FO7 fixtures check
this interpretation with an independent recursive checker, including
the negative cases: knowing only `a` does not witness `t` on
`fo7OrSheet`, and the empty set witnesses nothing it does not
contain. `Closure(M + Pr)` containing every target is the
corresponding decision procedure; a witness is the per-target
explanation of that fact.

## Cycles

Validation (`models.Load`) rejects the degenerate cycle: a formula
may not list itself as a prerequisite.

Beyond that, "cycle" in this engine means a cycle in the **union**
graph, and cycles through non-chosen alternatives are *unsupported*
by the ordering layer. Counterexample (fixture
`fo7AltCycleSheet`): `a` derives from `[b]` or, alternatively, from
`[c]`; `b` derives from `[a]`; `c` is a root. Under AND/OR semantics
the sheet is fully derivable from `{c}` (take `a`'s alternative
path, then `b` from `a`). But the union graph contains the cycle
`a, b, a`, so:

- `FindCycle` reports the cycle and `AssertAcyclic` fails;
- `TopoOrder`, `Levels`, `Waves`, `Depth`, and `Stats` fail, and
  every CLI command that builds a graph (`graph`, `study-order`,
  `practice`) exits with the cycle error;
- while `Closure`, `MinMemorizeSet`, and `MinMemorizeFor` still
  answer, because they work on the paths directly:
  `Closure({c})` covers the sheet and `MinMemorizeFor([b])` is
  `[c]`.

Sheet authors must keep the union of all paths acyclic, including
paths they expect solvers to ignore. Supporting
alternative-path-aware ordering is a separate, larger change and is
not promised by this contract.

## Complexity summary

| Routine | Worst case | Exact? |
| --- | --- | --- |
| New | O(P log P) | n/a |
| FindCycle | O(V + E) | n/a |
| TopoOrder | O(V log V + E) | n/a |
| Levels / Waves / Depth | O(V log V) | n/a |
| Descendants / Ancestors | O(V + E) | n/a |
| LeverageRanking / FoundationRanking | O(V * (V + E)) | n/a |
| Closure | O(V * P) | yes (fixed point) |
| MinMemorizeSet | O(V * P) | yes |
| MinMemorizeFor | exponential in unresolved targets/paths | yes |
| MinMemorizeForBounded | exponential in unresolved targets/paths, capped by budget | yes when complete, bounded otherwise |
| CoverageCurve | O(V^3 * P) | no (greedy heuristic) |

## Dependency pins

The module targets `go 1.24.2`. Dependencies are pinned to exact
versions in `go.mod`, with hashes in `go.sum`; `go mod tidy` is
clean and `go mod verify` passes. Direct dependencies:
`github.com/fogleman/gg v1.3.0` (GIF rendering),
`github.com/jung-kurt/gofpdf v1.16.2` (PDF export),
`github.com/spf13/cobra v1.10.2` (CLI), and
`gopkg.in/yaml.v3 v3.0.1` (sheet parsing). Indirect pins:
`github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0`,
`github.com/inconshreveable/mousetrap v1.1.0`,
`github.com/spf13/pflag v1.0.9`, and
`golang.org/x/image v0.0.0-20190910094157-69e4b8554b2a`. The FO7
fixtures fail if a pinned version changes or a direct dependency is
marked indirect again, so a dependency bump must update this
contract in the same change.

## CLI examples

These examples run against the embedded `psy103` sheet (22
formulas, 26 union edges; the eight shipped sheets total 265
formulas and 284 union edges). The FO7 fixtures execute the real
binary and pin the outputs quoted here.

Graph statistics and the exact whole-sheet memorize set:

```
$ formulaforge graph psy103
PSY 103 Introduction to Psychology
  formulas:   22
  derivations:26
  study waves:6 (longest chain: 5)
  roots: 5, leaves: 11
  highest leverage (unlocks the most):
    Mean: 17
    ...
  minimum memorize set (5):
    - Chi-square statistic
    - Mean
    - Median
    - Mode
    - Range
```

An exact target query. Both targets sit downstream of the single
root `mean`, so the exact covering set has size 1:

```
$ formulaforge graph psy103 --target regression-line,p-value
  ...
  smallest set covering targets (1):
    - Mean
```

Errors are stable strings on stderr with exit status 1:

```
$ formulaforge graph psy103 --target regression_line
error: unknown formula id: regression_line
$ formulaforge graph nosuchsheet
error: no sheet named "nosuchsheet"
```
