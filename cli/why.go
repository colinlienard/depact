package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"depact/metrics"
)

type whyReport struct {
	Entry  string   `json:"entry"`
	Target string   `json:"target"`
	Chain  []string `json:"chain"`
	Found  bool     `json:"found"`
}

func runWhy(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("why", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var flags commonFlags
	flags.bind(fs)
	if err := fs.Parse(permute(args)); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "why requires an entry and a target file")
		return 2
	}

	fsys, tsconfig, norm, err := flags.prepare(fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "depact: %v\n", err)
		return 1
	}
	p, err := flags.load(fsys, tsconfig)
	if err != nil {
		fmt.Fprintf(stderr, "depact: %v\n", err)
		return 1
	}
	g, err := p.Walker.Walk(norm[0])
	if err != nil {
		fmt.Fprintf(stderr, "depact: %v\n", err)
		return 1
	}

	entry, target := norm[0], norm[1]
	chain := metrics.Why(g, g.Entries[0], target)
	report := whyReport{Entry: entry, Target: target, Found: chain != nil}
	for _, n := range chain {
		report.Chain = append(report.Chain, n.Module.Path)
	}

	if flags.json {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "depact: %v\n", err)
			return 1
		}
		if !report.Found {
			return 1
		}
		return 0
	}

	st := styleFor(stdout)
	if !report.Found {
		fmt.Fprintf(stdout, "no import path from %s to %s\n", entry, target)
		return 1
	}
	writeChain(stdout, report.Chain, st)
	return 0
}

func writeChain(w io.Writer, chain []string, st style) {
	fmt.Fprintf(w, "%s\n", st.bold(chain[0]))
	for _, p := range chain[1:] {
		fmt.Fprintf(w, "%s %s\n", st.dim("→"), p)
	}
	fmt.Fprintf(w, "\n%s\n", st.dim(fmt.Sprintf("%d hops", len(chain)-1)))
}
