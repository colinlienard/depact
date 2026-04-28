package parser

func Parse(src []byte) (*Module, error) {
	scanner := &scanner{src: src}
	imports, exports, err := scanner.scan()
	if err != nil {
		return nil, err
	}
	return &Module{Imports: imports, Exports: exports}, nil
}
