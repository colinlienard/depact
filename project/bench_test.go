package project

import (
	"os"
	"testing"
)

// BenchmarkWalk walks a project entry. By default it runs against the
// repository's barrel fixture; set BENCH_DIR and BENCH_ENTRY to point it at a
// real codebase (e.g. a monorepo) to measure parallel scaling under -cpu.
func BenchmarkWalk(b *testing.B) {
	dir := os.Getenv("BENCH_DIR")
	entry := os.Getenv("BENCH_ENTRY")
	if dir == "" {
		dir, entry = "../fixtures/barrel", "src/entry.ts"
	}
	b.ReportAllocs()
	for b.Loop() {
		p, err := Open(dir, "")
		if err != nil {
			b.Fatal(err)
		}
		if _, err := p.Walker.Walk(entry); err != nil {
			b.Fatal(err)
		}
	}
}
