package cli

import (
	"flag"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"depact/project"
	"depact/walker"
)

const usage = `depact - dependency impact analyzer

usage:
  depact <command> [flags] <args>

commands:
  analyze <entry>...   report closure size, exclusive cost and barrels; with
                       several entries it prints a ranked summary instead.
                       Entries may be glob patterns ('src/**/*.test.{ts,tsx}',
                       quoted or not); node_modules and dot-dirs are pruned

flags (shared):
  --json               emit machine-readable JSON instead of text
  --type               include type-only imports (skipped by default) and
                       prefer the 'types' export condition over runtime
  --follow-externals   walk into node_modules dependencies
  --assets             count non-JS asset imports (css, images, fonts) as
                       modules; excluded by default
  --top <n>            show at most n rows in ranked lists (default 20)
  --project <path>     path to tsconfig.json; its directory becomes the
                       project root and entries are given relative to it.
                       When omitted, depact roots at the enclosing repository
                       (nearest .git) and uses the nearest tsconfig above
                       the first entry, so you can point it straight at files:
                       depact analyze path/to/index.ts
`

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "analyze":
		return runAnalyze(rest, stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", cmd, usage)
		return 2
	}
}

type commonFlags struct {
	json            bool
	typeImports     bool
	followExternals bool
	assets          bool
	project         string
}

func (c *commonFlags) bind(fs *flag.FlagSet) {
	fs.BoolVar(&c.json, "json", false, "emit JSON instead of text")
	fs.BoolVar(&c.typeImports, "type", false, "include type-only imports")
	fs.BoolVar(&c.followExternals, "follow-externals", false, "walk into node_modules")
	fs.BoolVar(&c.assets, "assets", false, "count non-JS asset imports (css, images, fonts) as modules")
	fs.StringVar(&c.project, "project", "", "path to tsconfig.json")
}

func (c *commonFlags) build(args []string) (*walker.Graph, error) {
	tgt, err := locate(c.project, args)
	if err != nil {
		return nil, err
	}
	fsys := os.DirFS(tgt.root)
	entries, err := expand(fsys, tgt.args)
	if err != nil {
		return nil, err
	}
	tsconfig := tgt.tsconfig
	if tsconfig == "" {
		tsconfig = findTsconfigIn(fsys, path.Dir(entries[0]))
	}
	p, err := project.Load(fsys, tsconfig)
	if err != nil {
		return nil, err
	}
	p.Walker.FollowExternals = c.followExternals
	p.Walker.SkipTypeOnly = !c.typeImports
	p.Walker.IncludeAssets = c.assets
	p.Resolver.IncludeTypes = c.typeImports

	return p.Walker.Walk(entries...)
}

var valueFlags = map[string]bool{"-project": true, "--project": true, "-top": true, "--top": true}

func permute(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
			if valueFlags[a] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positional = append(positional, a)
	}
	return append(append(flags, "--"), positional...)
}

type target struct {
	root     string
	tsconfig string
	args     []string
}

func locate(project string, args []string) (target, error) {
	if project != "" {
		return target{root: filepath.Dir(project), tsconfig: filepath.Base(project), args: args}, nil
	}

	root := findRoot(anchor(args))
	out := target{root: root}
	for _, a := range args {
		abs, err := filepath.Abs(a)
		if err != nil {
			return target{}, err
		}
		rel, err := relSlash(root, abs)
		if err != nil {
			return target{}, err
		}
		if rel == ".." || strings.HasPrefix(rel, "../") {
			return target{}, fmt.Errorf("%s is outside the project root %s", a, root)
		}
		out.args = append(out.args, rel)
	}
	return out, nil
}

func anchor(args []string) string {
	for _, a := range args {
		if isPattern(a) {
			prefix := literalPrefix(a)
			if prefix == "" {
				continue
			}
			if abs, err := filepath.Abs(prefix); err == nil {
				return abs
			}
			continue
		}
		if abs, err := filepath.Abs(a); err == nil {
			return filepath.Dir(abs)
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func literalPrefix(pattern string) string {
	segments := strings.Split(pattern, "/")
	var literal []string
	for _, s := range segments {
		if isPattern(s) {
			break
		}
		literal = append(literal, s)
	}
	return strings.Join(literal, "/")
}

func findRoot(dir string) string {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	if tsdir, ok := findTsconfig(dir); ok {
		return tsdir
	}
	return dir
}

func findTsconfigIn(fsys iofs.FS, dir string) string {
	for {
		cfg := path.Join(dir, "tsconfig.json")
		if _, err := iofs.Stat(fsys, cfg); err == nil {
			return cfg
		}
		if dir == "." {
			return ""
		}
		dir = path.Dir(dir)
	}
}

func findTsconfig(dir string) (string, bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func relSlash(base, target string) (string, error) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func normalizeEntry(entry string) string {
	key := path.Clean(entry)
	key = strings.TrimPrefix(key, "./")
	return key
}
