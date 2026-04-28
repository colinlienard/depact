package parser

type SymbolKind int

const (
	DefaultSym SymbolKind = iota
	NamedSym
	NamespaceSym
)

type Symbol struct {
	Kind     SymbolKind
	Name     string
	Alias    string
	TypeOnly bool
}

type Import struct {
	From    string
	Symbols []Symbol // empty = side-effect
	Dynamic bool
}

type Export struct {
	From    string
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
