package parser

type State int

const (
	Code         State = iota
	LineComment        // //
	BlockComment       // /* */
	StringDouble       // "..."
	StringSingle       // '...'
	Template           // `...`
)

type ImportEdgeType int

const (
	Default ImportEdgeType = iota
	Named
	Namespace
	SideEffect
	Dynamic
)

type ImportEdge struct {
	_type    ImportEdgeType
	typeOnly bool
	from     string
	symbols  []string
}

type Parser struct {
	src   []byte
	state State
	i     int
	edges []ImportEdge
}

func NewParser(src []byte) *Parser {
	return &Parser{
		src:   src,
		state: Code,
		i:     0,
		edges: []ImportEdge{},
	}
}

func (p *Parser) Parse() []ImportEdge {
	for p.i < len(p.src) {
		char := p.src[p.i]

		switch p.state {
		case Code:
			switch {
			case char == '/' && len(p.src) >= p.i+1 && p.src[p.i+1] == '/':
				p.state = LineComment
				p.i++
			case char == '/' && len(p.src) >= p.i+1 && p.src[p.i+1] == '*':
				p.state = BlockComment
				p.i++
			case char == '"':
				p.state = StringDouble
			case char == '\'':
				p.state = StringSingle
			case char == '`':
				p.state = Template
			default:
				p.parseCode()
			}

		case LineComment:
			if char == '\n' {
				p.state = Code
			}

		case BlockComment:
			if char == '*' && len(p.src) >= p.i+1 && p.src[p.i+1] == '/' {
				p.state = Code
				p.i++
			}

		case StringDouble:
			if char == '"' && p.i > 1 && p.src[p.i-1] != '\\' {
				p.state = Code
			}

		case StringSingle:
			if char == '\'' && p.i > 1 && p.src[p.i-1] != '\\' {
				p.state = Code
			}

		case Template: // TODO: handle `${}`
			if char == '`' {
				p.state = Code
			}
		}

		p.i++
	}

	return p.edges
}

func (p *Parser) parseCode() {
	if !p.isWord([]byte("import")) {
		return
	}
	edge := ImportEdge{}
outer:
	for {
		char := p.src[p.i]
		switch char {
		case ' ':
			p.i++
			continue
		case '"', '\'':
			if len(edge.symbols) == 0 {
				edge._type = SideEffect
			}
			p.i++
			edge.from = string(p.walkUntil(char))
			break outer
		case '{':
		// TODO
		case '*':
		// TODO
		default:
			symbol := string(p.walkUntil(' '))
			if symbol == "from" {
				continue
			}
			if symbol == "type" {
				edge.typeOnly = true
				continue
			}
			edge._type = Default
			edge.symbols = []string{(symbol)}
		}

		p.i++
	}
	p.edges = append(p.edges, edge)
}

func (p *Parser) isWord(word []byte) bool {
	for i := range len(word) {
		c := word[i]
		if c != p.src[p.i+i] {
			return false
		}
	}
	p.i += len(word)
	return true
}

func (p *Parser) walkUntil(char byte) []byte {
	pos := p.i
	for p.src[p.i] != char {
		p.i++
	}
	return p.src[pos:p.i]
}
