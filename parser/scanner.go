package parser

type state int

const (
	code         state = iota
	lineComment        // //
	blockComment       // /* */
	stringDouble       // "..."
	stringSingle       // '...'
	template           // `...`
)

type scanner struct {
	src     []byte
	state   state
	i       int
	imports []Import
	exports []Export
}

func (s *scanner) scan() ([]Import, []Export, error) {
	for s.i < len(s.src) {
		char := s.peek()

		switch s.state {
		case code:
			switch {
			case char == '/' && s.i+1 < len(s.src) && s.peekAt(1) == '/':
				s.state = lineComment
				s.i++

			case char == '/' && s.i+1 < len(s.src) && s.peekAt(1) == '*':
				s.state = blockComment
				s.i++

			case char == '"':
				s.state = stringDouble

			case char == '\'':
				s.state = stringSingle

			case char == '`':
				s.state = template

			default:
				if err := s.parseCode(); err != nil {
					return nil, nil, err
				}
			}

		case lineComment:
			if char == '\n' {
				s.state = code
			}

		case blockComment:
			if char == '*' && s.i+1 < len(s.src) && s.peekAt(1) == '/' {
				s.state = code
				s.i++
			}

		case stringDouble: // TODO: handle \\"
			if char == '"' && s.i > 0 && s.peekAt(-1) != '\\' {
				s.state = code
			}

		case stringSingle: // TODO: handle \\'
			if char == '\'' && s.i > 0 && s.peekAt(-1) != '\\' {
				s.state = code
			}

		case template: // TODO: handle `${}`
			if char == '`' {
				s.state = code
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
		imp.Kind = SideEffectEdge
		imp.From = s.readString()

	case '{':
		imp.Kind = NamedEdge
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
			imp.Symbols = append(imp.Symbols, sym)
		}
		s.skipSpace()
		s.nextWord() // from
		s.skipSpace()
		imp.From = s.readString()

	case '(':
		imp.Kind = DynamicEdge
		s.i++
		imp.From = s.readString()
		s.i++

	case '*':
		imp.Kind = NamespaceEdge
		s.i++
		s.skipSpace()
		s.nextWord() // as
		s.skipSpace()
		sym, err := s.symbolFromNextWord(onlyTypes)
		if err != nil {
			return err
		}
		imp.Symbols = append(imp.Symbols, sym)
		s.skipSpace()
		s.nextWord() // from
		s.skipSpace()
		imp.From = s.readString()

	default:
		imp.Kind = DefaultEdge
		sym, err := s.symbolFromNextWord(onlyTypes)
		if err != nil {
			return err
		}
		imp.Symbols = append(imp.Symbols, sym)
		s.skipSpace()
		s.nextWord() // from
		s.skipSpace()
		imp.From = s.readString()
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
	if (s.i > 0 && !isLetter(s.peekAt(-1))) || (s.i+len(word) > len(s.src) && !isLetter(s.peekAt(len(word)))) {
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

// TODO: handle error ""
func (s *scanner) nextWord() (string, error) {
	pos := s.i
	for isLetter(s.peek()) {
		s.i++
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

// TODO: handle error no closing quote
func (s *scanner) readString() string {
	quote := s.peek()
	s.i++
	start := s.i
	for s.peek() != quote {
		s.i++
	}
	s.i++
	return string(s.src[start : s.i-1])
}

func isLetter(char byte) bool {
	return 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z' || char == '_'
}

func (s *scanner) symbolFromNextWord(onlyTypes bool) (Symbol, error) {
	word, err := s.nextWord()
	if err != nil {
		return Symbol{}, err
	}
	symbol := Symbol{Name: word}
	if symbol.Name == "type" {
		symbol.TypeOnly = true
		s.skipSpace()
		word, err := s.nextWord()
		if err != nil {
			return Symbol{}, err
		}
		symbol.Name = word
	} else if onlyTypes {
		symbol.TypeOnly = true
	}
	return symbol, nil
}
