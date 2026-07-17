package resolver

import (
	"errors"
	"io/fs"
	"path"
	"strings"
	"sync"
)

const numShards = 256

type cacheKey struct {
	from      string
	specifier string
}

type cacheShard struct {
	mu sync.Mutex
	m  map[cacheKey]Resolved
}

type Resolver struct {
	fs    fs.FS
	paths map[string][]string // tsconfig paths

	cache [numShards]cacheShard

	pkgMu    sync.Mutex
	pkgEntry map[string]pkgEntryResult // specifier -> resolved package entry

	pkgFileMu sync.Mutex
	pkgFiles  map[string]pkgFileResult // package.json path -> parsed contents

	dirMu sync.Mutex
	dirs  map[string]map[string]bool // dir -> set of non-dir entry names

	linkMu sync.Mutex
	links  map[string]bool

	IncludeTypes bool
}

type pkgEntryResult struct {
	path string
	err  error
}

type pkgFileResult struct {
	pkg *PkgJSON
	err error
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
	r := &Resolver{
		fs:       fsys,
		paths:    paths,
		links:    map[string]bool{},
		pkgEntry: map[string]pkgEntryResult{},
		pkgFiles: map[string]pkgFileResult{},
		dirs:     map[string]map[string]bool{},
	}
	for i := range r.cache {
		r.cache[i].m = map[cacheKey]Resolved{}
	}
	return r
}

func (r *Resolver) Resolve(from, specifier string) (Resolved, error) {
	key := cacheKey{from: from, specifier: specifier}
	sh := &r.cache[(fnv1a(from)^fnv1a(specifier))%numShards]

	sh.mu.Lock()
	if res, ok := sh.m[key]; ok {
		sh.mu.Unlock()
		return res, nil
	}
	sh.mu.Unlock()

	res, err := r.resolve(from, specifier)
	if err != nil {
		return res, err
	}

	sh.mu.Lock()
	sh.m[key] = res
	sh.mu.Unlock()

	return res, nil
}

func fnv1a(s string) uint32 {
	const (
		offset = 2166136261
		prime  = 16777619
	)
	h := uint32(offset)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime
	}
	return h
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
		external := !r.isWorkspaceLink(pkgName(specifier))
		return Resolved{Path: p, Kind: pkgKind(p, external), External: external}, nil
	}
	if !errors.Is(err, ErrPkgNotFound) && !errors.Is(err, ErrPkgNoEntries) {
		return Resolved{}, err
	}

	return Resolved{Kind: ResolveKindUnresolved}, nil
}

func (r *Resolver) resolveFile(p string) (Resolved, bool) {
	entries := r.dirEntries(path.Dir(p))
	base := path.Base(p)
	if entries[base] {
		return Resolved{Path: p}, true
	}
	for _, ext := range extensions {
		if entries[base+ext] {
			return Resolved{Path: p + ext}, true
		}
	}
	index := r.dirEntries(p)
	for _, ext := range extensions {
		if index["index"+ext] {
			return Resolved{Path: path.Join(p, "index"+ext), Kind: ResolveKindIndex}, true
		}
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

var extensions = []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs", ".d.ts"}

func pkgKind(p string, external bool) ResolveKind {
	if !external && isIndexFile(p) {
		return ResolveKindIndex
	}
	return ResolveKindPackage
}

func isIndexFile(p string) bool {
	base := path.Base(p)
	for _, ext := range extensions {
		if base == "index"+ext {
			return true
		}
	}
	return false
}

func (r *Resolver) dirEntries(dir string) map[string]bool {
	r.dirMu.Lock()
	if entries, ok := r.dirs[dir]; ok {
		r.dirMu.Unlock()
		return entries
	}
	r.dirMu.Unlock()

	entries := map[string]bool{}
	if list, err := fs.ReadDir(r.fs, dir); err == nil {
		for _, e := range list {
			if e.IsDir() {
				continue
			}
			if e.Type()&fs.ModeSymlink != 0 {
				if info, err := fs.Stat(r.fs, path.Join(dir, e.Name())); err != nil || info.IsDir() {
					continue
				}
			}
			entries[e.Name()] = true
		}
	}

	r.dirMu.Lock()
	r.dirs[dir] = entries
	r.dirMu.Unlock()
	return entries
}

func (r *Resolver) exists(name string) bool {
	return r.dirEntries(path.Dir(name))[path.Base(name)]
}
