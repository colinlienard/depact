package resolver

import (
	"io/fs"
	"reflect"
	"sync"
	"testing"
	"testing/fstest"
)

func TestResolverWithoutPaths(t *testing.T) {
	tests := []struct {
		name      string
		fsys      fstest.MapFS
		from      string
		specifier string
		expected  Resolved
	}{
		{
			name: "with ts extension",
			fsys: fstest.MapFS{
				"src/entry.ts": {},
				"src/mod.ts":   {},
			},
			from:      "src/entry.ts",
			specifier: "./mod.ts",
			expected:  Resolved{Path: "src/mod.ts"},
		},
		{
			name: "with js extension",
			fsys: fstest.MapFS{
				"src/entry.ts": {},
				"src/mod.js":   {},
			},
			from:      "src/entry.ts",
			specifier: "./mod.js",
			expected:  Resolved{Path: "src/mod.js"},
		},
		{
			name: "with tsx extension",
			fsys: fstest.MapFS{
				"src/entry.ts": {},
				"src/mod.tsx":  {},
			},
			from:      "src/entry.ts",
			specifier: "./mod.tsx",
			expected:  Resolved{Path: "src/mod.tsx"},
		},
		{
			name: "without extension",
			fsys: fstest.MapFS{
				"src/entry.ts": {},
				"src/mod.ts":   {},
			},
			from:      "src/entry.ts",
			specifier: "./mod",
			expected:  Resolved{Path: "src/mod.ts"},
		},
		{
			name: "barrel",
			fsys: fstest.MapFS{
				"src/entry.ts":        {},
				"src/barrel/index.ts": {},
			},
			from:      "src/entry.ts",
			specifier: "./barrel",
			expected:  Resolved{Path: "src/barrel/index.ts", Kind: ResolveKindIndex},
		},
		{
			name: "deep",
			fsys: fstest.MapFS{
				"src/entry.ts":              {},
				"src/multiple/parts/mod.ts": {},
			},
			from:      "src/entry.ts",
			specifier: "./multiple/parts/mod.ts",
			expected:  Resolved{Path: "src/multiple/parts/mod.ts"},
		},
		{
			name: "one level up",
			fsys: fstest.MapFS{
				"src/entry.ts": {},
				"out.ts":       {},
			},
			from:      "src/entry.ts",
			specifier: "../out.ts",
			expected:  Resolved{Path: "out.ts"},
		},
		{
			name: "dependency with main export",
			fsys: fstest.MapFS{
				"src/entry.ts":                      {},
				"node_modules/lodash/package.json":  {Data: []byte(`{"main":"./dist/index.js"}`)},
				"node_modules/lodash/dist/index.js": {},
			},
			from:      "src/entry.ts",
			specifier: "lodash",
			expected:  Resolved{Path: "node_modules/lodash/dist/index.js", Kind: ResolveKindPackage, External: true},
		},
		{
			name: "dependency with exports",
			fsys: fstest.MapFS{
				"src/entry.ts":                     {},
				"node_modules/react/package.json":  {Data: []byte(`{"exports":{".":"./dist/index.js","./sub":"./dist/sub/index.js"}}`)},
				"node_modules/react/dist/index.js": {},
			},
			from:      "src/entry.ts",
			specifier: "react",
			expected:  Resolved{Path: "node_modules/react/dist/index.js", Kind: ResolveKindPackage, External: true},
		},
		{
			name: "dependency with sub-path-export",
			fsys: fstest.MapFS{
				"src/entry.ts":                         {},
				"node_modules/react/package.json":      {Data: []byte(`{"exports":{".":"./dist/index.js","./sub":"./dist/sub/index.js"}}`)},
				"node_modules/react/dist/sub/index.js": {},
			},
			from:      "src/entry.ts",
			specifier: "react/sub",
			expected:  Resolved{Path: "node_modules/react/dist/sub/index.js", Kind: ResolveKindPackage, External: true},
		},
		{
			name:      "node builtin",
			fsys:      fstest.MapFS{"src/entry.ts": {}},
			from:      "src/entry.ts",
			specifier: "fs",
			expected:  Resolved{Kind: ResolveKindBuiltin},
		},
		{
			name:      "node builtin with prefix",
			fsys:      fstest.MapFS{"src/entry.ts": {}},
			from:      "src/entry.ts",
			specifier: "node:fs",
			expected:  Resolved{Kind: ResolveKindBuiltin},
		},
		{
			name:      "bun builtin with prefix",
			fsys:      fstest.MapFS{"src/entry.ts": {}},
			from:      "src/entry.ts",
			specifier: "bun:sqlite",
			expected:  Resolved{Kind: ResolveKindBuiltin},
		},
		{
			name:      "external https URL",
			fsys:      fstest.MapFS{"src/entry.ts": {}},
			from:      "src/entry.ts",
			specifier: "https://esm.sh/foo",
			expected:  Resolved{Kind: ResolveKindExternal, External: true},
		},
		{
			name:      "external http URL",
			fsys:      fstest.MapFS{"src/entry.ts": {}},
			from:      "src/entry.ts",
			specifier: "http://example.com/mod.js",
			expected:  Resolved{Kind: ResolveKindExternal, External: true},
		},
		{
			name:      "external jsr scheme",
			fsys:      fstest.MapFS{"src/entry.ts": {}},
			from:      "src/entry.ts",
			specifier: "jsr:@scope/pkg",
			expected:  Resolved{Kind: ResolveKindExternal, External: true},
		},
		{
			name:      "external npm scheme",
			fsys:      fstest.MapFS{"src/entry.ts": {}},
			from:      "src/entry.ts",
			specifier: "npm:lodash",
			expected:  Resolved{Kind: ResolveKindExternal, External: true},
		},
		{
			name:      "external file URL",
			fsys:      fstest.MapFS{"src/entry.ts": {}},
			from:      "src/entry.ts",
			specifier: "file:///abs/path.js",
			expected:  Resolved{Kind: ResolveKindExternal, External: true},
		},
		{
			name:      "external data URL",
			fsys:      fstest.MapFS{"src/entry.ts": {}},
			from:      "src/entry.ts",
			specifier: "data:text/javascript,export%20const%20a=1",
			expected:  Resolved{Kind: ResolveKindExternal, External: true},
		},
		{
			name:      "unresolved path",
			fsys:      fstest.MapFS{"src/entry.ts": {}},
			from:      "src/entry.ts",
			specifier: "./unresolved",
			expected:  Resolved{Kind: ResolveKindUnresolved},
		},
		{
			name:      "unresolved package",
			fsys:      fstest.MapFS{"src/entry.ts": {}},
			from:      "src/entry.ts",
			specifier: "unresolved",
			expected:  Resolved{Kind: ResolveKindUnresolved},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.fsys, nil)
			got, err := r.Resolve(tt.from, tt.specifier)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, got)
			}
		})
	}
}

