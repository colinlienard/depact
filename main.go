package main

import (
	"depact/parser"
)

func main() {
	_ = parser.Parse([]byte("import x from 'mod'"))
}
