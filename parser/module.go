package parser

type EdgeType int

const (
	DefaultEdge EdgeType = iota
	NamedEdge
	NamespaceEdge
	SideEffectEdge
	DynamicEdge
)

type Import struct {
	Kind     EdgeType
	typeOnly bool
	from     string
	symbols  []string
}

type Export struct {
	Kind     EdgeType
	typeOnly bool
	symbols  []string
}

type Module struct {
	Path    string
	Imports []Import
	Exports []Export
}
