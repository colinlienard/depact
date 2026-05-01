package resolver

import (
	"reflect"
	"testing"
	"testing/fstest"
)

// TODO: handle .js that references .ts (support passing tsconfig options, not only paths)

func TestResolverWithoutPaths(t *testing.T) {
	tests := []struct {
		name      string
		specifier string
		expected  Resolved
	}{
		{"with ts extension", "./mod.ts", Resolved{Path: "src/mod.ts"}},
		{"with js extension", "./mod.js", Resolved{Path: "src/mod.js"}},
		{"with tsx extension", "./mod.tsx", Resolved{Path: "src/mod.tsx"}},
		{"without extension", "./mod", Resolved{Path: "src/mod.ts"}},
		{"barrel", "./barrel", Resolved{Path: "src/barrel/index.ts"}},
		{"deep", "./multiple/parts/mod.ts", Resolved{Path: "src/multiple/parts/mod.ts"}},
		{"one level up", "../out.ts", Resolved{Path: "out.ts"}},
		// {"dependency with main export", "lodash", Resolved{Path: "node_modules/lodash/dist/index.js"}},
		// {"dependency with exports", "react", Resolved{Path: "node_modules/react/dist/index.js"}},
		// {"dependency with sub-path-export", "react/sub", Resolved{Path: "node_modules/react/dist/sub/index.js"}},
		// {"node builtin", "fs", Resolved{Kind: ResolveKindBuiltin}},
		// {"node builtin with prefix", "node:fs", Resolved{Kind: ResolveKindBuiltin}},
		// {"bun builtin with prefix", "bun:sqlite", Resolved{Kind: ResolveKindBuiltin}},
		// {"unresolved path", "./unresolved", Resolved{Kind: ResolveKindUnresolved}},
		// {"unresolved package", "unresolved", Resolved{Kind: ResolveKindUnresolved}},
	}

	fsys := fstest.MapFS{
		"src/entry.ts":                      {},
		"src/mod.ts":                        {},
		"src/mod.js":                        {},
		"src/mod.tsx":                       {},
		"src/barrel/index.ts":               {},
		"src/multiple/parts/mod.ts":         {},
		"out.ts":                            {},
		"node_modules/lodash/package.json":  {Data: []byte(`{"main":"./dist/index.js"}`)},
		"node_modules/lodash/dist/index.js": {},
		"node_modules/react/package.json":   {},
		"node_modules/react/dist/index.js":  {Data: []byte(`{"exports":{".":"./dist/index.js"},{"./sub":"./dist/sub/index.js"}}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := New(fsys, map[string][]string{}) // resolver.New(os.DirFS("/Users/..."))
			resolved, _ := resolver.Resolve("src/entry.ts", tt.specifier)

			if !reflect.DeepEqual(resolved, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, resolved)
			}
		})
	}
}
