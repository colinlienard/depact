package parser

import "fmt"

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

			default:
				if err := s.parseCode(); err != nil {
					return nil, nil, err
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

func (s *scanner) parseCode() error {
	if !s.isWord([]byte("import")) && !s.isWord([]byte("export")) {
		return nil
	}

	imp := Import{}
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
	case '"', '\'':
		from, err := s.readString()
		if err != nil {
			return err
		}
		imp.From = from

	case '(':
		imp.Dynamic = true
		s.i++
		from, err := s.readString()
		if err != nil {
			return err
		}
		imp.From = from
		s.i++

	default:
		if s.peek() != '{' && s.peek() != '*' {
			sym, err := s.symbolFromNextWord(onlyTypes)
			if err != nil {
				return err
			}
			sym.Kind = DefaultSym
			imp.Symbols = append(imp.Symbols, sym)
			s.skipSpace()
			if s.peek() == ',' {
				s.i++
				s.skipSpace()
			}
		}

		if s.peek() == '*' {
			s.i++
			s.skipSpace()
			s.nextWord() // as
			s.skipSpace()
			sym, err := s.symbolFromNextWord(onlyTypes)
			if err != nil {
				return err
			}
			sym.Kind = NamespaceSym
			imp.Symbols = append(imp.Symbols, sym)
		} else if s.peek() == '{' {
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
				sym, err := s.symbolFromNextWord(onlyTypes)
				if err != nil {
					return err
				}
				sym.Kind = NamedSym
				imp.Symbols = append(imp.Symbols, sym)
			}
		}

		s.skipSpace()
		s.nextWord() // from
		s.skipSpace()
		from, err := s.readString()
		if err != nil {
			return err
		}
		imp.From = from
	}

	s.imports = append(s.imports, imp)
	s.exports = append(s.exports, exp)

	return nil
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
	if s.i > 0 {
		prev := s.peekAt(-1)
		if isLetter(prev) || prev == '.' {
			return false
		}
	}
	if s.i+len(word) < len(s.src) && isLetter(s.peekAt(len(word))) {
		return false
	}
	for i := range len(word) {
		c := word[i]
		if c != s.peekAt(i) {
			return false
		}
	}
	s.i += len(word)
	return true
}

func (s *scanner) nextWord() (string, error) {
	pos := s.i
	for isLetter(s.peek()) {
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
	s.i++
	return string(s.src[start : s.i-1]), nil
}

func isLetter(char byte) bool {
	return 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z' || char == '_'
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
	if s.isWord([]byte("as")) {
		s.skipSpace()
		word, err := s.nextWord()
		if err != nil {
			return Symbol{}, err
		}
		symbol.Alias = word
	}
	return symbol, nil
}
