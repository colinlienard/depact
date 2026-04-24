package parser

import (
	"reflect"
	"testing"
)

func TestParserEveryEdge(t *testing.T) {
	tests := []struct {
		input    string
		expected ImportEdge
	}{
		{input: "import 'mod'", expected: ImportEdge{_type: SideEffect, typeOnly: false, from: "mod", symbols: nil}},
		{input: "import \"mod\"", expected: ImportEdge{_type: SideEffect, typeOnly: false, from: "mod", symbols: nil}},
		{input: "import foo from 'mod'", expected: ImportEdge{_type: Default, typeOnly: false, from: "mod", symbols: []string{"foo"}}},
		{input: "import type foo from 'mod'", expected: ImportEdge{_type: Default, typeOnly: true, from: "mod", symbols: []string{"foo"}}},
	}

	for _, test := range tests {
		parser := NewParser([]byte(test.input))
		edges := parser.Parse()

		if len(edges) != 1 {
			t.Errorf("expected 1 edge, got %d", len(edges))
		}

		if !reflect.DeepEqual(edges[0], test.expected) {
			t.Errorf("expected %+v, got %+v", test.expected, edges[0])
		}
	}
}

// import x from "mod"
// import { a, b } from "mod"
// import * as ns from "mod"
// import "mod"                    // side-effect
// import type { T } from "mod"
// const x = require("mod")
// const x = await import("mod")   // dynamic
