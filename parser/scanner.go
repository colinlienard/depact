package parser

import (
	"fmt"
)

var (
	importKeyword = []byte("import")
	exportKeyword = []byte("export")
	fromKeyword   = []byte("from")
	asKeyword     = []byte("as")
)

type state int

const (
	code         state = iota
	lineComment        // //
	blockComment       // /* */
	stringDouble       // "..."
	stringSingle       // '...'
	template           // `...`
)

type frame struct {
	state state
	depth int // brace depth, used only for code frames pushed by `${`
}

type scanner struct {
	src     []byte
	stack   []frame
	i       int
	imports []Import
	exports []Export
}

func (s *scanner) push(st state) {
	s.stack = append(s.stack, frame{state: st})
}

func (s *scanner) pop() {
	s.stack = s.stack[:len(s.stack)-1]
}

func (s *scanner) scan() ([]Import, []Export, error) {
	if len(s.stack) == 0 {
		s.push(code)
	}

	for s.i < len(s.src) {
		char := s.peek()
		top := &s.stack[len(s.stack)-1]

		switch top.state {
		case code:
			switch {
			case char == 'i':
				if !s.isWord(importKeyword) {
					break
				}
				if err := s.parseImport(); err != nil {
					return nil, nil, err
				}

			case char == 'e':
				if !s.isWord(exportKeyword) {
					break
				}
				if err := s.parseExport(); err != nil {
					return nil, nil, err
				}

			case char == '/' && s.i+1 < len(s.src) && s.peekAt(1) == '/':
				s.push(lineComment)
				s.i++

			case char == '/' && s.i+1 < len(s.src) && s.peekAt(1) == '*':
				s.push(blockComment)
				s.i++

			case char == '"':
				s.push(stringDouble)

			case char == '\'':
				s.push(stringSingle)

			case char == '`':
				s.push(template)

			case char == '{' && len(s.stack) > 1:
				top.depth++

			case char == '}' && len(s.stack) > 1:
				if top.depth == 0 {
					s.pop() // closing brace of `${...}`
				} else {
					top.depth--
				}
			}

		case lineComment:
			if char == '\n' {
				s.pop()
			}

		case blockComment:
			if char == '*' && s.i+1 < len(s.src) && s.peekAt(1) == '/' {
				s.pop()
				s.i++
			}

		case stringDouble:
			switch char {
			case '\\':
				s.i++
			case '"':
				s.pop()
			}

		case stringSingle:
			switch char {
			case '\\':
				s.i++
			case '\'':
				s.pop()
			}

		case template:
			switch {
			case char == '\\':
				s.i++
			case char == '$' && s.peekAt(1) == '{':
				s.push(code)
				s.i++
			case char == '`':
				s.pop()
			}
		}

		s.i++
	}

	return s.imports, s.exports, nil
}

func (s *scanner) parseImport() error {
	imp := Import{}

	s.skipSpace()

	if s.peek() == '.' {
		return nil // import.meta
	}

	onlyTypes := false
	if s.peek() == 't' {
		word, err := s.nextWord()
		if err != nil {
			return err
		}
		if word == "type" {
			onlyTypes = true
			s.skipSpace()
		}
	}

	switch s.peek() {
	case '"', '\'':
		from, err := s.readString()
		if err != nil {
			return err
		}
		imp.From = from

	case '(':
		s.i++
		s.skipSpace()
		if s.peek() != '"' && s.peek() != '\'' {
			return nil // unsupported dynamic import with non-literal argument
		}
		imp.Dynamic = true
		from, err := s.readString()
		if err != nil {
			return err
		}
		imp.From = from
		s.skipSpace()
		s.i++

	default:
		if s.peek() != '{' && s.peek() != '*' {
			symbol, err := s.symbolFromNextWord(onlyTypes)
			if err != nil {
				return err
			}
			symbol.Kind = DefaultSymbol
			imp.Symbols = append(imp.Symbols, symbol)
			s.skipSpace()
			if s.peek() == ',' {
				s.i++
				s.skipSpace()
			}
		}

		if s.peek() == '*' {
			s.i++
			s.skipSpace()
			if isAs := s.isWord(asKeyword); !isAs {
				return fmt.Errorf("expected 'as' after '*' in namespace import")
			}
			s.skipSpace()
			symbol, err := s.symbolFromNextWord(onlyTypes)
			if err != nil {
				return err
			}
			symbol.Kind = NamespaceSymbol
			imp.Symbols = append(imp.Symbols, symbol)
		} else if s.peek() == '{' {
			symbols, err := s.parseNamed(onlyTypes)
			if err != nil {
				return err
			}
			imp.Symbols = append(imp.Symbols, symbols...)
		}

		s.skipSpace()
		if isFrom := s.isWord(fromKeyword); !isFrom {
			return fmt.Errorf("expected 'from' after import symbols")
		}
		s.skipSpace()
		from, err := s.readString()
		if err != nil {
			return err
		}
		imp.From = from
	}

	s.imports = append(s.imports, imp)

	return nil
}

