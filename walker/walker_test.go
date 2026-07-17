package walker

import (
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"

	"depact/resolver"
)

func walk(t *testing.T, fsys fstest.MapFS, entries ...string) *Graph {
	t.Helper()
	g, err := New(fsys, resolver.New(fsys, nil)).Walk(entries...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return g
}

func file(src string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(src)}
}

func edges(n *Node) []string {
	var out []string
	for _, e := range n.Edges {
		out = append(out, e.To.Module.Path)
	}
	return out
}

func TestWalkChain(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts": file(`import { a } from './a'`),
		"src/a.ts":     file(`import { b } from './b'`),
		"src/b.ts":     file(`export const b = 1`),
	}, "src/entry.ts")

	if len(g.Modules) != 3 {
		t.Fatalf("expected 3 modules, got %d", len(g.Modules))
	}
	if g.Entries[0].Module.Path != "src/entry.ts" {
		t.Errorf("expected entry src/entry.ts, got %s", g.Entries[0].Module.Path)
	}
	if got := edges(g.Entries[0]); len(got) != 1 || got[0] != "src/a.ts" {
		t.Errorf("expected entry edge to src/a.ts, got %v", got)
	}
	if got := edges(g.Modules["src/a.ts"]); len(got) != 1 || got[0] != "src/b.ts" {
		t.Errorf("expected a.ts edge to src/b.ts, got %v", got)
	}
	if got := edges(g.Modules["src/b.ts"]); got != nil {
		t.Errorf("expected b.ts to be a leaf, got %v", got)
	}
}

func TestWalkDiamondSharesNode(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts":  file(`import './a'; import './b'`),
		"src/a.ts":      file(`import { s } from './shared'`),
		"src/b.ts":      file(`import { s } from './shared'`),
		"src/shared.ts": file(`export const s = 1`),
	}, "src/entry.ts")

	if len(g.Modules) != 4 {
		t.Fatalf("expected 4 modules, got %d", len(g.Modules))
	}
	shared := g.Modules["src/shared.ts"]
	if g.Modules["src/a.ts"].Edges[0].To != shared || g.Modules["src/b.ts"].Edges[0].To != shared {
		t.Errorf("expected a.ts and b.ts to point at the same node")
	}
}

func TestWalkCycle(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/a.ts": file(`import { b } from './b'`),
		"src/b.ts": file(`import { a } from './a'`),
	}, "src/a.ts")

	if len(g.Modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(g.Modules))
	}
	if g.Modules["src/a.ts"].Edges[0].To != g.Modules["src/b.ts"] {
		t.Errorf("expected a.ts -> b.ts")
	}
	if g.Modules["src/b.ts"].Edges[0].To != g.Modules["src/a.ts"] {
		t.Errorf("expected b.ts -> a.ts")
	}
}

func TestWalkSelfImport(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/a.ts": file(`import { a } from './a'`),
	}, "src/a.ts")

	if len(g.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(g.Modules))
	}
	if g.Entries[0].Edges[0].To != g.Entries[0] {
		t.Errorf("expected self edge")
	}
}

func TestWalkFollowsReExports(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts":     file(`import { Button } from './ui'`),
		"src/ui/index.ts":  file("export { Button } from './Button'\nexport * from './Card'"),
		"src/ui/Button.ts": file(`export const Button = 1`),
		"src/ui/Card.ts":   file(`export const Card = 1`),
	}, "src/entry.ts")

	if len(g.Modules) != 4 {
		t.Fatalf("expected 4 modules, got %d", len(g.Modules))
	}
	barrel := g.Modules["src/ui/index.ts"]
	if barrel == nil || g.Entries[0].Edges[0].Kind != resolver.ResolveKindIndex {
		t.Fatalf("expected barrel index edge, got %+v", g.Entries[0].Edges[0])
	}
	got := edges(barrel)
	if len(got) != 2 || got[0] != "src/ui/Button.ts" || got[1] != "src/ui/Card.ts" {
		t.Errorf("expected barrel edges to Button and Card, got %v", got)
	}
}

func TestWalkExternalIsLeaf(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts":                      file(`import _ from 'lodash'`),
		"node_modules/lodash/package.json":  file(`{"main":"./dist/index.js"}`),
		"node_modules/lodash/dist/index.js": file(`import 'should-not-be-walked'`),
	}, "src/entry.ts")

	if len(g.Modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(g.Modules))
	}
	ext := g.Modules["node_modules/lodash/dist/index.js"]
	if ext == nil || !ext.External {
		t.Fatalf("expected external node, got %+v", ext)
	}
	if len(ext.Edges) != 0 {
		t.Errorf("expected external node not to be walked, got edges %v", edges(ext))
	}
}

