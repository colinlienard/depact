package parser

import (
	"reflect"
	"testing"
)

func TestScannerEveryImport(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Import
	}{
		{"side-effect single quote", "import 'mod'", Import{Kind: SideEffectEdge, from: "mod"}},
		{"side-effect double quote", "import \"mod\"", Import{Kind: SideEffectEdge, from: "mod"}},
		{"default import", "import foo from 'mod'", Import{Kind: DefaultEdge, from: "mod", symbols: []string{"foo"}}},
		{"default type import", "import type foo from 'mod'", Import{Kind: DefaultEdge, typeOnly: true, from: "mod", symbols: []string{"foo"}}},
		// { "import { foo, bar } from 'mod'", Import{Kind: DefaultEdge, typeOnly: true, from: "mod", symbols: []string{"foo", "bar"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := &scanner{src: []byte(tt.input)}
			imports, _ := scanner.scan()

			if len(imports) != 1 {
				t.Fatalf("expected 1 edge, got %d", len(imports))
			}

			if !reflect.DeepEqual(imports[0], tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, imports[0])
			}
		})
	}
}

// import x from "mod"
// import { a, b } from "mod"
// import * as ns from "mod"
// import "mod"
// import type { T } from "mod"
// const x = require("mod")
// const x = await import("mod")

// export const x = ...
// export function f() {}
// export default ...
// export { a, b as c }
// export { x } from "mod"
// export * from "mod"
// export * as ns from "mod"
