package resolver

import (
	"reflect"
	"testing"
	"testing/fstest"
)

func TestResolvePkgImports(t *testing.T) {
	tests := []struct {
		name      string
		fsys      fstest.MapFS
		from      string
		specifier string
		expected  Resolved
	}{
		{
			name: "imports exact key with string value",
			fsys: fstest.MapFS{
				"package.json":      {Data: []byte(`{"imports":{"#utils":"./src/utils.js"}}`)},
				"src/entry.ts":      {},
				"src/utils.js":      {},
			},
			from:      "src/entry.ts",
			specifier: "#utils",
			expected:  Resolved{Path: "src/utils.js"},
		},
		{
			name: "imports with conditional value",
			fsys: fstest.MapFS{
				"package.json": {Data: []byte(`{"imports":{"#log":{"types":"./src/log.d.ts","import":"./src/log.js"}}}`)},
				"src/entry.ts": {},
				"src/log.d.ts": {},
			},
			from:      "src/entry.ts",
			specifier: "#log",
			expected:  Resolved{Path: "src/log.d.ts"},
		},
		{
			name: "imports with wildcard pattern",
			fsys: fstest.MapFS{
				"package.json":          {Data: []byte(`{"imports":{"#util/*":"./src/util/*.js"}}`)},
				"src/entry.ts":          {},
				"src/util/format.js":    {},
			},
			from:      "src/entry.ts",
			specifier: "#util/format",
			expected:  Resolved{Path: "src/util/format.js"},
		},
		{
			name: "imports specifier not in map is unresolved",
			fsys: fstest.MapFS{
				"package.json": {Data: []byte(`{"imports":{"#utils":"./src/utils.js"}}`)},
				"src/entry.ts": {},
			},
			from:      "src/entry.ts",
			specifier: "#missing",
			expected:  Resolved{Kind: ResolveKindUnresolved},
		},
		{
			name: "no imports field is unresolved",
			fsys: fstest.MapFS{
				"package.json": {Data: []byte(`{"main":"./index.js"}`)},
				"src/entry.ts": {},
			},
			from:      "src/entry.ts",
			specifier: "#anything",
			expected:  Resolved{Kind: ResolveKindUnresolved},
		},
		{
			name: "imports walks up to enclosing package.json",
			fsys: fstest.MapFS{
				"package.json":          {Data: []byte(`{"imports":{"#utils":"./src/utils.js"}}`)},
				"src/deep/nested/x.ts":  {},
				"src/utils.js":          {},
			},
			from:      "src/deep/nested/x.ts",
			specifier: "#utils",
			expected:  Resolved{Path: "src/utils.js"},
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