func TestResolverWithPaths(t *testing.T) {
	tests := []struct {
		name      string
		fsys      fstest.MapFS
		paths     map[string][]string
		from      string
		specifier string
		expected  Resolved
	}{
		{
			name: "wildcard mapping",
			fsys: fstest.MapFS{
				"src/entry.ts": {},
				"src/mod.ts":   {},
			},
			paths:     map[string][]string{"@/*": {"src/*"}},
			from:      "src/entry.ts",
			specifier: "@/mod",
			expected:  Resolved{Path: "src/mod.ts"},
		},
		{
			name: "exact mapping",
			fsys: fstest.MapFS{
				"src/entry.ts":       {},
				"src/lib/index.ts":   {},
				"src/lib/helpers.ts": {},
			},
			paths:     map[string][]string{"~lib": {"src/lib/index.ts"}},
			from:      "src/entry.ts",
			specifier: "~lib",
			expected:  Resolved{Path: "src/lib/index.ts"},
		},
		{
			name: "wildcard resolves to barrel directory",
			fsys: fstest.MapFS{
				"src/entry.ts":              {},
				"src/components/index.ts":   {},
				"src/components/button.tsx": {},
			},
			paths:     map[string][]string{"@/*": {"src/*"}},
			from:      "src/entry.ts",
			specifier: "@/components",
			expected:  Resolved{Path: "src/components/index.ts", Kind: ResolveKindIndex},
		},
		{
			name: "substitutions tried in order",
			fsys: fstest.MapFS{
				"src/entry.ts": {},
				"src/mod.ts":   {},
			},
			paths:     map[string][]string{"@/*": {"generated/*", "src/*"}},
			from:      "src/entry.ts",
			specifier: "@/mod",
			expected:  Resolved{Path: "src/mod.ts"},
		},
		{
			name: "longest matching prefix wins",
			fsys: fstest.MapFS{
				"src/entry.ts":    {},
				"src/ui/thing.ts": {},
				"design/thing.ts": {},
			},
			paths: map[string][]string{
				"@/*":    {"src/*"},
				"@/ui/*": {"design/*"},
			},
			from:      "src/entry.ts",
			specifier: "@/ui/thing",
			expected:  Resolved{Path: "design/thing.ts"},
		},
		{
			name: "exact key preferred over wildcard",
			fsys: fstest.MapFS{
				"src/entry.ts":      {},
				"src/real.ts":       {},
				"generated/real.ts": {},
			},
			paths: map[string][]string{
				"@/*":    {"generated/*"},
				"@/real": {"src/real.ts"},
			},
			from:      "src/entry.ts",
			specifier: "@/real",
			expected:  Resolved{Path: "src/real.ts"},
		},
		{
			name: "non-matching specifier falls through to node_modules",
			fsys: fstest.MapFS{
				"src/entry.ts":                      {},
				"node_modules/lodash/package.json":  {Data: []byte(`{"main":"./dist/index.js"}`)},
				"node_modules/lodash/dist/index.js": {},
			},
			paths:     map[string][]string{"@/*": {"src/*"}},
			from:      "src/entry.ts",
			specifier: "lodash",
			expected:  Resolved{Path: "node_modules/lodash/dist/index.js", Kind: ResolveKindPackage, External: true},
		},
		{
			name: "matching pattern with missing target is unresolved",
			fsys: fstest.MapFS{
				"src/entry.ts": {},
			},
			paths:     map[string][]string{"@/*": {"src/*"}},
			from:      "src/entry.ts",
			specifier: "@/missing",
			expected:  Resolved{Kind: ResolveKindUnresolved},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.fsys, tt.paths)
			got, err := r.Resolve(tt.from, tt.specifier)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, got)
			}
		})
	}
}

