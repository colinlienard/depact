package main

import (
	"depact/parser"
	"fmt"
)

func main() {
	ok := []byte("Hello\n")
	fmt.Println(ok[5])
	_ = parser.Parser{}
}
