# depact

Dependency impact analyzer for large TypeScript codebases.

## What this is

`depact` builds the exact transitive import graph from an entry file and surfaces actionable metrics: exclusive cost per contributor, barrel amplification and wasted imports (modules dragged in by re-exports nobody uses), shortest import chain to a target (`why`), affected test selection from changed files.

Cost is measured in **module count** — what actually loads in a JS runtime — not bytes.

## Architecture

A pipeline of small, independently tested packages, each layered on the one below:

- `parser` — scans imports/exports from TS/JS source into a `Module`.
- `resolver` — resolves a specifier to a path: relative, tsconfig `paths`/`baseUrl`, package entries, `#imports`, builtins, externals.
- `tsconfig` — parses `paths` and `baseUrl`.
- `walker` — builds the transitive `Graph` in parallel; knobs: `FollowExternals`, `SkipTypeOnly`.
- `project` — wires an `fs.FS` + tsconfig + resolver + walker together (`project.Open(dir, tsconfig)`).
- `metrics` — graph analyses over the `Graph`: `Closure`, `Exclusive`, `Contributors` (the entry's direct imports ranked by edge-exclusive cost, with each one's total subtree), `Why`, `Barrels` (incl. wasted-import attribution: re-export targets no importer uses, costed as barrel-exclusive modules).
- `cli` — thin formatting layer over `project` + `metrics`. Front-ends (e.g. a future TUI) reuse the engine directly rather than going through it.

## Commands (target CLI)

`scan`, `check`, `why`, `diff`, `tui`. Shared flags: `--json`, `--type` (include type-only imports), `--follow-externals`, `--project`.

The FS is rooted at the project directory (dir of `--project`), so tsconfig paths, `node_modules` and the entry all resolve against one root; entries are given relative to that root.

`scan` takes one or more entries, including glob patterns (`**`, `{ts,tsx}`) expanded internally against the root — shells expand these inconsistently, so depact handles both pre-expanded args and raw patterns; expansion prunes `node_modules` and dot-dirs. All entries are walked as a single union graph (`Walk(entries...)`, `Graph.Entries`) so overlapping closures parse once — this is the core speed play on monorepos, and the same union graph is the substrate `diff` needs (reverse reachability: changed files → affected entries).

Output adapts to arity: one entry → full detail (top contributors as `exclusive / subtree`, barrels); many → summary (closure-size distribution, heaviest entries, union barrels). The detail view ranks the entry's direct imports by edge-exclusive cost (drop this import → closure shrinks by this many) alongside each import's total subtree (its whole footprint) — the gap between them flags owned vs shared cost. A future `--deep` flag will rank every module in the closure (not just direct imports) to surface deep chokepoints. Intended drill-down loop: summary → `scan <worst entry>` → `why <entry> <target>`.

The CLI is the agent interface — compact, ranked, stable output, `--json` with one schema across arities. `tui` is the human front-end for the same navigation.

Status: `scan` implemented, including multi-entry walk, globs and the summary view. `why` implemented on top of `metrics.Why` (shortest import chain, `--json`, non-zero exit when unreachable); `diff` needs reverse-reachability in `metrics`; `check` needs a budget/threshold config and non-zero exit on violation. `Walk` is error-tolerant: read/parse/resolve failures on a module are recorded in `Graph.Failures` (and surfaced by the CLI) rather than aborting the union, so `scan` runs over large monorepos best-effort. Symlinked pnpm workspace packages (`packages/*` linked into `node_modules`) are detected via their symlink target and treated as internal, so their source is walked into the closure; only symlinks that stay within `node_modules` (e.g. pnpm's `.pnpm` store) remain external leaves.

## Conventions

- Engine packages (`parser` through `metrics`, `project`) are standard library only. Front-end packages (`cli`, future `tui`) may take a well-justified dependency rather than reinvent the wheel (e.g. `**`/brace globbing, a TUI framework).
- Every package has table-driven tests; use `testing/fstest` for in-memory trees and `fixtures/` for realistic cases (barrels, externals, tsconfig paths).

## Code style

- Structs at the top of files.
- Public functions first, private functions last.
- No comments describing functions or logic, code must be self-explanatory.
