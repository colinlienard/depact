package parser

func Parse(src []byte) *Module {
	scanner := &scanner{src: src}
	imports, exports, _ := scanner.scan()
	return &Module{Imports: imports, Exports: exports}
}
