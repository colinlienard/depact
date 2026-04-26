package parser

func Parse(src []byte) *Module {
	scanner := &scanner{src: src}
	imports, exports := scanner.scan() // TODO: handle errors
	return &Module{Imports: imports, Exports: exports}
}
