package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"depact/metrics"
	"depact/walker"
)

type scanReport struct {
	Entries   []entryReport    `json:"entries"`
	Union     unionStats       `json:"union"`
	Barrels   []barrelInfo     `json:"barrels"`
	Externals []externalImport `json:"externals,omitempty"`
	Failures  []failureInfo    `json:"failures,omitempty"`
}

type externalImport struct {
	Specifier string `json:"specifier"`
	Scope     string `json:"scope,omitempty"`
	Importers int    `json:"importers"`
}

type failureInfo struct {
	Path string `json:"path"`
	Err  string `json:"error"`
}

type entryReport struct {
	Path         string        `json:"path"`
	Modules      int           `json:"modules"`
	Externals    int           `json:"externals"`
	Contributors []contributor `json:"contributors,omitempty"`
	Closure      []string      `json:"closure,omitempty"`
}

type unionStats struct {
	Modules     int `json:"modules"`
	Externals   int `json:"externals"`
	SharedByAll int `json:"sharedByAll"`
}

type contributor struct {
	Path      string `json:"path"`
	Exclusive int    `json:"exclusive"`
	Subtree   int    `json:"subtree"`
}

type barrelInfo struct {
	Path          string   `json:"path"`
	Importers     int      `json:"importers"`
	Symbols       int      `json:"symbols"`
	Deps          int      `json:"deps"`
	Namespace     bool     `json:"namespace"`
	Reexports     int      `json:"reexports"`
	UsedTargets   int      `json:"usedTargets"`
	Wasted        int      `json:"wasted"`
	WastedTargets []string `json:"wastedTargets,omitempty"`
	Unprovable    bool     `json:"unprovable,omitempty"`
}

func runScan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var flags commonFlags
	flags.bind(fs)
	top := fs.Int("top", 20, "show at most n rows in ranked lists")
	closure := fs.Bool("closure", false, "include the full module closure per entry in JSON output")
	if err := fs.Parse(permute(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "scan requires at least one entry file")
		return 2
	}
	if *top < 0 {
		*top = 0
	}

	g, err := flags.build(fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "depact: %v\n", err)
		return 1
	}

	report := buildReport(g)
	if *closure && flags.json {
		addClosures(&report, g)
	}
	if flags.json {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "depact: %v\n", err)
			return 1
		}
		return 0
	}
	st := styleFor(stdout)
	if len(report.Entries) == 1 {
		writeDetail(stdout, report, *top, st)
	} else {
		writeSummary(stdout, report, *top, st)
	}
	return 0
}

func buildReport(g *walker.Graph) scanReport {
	entryNodes := map[*walker.Node]bool{}
	for _, e := range g.Entries {
		entryNodes[e] = true
	}

	shared := map[*walker.Node]int{}
	entries := make([]entryReport, 0, len(g.Entries))
	for _, e := range g.Entries {
		reach := metrics.Reach(e)
		er := entryReport{Path: e.Module.Path, Modules: len(reach) - 1}
		for n := range reach {
			if n.External {
				er.Externals++
			}
			if !entryNodes[n] {
				shared[n]++
			}
		}
		if len(g.Entries) == 1 {
			er.Contributors = contributorList(e)
		}
		entries = append(entries, er)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Modules != entries[j].Modules {
			return entries[i].Modules > entries[j].Modules
		}
		return entries[i].Path < entries[j].Path
	})

	sharedByAll := 0
	for _, count := range shared {
		if count == len(g.Entries) {
			sharedByAll++
		}
	}
	externals := 0
	for _, n := range g.Modules {
		if n.External {
			externals++
		}
	}

	failures := make([]failureInfo, 0, len(g.Failures))
	for _, f := range g.Failures {
		failures = append(failures, failureInfo{Path: f.Path, Err: f.Err})
	}
	sort.Slice(failures, func(i, j int) bool { return failures[i].Path < failures[j].Path })

	return scanReport{
		Entries:   entries,
		Union:     unionStats{Modules: len(g.Modules), Externals: externals, SharedByAll: sharedByAll},
		Barrels:   barrelList(g),
		Externals: externalList(g),
		Failures:  failures,
	}
}

func externalList(g *walker.Graph) []externalImport {
	imports := metrics.ExternalImports(g)
	out := make([]externalImport, 0, len(imports))
	for _, e := range imports {
		out = append(out, externalImport{Specifier: e.Specifier, Scope: e.Scope, Importers: e.Importers})
	}
	return out
}

