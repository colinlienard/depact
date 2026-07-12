package resolver

import (
	"errors"
	"io/fs"
	"path"
	"strings"
	"sync"
)

type Resolver struct {
	fs    fs.FS
	paths map[string][]string // tsconfig paths
	mu    sync.Mutex
	cache map[string]Resolved
}

type ResolveKind int

const (
	ResolveKindFile ResolveKind = iota
	ResolveKindIndex
	ResolveKindPackage
	ResolveKindBuiltin
	ResolveKindExternal
	ResolveKindUnresolved
)

type Resolved struct {
	Path     string
	External bool
	Kind     ResolveKind
}

func New(fsys fs.FS, paths map[string][]string) *Resolver {
	return &Resolver{
		fs:    fsys,
		paths: paths,
		cache: map[string]Resolved{},
	}
}

func (r *Resolver) Resolve(from, specifier string) (Resolved, error) {
	key := from + "\x00" + specifier

	r.mu.Lock()
	if res, ok := r.cache[key]; ok {
		r.mu.Unlock()
		return res, nil
	}
	r.mu.Unlock()

	res, err := r.resolve(from, specifier)
	if err != nil {
		return res, err
	}

	r.mu.Lock()
	r.cache[key] = res
	r.mu.Unlock()

	return res, nil
}

func (r *Resolver) resolve(from, specifier string) (Resolved, error) {
	if strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") {
		p := path.Join(path.Dir(from), specifier)
		if res, ok := r.resolveFile(p); ok {
			return res, nil
		}
		return Resolved{Kind: ResolveKindUnresolved}, nil
	}

	if res, ok := r.resolvePaths(specifier); ok {
		return res, nil
	}

	if strings.HasPrefix(specifier, "#") {
		p, err := r.resolvePkgImports(from, specifier)
		switch {
		case err == nil:
			return Resolved{Path: p, Kind: ResolveKindFile}, nil
		case errors.Is(err, ErrPkgNotFound):
			return Resolved{Kind: ResolveKindUnresolved}, nil
		default:
			return Resolved{}, err
		}
	}

	if isBuiltin(specifier) {
		return Resolved{Kind: ResolveKindBuiltin}, nil
	}

	if isExternalScheme(specifier) {
		return Resolved{Kind: ResolveKindExternal, External: true}, nil
	}

	p, err := r.resolvePkgEntry(specifier)
	if err == nil {
		return Resolved{Path: p, Kind: ResolveKindPackage, External: true}, nil
	}
	if !errors.Is(err, ErrPkgNotFound) && !errors.Is(err, ErrPkgNoEntries) {
		return Resolved{}, err
	}

	return Resolved{Kind: ResolveKindUnresolved}, nil
}

func (r *Resolver) resolveFile(p string) (Resolved, bool) {
	if stat, err := r.stat(p); err == nil {
		if stat.IsDir() {
			indexPath, found := r.find(path.Join(p, "index"))
			if !found {
				return Resolved{}, false
			}
			return Resolved{Path: indexPath, Kind: ResolveKindIndex}, true
		}
		return Resolved{Path: p}, true
	}
	if name, found := r.find(p); found {
		return Resolved{Path: name}, true
	}
	return Resolved{}, false
}

func (r *Resolver) resolvePaths(specifier string) (Resolved, bool) {
	subs, match, ok := matchPaths(r.paths, specifier)
	if !ok {
		return Resolved{}, false
	}
	for _, sub := range subs {
		candidate := strings.Replace(sub, "*", match, 1)
		if res, ok := r.resolveFile(candidate); ok {
			return res, true
		}
	}
	return Resolved{}, false
}

func matchPaths(paths map[string][]string, specifier string) ([]string, string, bool) {
	var bestSubs []string
	var bestMatch string
	bestPrefix := -1
	for pattern, subs := range paths {
		prefix, suffix, wildcard := strings.Cut(pattern, "*")
		if !wildcard {
			if pattern == specifier && len(pattern) > bestPrefix {
				bestSubs, bestMatch, bestPrefix = subs, "", len(pattern)
			}
			continue
		}
		if len(specifier) < len(prefix)+len(suffix) {
			continue
		}
		if !strings.HasPrefix(specifier, prefix) || !strings.HasSuffix(specifier, suffix) {
			continue
		}
		if len(prefix) <= bestPrefix {
			continue
		}
		bestSubs = subs
		bestMatch = specifier[len(prefix) : len(specifier)-len(suffix)]
		bestPrefix = len(prefix)
	}
	if bestPrefix < 0 {
		return nil, "", false
	}
	return bestSubs, bestMatch, true
}

var extensions = []string{".ts", ".tsx", ".d.ts", ".js", ".jsx"}

func (r *Resolver) find(name string) (string, bool) {
	for _, ext := range extensions {
		if r.exists(name + ext) {
			return name + ext, true
		}
	}
	return "", false
}

func (r *Resolver) stat(name string) (fs.FileInfo, error) {
	return fs.Stat(r.fs, name)
}

func (r *Resolver) exists(name string) bool {
	_, err := r.stat(name)
	return err == nil
}
