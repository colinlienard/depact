package parser

type Symbol struct {
	Name     string
	TypeOnly bool
}

type EdgeType int

const (
	DefaultEdge EdgeType = iota
	NamedEdge
	NamespaceEdge
	SideEffectEdge
	DynamicEdge
)

type Import struct {
	Kind    EdgeType
	From    string
	Symbols []Symbol
}

type Export struct {
	Kind    EdgeType
	Symbols []Symbol
}

type Module struct {
	Path    string
	Imports []Import
	Exports []Export
}

func (i Import) TypeOnly() bool {
	if len(i.Symbols) == 0 {
		return false
	}
	for _, s := range i.Symbols {
		if !s.TypeOnly {
			return false
		}
	}
	return true
}
