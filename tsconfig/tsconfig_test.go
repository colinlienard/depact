package tsconfig

import (
	"reflect"
	"testing"
	"testing/fstest"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		fsys     fstest.MapFS
		path     string
		expected Config
	}{
		{
			name: "empty config",
			fsys: fstest.MapFS{
				"tsconfig.json": {Data: []byte(`{}`)},
			},
			path:     "tsconfig.json",
			expected: Config{},
		},
		{
			name: "baseUrl and paths",
			fsys: fstest.MapFS{
				"tsconfig.json": {Data: []byte(`{
					"compilerOptions": {
						"baseUrl": ".",
						"paths": {
							"@/*": ["src/*"],
							"~lib": ["src/lib/index.ts"]
						}
					}
				}`)},
			},
			path: "tsconfig.json",
			expected: Config{
				BaseURL: ".",
				Paths: map[string][]string{
					"@/*":  {"src/*"},
					"~lib": {"src/lib/index.ts"},
				},
			},
		},
		// {
		// 	name: "moduleResolution and allowImportingTsExtensions",
		// 	fsys: fstest.MapFS{
		// 		"tsconfig.json": {Data: []byte(`{
		// 			"compilerOptions": {
		// 				"moduleResolution": "bundler",
		// 				"allowImportingTsExtensions": true
		// 			}
		// 		}`)},
		// 	},
		// 	path: "tsconfig.json",
		// 	expected: Config{
		// 		ModuleResolution:           "bundler",
		// 		AllowImportingTsExtensions: true,
		// 	},
		// },
		// {
		// 	name: "JSONC line comments",
		// 	fsys: fstest.MapFS{
		// 		"tsconfig.json": {Data: []byte(`{
		// 			// top-level comment
		// 			"compilerOptions": {
		// 				"baseUrl": "." // trailing comment
		// 			}
		// 		}`)},
		// 	},
		// 	path:     "tsconfig.json",
		// 	expected: Config{BaseURL: "."},
		// },
		// {
		// 	name: "JSONC block comments",
		// 	fsys: fstest.MapFS{
		// 		"tsconfig.json": {Data: []byte(`{
		// 			/* block
		// 			   comment */
		// 			"compilerOptions": { "baseUrl": "." }
		// 		}`)},
		// 	},
		// 	path:     "tsconfig.json",
		// 	expected: Config{BaseURL: "."},
		// },
		// {
		// 	name: "JSONC trailing commas",
		// 	fsys: fstest.MapFS{
		// 		"tsconfig.json": {Data: []byte(`{
		// 			"compilerOptions": {
		// 				"baseUrl": ".",
		// 				"paths": { "@/*": ["src/*",], },
		// 			},
		// 		}`)},
		// 	},
		// 	path: "tsconfig.json",
		// 	expected: Config{
		// 		BaseURL: ".",
		// 		Paths:   map[string][]string{"@/*": {"src/*"}},
		// 	},
		// },
		// {
		// 	name: "extends relative",
		// 	fsys: fstest.MapFS{
		// 		"tsconfig.base.json": {Data: []byte(`{
		// 			"compilerOptions": {
		// 				"baseUrl": ".",
		// 				"moduleResolution": "bundler"
		// 			}
		// 		}`)},
		// 		"tsconfig.json": {Data: []byte(`{
		// 			"extends": "./tsconfig.base.json",
		// 			"compilerOptions": {
		// 				"paths": { "@/*": ["src/*"] }
		// 			}
		// 		}`)},
		// 	},
		// 	path: "tsconfig.json",
		// 	expected: Config{
		// 		BaseURL:          ".",
		// 		ModuleResolution: "bundler",
		// 		Paths:            map[string][]string{"@/*": {"src/*"}},
		// 	},
		// },
		// {
		// 	name: "extends child overrides parent",
		// 	fsys: fstest.MapFS{
		// 		"tsconfig.base.json": {Data: []byte(`{
		// 			"compilerOptions": { "moduleResolution": "node" }
		// 		}`)},
		// 		"tsconfig.json": {Data: []byte(`{
		// 			"extends": "./tsconfig.base.json",
		// 			"compilerOptions": { "moduleResolution": "bundler" }
		// 		}`)},
		// 	},
		// 	path:     "tsconfig.json",
		// 	expected: Config{ModuleResolution: "bundler"},
		// },
		// {
		// 	name: "extends from node_modules package",
		// 	fsys: fstest.MapFS{
		// 		"node_modules/@acme/tsconfig/tsconfig.json": {Data: []byte(`{
		// 			"compilerOptions": { "moduleResolution": "bundler" }
		// 		}`)},
		// 		"tsconfig.json": {Data: []byte(`{
		// 			"extends": "@acme/tsconfig/tsconfig.json"
		// 		}`)},
		// 	},
		// 	path:     "tsconfig.json",
		// 	expected: Config{ModuleResolution: "bundler"},
		// },
		// {
		// 	name: "extends chain",
		// 	fsys: fstest.MapFS{
		// 		"tsconfig.a.json": {Data: []byte(`{
		// 			"compilerOptions": { "baseUrl": "." }
		// 		}`)},
		// 		"tsconfig.b.json": {Data: []byte(`{
		// 			"extends": "./tsconfig.a.json",
		// 			"compilerOptions": { "moduleResolution": "bundler" }
		// 		}`)},
		// 		"tsconfig.json": {Data: []byte(`{
		// 			"extends": "./tsconfig.b.json",
		// 			"compilerOptions": { "allowImportingTsExtensions": true }
		// 		}`)},
		// 	},
		// 	path: "tsconfig.json",
		// 	expected: Config{
		// 		BaseURL:                    ".",
		// 		ModuleResolution:           "bundler",
		// 		AllowImportingTsExtensions: true,
		// 	},
		// },
		// {
		// 	name: "extends array merges left to right",
		// 	fsys: fstest.MapFS{
		// 		"tsconfig.a.json": {Data: []byte(`{
		// 			"compilerOptions": { "baseUrl": ".", "moduleResolution": "node" }
		// 		}`)},
		// 		"tsconfig.b.json": {Data: []byte(`{
		// 			"compilerOptions": { "moduleResolution": "bundler" }
		// 		}`)},
		// 		"tsconfig.json": {Data: []byte(`{
		// 			"extends": ["./tsconfig.a.json", "./tsconfig.b.json"]
		// 		}`)},
		// 	},
		// 	path: "tsconfig.json",
		// 	expected: Config{
		// 		BaseURL:          ".",
		// 		ModuleResolution: "bundler",
		// 	},
		// },
		// {
		// 	name: "paths from parent are replaced not merged",
		// 	fsys: fstest.MapFS{
		// 		"tsconfig.base.json": {Data: []byte(`{
		// 			"compilerOptions": { "paths": { "@/*": ["src/*"] } }
		// 		}`)},
		// 		"tsconfig.json": {Data: []byte(`{
		// 			"extends": "./tsconfig.base.json",
		// 			"compilerOptions": { "paths": { "~/*": ["lib/*"] } }
		// 		}`)},
		// 	},
		// 	path: "tsconfig.json",
		// 	expected: Config{
		// 		Paths: map[string][]string{"~/*": {"lib/*"}},
		// 	},
		// },
		// {
		// 	name: "nested tsconfig path",
		// 	fsys: fstest.MapFS{
		// 		"packages/app/tsconfig.json": {Data: []byte(`{
		// 			"compilerOptions": { "baseUrl": "." }
		// 		}`)},
		// 	},
		// 	path:     "packages/app/tsconfig.json",
		// 	expected: Config{BaseURL: "."},
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(tt.fsys, tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(*got, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, *got)
			}
		})
	}
}

