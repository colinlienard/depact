package walker

import (
	"depact/parser"
	"depact/resolver"
)

type Graph struct {
	Entry   *Node
	Modules map[string]*Node
}

type Node struct {
	Module   *parser.Module
	External bool
	Edges    []Edge
	walked   bool
}

type Edge struct {
	Import *parser.Import
	Kind   resolver.ResolveKind
	To     *Node
}