func TestResolverWorkspaceLink(t *testing.T) {
	tests := []struct {
		name      string
		fsys      fstest.MapFS
		specifier string
		expected  Resolved
	}{
		{
			name: "symlink escaping node_modules is internal",
			fsys: fstest.MapFS{
				"node_modules/@scope/pkg":   {Mode: fs.ModeSymlink, Data: []byte("../../packages/pkg")},
				"packages/pkg/package.json": {Data: []byte(`{"main":"src/index.ts"}`)},
				"packages/pkg/src/index.ts": {},
			},
			specifier: "@scope/pkg",
			expected:  Resolved{Path: "node_modules/@scope/pkg/src/index.ts", Kind: ResolveKindIndex, External: false},
		},
		{
			name: "workspace non-index entry stays package kind",
			fsys: fstest.MapFS{
				"node_modules/@scope/pkg":   {Mode: fs.ModeSymlink, Data: []byte("../../packages/pkg")},
				"packages/pkg/package.json": {Data: []byte(`{"main":"src/main.ts"}`)},
				"packages/pkg/src/main.ts":  {},
			},
			specifier: "@scope/pkg",
			expected:  Resolved{Path: "node_modules/@scope/pkg/src/main.ts", Kind: ResolveKindPackage, External: false},
		},
		{
			name: "unscoped symlink escaping node_modules is internal",
			fsys: fstest.MapFS{
				"node_modules/pkg":          {Mode: fs.ModeSymlink, Data: []byte("../packages/pkg")},
				"packages/pkg/package.json": {Data: []byte(`{"main":"index.js"}`)},
				"packages/pkg/index.js":     {},
			},
			specifier: "pkg",
			expected:  Resolved{Path: "node_modules/pkg/index.js", Kind: ResolveKindIndex, External: false},
		},
		{
			name: "plain package stays external",
			fsys: fstest.MapFS{
				"node_modules/react/package.json": {Data: []byte(`{"main":"index.js"}`)},
				"node_modules/react/index.js":     {},
			},
			specifier: "react",
			expected:  Resolved{Path: "node_modules/react/index.js", Kind: ResolveKindPackage, External: true},
		},
		{
			name: "symlink still inside node_modules stays external",
			fsys: fstest.MapFS{
				"node_modules/react": {Mode: fs.ModeSymlink, Data: []byte("./.pnpm/react/node_modules/react")},
				"node_modules/.pnpm/react/node_modules/react/package.json": {Data: []byte(`{"main":"index.js"}`)},
				"node_modules/.pnpm/react/node_modules/react/index.js":     {},
			},
			specifier: "react",
			expected:  Resolved{Path: "node_modules/react/index.js", Kind: ResolveKindPackage, External: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.fsys, nil)
			got, err := r.Resolve("src/entry.ts", tt.specifier)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, got)
			}
		})
	}
}

func TestResolverCache(t *testing.T) {
	fsys := fstest.MapFS{
		"src/entry.ts": {},
		"src/mod.ts":   {},
	}
	r := New(fsys, nil)

	first, err := r.Resolve("src/entry.ts", "./mod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := Resolved{Path: "src/mod.ts"}
	if !reflect.DeepEqual(first, expected) {
		t.Fatalf("expected %+v, got %+v", expected, first)
	}
	if _, ok := r.cache["src/entry.ts\x00./mod"]; !ok {
		t.Fatalf("expected result to be cached")
	}

	// Concurrent resolves must be race-free and consistent.
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := r.Resolve("src/entry.ts", "./mod")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, expected) {
				t.Errorf("expected %+v, got %+v", expected, got)
			}
		}()
	}
	wg.Wait()
}
