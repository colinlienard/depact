package main

import (
	"depact/parser"
)

func main() {
	_, _ = parser.Parse([]byte("import x from 'mod'"))
}
