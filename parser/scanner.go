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

func (s *scanner) scan() ([]Import, []Export) {
	for s.i < len(s.src) {
		char := s.src[s.i]

		switch s.state {
		case code:
			switch {
			case char == '/' && s.i+1 < len(s.src) && s.src[s.i+1] == '/':
				s.state = lineComment
				s.i++

			case char == '/' && s.i+1 < len(s.src) && s.src[s.i+1] == '*':
				s.state = blockComment
				s.i++

			case char == '"':
				s.state = stringDouble

			case char == '\'':
				s.state = stringSingle

			case char == '`':
				s.state = template

			default:
				s.parseCode()
			}

		case lineComment:
			if char == '\n' {
				s.state = code
			}

		case blockComment:
			if char == '*' && s.i+1 < len(s.src) && s.src[s.i+1] == '/' {
				s.state = code
				s.i++
			}

		case stringDouble:
			if char == '"' && s.i > 1 && s.src[s.i-1] != '\\' {
				s.state = code
			}

		case stringSingle:
			if char == '\'' && s.i > 1 && s.src[s.i-1] != '\\' {
				s.state = code
			}

		case template: // TODO: handle `${}`
			if char == '`' {
				s.state = code
			}
		}

		s.i++
	}

	return s.imports, s.exports
}

func (s *scanner) parseCode() {
	if !s.isWord([]byte("import")) {
		return
	}

	imp := Import{}
	exp := Export{}

	s.skipSpace()

	onlyTypes := false
	if s.src[s.i] == 't' && s.nextWord() == "type" {
		onlyTypes = true
		s.skipSpace()
	}

outer:
	for s.i < len(s.src) {
		switch s.src[s.i] {
		case '"', '\'':
			imp.Kind = SideEffectEdge
			imp.From = s.readString()
			break outer

		case '{':
			imp.Kind = NamedEdge
			s.i++
			for {
				s.skipSpace()
				if s.src[s.i] == '}' {
					s.i++
					break
				}
				if s.src[s.i] == ',' {
					s.i++
					continue
				}
				imp.Symbols = append(imp.Symbols, s.symbolFromNextWord(onlyTypes))
			}
			s.i++
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
			imp.Symbols = append(imp.Symbols, s.symbolFromNextWord(onlyTypes))
			s.skipSpace()
			s.nextWord() // from
			s.skipSpace()
			imp.From = s.readString()

		default:
			imp.Kind = DefaultEdge
			imp.Symbols = append(imp.Symbols, s.symbolFromNextWord(onlyTypes))
			s.skipSpace()
			s.nextWord() // from
			s.skipSpace()
			imp.From = s.readString()
		}

		s.i++
	}

	s.imports = append(s.imports, imp)
	s.exports = append(s.exports, exp)
}

func (s *scanner) isWord(word []byte) bool {
	for i := range len(word) {
		c := word[i]
		if c != s.src[s.i+i] {
			return false
		}
	}
	s.i += len(word)
	return true
}

func (s *scanner) nextWord() string {
	pos := s.i
	for isLetter(s.src[s.i]) {
		s.i++
	}
	return string(s.src[pos:s.i])
}

func (s *scanner) skipSpace() {
	for s.i < len(s.src) {
		char := s.src[s.i]
		if char == ' ' || char == '\t' || char == '\n' || char == '\r' {
			s.i++
		} else {
			break
		}
	}
}

func (s *scanner) readString() string {
	quote := s.src[s.i]
	s.i++
	start := s.i
	for s.src[s.i] != quote {
		s.i++
	}
	s.i++
	return string(s.src[start : s.i-1])
}

func isLetter(char byte) bool {
	return 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z' || char == '_'
}

func (s *scanner) symbolFromNextWord(onlyTypes bool) Symbol {
	symbol := Symbol{Name: s.nextWord()}
	if symbol.Name == "type" {
		symbol.TypeOnly = true
		s.skipSpace()
		symbol.Name = s.nextWord()
	} else if onlyTypes {
		symbol.TypeOnly = true
	}
	return symbol
}
