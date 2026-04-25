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
	// TODO: export (for wasted symbols)
	edge := Import{}
outer:
	for {
		char := s.src[s.i]
		switch char {
		case ' ':
			s.i++
			continue
		case '"', '\'':
			if len(edge.symbols) == 0 {
				edge.Kind = SideEffectEdge
			}
			s.i++
			edge.from = string(s.walkUntil(char))
			break outer
		case '{':
		// TODO
		case '*':
		// TODO
		default:
			symbol := string(s.walkUntil(' '))
			if symbol == "from" {
				continue
			}
			if symbol == "type" {
				edge.typeOnly = true
				continue
			}
			edge.Kind = DefaultEdge
			edge.symbols = []string{(symbol)}
		}

		s.i++
	}
	s.imports = append(s.imports, edge)
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

func (s *scanner) walkUntil(char byte) []byte {
	pos := s.i
	for s.src[s.i] != char {
		s.i++
	}
	return s.src[pos:s.i]
}