func TestWalkWorkspacePackageIsInternal(t *testing.T) {
	fsys := fstest.MapFS{
		"webapps/app/src/index.ts":         file("import { shared } from '@ws/lib'\nimport _ from 'lodash'"),
		"node_modules/@ws/lib":             {Mode: fs.ModeSymlink, Data: []byte("../../packages/lib")},
		"packages/lib/package.json":        file(`{"main":"src/index.ts"}`),
		"packages/lib/src/index.ts":        file("export { shared } from './shared'"),
		"packages/lib/src/shared.ts":       file("export const shared = 1"),
		"node_modules/lodash/package.json": file(`{"main":"./index.js"}`),
		"node_modules/lodash/index.js":     file("import './internal'"),
		"node_modules/lodash/internal.js":  file("export const d = 1"),
	}
	g, err := New(fsys, resolver.New(fsys, nil)).Walk("webapps/app/src/index.ts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lib := g.Modules["node_modules/@ws/lib/src/index.ts"]
	if lib == nil || lib.External {
		t.Fatalf("expected workspace package to be internal, got %+v", lib)
	}
	if got := edges(lib); len(got) != 1 || got[0] != "node_modules/@ws/lib/src/shared.ts" {
		t.Errorf("expected workspace source to be walked, got edges %v", got)
	}
	if g.Modules["node_modules/@ws/lib/src/shared.ts"] == nil {
		t.Errorf("expected transitive workspace source to be walked")
	}
	if g.Entries[0].Edges[0].Kind != resolver.ResolveKindIndex {
		t.Errorf("expected cross-package barrel edge kind Index, got %v", g.Entries[0].Edges[0].Kind)
	}

	lodash := g.Modules["node_modules/lodash/index.js"]
	if lodash == nil || !lodash.External {
		t.Fatalf("expected real third-party dep to stay external, got %+v", lodash)
	}
	if len(lodash.Edges) != 0 {
		t.Errorf("expected third-party dep to remain a leaf, got edges %v", edges(lodash))
	}
	if g.Modules["node_modules/lodash/internal.js"] != nil {
		t.Errorf("expected third-party source not to be walked")
	}
}

func TestWalkLeafKinds(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts": file("import fs from 'node:fs'\nimport x from './missing'\nimport y from 'https://esm.sh/y'"),
	}, "src/entry.ts")

	tests := []struct {
		key      string
		kind     resolver.ResolveKind
		external bool
	}{
		{"node:fs", resolver.ResolveKindBuiltin, false},
		{"src/missing", resolver.ResolveKindUnresolved, false},
		{"https://esm.sh/y", resolver.ResolveKindExternal, true},
	}
	for i, tt := range tests {
		n := g.Modules[tt.key]
		if n == nil {
			t.Errorf("expected node for %s", tt.key)
			continue
		}
		e := g.Entries[0].Edges[i]
		if e.To != n || e.Kind != tt.kind || n.External != tt.external || len(n.Edges) != 0 {
			t.Errorf("%s: expected kind=%v external=%v leaf, got edge %+v node %+v", tt.key, tt.kind, tt.external, e, n)
		}
	}
}

func TestWalkUnresolvedRelativesStayDistinct(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts": file("import './a/x'\nimport './b/y'"),
		"src/a/x.ts":   file(`import './missing'`),
		"src/b/y.ts":   file(`import './missing'`),
	}, "src/entry.ts")

	if len(g.Modules) != 5 {
		t.Fatalf("expected 5 modules, got %d", len(g.Modules))
	}
	a := g.Modules["src/a/x.ts"].Edges[0].To
	b := g.Modules["src/b/y.ts"].Edges[0].To
	if a == b {
		t.Errorf("expected distinct unresolved targets, both keyed %s", a.Module.Path)
	}
	if a.Module.Path != "src/a/missing" || b.Module.Path != "src/b/missing" {
		t.Errorf("expected keys src/a/missing and src/b/missing, got %s and %s", a.Module.Path, b.Module.Path)
	}
}

func TestWalkEdgeKindsPerRoute(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts":     file("import './a'\nimport './b'"),
		"src/a.ts":         file(`import { Button } from './ui'`),
		"src/b.ts":         file(`import { Button } from './ui/index'`),
		"src/ui/index.ts":  file(`export { Button } from './Button'`),
		"src/ui/Button.ts": file(`export const Button = 1`),
	}, "src/entry.ts")

	viaBarrel := g.Modules["src/a.ts"].Edges[0]
	direct := g.Modules["src/b.ts"].Edges[0]
	if viaBarrel.To != direct.To {
		t.Fatalf("expected both routes to share the node")
	}
	if viaBarrel.Kind != resolver.ResolveKindIndex {
		t.Errorf("expected index kind via './ui', got %v", viaBarrel.Kind)
	}
	if direct.Kind != resolver.ResolveKindFile {
		t.Errorf("expected file kind via './ui/index', got %v", direct.Kind)
	}
}