func (s *scanner) parseExport() error {
	exp := Export{}

	s.skipSpace()

	onlyTypes := false
	if s.peek() == 't' {
		word, err := s.nextWord()
		if err != nil {
			return err
		}
		if word == "type" {
			onlyTypes = true
			s.skipSpace()
		}
	}

	switch s.peek() {
	case '{':
		symbols, err := s.parseNamed(onlyTypes)
		if err != nil {
			return err
		}
		exp.Symbols = symbols
		s.skipSpace()
		if isFrom := s.isWord(fromKeyword); isFrom {
			s.skipSpace()
			from, err := s.readString()
			if err != nil {
				return err
			}
			exp.From = from
			s.imports = append(s.imports, Import{From: from, Symbols: exp.Symbols})
		}

	case '*':
		s.i++
		s.skipSpace()
		if isAs := s.isWord(asKeyword); isAs {
			s.skipSpace()
			symbol, err := s.symbolFromNextWord(onlyTypes)
			if err != nil {
				return err
			}
			symbol.Kind = NamespaceSymbol
			exp.Symbols = append(exp.Symbols, symbol)
		} else {
			exp.Symbols = append(exp.Symbols, Symbol{Kind: NamespaceSymbol, TypeOnly: onlyTypes})
		}
		if isFrom := s.isWord(fromKeyword); !isFrom {
			return fmt.Errorf("expected 'from' after export symbols")
		}
		s.skipSpace()
		from, err := s.readString()
		if err != nil {
			return err
		}
		exp.From = from
		s.imports = append(s.imports, Import{From: from, Symbols: exp.Symbols})

	default:
	outer:
		for {
			word, err := s.nextWord()
			if err != nil {
				return err
			}

			s.skipSpace()

			switch word {
			case "default":
				exp.Symbols = append(exp.Symbols, Symbol{Kind: DefaultSymbol})
				s.exports = append(s.exports, exp)
				return nil

			case "declare":
				onlyTypes = true

			case "async", "abstract":
				continue

			case "const", "let", "var", "function", "class", "enum":
				break outer

			case "interface":
				onlyTypes = true
				break outer

			default:
				if onlyTypes {
					exp.Symbols = append(exp.Symbols, Symbol{Name: word, Kind: NamedSymbol, TypeOnly: true})
					s.exports = append(s.exports, exp)
					return nil
				} else {
					return fmt.Errorf("unexpected token %q", s.peek())
				}
			}
		}

		s.skipSpace()
		symbol, err := s.symbolFromNextWord(onlyTypes)
		if err != nil {
			return err
		}
		symbol.Kind = NamedSymbol
		exp.Symbols = append(exp.Symbols, symbol)
	}

	s.exports = append(s.exports, exp)

	return nil
}

func (s *scanner) parseNamed(onlyTypes bool) ([]Symbol, error) {
	var symbols []Symbol
	s.i++
	for {
		s.skipSpace()
		if s.peek() == '}' {
			s.i++
			break
		}
		if s.peek() == ',' {
			s.i++
			continue
		}
		symbol, err := s.symbolFromNextWord(onlyTypes)
		if err != nil {
			return nil, err
		}
		symbol.Kind = NamedSymbol
		symbols = append(symbols, symbol)
	}
	return symbols, nil
}

func (s *scanner) peek() byte {
	if s.i >= len(s.src) {
		return 0
	}
	return s.src[s.i]
}

func (s *scanner) peekAt(n int) byte {
	if s.i+n >= len(s.src) || s.i+n < 0 {
		return 0
	}
	return s.src[s.i+n]
}

func (s *scanner) isWord(word []byte) bool {
	if s.i+len(word) > len(s.src) {
		return false
	}
	for i := range len(word) {
		c := word[i]
		if c != s.peekAt(i) {
			return false
		}
	}
	if s.i > 0 {
		prev := s.peekAt(-1)
		if isIdent(prev) || prev == '.' {
			return false
		}
	}
	if s.i+len(word) < len(s.src) && isIdent(s.peekAt(len(word))) {
		return false
	}
	s.i += len(word)
	return true
}

func (s *scanner) nextWord() (string, error) {
	pos := s.i
	for isIdent(s.peek()) {
		s.i++
	}
	if pos == s.i {
		return "", fmt.Errorf("expected word, got nothing")
	}
	return string(s.src[pos:s.i]), nil
}

func (s *scanner) skipSpace() {
	for s.i < len(s.src) {
		char := s.peek()
		if char == ' ' || char == '\t' || char == '\n' || char == '\r' {
			s.i++
		} else {
			break
		}
	}
}

func (s *scanner) readString() (string, error) {
	quote := s.peek()
	s.i++
	start := s.i
	for {
		if s.peek() == quote {
			break
		}
		if s.peek() == 0 {
			return "", fmt.Errorf("expected closing quote, got EOF")
		}
		s.i++
	}
	return string(s.src[start:s.i]), nil
}

func isIdent(char byte) bool {
	return 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z' || '0' <= char && char <= '9' || char == '_' || char == '$'
}

func (s *scanner) symbolFromNextWord(onlyTypes bool) (Symbol, error) {
	word, err := s.nextWord()
	if err != nil {
		return Symbol{}, err
	}
	symbol := Symbol{Name: word, TypeOnly: onlyTypes}
	if symbol.Name == "type" {
		symbol.TypeOnly = true
		s.skipSpace()
		word, err := s.nextWord()
		if err != nil {
			return Symbol{}, err
		}
		symbol.Name = word
	}
	s.skipSpace()
	if s.isWord(asKeyword) {
		s.skipSpace()
		word, err := s.nextWord()
		if err != nil {
			return Symbol{}, err
		}
		symbol.Alias = word
	}
	return symbol, nil
}