// func TestLoadErrors(t *testing.T) {
// 	tests := []struct {
// 		name string
// 		fsys fstest.MapFS
// 		path string
// 	}{
// 		{
// 			name: "missing file",
// 			fsys: fstest.MapFS{},
// 			path: "tsconfig.json",
// 		},
// 		{
// 			name: "invalid JSON",
// 			fsys: fstest.MapFS{
// 				"tsconfig.json": {Data: []byte(`{ not json `)},
// 			},
// 			path: "tsconfig.json",
// 		},
// 		{
// 			name: "extends missing file",
// 			fsys: fstest.MapFS{
// 				"tsconfig.json": {Data: []byte(`{"extends": "./nope.json"}`)},
// 			},
// 			path: "tsconfig.json",
// 		},
// 		{
// 			name: "extends cycle",
// 			fsys: fstest.MapFS{
// 				"tsconfig.a.json": {Data: []byte(`{"extends": "./tsconfig.b.json"}`)},
// 				"tsconfig.b.json": {Data: []byte(`{"extends": "./tsconfig.a.json"}`)},
// 				"tsconfig.json":   {Data: []byte(`{"extends": "./tsconfig.a.json"}`)},
// 			},
// 			path: "tsconfig.json",
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if _, err := Load(tt.fsys, tt.path); err == nil {
// 				t.Errorf("expected error, got nil")
// 			}
// 		})
// 	}
// }
