# depact

Dependency impact analyzer for large TypeScript codebases

## What this is

`depact` builds the exact transitive import graph from an entry file and surfaces actionable metrics: exclusive cost per contributor, barrel amplification, shortest import chain to a target (`why`), affected test selection from changed files.

## Key architecture decisions

Custom import scanner and resolver written in Go for speed and parallelism.

## Commands (target CLI)

`analyze`, `check`, `why`, `diff`.
All commands support `--json`, `--type`, `--follow-externals`, `--project`.