func addClosures(r *scanReport, g *walker.Graph) {
	byPath := map[string][]string{}
	for _, e := range g.Entries {
		reach := metrics.Reach(e)
		paths := make([]string, 0, len(reach))
		for n := range reach {
			paths = append(paths, n.Module.Path)
		}
		sort.Strings(paths)
		byPath[e.Module.Path] = paths
	}
	for i := range r.Entries {
		r.Entries[i].Closure = byPath[r.Entries[i].Path]
	}
}

func writeDetail(w io.Writer, r scanReport, top int, st style) {
	e := r.Entries[0]
	fmt.Fprintf(w, "%s\n", st.bold(e.Path))
	fmt.Fprintf(w, "  %s modules  %s\n\n", st.num(e.Modules),
		st.dim(fmt.Sprintf("%d internal, %d external (not expanded, --follow-externals to walk them)", e.Modules-e.Externals, e.Externals)))

	fmt.Fprintf(w, "%s  %s\n", st.bold("Top contributors"),
		st.dim(fmt.Sprintf("heaviest of this file's %d imports (exclusive reclaimed / total subtree)", len(e.Contributors))))
	if len(e.Contributors) == 0 {
		fmt.Fprintln(w, st.dim("  (none)"))
	}
	shown := e.Contributors
	if len(shown) > top {
		shown = shown[:top]
	}
	for _, c := range shown {
		nums := fmt.Sprintf("%s / %s", st.cyan(fmt.Sprintf("%-4d", c.Exclusive)), st.dim(fmt.Sprintf("%-4d", c.Subtree)))
		fmt.Fprintf(w, "  %s  %s%s\n", nums, c.Path, ownedTag(c, st))
	}
	if len(e.Contributors) > len(shown) {
		fmt.Fprintf(w, "  %s\n", st.dim(fmt.Sprintf("... and %d more imports", len(e.Contributors)-len(shown))))
	}

	writeBarrels(w, r.Barrels, top, st)
	writeExternals(w, r.Externals, top, st)
	writeFailures(w, r.Failures, top, st)
}

func writeSummary(w io.Writer, r scanReport, top int, st style) {
	fmt.Fprintf(w, "%s entries, %s modules in union (%s shared by every entry)\n\n",
		st.num(len(r.Entries)), st.num(r.Union.Modules), st.num(r.Union.SharedByAll))

	sizes := make([]int, len(r.Entries))
	for i, e := range r.Entries {
		sizes[len(r.Entries)-1-i] = e.Modules
	}
	fmt.Fprintf(w, "closure size   min %s   median %s   p90 %s   max %s\n",
		st.num(sizes[0]), st.num(percentile(sizes, 50)), st.num(percentile(sizes, 90)), st.num(sizes[len(sizes)-1]))

	fmt.Fprintf(w, "\n%s\n", st.bold("Heaviest entries"))
	shown := r.Entries
	if len(shown) > top {
		shown = shown[:top]
	}
	for _, e := range shown {
		fmt.Fprintf(w, "  %s  %s\n", st.cyan(fmt.Sprintf("%-5d", e.Modules)), e.Path)
	}
	if len(r.Entries) > len(shown) {
		fmt.Fprintf(w, "  %s\n", st.dim(fmt.Sprintf("... and %d more (--top to adjust)", len(r.Entries)-len(shown))))
	}

	writeBarrels(w, r.Barrels, top, st)
	writeExternals(w, r.Externals, top, st)
	writeFailures(w, r.Failures, top, st)
}

func writeExternals(w io.Writer, exts []externalImport, top int, st style) {
	if len(exts) == 0 {
		return
	}
	scopeOrder := make([]string, 0)
	scopeCount := map[string]int{}
	for _, e := range exts {
		if scopeCount[e.Scope] == 0 {
			scopeOrder = append(scopeOrder, e.Scope)
		}
		scopeCount[e.Scope]++
	}
	sort.SliceStable(scopeOrder, func(i, j int) bool {
		return scopeCount[scopeOrder[i]] > scopeCount[scopeOrder[j]]
	})

	fmt.Fprintf(w, "\n%s  %s\n", st.bold("External footprint"),
		st.dim(fmt.Sprintf("%d packages across %d scopes", len(exts), len(scopeCount))))
	shown := exts
	if len(shown) > top {
		shown = shown[:top]
	}
	for _, e := range shown {
		fmt.Fprintf(w, "  %s  %s\n", st.cyan(fmt.Sprintf("%-4s", fmt.Sprintf("%d×", e.Importers))), e.Specifier)
	}
	if len(exts) > len(shown) {
		fmt.Fprintf(w, "  %s\n", st.dim(fmt.Sprintf("... and %d more packages", len(exts)-len(shown))))
	}

	parts := make([]string, len(scopeOrder))
	for i, s := range scopeOrder {
		parts[i] = fmt.Sprintf("%s %d", scopeLabel(s), scopeCount[s])
	}
	fmt.Fprintf(w, "  %s %s\n", "Scopes", st.dim(strings.Join(parts, ", ")))
}

