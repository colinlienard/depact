package main

import (
	"os"
	"runtime/debug"

	"depact/cli"
)

func main() {
	// depact is a short-lived batch process whose heap is bounded by the
	// graph size, so collecting mid-walk is mostly wasted work. Back off the
	// GC unless the user pinned GOGC themselves.
	if _, ok := os.LookupEnv("GOGC"); !ok {
		debug.SetGCPercent(400)
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
