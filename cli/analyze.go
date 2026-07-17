package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"

	"depact/metrics"
	"depact/walker"
)

type analyzeReport struct {
	Entries  []entryReport `json:"entries"`
	Union    unionStats    `json:"union"`
	Barrels  []barrelInfo  `json:"barrels"`
	Failures []failureInfo `json:"failures,omitempty"`
}

type failureInfo struct {
	Path string `json:"path"`
	Err  string `json:"error"`
}

type entryReport struct {
	Path      string        `json:"path"`
	Modules   int           `json:"modules"`
	Externals int           `json:"externals"`
	Exclusive []contributor `json:"exclusive,omitempty"`
	Closure   []string      `json:"closure,omitempty"`
}

type unionStats struct {
	Modules     int `json:"modules"`
	Externals   int `json:"externals"`
	SharedByAll int `json:"sharedByAll"`
}

type contributor struct {
	Path string `json:"path"`
	Cost int    `json:"cost"`
}

type barrelInfo struct {
	Path      string `json:"path"`
	Importers int    `json:"importers"`
	Symbols   int    `json:"symbols"`
	Deps      int    `json:"deps"`
	Namespace bool   `json:"namespace"`
}

func runAnalyze(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var flags commonFlags
	flags.bind(fs)
	top := fs.Int("top", 20, "show at most n rows in ranked lists")
	closure := fs.Bool("closure", false, "include the full module closure per entry in JSON output")
	if err := fs.Parse(permute(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "analyze requires at least one entry file")
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
	if *closure {
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
	if len(report.Entries) == 1 {
		writeDetail(stdout, report, *top)
	} else {
		writeSummary(stdout, report, *top)
	}
	return 0
}

func buildReport(g *walker.Graph) analyzeReport {
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
			er.Exclusive = exclusiveList(g, e)
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

	return analyzeReport{
		Entries:  entries,
		Union:    unionStats{Modules: len(g.Modules), Externals: externals, SharedByAll: sharedByAll},
		Barrels:  barrelList(g),
		Failures: failures,
	}
}

func addClosures(r *analyzeReport, g *walker.Graph) {
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

func writeDetail(w io.Writer, r analyzeReport, top int) {
	e := r.Entries[0]
	fmt.Fprintf(w, "%s\n", e.Path)
	fmt.Fprintf(w, "  %d modules, %d external\n\n", e.Modules, e.Externals)

	fmt.Fprintln(w, "exclusive cost")
	if len(e.Exclusive) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	shown := e.Exclusive
	if len(shown) > top {
		shown = shown[:top]
	}
	for _, c := range shown {
		fmt.Fprintf(w, "  %5d  %s\n", c.Cost, c.Path)
	}
	if len(e.Exclusive) > len(shown) {
		fmt.Fprintf(w, "  ... and %d more\n", len(e.Exclusive)-len(shown))
	}

	writeBarrels(w, r.Barrels, top)
	writeFailures(w, r.Failures, top)
}

func writeSummary(w io.Writer, r analyzeReport, top int) {
	fmt.Fprintf(w, "%d entries, %d modules in union (%d shared by every entry)\n\n",
		len(r.Entries), r.Union.Modules, r.Union.SharedByAll)

	sizes := make([]int, len(r.Entries))
	for i, e := range r.Entries {
		sizes[len(r.Entries)-1-i] = e.Modules
	}
	fmt.Fprintf(w, "closure size   min %d   median %d   p90 %d   max %d\n",
		sizes[0], percentile(sizes, 50), percentile(sizes, 90), sizes[len(sizes)-1])

	fmt.Fprintln(w, "\nheaviest entries")
	shown := r.Entries
	if len(shown) > top {
		shown = shown[:top]
	}
	for _, e := range shown {
		fmt.Fprintf(w, "  %5d  %s\n", e.Modules, e.Path)
	}
	if len(r.Entries) > len(shown) {
		fmt.Fprintf(w, "  ... and %d more (--top to adjust)\n", len(r.Entries)-len(shown))
	}

	writeBarrels(w, r.Barrels, top)
	writeFailures(w, r.Failures, top)
}

func writeFailures(w io.Writer, failures []failureInfo, top int) {
	if len(failures) == 0 {
		return
	}
	fmt.Fprintf(w, "\nunreadable (%d, skipped)\n", len(failures))
	shown := failures
	if len(shown) > top {
		shown = shown[:top]
	}
	for _, f := range shown {
		fmt.Fprintf(w, "  %s: %s\n", f.Path, f.Err)
	}
	if len(failures) > len(shown) {
		fmt.Fprintf(w, "  ... and %d more\n", len(failures)-len(shown))
	}
}

func writeBarrels(w io.Writer, barrels []barrelInfo, top int) {
	fmt.Fprintln(w, "\nbarrels")
	if len(barrels) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	shown := barrels
	if len(shown) > top {
		shown = shown[:top]
	}
	for _, b := range shown {
		ns := ""
		if b.Namespace {
			ns = "  namespace"
		}
		fmt.Fprintf(w, "  %s\n    importers %d  symbols %d  deps %d%s\n", b.Path, b.Importers, b.Symbols, b.Deps, ns)
	}
	if len(barrels) > len(shown) {
		fmt.Fprintf(w, "  ... and %d more\n", len(barrels)-len(shown))
	}
}

func exclusiveList(g *walker.Graph, entry *walker.Node) []contributor {
	exclusive := metrics.Exclusive(g, entry)
	out := make([]contributor, 0, len(exclusive))
	for p, cost := range exclusive {
		out = append(out, contributor{Path: p, Cost: cost})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cost != out[j].Cost {
			return out[i].Cost > out[j].Cost
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func barrelList(g *walker.Graph) []barrelInfo {
	out := make([]barrelInfo, 0)
	for p, b := range metrics.Barrels(g) {
		out = append(out, barrelInfo{
			Path:      p,
			Importers: b.Importers,
			Symbols:   b.Symbols,
			Deps:      b.Deps,
			Namespace: b.Namespace,
		})
	}
	sort.Slice(out, func(i, j int) bool {
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
