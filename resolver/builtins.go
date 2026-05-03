package resolver

import "strings"

// Run in node to see all builtins:
// require('node:module').builtinModules
var nodeBuiltins = map[string]struct{}{
	"assert":              {},
	"assert/strict":       {},
	"async_hooks":         {},
	"buffer":              {},
	"child_process":       {},
	"cluster":             {},
	"console":             {},
	"constants":           {},
	"crypto":              {},
	"dgram":               {},
	"diagnostics_channel": {},
	"dns":                 {},
	"dns/promises":        {},
	"domain":              {},
	"events":              {},
	"fs":                  {},
	"fs/promises":         {},
	"http":                {},
	"http2":               {},
	"https":               {},
	"inspector":           {},
	"inspector/promises":  {},
	"module":              {},
	"net":                 {},
	"os":                  {},
	"path":                {},
	"path/posix":          {},
	"path/win32":          {},
	"perf_hooks":          {},
	"process":             {},
	"punycode":            {},
	"querystring":         {},
	"readline":            {},
	"readline/promises":   {},
	"repl":                {},
	"stream":              {},
	"stream/consumers":    {},
	"stream/promises":     {},
	"stream/web":          {},
	"string_decoder":      {},
	"sys":                 {},
	"timers":              {},
	"timers/promises":     {},
	"tls":                 {},
	"trace_events":        {},
	"tty":                 {},
	"url":                 {},
	"util":                {},
	"util/types":          {},
	"v8":                  {},
	"vm":                  {},
	"wasi":                {},
	"worker_threads":      {},
	"zlib":                {},
}

var builtinSchemes = []string{
	"node:",
	"bun:",
}

var externalSchemes = []string{
	"jsr:",
	"npm:",
	"https://",
	"http://",
	"file://",
	"data:",
}

func isBuiltin(specifier string) bool {
	for _, s := range builtinSchemes {
		if strings.HasPrefix(specifier, s) {
			return true
		}
	}
	_, ok := nodeBuiltins[specifier]
	return ok
}

func isExternalScheme(specifier string) bool {
	for _, s := range externalSchemes {
		if strings.HasPrefix(specifier, s) {
			return true
		}
	}
	return false
}
