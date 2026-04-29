package resolver

import "depact/parser"

type Graph struct {
	Entry   *Node
	Modules map[string]*Node
}

type Node struct {
	Module   *parser.Module
	External bool
	Edges    []Edge
}

type Edge struct {
	Import *parser.Import
	To     *Node
}
