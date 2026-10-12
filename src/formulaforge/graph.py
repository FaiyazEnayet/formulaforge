"""Dependency-graph algorithms over a formula sheet.

The graph is a DAG: an edge A -> B means "B derives from A" (learn A first).
Algorithms: cycle detection, topological study order grouped into waves,
transitive dependents (leverage), longest-chain depth, and a greedy
minimum-memorize set cover.
"""

from __future__ import annotations

from collections import defaultdict, deque


class FormulaGraph:
    def __init__(self, sheet):
        self.sheet = sheet
        self.nodes = sheet.ids()
        # prereqs: node -> set of direct prerequisites
        self.prereqs = {f.id: set(f.derives_from) for f in sheet.formulas}
        # children: node -> set of direct dependents
        self.children = defaultdict(set)
        for n in self.nodes:
            for p in self.prereqs[n]:
                self.children[p].add(n)

    # ---- structure ------------------------------------------------------

    def find_cycle(self):
        """Return a list of node ids forming a cycle, or None if acyclic."""
        WHITE, GRAY, BLACK = 0, 1, 2
        color = {n: WHITE for n in self.nodes}
        stack = []

        def dfs(u):
            color[u] = GRAY
            stack.append(u)
            for v in self.children[u]:
                if color[v] == GRAY:
                    return stack[stack.index(v):] + [v]
                if color[v] == WHITE:
                    hit = dfs(v)
                    if hit:
                        return hit
            stack.pop()
            color[u] = BLACK
            return None

        for n in self.nodes:
            if color[n] == WHITE:
                hit = dfs(n)
                if hit:
                    return hit
        return None

    def assert_acyclic(self):
        cycle = self.find_cycle()
        if cycle:
            raise ValueError("dependency cycle: " + " -> ".join(cycle))

    def topo_order(self) -> list:
        """Kahn's algorithm. Raises ValueError on cycles."""
        indeg = {n: len(self.prereqs[n]) for n in self.nodes}
        queue = deque(sorted(n for n in self.nodes if indeg[n] == 0))
        order = []
        while queue:
            u = queue.popleft()
            order.append(u)
            for v in sorted(self.children[u]):
                indeg[v] -= 1
                if indeg[v] == 0:
                    queue.append(v)
        if len(order) != len(self.nodes):
            raise ValueError("dependency cycle detected")
        return order

    def levels(self) -> dict:
        """Longest-path layering: level 0 = roots. Returns {node: level}."""
        order = self.topo_order()
        level = {}
        for n in order:
            if not self.prereqs[n]:
                level[n] = 0
            else:
                level[n] = 1 + max(level[p] for p in self.prereqs[n])
        return level

    def waves(self) -> list:
        """Study waves: lists of node ids grouped by level, topo-ordered."""
        level = self.levels()
        order = self.topo_order()
        pos = {n: i for i, n in enumerate(order)}
        buckets = defaultdict(list)
        for n in self.nodes:
            buckets[level[n]].append(n)
        return [sorted(buckets[k], key=lambda n: pos[n]) for k in sorted(buckets)]

    def depth(self) -> int:
        level = self.levels()
        return max(level.values()) if level else 0

    # ---- reachability ----------------------------------------------------

    def descendants(self, node: str) -> set:
        """All formulas transitively derived from node (it unlocks these)."""
        seen, stack = set(), [node]
        while stack:
            u = stack.pop()
            for v in self.children[u]:
                if v not in seen and v != node:
                    seen.add(v)
                    stack.append(v)
        return seen

    def ancestors(self, node: str) -> set:
        """All formulas node transitively builds on."""
        seen, stack = set(), [node]
        while stack:
            u = stack.pop()
            for p in self.prereqs[u]:
                if p not in seen and p != node:
                    seen.add(p)
                    stack.append(p)
        return seen

    # ---- centrality -------------------------------------------------------

    def leverage(self, node: str) -> int:
        """Number of formulas unlocked (transitively) by learning node."""
        return len(self.descendants(node))

    def leverage_ranking(self) -> list:
        """(node, leverage) sorted desc, ties broken by id for determinism."""
        return sorted(
            ((n, self.leverage(n)) for n in self.nodes),
            key=lambda t: (-t[1], t[0]),
        )

    def foundation_ranking(self) -> list:
        """(node, depth-of-prerequisites) sorted desc: deepest formulas."""
        return sorted(
            ((n, len(self.ancestors(n))) for n in self.nodes),
            key=lambda t: (-t[1], t[0]),
        )

    # ---- minimum-memorize set cover ---------------------------------------

    def coverage_of(self, node: str) -> set:
        """Formulas you get by memorizing node: itself + everything it unlocks."""
        return {node} | self.descendants(node)

    def min_memorize_set(self) -> list:
        """Greedy set cover: smallest-ish set of formulas whose coverage is all.

        Each formula covers itself plus everything derivable from it.
        Greedy is a classic ln(n)-approximation; ties broken deterministically.
        """
        uncovered = set(self.nodes)
        chosen = []
        while uncovered:
            best = max(
                self.nodes,
                key=lambda n: (len(self.coverage_of(n) & uncovered), n),
            )
            gain = self.coverage_of(best) & uncovered
            if not gain:
                break  # pragma: no cover - cannot happen on a DAG
            chosen.append(best)
            uncovered -= gain
        return chosen

    def coverage_curve(self) -> list:
        """[(k, fraction_covered)] as the greedy set grows: for the dashboard."""
        uncovered = set(self.nodes)
        total = len(self.nodes)
        curve = [(0, 0.0)]
        chosen = []
        while uncovered:
            best = max(
                self.nodes,
                key=lambda n: (len(self.coverage_of(n) & uncovered), n),
            )
            gain = self.coverage_of(best) & uncovered
            if not gain:
                break  # pragma: no cover
            chosen.append(best)
            uncovered -= gain
            curve.append((len(chosen), 1.0 - len(uncovered) / total))
        return curve

    # ---- stats -------------------------------------------------------------

    def stats(self) -> dict:
        n = len(self.nodes)
        e = sum(len(v) for v in self.prereqs.values())
        waves = self.waves()
        ranking = self.leverage_ranking()
        minset = self.min_memorize_set()
        return {
            "nodes": n,
            "edges": e,
            "depth": self.depth(),
            "waves": len(waves),
            "roots": sum(1 for x in self.nodes if not self.prereqs[x]),
            "leaves": sum(1 for x in self.nodes if not self.children[x]),
            "top_leverage": ranking[:5],
            "min_memorize_set": minset,
            "min_memorize_size": len(minset),
        }
