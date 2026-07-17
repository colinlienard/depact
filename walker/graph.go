package walker

import (
	"depact/parser"
	"depact/resolver"
)

type Graph struct {
	Entries  []*Node
	Modules  map[string]*Node
	Failures []Failure
}

type Node struct {
	Module   *parser.Module
	External bool
	Failed   bool
	Edges    []Edge
	walked   bool
}

type Failure struct {
	Path string
	Err  string
}

type Edge struct {
	Import *parser.Import
	Kind   resolver.ResolveKind
	To     *Node
}
