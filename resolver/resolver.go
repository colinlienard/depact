package resolver

import (
	"errors"
	"io/fs"
	"path"
	"strings"
)

type Resolver struct {
	fs    fs.FS
	root  string
	paths map[string][]string // tsconfig paths
	cache map[string]Resolved
}

type ResolveKind int

const (
	ResolveKindFile ResolveKind = iota
	ResolveKindIndex
	ResolveKindPackage
	ResolveKindBuiltin
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
	if strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") {
		p := path.Join(path.Dir(from), specifier)

		if stat, err := r.stat(p); err == nil {
			if stat.IsDir() {
				indexPath, found := r.find(path.Join(p, "index"))
				if !found {
					return Resolved{Kind: ResolveKindUnresolved}, nil
				}
				return Resolved{Path: indexPath, Kind: ResolveKindIndex}, nil
			}
			return Resolved{Path: p}, nil
		}

		p, found := r.find(p)
		if !found {
			return Resolved{Kind: ResolveKindUnresolved}, nil
		}
		return Resolved{Path: p}, nil
	}

	if isBuiltin(specifier) {
		return Resolved{Kind: ResolveKindBuiltin}, nil
	}

	p, err := r.resolvePkgEntry(specifier)
	switch {
	case err == nil:
		return Resolved{Path: p, Kind: ResolveKindPackage, External: true}, nil
	case errors.Is(err, ErrPkgNotFound):
	case errors.Is(err, ErrPkgNoEntries):
	default:
		return Resolved{}, err
	}

	return Resolved{Kind: ResolveKindUnresolved}, nil
}

var extensions = []string{".ts", ".tsx", ".js", ".jsx"}

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
