package resolver

import (
	"reflect"
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
