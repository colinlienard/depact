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
	IncludeAssets   bool
}

type state struct {
	walker *Walker
	graph  *Graph
	mu     sync.Mutex
	wg     sync.WaitGroup
	failMu sync.Mutex
}

func New(fsys fs.FS, r *resolver.Resolver) *Walker {
	return &Walker{fs: fsys, resolver: r}
}

func (w *Walker) Walk(entries ...string) (*Graph, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("walk: no entries")
	}
	s := &state{
		walker: w,
		graph:  &Graph{Modules: map[string]*Node{}},
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if seen[entry] {
			continue
		}
		seen[entry] = true
		s.graph.Entries = append(s.graph.Entries, s.visit(entry, resolver.Resolved{Path: entry}, true))
	}
	s.wg.Wait()
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
		s.scan(n)
	}()
	return n
}

func (s *state) scan(n *Node) {
	src, err := fs.ReadFile(s.walker.fs, n.Module.Path)
	if err != nil {
		s.recordFailure(n, err)
		return
	}
	mod, err := parser.Parse(src)
	if err != nil {
		s.recordFailure(n, err)
		return
	}
	mod.Path = n.Module.Path
	n.Module = mod

	for i := range mod.Imports {
		if err := s.link(n, &mod.Imports[i]); err != nil {
			s.recordFailure(n, err)
		}
	}
}

func (s *state) link(n *Node, imp *parser.Import) error {
	if s.walker.SkipTypeOnly && imp.TypeOnly() {
		return nil
	}
	res, err := s.walker.resolver.Resolve(n.Module.Path, imp.From)
	if err != nil {
		return err
	}
	if !s.walker.IncludeAssets && isAsset(res.Path) {
		return nil
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

func (s *state) recordFailure(n *Node, err error) {
	n.Failed = true
	s.failMu.Lock()
	s.graph.Failures = append(s.graph.Failures, Failure{Path: n.Module.Path, Err: err.Error()})
	s.failMu.Unlock()
}

var assetExtensions = map[string]bool{
	".css": true, ".scss": true, ".sass": true, ".less": true,
	".svg": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".avif": true, ".ico": true, ".bmp": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".otf": true,
	".mp4": true, ".webm": true, ".mp3": true, ".wav": true,
	".pdf": true, ".docx": true, ".xlsx": true, ".csv": true,
}

func isAsset(p string) bool {
	return assetExtensions[strings.ToLower(path.Ext(p))]
}
