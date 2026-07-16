package project

import (
	"reflect"
	"testing"
	"testing/fstest"

	"depact/tsconfig"
)

func TestRebasePaths(t *testing.T) {
	tests := []struct {
		name     string
		cfgPath  string
		cfg      tsconfig.Config
		expected map[string][]string
	}{
		{
			name:     "no paths no baseUrl",
			cfgPath:  "tsconfig.json",
			cfg:      tsconfig.Config{},
			expected: nil,
		},
		{
			name:     "paths at root",
			cfgPath:  "tsconfig.json",
			cfg:      tsconfig.Config{Paths: map[string][]string{"@/*": {"./src/*"}}},
			expected: map[string][]string{"@/*": {"src/*"}},
		},
		{
			name:    "paths relative to baseUrl",
			cfgPath: "tsconfig.json",
			cfg:     tsconfig.Config{BaseURL: "./src", Paths: map[string][]string{"@lib/*": {"lib/*"}}},
			expected: map[string][]string{
				"@lib/*": {"src/lib/*"},
				"*":      {"src/*"},
			},
		},
		{
			name:    "tsconfig in subdirectory",
			cfgPath: "packages/app/tsconfig.json",
			cfg:     tsconfig.Config{BaseURL: ".", Paths: map[string][]string{"@/*": {"src/*"}}},
			expected: map[string][]string{
				"@/*": {"packages/app/src/*"},
				"*":   {"packages/app/*"},
			},
		},
		{
			name:    "explicit star mapping not overridden by baseUrl",
			cfgPath: "tsconfig.json",
			cfg:     tsconfig.Config{BaseURL: ".", Paths: map[string][]string{"*": {"vendor/*"}}},
			expected: map[string][]string{
				"*": {"vendor/*"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rebasePaths(tt.cfgPath, &tt.cfg)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestLoadWithoutTsconfig(t *testing.T) {
	p, err := Load(fstest.MapFS{"src/entry.ts": {}}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Config == nil || p.Walker == nil || p.Resolver == nil {
		t.Fatalf("expected a fully initialized project, got %+v", p)
	}
}

func TestLoadExplicitMissingTsconfig(t *testing.T) {
	_, err := Load(fstest.MapFS{"src/entry.ts": {}}, "tsconfig.build.json")
	if err == nil {
		t.Fatalf("expected error for missing explicit tsconfig")
	}
}

func TestOpenPathsFixture(t *testing.T) {
	p, err := Open("../fixtures/paths", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g, err := p.Walker.Walk("src/app/page.tsx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"src/app/page.tsx", "src/lib/hook.ts", "src/lib/icon.tsx"} {
		if g.Modules[want] == nil {
			t.Errorf("expected %s in graph, got %v", want, keys(g.Modules))
		}
	}
}

func TestOpenExternalsFixture(t *testing.T) {
	p, err := Open("../fixtures/externals", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g, err := p.Walker.Walk("src/entry.ts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	acme := g.Modules["node_modules/acme/index.js"]
	if acme == nil || !acme.External || len(acme.Edges) != 0 {
		t.Errorf("expected external leaf for acme, got %+v", acme)
	}
}

func TestOpenExternalsFixtureFollowed(t *testing.T) {
	p, err := Open("../fixtures/externals", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p.Walker.FollowExternals = true
	g, err := p.Walker.Walk("src/entry.ts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Modules["node_modules/acme/helper.js"] == nil {
		t.Errorf("expected acme helper to be walked, got %v", keys(g.Modules))
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
