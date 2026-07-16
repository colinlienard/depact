package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"depact/resolver"
	"depact/walker"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPermute(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"flags first", []string{"--json", "entry.ts"}, []string{"--json", "entry.ts"}},
		{"flags after entry", []string{"entry.ts", "--json"}, []string{"--json", "entry.ts"}},
		{"value flag after entry", []string{"entry.ts", "--project", "tsconfig.json"}, []string{"--project", "tsconfig.json", "entry.ts"}},
		{"top flag after entry", []string{"entry.ts", "--top", "5"}, []string{"--top", "5", "entry.ts"}},
		{"double dash stops", []string{"--json", "--", "-weird.ts"}, []string{"--json", "-weird.ts"}},
		{"mixed", []string{"entry.ts", "--follow-externals", "--project", "p.json"}, []string{"--follow-externals", "--project", "p.json", "entry.ts"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := permute(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("permute(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeEntry(t *testing.T) {
	for in, want := range map[string]string{
		"./src/entry.ts":  "src/entry.ts",
		"src/entry.ts":    "src/entry.ts",
		"src/./a/../a.ts": "src/a.ts",
	} {
		if got := normalizeEntry(in); got != want {
			t.Errorf("normalizeEntry(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLocateWithProject(t *testing.T) {
	// --project fixes the root at its directory; args stay relative to it.
	tgt, err := locate("fixtures/barrel/tsconfig.json", []string{"src/entry.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if tgt.root != "fixtures/barrel" || tgt.tsconfig != "tsconfig.json" || !reflect.DeepEqual(tgt.args, []string{"src/entry.ts"}) {
		t.Errorf("got root=%q tsconfig=%q args=%v", tgt.root, tgt.tsconfig, tgt.args)
	}
}

func TestLocateAutoRoot(t *testing.T) {
	// no --project: walk up from the first entry to the nearest tsconfig and root there.
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "tsconfig.json"), "{}")
	entry := filepath.Join(dir, "src", "index.ts")
	mustWrite(t, entry, "export const x = 1")

	tgt, err := locate("", []string{entry})
	if err != nil {
		t.Fatal(err)
	}
	if tgt.root != dir || !reflect.DeepEqual(tgt.args, []string{"src/index.ts"}) {
		t.Errorf("got root=%q args=%v, want root=%q", tgt.root, tgt.args, dir)
	}
}

func TestLocateNoTsconfig(t *testing.T) {
	// no tsconfig anywhere above: root at the entry's own directory.
	dir := t.TempDir()
	entry := filepath.Join(dir, "index.ts")
	mustWrite(t, entry, "export const x = 1")

	tgt, err := locate("", []string{entry})
	if err != nil {
		t.Fatal(err)
	}
	if tgt.root != dir || tgt.tsconfig != "" || !reflect.DeepEqual(tgt.args, []string{"index.ts"}) {
		t.Errorf("got root=%q tsconfig=%q args=%v, want root=%q", tgt.root, tgt.tsconfig, tgt.args, dir)
	}
}

func TestLocatePatternOnly(t *testing.T) {
	// patterns are relative to cwd; root discovery falls back to cwd.
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "tsconfig.json"), "{}")
	t.Chdir(dir)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tgt, err := locate("", []string{"src/**/*.test.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if tgt.root != cwd || !reflect.DeepEqual(tgt.args, []string{"src/**/*.test.ts"}) {
		t.Errorf("got root=%q args=%v, want root=%q", tgt.root, tgt.args, cwd)
	}
}

func TestFindTsconfigIn(t *testing.T) {
	fsys := fstest.MapFS{
		"tsconfig.json":     {Data: []byte("{}")},
		"pkg/tsconfig.json": {Data: []byte("{}")},
		"pkg/src/a.ts":      {Data: []byte("export const a = 1")},
	}
	if got := findTsconfigIn(fsys, "pkg/src"); got != "pkg/tsconfig.json" {
		t.Errorf("pkg/src: got %q, want pkg/tsconfig.json", got)
	}
	if got := findTsconfigIn(fsys, "other"); got != "tsconfig.json" {
		t.Errorf("other: got %q, want tsconfig.json", got)
	}
	if got := findTsconfigIn(fstest.MapFS{}, "src"); got != "" {
		t.Errorf("empty fs: got %q, want empty", got)
	}
}

func TestExpand(t *testing.T) {
	fsys := fstest.MapFS{
		"src/a.test.ts":            {Data: []byte("")},
		"src/b.test.tsx":           {Data: []byte("")},
		"src/deep/c.test.ts":       {Data: []byte("")},
		"src/main.ts":              {Data: []byte("")},
		"node_modules/x/y.test.ts": {Data: []byte("")},
		".hidden/z.test.ts":        {Data: []byte("")},
	}
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"concrete pass-through", []string{"./src/main.ts"}, []string{"src/main.ts"}},
		{"star", []string{"src/*.test.ts"}, []string{"src/a.test.ts"}},
		{"doublestar prunes", []string{"**/*.test.ts"}, []string{"src/a.test.ts", "src/deep/c.test.ts"}},
		{"braces", []string{"src/**/*.test.{ts,tsx}"}, []string{"src/a.test.ts", "src/b.test.tsx", "src/deep/c.test.ts"}},
		{"mixed dedup", []string{"src/deep/c.test.ts", "**/*.test.ts"}, []string{"src/deep/c.test.ts", "src/a.test.ts"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expand(fsys, tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expand(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestExpandNoMatch(t *testing.T) {
	if _, err := expand(fstest.MapFS{}, []string{"src/*.test.ts"}); err == nil {
		t.Fatal("expected error for pattern with no matches")
	}
}

func graph(t *testing.T, files fstest.MapFS, entries ...string) *walker.Graph {
	t.Helper()
	g, err := walker.New(files, resolver.New(files, nil)).Walk(entries...)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return g
}

func TestBuildReport(t *testing.T) {
	// entry -> ui (barrel) -> Button, Card; entry -> a
	g := graph(t, fstest.MapFS{
		"src/entry.ts":     {Data: []byte("import { Button } from './ui'\nimport './a'")},
		"src/a.ts":         {Data: []byte("export const a = 1")},
		"src/ui/index.ts":  {Data: []byte("export * from './Button'\nexport * from './Card'")},
		"src/ui/Button.ts": {Data: []byte("export const Button = 1")},
		"src/ui/Card.ts":   {Data: []byte("export const Card = 1")},
	}, "src/entry.ts")

	r := buildReport(g)
	if len(r.Entries) != 1 || r.Entries[0].Path != "src/entry.ts" {
		t.Fatalf("entries = %+v", r.Entries)
	}
	e := r.Entries[0]
	if e.Modules != 4 { // a, ui, Button, Card
		t.Errorf("modules = %d, want 4", e.Modules)
	}
	if e.Externals != 0 {
		t.Errorf("externals = %d, want 0", e.Externals)
	}
	if r.Union.Modules != 5 || r.Union.Externals != 0 {
		t.Errorf("union = %+v, want 5 modules 0 externals", r.Union)
	}
	// exclusive must be sorted by cost descending; ui owns Button+Card = 3
	if len(e.Exclusive) == 0 || e.Exclusive[0].Path != "src/ui/index.ts" || e.Exclusive[0].Cost != 3 {
		t.Errorf("top contributor = %+v, want src/ui/index.ts cost 3", e.Exclusive)
	}
	for i := 1; i < len(e.Exclusive); i++ {
		if e.Exclusive[i-1].Cost < e.Exclusive[i].Cost {
			t.Errorf("exclusive not sorted descending: %+v", e.Exclusive)
		}
	}
	if len(r.Barrels) != 1 || r.Barrels[0].Path != "src/ui/index.ts" || r.Barrels[0].Deps != 2 {
		t.Errorf("barrels = %+v, want one ui barrel with deps 2", r.Barrels)
	}
}

func TestBuildReportMultiEntry(t *testing.T) {
	// a.test -> shared; b.test -> shared + only: summary stats over the union graph.
	g := graph(t, fstest.MapFS{
		"src/a.test.ts": {Data: []byte("import './shared'")},
		"src/b.test.ts": {Data: []byte("import './shared'\nimport './only'")},
		"src/shared.ts": {Data: []byte("export const s = 1")},
		"src/only.ts":   {Data: []byte("export const o = 1")},
	}, "src/a.test.ts", "src/b.test.ts")

	r := buildReport(g)
	if len(r.Entries) != 2 {
		t.Fatalf("entries = %+v", r.Entries)
	}
	// sorted by closure size descending
	if r.Entries[0].Path != "src/b.test.ts" || r.Entries[0].Modules != 2 {
		t.Errorf("heaviest = %+v, want src/b.test.ts with 2 modules", r.Entries[0])
	}
	if r.Entries[1].Path != "src/a.test.ts" || r.Entries[1].Modules != 1 {
		t.Errorf("lightest = %+v, want src/a.test.ts with 1 module", r.Entries[1])
	}
	if r.Entries[0].Exclusive != nil || r.Entries[1].Exclusive != nil {
		t.Errorf("expected no exclusive lists in multi-entry report")
	}
	if r.Union.Modules != 4 || r.Union.SharedByAll != 1 {
		t.Errorf("union = %+v, want 4 modules, 1 shared by all", r.Union)
	}
}

func TestBuildReportExternals(t *testing.T) {
	// only resolved node_modules packages count as external; builtins do not.
	g := graph(t, fstest.MapFS{
		"src/entry.ts":                   {Data: []byte("import 'acme'\nimport 'node:fs'\nimport './a'")},
		"src/a.ts":                       {Data: []byte("export const a = 1")},
		"node_modules/acme/package.json": {Data: []byte(`{"main":"index.js"}`)},
		"node_modules/acme/index.js":     {Data: []byte("module.exports = 1")},
	}, "src/entry.ts")
	r := buildReport(g)
	if r.Union.Externals != 1 {
		t.Errorf("externals = %d, want 1 (acme only)", r.Union.Externals)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := Run([]string{"bogus"}, &out, &errBuf); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "unknown command") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestRunNoArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := Run(nil, &out, &errBuf); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunAnalyzeFixtureJSON(t *testing.T) {
	t.Chdir("..") // run from repo root so fixtures resolve under os.DirFS(".")
	var out, errBuf bytes.Buffer
	// entry is relative to the project root (dir of --project).
	code := Run([]string{
		"analyze", "src/entry.ts",
		"--project", "fixtures/barrel/tsconfig.json", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	var r analyzeReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(r.Entries) != 1 || r.Entries[0].Path != "src/entry.ts" || r.Entries[0].Modules == 0 {
		t.Errorf("unexpected report: %+v", r)
	}
	if len(r.Entries[0].Exclusive) == 0 {
		t.Errorf("expected exclusive list for single entry, got %+v", r.Entries[0])
	}
}

func TestRunAnalyzeExternalsFixture(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	// --follow-externals walks into node_modules, which resolves against the
	// project root (fixtures/externals), so acme/leftpad count as externals.
	code := Run([]string{
		"analyze", "src/entry.ts",
		"--project", "fixtures/externals/tsconfig.json",
		"--follow-externals", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	var r analyzeReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if r.Union.Externals != 2 {
		t.Errorf("externals = %d, want 2 (acme, leftpad); report %+v", r.Union.Externals, r)
	}
}

func TestRunAnalyzeAutoRoot(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	// no --project: depact roots at this repository (nearest .git), so the entry
	// key is repo-relative and its nearest tsconfig still governs resolution.
	code := Run([]string{"analyze", "fixtures/barrel/src/entry.ts", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	var r analyzeReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(r.Entries) != 1 || r.Entries[0].Path != "fixtures/barrel/src/entry.ts" || r.Entries[0].Modules == 0 {
		t.Errorf("got %+v, want fixtures/barrel/src/entry.ts and >0 modules", r.Entries)
	}
}

func TestRunAnalyzeGlobSummary(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	// a quoted pattern reaches depact unexpanded; it expands against the
	// project root and the multi-entry run prints a summary.
	code := Run([]string{
		"analyze", "src/**/*.test.ts",
		"--project", "fixtures/multi/tsconfig.json", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	var r analyzeReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(r.Entries) != 3 {
		t.Fatalf("entries = %+v, want 3", r.Entries)
	}
	if r.Entries[0].Path != "src/c.test.ts" || r.Entries[0].Modules != 3 {
		t.Errorf("heaviest = %+v, want src/c.test.ts with 3 modules", r.Entries[0])
	}
	if r.Union.Modules != 6 || r.Union.SharedByAll != 1 {
		t.Errorf("union = %+v, want 6 modules, 1 shared by all", r.Union)
	}
	for _, e := range r.Entries {
		if e.Exclusive != nil {
			t.Errorf("expected no exclusive list in summary, got %+v", e)
		}
	}
}

func TestRunAnalyzeSummaryText(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	code := Run([]string{
		"analyze", "src/**/*.test.ts",
		"--project", "fixtures/multi/tsconfig.json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	text := out.String()
	for _, want := range []string{"3 entries", "heaviest entries", "closure size", "src/c.test.ts"} {
		if !strings.Contains(text, want) {
			t.Errorf("summary missing %q:\n%s", want, text)
		}
	}
}

func TestRunAnalyzeMissingEntry(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := Run([]string{"analyze"}, &out, &errBuf); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}
