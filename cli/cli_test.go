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
		{"flags first", []string{"--json", "entry.ts"}, []string{"--json", "--", "entry.ts"}},
		{"flags after entry", []string{"entry.ts", "--json"}, []string{"--json", "--", "entry.ts"}},
		{"value flag after entry", []string{"entry.ts", "--project", "tsconfig.json"}, []string{"--project", "tsconfig.json", "--", "entry.ts"}},
		{"top flag after entry", []string{"entry.ts", "--top", "5"}, []string{"--top", "5", "--", "entry.ts"}},
		{"double dash stops", []string{"--json", "--", "-weird.ts"}, []string{"--json", "--", "-weird.ts"}},
		{"mixed", []string{"entry.ts", "--follow-externals", "--project", "p.json"}, []string{"--follow-externals", "--project", "p.json", "--", "entry.ts"}},
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
	if tgt.root != filepath.FromSlash("fixtures/barrel") || tgt.tsconfig != "tsconfig.json" || !reflect.DeepEqual(tgt.args, []string{"src/entry.ts"}) {
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

func TestLocateGlobPrefixRoot(t *testing.T) {
	// an absolute glob anchors on its literal prefix, so root discovery walks up
	// to the target repo (.git) without needing --project.
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main")
	mustWrite(t, filepath.Join(dir, "tsconfig.json"), "{}")
	mustWrite(t, filepath.Join(dir, "src", "a.test.ts"), "export const a = 1")

	pattern := filepath.ToSlash(dir) + "/src/**/*.test.ts"
	tgt, err := locate("", []string{pattern})
	if err != nil {
		t.Fatal(err)
	}
	if tgt.root != dir || !reflect.DeepEqual(tgt.args, []string{"src/**/*.test.ts"}) {
		t.Errorf("got root=%q args=%v, want root=%q args=[src/**/*.test.ts]", tgt.root, tgt.args, dir)
	}
}

func TestLiteralPrefix(t *testing.T) {
	for in, want := range map[string]string{
		"/abs/src/**/*.test.ts": "/abs/src",
		"src/**/*.ts":           "src",
		"src/*.ts":              "src",
		"**/*.ts":               "",
		"*.ts":                  "",
		"a/b/c.ts":              "a/b/c.ts",
	} {
		if got := literalPrefix(in); got != want {
			t.Errorf("literalPrefix(%q) = %q, want %q", in, got, want)
		}
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
	if len(e.Contributors) == 0 || e.Contributors[0].Path != "src/ui/index.ts" ||
		e.Contributors[0].Exclusive != 3 || e.Contributors[0].Subtree != 3 {
		t.Errorf("top contributor = %+v, want src/ui/index.ts exclusive 3 subtree 3", e.Contributors)
	}
	for i := 1; i < len(e.Contributors); i++ {
		if e.Contributors[i-1].Exclusive < e.Contributors[i].Exclusive {
			t.Errorf("contributors not sorted by exclusive descending: %+v", e.Contributors)
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
	if r.Entries[0].Contributors != nil || r.Entries[1].Contributors != nil {
		t.Errorf("expected no contributor lists in multi-entry report")
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

func TestBuildReportExternalImports(t *testing.T) {
	g := graph(t, fstest.MapFS{
		"src/entry.ts":                         {Data: []byte("import 'acme'\nimport '@scope/one'\nimport './a'")},
		"src/a.ts":                             {Data: []byte("import 'acme'")},
		"node_modules/acme/package.json":       {Data: []byte(`{"main":"index.js"}`)},
		"node_modules/acme/index.js":           {Data: []byte("module.exports = 1")},
		"node_modules/@scope/one/package.json": {Data: []byte(`{"main":"index.js"}`)},
		"node_modules/@scope/one/index.js":     {Data: []byte("module.exports = 1")},
	}, "src/entry.ts")

	r := buildReport(g)
	want := []externalImport{
		{Specifier: "acme", Scope: "", Importers: 2},
		{Specifier: "@scope/one", Scope: "@scope", Importers: 1},
	}
	if !reflect.DeepEqual(r.Externals, want) {
		t.Errorf("externals = %+v, want %+v", r.Externals, want)
	}
}

func TestBuildReportSharedByAllExcludesEntries(t *testing.T) {
	// a.test imports b.test (also an entry) plus shared; b.test imports shared.
	// shared is reachable from every entry, b.test is too, but entry nodes are
	// never counted as "shared by all" — only shared qualifies.
	g := graph(t, fstest.MapFS{
		"src/a.test.ts": {Data: []byte("import './b.test'\nimport './shared'")},
		"src/b.test.ts": {Data: []byte("import './shared'")},
		"src/shared.ts": {Data: []byte("export const s = 1")},
	}, "src/a.test.ts", "src/b.test.ts")

	r := buildReport(g)
	if r.Union.SharedByAll != 1 {
		t.Errorf("sharedByAll = %d, want 1 (shared only, entry b.test excluded)", r.Union.SharedByAll)
	}
}

func TestPercentile(t *testing.T) {
	ascending := []int{10, 20, 30, 40, 50}
	tests := []struct {
		p    int
		want int
	}{
		{0, 10},
		{50, 30},
		{90, 40}, // (5-1)*90/100 = 3 -> element 40
		{100, 50},
	}
	for _, tt := range tests {
		if got := percentile(ascending, tt.p); got != tt.want {
			t.Errorf("percentile(%v, %d) = %d, want %d", ascending, tt.p, got, tt.want)
		}
	}
	if got := percentile([]int{7}, 90); got != 7 {
		t.Errorf("percentile single = %d, want 7", got)
	}
}

func TestWriteBarrelsTop(t *testing.T) {
	barrels := []barrelInfo{
		{Path: "a", Wasted: 3, Reexports: 2, UsedTargets: 1},
		{Path: "b", Wasted: 2, Reexports: 2, UsedTargets: 1},
		{Path: "c", Wasted: 1, Reexports: 2, UsedTargets: 1},
	}
	var buf bytes.Buffer
	writeBarrels(&buf, barrels, 2, style{})
	text := buf.String()
	if !strings.Contains(text, "\n  a\n") || !strings.Contains(text, "\n  b\n") {
		t.Errorf("expected top-2 barrels shown:\n%s", text)
	}
	if strings.Contains(text, "\n  c\n") {
		t.Errorf("expected barrel c truncated:\n%s", text)
	}
	if !strings.Contains(text, "1 more with waste") {
		t.Errorf("expected truncation notice:\n%s", text)
	}
}

func TestWriteBarrelsNoWaste(t *testing.T) {
	barrels := []barrelInfo{
		{Path: "a", Deps: 3},
		{Path: "b", Unprovable: true},
	}
	var buf bytes.Buffer
	writeBarrels(&buf, barrels, 20, style{})
	text := buf.String()
	if !strings.Contains(text, "0 files with 0 wasted imports") {
		t.Errorf("expected waste summary in header:\n%s", text)
	}
	if strings.Contains(text, "\n  a\n") || strings.Contains(text, "\n  b\n") {
		t.Errorf("non-wasteful barrels must not be listed:\n%s", text)
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

func TestRunVersion(t *testing.T) {
	for _, arg := range []string{"version", "-v", "--version"} {
		var out, errBuf bytes.Buffer
		if code := Run([]string{arg}, &out, &errBuf); code != 0 {
			t.Errorf("%s: exit code = %d, want 0", arg, code)
		}
		if strings.TrimSpace(out.String()) != Version {
			t.Errorf("%s: out = %q, want %q", arg, out.String(), Version)
		}
	}
}

func TestRunWhyChain(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	code := Run([]string{
		"why", "src/entry.ts", "src/d.ts",
		"--project", "fixtures/linear/tsconfig.json", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	var r whyReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	want := []string{"src/entry.ts", "src/b.ts", "src/c.ts", "src/d.ts"}
	if !r.Found || !reflect.DeepEqual(r.Chain, want) {
		t.Errorf("chain = %+v, want %v", r, want)
	}
}

func TestRunWhyNoPath(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	code := Run([]string{
		"why", "src/d.ts", "src/entry.ts",
		"--project", "fixtures/linear/tsconfig.json", "--json",
	}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr = %q", code, errBuf.String())
	}
	var r whyReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if r.Found || r.Chain != nil {
		t.Errorf("expected no path, got %+v", r)
	}
}

func TestRunWhyArgCount(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := Run([]string{"why", "only-one.ts"}, &out, &errBuf); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunScanFixtureJSON(t *testing.T) {
	t.Chdir("..") // run from repo root so fixtures resolve under os.DirFS(".")
	var out, errBuf bytes.Buffer
	// entry is relative to the project root (dir of --project).
	code := Run([]string{
		"scan", "src/entry.ts",
		"--project", "fixtures/barrel/tsconfig.json", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	var r scanReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(r.Entries) != 1 || r.Entries[0].Path != "src/entry.ts" || r.Entries[0].Modules == 0 {
		t.Errorf("unexpected report: %+v", r)
	}
	if len(r.Entries[0].Contributors) == 0 {
		t.Errorf("expected exclusive list for single entry, got %+v", r.Entries[0])
	}
}

func TestRunScanWasteFixtureJSON(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	code := Run([]string{
		"scan", "src/entry.ts",
		"--project", "fixtures/waste/tsconfig.json", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	var r scanReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	var barrel *barrelInfo
	for i := range r.Barrels {
		if r.Barrels[i].Path == "src/ui/index.ts" {
			barrel = &r.Barrels[i]
		}
	}
	if barrel == nil {
		t.Fatalf("expected barrel src/ui/index.ts, got %+v", r.Barrels)
	}
	if barrel.Reexports != 4 || barrel.UsedTargets != 1 || barrel.Wasted != 6 {
		t.Errorf("expected reexports=4 usedTargets=1 wasted=6, got %+v", barrel)
	}
	if len(barrel.WastedTargets) != 3 || barrel.WastedTargets[0] != "src/ui/gamma.ts" {
		t.Errorf("unexpected wastedTargets: %v", barrel.WastedTargets)
	}
}

func TestRunScanExternalsFixture(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	// --follow-externals walks into node_modules, which resolves against the
	// project root (fixtures/externals), so acme/leftpad count as externals.
	code := Run([]string{
		"scan", "src/entry.ts",
		"--project", "fixtures/externals/tsconfig.json",
		"--follow-externals", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	var r scanReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if r.Union.Externals != 2 {
		t.Errorf("externals = %d, want 2 (acme, leftpad); report %+v", r.Union.Externals, r)
	}
}

func TestRunScanAutoRoot(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	// no --project: depact roots at this repository (nearest .git), so the entry
	// key is repo-relative and its nearest tsconfig still governs resolution.
	code := Run([]string{"scan", "fixtures/barrel/src/entry.ts", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	var r scanReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(r.Entries) != 1 || r.Entries[0].Path != "fixtures/barrel/src/entry.ts" || r.Entries[0].Modules == 0 {
		t.Errorf("got %+v, want fixtures/barrel/src/entry.ts and >0 modules", r.Entries)
	}
}

func TestRunScanGlobSummary(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	// a quoted pattern reaches depact unexpanded; it expands against the
	// project root and the multi-entry run prints a summary.
	code := Run([]string{
		"scan", "src/**/*.test.ts",
		"--project", "fixtures/multi/tsconfig.json", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	var r scanReport
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
		if e.Contributors != nil {
			t.Errorf("expected no exclusive list in summary, got %+v", e)
		}
	}
}

func TestRunScanSummaryText(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	code := Run([]string{
		"scan", "src/**/*.test.ts",
		"--project", "fixtures/multi/tsconfig.json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	text := out.String()
	for _, want := range []string{"3 entries", "Heaviest entries", "closure size", "src/c.test.ts"} {
		if !strings.Contains(text, want) {
			t.Errorf("summary missing %q:\n%s", want, text)
		}
	}
}

func TestRunScanMissingEntry(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := Run([]string{"scan"}, &out, &errBuf); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunScanNegativeTop(t *testing.T) {
	t.Chdir("..")
	var out, errBuf bytes.Buffer
	code := Run([]string{
		"scan", "src/entry.ts",
		"--project", "fixtures/barrel/tsconfig.json", "--top", "-1",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
}

func TestRunScanDashDashEntry(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "tsconfig.json"), "{}")
	mustWrite(t, filepath.Join(dir, "-weird.ts"), "export const x = 1")
	t.Chdir(dir)

	var out, errBuf bytes.Buffer
	code := Run([]string{"scan", "--json", "--", "-weird.ts"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	var r scanReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(r.Entries) != 1 || r.Entries[0].Path != "-weird.ts" {
		t.Errorf("got %+v, want single entry -weird.ts", r.Entries)
	}
}
