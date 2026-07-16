package cli

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func isPattern(arg string) bool {
	return strings.ContainsAny(arg, "*?[{")
}

func expand(fsys fs.FS, args []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	add := func(key string) {
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}

	for _, arg := range args {
		if !isPattern(arg) {
			add(normalizeEntry(arg))
			continue
		}
		pattern := normalizeEntry(arg)
		if !doublestar.ValidatePattern(pattern) {
			return nil, fmt.Errorf("invalid pattern %q", arg)
		}
		matches, err := match(fsys, pattern)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no files match %q", arg)
		}
		sort.Strings(matches)
		for _, m := range matches {
			add(m)
		}
	}
	return out, nil
}

func match(fsys fs.FS, pattern string) ([]string, error) {
	var matches []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && pruned(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if ok, err := doublestar.Match(pattern, p); err != nil {
			return err
		} else if ok {
			matches = append(matches, p)
		}
		return nil
	})
	return matches, err
}

func pruned(name string) bool {
	return name == "node_modules" || strings.HasPrefix(name, ".")
}
