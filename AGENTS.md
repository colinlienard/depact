# depact

Dependency impact analyzer for large TypeScript codebases.

## What this is

`depact` builds the exact transitive import graph from an entry file and surfaces actionable metrics: exclusive cost per contributor, barrel amplification, shortest import chain to a target (`why`), affected test selection from changed files.

Cost is measured in **module count** — what actually loads in a JS runtime — not bytes.

## Architecture

A pipeline of small, independently tested packages, each layered on the one below:

- `parser` — scans imports/exports from TS/JS source into a `Module`.
- `resolver` — resolves a specifier to a path: relative, tsconfig `paths`/`baseUrl`, package entries, `#imports`, builtins, externals.
- `tsconfig` — parses `paths` and `baseUrl`.
- `walker` — builds the transitive `Graph` in parallel; knobs: `FollowExternals`, `SkipTypeOnly`.
- `project` — wires an `fs.FS` + tsconfig + resolver + walker together (`project.Open(dir, tsconfig)`).
- `metrics` — graph analyses over the `Graph`: `Closure`, `Exclusive`, `Why`, `Barrels`.
- `cli` — thin formatting layer over `project` + `metrics`. Front-ends (e.g. a future TUI) reuse the engine directly rather than going through it.

## Commands (target CLI)

`analyze`, `check`, `why`, `diff`, `tui`. Shared flags: `--json`, `--type` (include type-only imports), `--follow-externals`, `--project`.

The FS is rooted at the project directory (dir of `--project`), so tsconfig paths, `node_modules` and the entry all resolve against one root; entries are given relative to that root.

`analyze` takes one or more entries, including glob patterns (`**`, `{ts,tsx}`) expanded internally against the root — shells expand these inconsistently, so depact handles both pre-expanded args and raw patterns; expansion prunes `node_modules` and dot-dirs. All entries are walked as a single union graph (`Walk(entries...)`, `Graph.Entries`) so overlapping closures parse once — this is the core speed play on monorepos, and the same union graph is the substrate `diff` needs (reverse reachability: changed files → affected entries).

Output adapts to arity: one entry → full detail (exclusive cost, barrels); many → summary (closure-size distribution, heaviest entries, union barrels). The summary never computes `Exclusive` — it's the O(V·E) analysis, reserved for the detail view. Intended drill-down loop: summary → `analyze <worst entry>` → `why <entry> <target>`.

The CLI is the agent interface — compact, ranked, stable output, `--json` with one schema across arities. `tui` is the human front-end for the same navigation.

Status: `analyze` implemented, including multi-entry walk, globs and the summary view. `why` maps directly onto `metrics.Why`; `diff` needs reverse-reachability in `metrics`; `check` needs a budget/threshold config and non-zero exit on violation. `Walk` still aborts the whole union on the first read/parse/resolve failure — a per-entry error-tolerance mode (best-effort summary that records failures rather than failing the run) is needed before pointing `analyze` at large monorepos.

## Conventions

- Engine packages (`parser` through `metrics`, `project`) are standard library only. Front-end packages (`cli`, future `tui`) may take a well-justified dependency rather than reinvent the wheel (e.g. `**`/brace globbing, a TUI framework).
- Every package has table-driven tests; use `testing/fstest` for in-memory trees and `fixtures/` for realistic cases (barrels, externals, tsconfig paths).

## Code style

- Structs at the top of files.
- Public functions first, private functions last.
- No comments describing functions or logic, code must be self-explanatory.