func TestWalkLocalRouteUpgradesExternalLeaf(t *testing.T) {
	fsys := fstest.MapFS{
		"src/entry.ts":                     file("import _ from 'lodash'\nimport { x } from '@lib/index'"),
		"node_modules/lodash/package.json": file(`{"main":"./index.js"}`),
		"node_modules/lodash/index.js":     file(`import './dep'`),
		"node_modules/lodash/dep.js":       file(`export const d = 1`),
	}
	r := resolver.New(fsys, map[string][]string{"@lib/*": {"node_modules/lodash/*"}})
	g, err := New(fsys, r).Walk("src/entry.ts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n := g.Modules["node_modules/lodash/index.js"]
	if n == nil || n.External {
		t.Fatalf("expected local classification to win, got %+v", n)
	}
	if got := edges(n); len(got) != 1 || got[0] != "node_modules/lodash/dep.js" {
		t.Errorf("expected aliased module to be walked, got edges %v", got)
	}
}

func TestWalkEdgeKeepsImport(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts": file(`import type { T } from './t'`),
		"src/t.ts":     file(`export type T = string`),
	}, "src/entry.ts")

	e := g.Entries[0].Edges[0]
	if e.Import.From != "./t" || !e.Import.TypeOnly() {
		t.Errorf("expected type-only import edge for ./t, got %+v", e.Import)
	}
}

func TestWalkSkipTypeOnly(t *testing.T) {
	fsys := fstest.MapFS{
		"src/entry.ts": file("import type { T } from './t'\nimport { v } from './v'"),
		"src/t.ts":     file(`export type T = string`),
		"src/v.ts":     file(`export const v = 1`),
	}
	w := New(fsys, resolver.New(fsys, nil))
	w.SkipTypeOnly = true
	g, err := w.Walk("src/entry.ts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := edges(g.Entries[0]); len(got) != 1 || got[0] != "src/v.ts" {
		t.Errorf("expected only value edge to src/v.ts, got %v", got)
	}
	if g.Modules["src/t.ts"] != nil {
		t.Errorf("expected type-only module to be absent")
	}
}

func TestWalkMultiEntry(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/a.test.ts": file(`import { s } from './shared'`),
		"src/b.test.ts": file(`import { s } from './shared'`),
		"src/shared.ts": file(`export const s = 1`),
	}, "src/a.test.ts", "src/b.test.ts")

	if len(g.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(g.Entries))
	}
	if len(g.Modules) != 3 {
		t.Fatalf("expected 3 modules, got %d", len(g.Modules))
	}
	if g.Entries[0].Edges[0].To != g.Entries[1].Edges[0].To {
		t.Errorf("expected entries to share the same node")
	}
}

func TestWalkDuplicateEntries(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/a.ts": file(`export const a = 1`),
	}, "src/a.ts", "src/a.ts")

	if len(g.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(g.Entries))
	}
}

func TestWalkTolerantOfBrokenDependency(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts":  file(`import './ok'` + "\n" + `import './broken'`),
		"src/ok.ts":     file(`export const ok = 1`),
		"src/broken.ts": file(`import * x from 'm'`), // namespace import missing `as` -> parse error
	}, "src/entry.ts")

	if n, ok := g.Modules["src/ok.ts"]; !ok || n.Failed {
		t.Fatalf("expected the healthy sibling to still be walked")
	}
	if len(g.Failures) != 1 {
		t.Fatalf("expected 1 recorded failure, got %d: %+v", len(g.Failures), g.Failures)
	}
	if g.Failures[0].Path != "src/broken.ts" {
		t.Fatalf("expected failure on src/broken.ts, got %q", g.Failures[0].Path)
	}
	if !g.Modules["src/broken.ts"].Failed {
		t.Fatalf("expected src/broken.ts node to be marked Failed")
	}
}

func TestWalkNoEntries(t *testing.T) {
	if _, err := New(fstest.MapFS{}, resolver.New(fstest.MapFS{}, nil)).Walk(); err == nil {
		t.Fatalf("expected error for no entries")
	}
}

func TestWalkMissingEntry(t *testing.T) {
	g, err := New(fstest.MapFS{}, resolver.New(fstest.MapFS{}, nil)).Walk("src/entry.ts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.Failures) != 1 || g.Failures[0].Path != "src/entry.ts" {
		t.Fatalf("expected one recorded failure for the missing entry, got %+v", g.Failures)
	}
	if !g.Entries[0].Failed {
		t.Fatalf("expected the entry node to be marked Failed")
	}
}

func TestWalkWide(t *testing.T) {
	fsys := fstest.MapFS{}
	entry := ""
	for i := range 200 {
		entry += fmt.Sprintf("import './mod%d'\n", i)
		fsys[fmt.Sprintf("src/mod%d.ts", i)] = file(`import { s } from './shared'`)
	}
	fsys["src/entry.ts"] = file(entry)
	fsys["src/shared.ts"] = file(`export const s = 1`)

	g := walk(t, fsys, "src/entry.ts")
	if len(g.Modules) != 202 {
		t.Fatalf("expected 202 modules, got %d", len(g.Modules))
	}
	if len(g.Entries[0].Edges) != 200 {
		t.Fatalf("expected 200 entry edges, got %d", len(g.Entries[0].Edges))
	}
}
