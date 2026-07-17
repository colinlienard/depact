package metrics

import (
	"reflect"
	"testing"
	"testing/fstest"

	"depact/resolver"
	"depact/walker"
)

func walk(t *testing.T, fsys fstest.MapFS, entries ...string) *walker.Graph {
	t.Helper()
	g, err := walker.New(fsys, resolver.New(fsys, nil)).Walk(entries...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return g
}

func file(src string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(src)}
}

// entry -> a -> shared, entry -> b -> shared, b -> only
func diamond(t *testing.T) *walker.Graph {
	t.Helper()
	return walk(t, fstest.MapFS{
		"src/entry.ts":  file("import './a'\nimport './b'"),
		"src/a.ts":      file(`import './shared'`),
		"src/b.ts":      file("import './shared'\nimport './only'"),
		"src/shared.ts": file(`export const s = 1`),
		"src/only.ts":   file(`export const o = 1`),
	}, "src/entry.ts")
}

func TestClosure(t *testing.T) {
	got := Closure(diamond(t))
	expected := map[string]int{
		"src/entry.ts":  4,
		"src/a.ts":      1,
		"src/b.ts":      2,
		"src/shared.ts": 0,
		"src/only.ts":   0,
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestClosureWithCycle(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/a.ts": file(`import './b'`),
		"src/b.ts": file(`import './a'`),
	}, "src/a.ts")
	got := Closure(g)
	expected := map[string]int{"src/a.ts": 1, "src/b.ts": 1}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestExclusive(t *testing.T) {
	g := diamond(t)
	got := Exclusive(g, g.Entries[0])
	expected := map[string]int{
		"src/a.ts":      1, // shared survives via b
		"src/b.ts":      2, // b and only
		"src/shared.ts": 1,
		"src/only.ts":   1,
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestExclusivePerEntry(t *testing.T) {
	// a -> shared, b -> shared + only: exclusive cost depends on the entry.
	g := walk(t, fstest.MapFS{
		"src/a.ts":      file(`import './shared'`),
		"src/b.ts":      file("import './shared'\nimport './only'"),
		"src/shared.ts": file(`export const s = 1`),
		"src/only.ts":   file(`export const o = 1`),
	}, "src/a.ts", "src/b.ts")

	fromA := Exclusive(g, g.Entries[0])
	fromB := Exclusive(g, g.Entries[1])
	if !reflect.DeepEqual(fromA, map[string]int{"src/shared.ts": 1}) {
		t.Errorf("from a: got %v", fromA)
	}
	if !reflect.DeepEqual(fromB, map[string]int{"src/shared.ts": 1, "src/only.ts": 1}) {
		t.Errorf("from b: got %v", fromB)
	}
}

func TestWhyShortestPath(t *testing.T) {
	// Two routes to target: entry -> long1 -> long2 -> target, entry -> short -> target.
	g := walk(t, fstest.MapFS{
		"src/entry.ts":  file("import './long1'\nimport './short'"),
		"src/long1.ts":  file(`import './long2'`),
		"src/long2.ts":  file(`import './target'`),
		"src/short.ts":  file(`import './target'`),
		"src/target.ts": file(`export const t = 1`),
	}, "src/entry.ts")

	chain := Why(g, g.Entries[0], "src/target.ts")
	var got []string
	for _, n := range chain {
		got = append(got, n.Module.Path)
	}
	expected := []string{"src/entry.ts", "src/short.ts", "src/target.ts"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestWhyEntryIsTarget(t *testing.T) {
	g := diamond(t)
	chain := Why(g, g.Entries[0], "src/entry.ts")
	if len(chain) != 1 || chain[0] != g.Entries[0] {
		t.Errorf("expected [entry], got %v", chain)
	}
}

func TestWhyUnknownTarget(t *testing.T) {
	g := diamond(t)
	if chain := Why(g, g.Entries[0], "src/nope.ts"); chain != nil {
		t.Errorf("expected nil, got %v", chain)
	}
}

func TestBarrels(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts":     file(`import { Button } from './ui'`),
		"src/other.ts":     file(`import { Button, Card } from './ui'`),
		"src/main.ts":      file("import './entry'\nimport './other'"),
		"src/ui/index.ts":  file("export { Button } from './Button'\nexport * from './Card'"),
		"src/ui/Button.ts": file(`export const Button = 1`),
		"src/ui/Card.ts":   file(`export const Card = 1`),
	}, "src/main.ts")

	barrels := Barrels(g)
	if len(barrels) != 1 {
		t.Fatalf("expected 1 barrel, got %v", barrels)
	}
	b := barrels["src/ui/index.ts"]
	if b.Importers != 2 || b.Symbols != 2 || b.Namespace || b.Deps != 2 {
		t.Errorf("expected importers=2 symbols=2 namespace=false deps=2, got %+v", b)
	}
}

func TestBarrelsNamespaceImport(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts":     file(`import * as UI from './ui'`),
		"src/ui/index.ts":  file(`export { Button } from './Button'`),
		"src/ui/Button.ts": file(`export const Button = 1`),
	}, "src/entry.ts")

	b := Barrels(g)["src/ui/index.ts"]
	if b == nil || !b.Namespace || b.Symbols != 0 {
		t.Errorf("expected namespace barrel with 0 named symbols, got %+v", b)
	}
}

func TestBarrelsNone(t *testing.T) {
	g := walk(t, fstest.MapFS{
		"src/entry.ts": file(`import './a'`),
		"src/a.ts":     file(`export const a = 1`),
	}, "src/entry.ts")
	if barrels := Barrels(g); len(barrels) != 0 {
		t.Errorf("expected no barrels, got %v", barrels)
	}
}
