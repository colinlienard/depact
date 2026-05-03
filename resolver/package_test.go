package resolver

import (
	"errors"
	"testing"
	"testing/fstest"
)

func TestResolvePkgEntry(t *testing.T) {
	tests := []struct {
		name      string
		fsys      fstest.MapFS
		specifier string
		want      string
		wantErr   error
	}{
		{
			name: "main field",
			fsys: fstest.MapFS{
				"node_modules/lodash/package.json":  {Data: []byte(`{"main":"./dist/index.js"}`)},
				"node_modules/lodash/dist/index.js": {},
			},
			specifier: "lodash",
			want:      "node_modules/lodash/dist/index.js",
		},
		{
			name: "main field without leading dot-slash",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json": {Data: []byte(`{"main":"index.js"}`)},
				"node_modules/foo/index.js":     {},
			},
			specifier: "foo",
			want:      "node_modules/foo/index.js",
		},
		{
			name:      "package not installed",
			fsys:      fstest.MapFS{},
			specifier: "missing",
			wantErr:   ErrPkgNotFound,
		},
		{
			name: "package.json without main or exports",
			fsys: fstest.MapFS{
				"node_modules/empty/package.json": {Data: []byte(`{"name":"empty"}`)},
			},
			specifier: "empty",
			wantErr:   ErrPkgNoEntries,
		},
		{
			name: "exports as string",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json": {Data: []byte(`{"exports":"./index.js"}`)},
				"node_modules/foo/index.js":     {},
			},
			specifier: "foo",
			want:      "node_modules/foo/index.js",
		},
		{
			name: "exports subpath map, root only",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json": {Data: []byte(`{"exports":{".":"./index.js"}}`)},
				"node_modules/foo/index.js":     {},
			},
			specifier: "foo",
			want:      "node_modules/foo/index.js",
		},
		{
			name: "exports subpath map, deep import",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json": {Data: []byte(`{"exports":{".":"./index.js","./utils":"./utils.js"}}`)},
				"node_modules/foo/utils.js":     {},
			},
			specifier: "foo/utils",
			want:      "node_modules/foo/utils.js",
		},
		{
			name: "exports subpath not listed is unresolved",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json": {Data: []byte(`{"exports":{".":"./index.js"}}`)},
			},
			specifier: "foo/private",
			wantErr:   ErrPkgNotFound,
		},
		{
			name: "exports conditional map at root",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json": {Data: []byte(`{"exports":{"import":"./esm/index.js","require":"./cjs/index.js","default":"./cjs/index.js"}}`)},
				"node_modules/foo/esm/index.js": {},
			},
			specifier: "foo",
			want:      "node_modules/foo/esm/index.js",
		},
		{
			name: "exports types condition wins for type-aware resolution",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json": {Data: []byte(`{"exports":{"types":"./index.d.ts","import":"./index.js"}}`)},
				"node_modules/foo/index.d.ts":   {},
			},
			specifier: "foo",
			want:      "node_modules/foo/index.d.ts",
		},
		{
			name: "exports nested: subpath then conditional",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json": {Data: []byte(`{"exports":{".":{"import":"./esm/index.js","default":"./cjs/index.js"}}}`)},
				"node_modules/foo/esm/index.js": {},
			},
			specifier: "foo",
			want:      "node_modules/foo/esm/index.js",
		},
		{
			name: "exports falls back to default condition",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json": {Data: []byte(`{"exports":{".":{"node":"./node.js","default":"./fallback.js"}}}`)},
				"node_modules/foo/fallback.js":  {},
			},
			specifier: "foo",
			want:      "node_modules/foo/fallback.js",
		},
		{
			name: "exports takes precedence over main",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json": {Data: []byte(`{"main":"./main.js","exports":"./exported.js"}`)},
				"node_modules/foo/exported.js":  {},
			},
			specifier: "foo",
			want:      "node_modules/foo/exported.js",
		},
		{
			name: "exports subpath pattern with wildcard",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json":   {Data: []byte(`{"exports":{"./utils/*":"./src/utils/*.js"}}`)},
				"node_modules/foo/src/utils/x.js": {},
			},
			specifier: "foo/utils/x",
			want:      "node_modules/foo/src/utils/x.js",
		},
		{
			name: "exports wildcard prefers longer prefix",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json":     {Data: []byte(`{"exports":{"./*":"./generic/*.js","./utils/*":"./src/utils/*.js"}}`)},
				"node_modules/foo/src/utils/x.js":   {},
			},
			specifier: "foo/utils/x",
			want:      "node_modules/foo/src/utils/x.js",
		},
		{
			name: "exports wildcard with deep match",
			fsys: fstest.MapFS{
				"node_modules/foo/package.json":         {Data: []byte(`{"exports":{"./utils/*":"./src/utils/*.js"}}`)},
				"node_modules/foo/src/utils/a/b/c.js":   {},
			},
			specifier: "foo/utils/a/b/c",
			want:      "node_modules/foo/src/utils/a/b/c.js",
		},
		{
			name: "scoped package with exports",
			fsys: fstest.MapFS{
				"node_modules/@scope/pkg/package.json": {Data: []byte(`{"exports":"./index.js"}`)},
				"node_modules/@scope/pkg/index.js":     {},
			},
			specifier: "@scope/pkg",
			want:      "node_modules/@scope/pkg/index.js",
		},
		{
			name: "scoped package with subpath",
			fsys: fstest.MapFS{
				"node_modules/@scope/pkg/package.json": {Data: []byte(`{"exports":{".":"./index.js","./util":"./util.js"}}`)},
				"node_modules/@scope/pkg/util.js":      {},
			},
			specifier: "@scope/pkg/util",
			want:      "node_modules/@scope/pkg/util.js",
		},
		// {
		// 	name: "exports array fallback picks first existing",
		// 	fsys: fstest.MapFS{
		// 		"node_modules/foo/package.json": {Data: []byte(`{"exports":{".":["./a.js","./b.js"]}}`)},
		// 		"node_modules/foo/b.js":         {},
		// 	},
		// 	specifier: "foo",
		// 	want:      "node_modules/foo/b.js",
		// },
		// {
		// 	name: "exports null blocks subpath",
		// 	fsys: fstest.MapFS{
		// 		"node_modules/foo/package.json": {Data: []byte(`{"exports":{".":"./index.js","./internal":null}}`)},
		// 	},
		// 	specifier: "foo/internal",
		// 	wantErr:   ErrPkgNotFound,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.fsys, nil)
			got, err := r.resolvePkgEntry(tt.specifier)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