func scopeLabel(scope string) string {
	if scope == "" {
		return "unscoped"
	}
	return scope
}

func writeFailures(w io.Writer, failures []failureInfo, top int, st style) {
	if len(failures) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", st.red(fmt.Sprintf("Unreadable (%d, skipped)", len(failures))))
	shown := failures
	if len(shown) > top {
		shown = shown[:top]
	}
	for _, f := range shown {
		fmt.Fprintf(w, "  %s: %s\n", f.Path, st.dim(f.Err))
	}
	if len(failures) > len(shown) {
		fmt.Fprintf(w, "  %s\n", st.dim(fmt.Sprintf("... and %d more", len(failures)-len(shown))))
	}
}

func writeBarrels(w io.Writer, barrels []barrelInfo, top int, st style) {
	var wasteful []barrelInfo
	totalWasted := 0
	for _, b := range barrels {
		if b.Wasted > 0 {
			wasteful = append(wasteful, b)
			totalWasted += b.Wasted
		}
	}

	fmt.Fprintf(w, "\n%s  %s\n", st.bold("Barrels"),
		st.dim(fmt.Sprintf("%d files with %d wasted imports", len(wasteful), totalWasted)))
	if len(barrels) == 0 {
		fmt.Fprintln(w, st.dim("  (none)"))
		return
	}

	shown := wasteful
	if len(shown) > top {
		shown = shown[:top]
	}
	for _, b := range shown {
		fmt.Fprintf(w, "  %s\n", b.Path)
		tail := fmt.Sprintf("%d/%d unused re-exports", b.Reexports-b.UsedTargets, b.Reexports)
		if len(b.WastedTargets) > 0 {
			tail += " → " + joinTargets(b.WastedTargets, top)
		}
		fmt.Fprintf(w, "    %s / %s modules wasted, %s\n",
			st.cyan(fmt.Sprintf("%d", b.Wasted)), st.dim(fmt.Sprintf("%d", b.Deps)), st.dim(tail))
	}

	if len(wasteful) > len(shown) {
		fmt.Fprintf(w, "  %s\n", st.dim(fmt.Sprintf("... and %d more with waste", len(wasteful)-len(shown))))
	}
}

func joinTargets(targets []string, max int) string {
	shown := targets
	if len(shown) > max {
		shown = shown[:max]
	}
	tails := make([]string, len(shown))
	for i, t := range shown {
		tails[i] = tailPath(t)
	}
	out := strings.Join(tails, ", ")
	if len(targets) > len(shown) {
		out += fmt.Sprintf(", +%d", len(targets)-len(shown))
	}
	return out
}

func tailPath(p string) string {
	segs := strings.Split(p, "/")
	if len(segs) <= 2 {
		return p
	}
	return strings.Join(segs[len(segs)-2:], "/")
}

func ownedTag(c contributor, st style) string {
	if c.Subtree >= 5 && c.Exclusive*10 >= c.Subtree*7 {
		return "  " + st.dim("owned")
	}
	return ""
}

func contributorList(entry *walker.Node) []contributor {
	contributors := metrics.Contributors(entry)
	out := make([]contributor, 0, len(contributors))
	for _, c := range contributors {
		out = append(out, contributor{Path: c.Path, Exclusive: c.Exclusive, Subtree: c.Subtree})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Exclusive != out[j].Exclusive {
			return out[i].Exclusive > out[j].Exclusive
		}
		if out[i].Subtree != out[j].Subtree {
			return out[i].Subtree > out[j].Subtree
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func barrelList(g *walker.Graph) []barrelInfo {
	out := make([]barrelInfo, 0)
	for p, b := range metrics.Barrels(g) {
		out = append(out, barrelInfo{
			Path:          p,
			Importers:     b.Importers,
			Symbols:       b.Symbols,
			Deps:          b.Deps,
			Namespace:     b.Namespace,
			Reexports:     b.Reexports,
			UsedTargets:   b.UsedTargets,
			Wasted:        b.Wasted,
			WastedTargets: b.WastedTargets,
			Unprovable:    b.Unprovable,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Wasted != out[j].Wasted {
			return out[i].Wasted > out[j].Wasted
		}
		if out[i].Deps != out[j].Deps {
			return out[i].Deps > out[j].Deps
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func percentile(ascending []int, p int) int {
	return ascending[(len(ascending)-1)*p/100]
}
