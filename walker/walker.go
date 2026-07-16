package walker

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"

	"depact/parser"
	"depact/resolver"
)

type Walker struct {
	fs       fs.FS
	resolver *resolver.Resolver

	FollowExternals bool
	SkipTypeOnly    bool
}

type state struct {
	walker *Walker
	graph  *Graph
	mu     sync.Mutex
	wg     sync.WaitGroup
	errMu  sync.Mutex
	err    error
}

func New(fsys fs.FS, r *resolver.Resolver) *Walker {
	return &Walker{fs: fsys, resolver: r}
}

func (w *Walker) Walk(entry string) (*Graph, error) {
	s := &state{
		walker: w,
		graph:  &Graph{Modules: map[string]*Node{}},
	}
	s.graph.Entry = s.visit(entry, resolver.Resolved{Path: entry}, true)
	s.wg.Wait()
	if s.err != nil {
		return nil, s.err
	}
	return s.graph, nil
}

func (s *state) visit(key string, res resolver.Resolved, walkable bool) *Node {
	s.mu.Lock()
	n, ok := s.graph.Modules[key]
	if !ok {
		n = &Node{Module: &parser.Module{Path: key}, External: res.External}
		s.graph.Modules[key] = n
	}
	if !res.External {
		n.External = false
	}
	scan := walkable && !n.walked
	if scan {
		n.walked = true
	}
	s.mu.Unlock()

	if !scan {
		return n
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.scan(n); err != nil {
			s.fail(err)
		}
	}()
	return n
}

func (s *state) scan(n *Node) error {
	src, err := fs.ReadFile(s.walker.fs, n.Module.Path)
	if err != nil {
		return err
	}
	mod, err := parser.Parse(src)
	if err != nil {
		return fmt.Errorf("%s: %w", n.Module.Path, err)
	}
	mod.Path = n.Module.Path
	n.Module = mod

	for i := range mod.Imports {
		if err := s.link(n, &mod.Imports[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *state) link(n *Node, imp *parser.Import) error {
	if s.walker.SkipTypeOnly && imp.TypeOnly() {
		return nil
	}
	res, err := s.walker.resolver.Resolve(n.Module.Path, imp.From)
	if err != nil {
		return err
	}
	key := res.Path
	if key == "" {
		key = imp.From
		if strings.HasPrefix(key, "./") || strings.HasPrefix(key, "../") {
			key = path.Join(path.Dir(n.Module.Path), key)
		}
	}
	walkable := res.Path != "" && (!res.External || s.walker.FollowExternals)
	to := s.visit(key, res, walkable)
	n.Edges = append(n.Edges, Edge{Import: imp, Kind: res.Kind, To: to})
	return nil
}

func (s *state) fail(err error) {
	s.errMu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.errMu.Unlock()
}
