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
		{"side-effect single quote", "import 'mod'", Import{Kind: SideEffectEdge, From: "mod"}},
		{"side-effect double quote", "import \"mod\"", Import{Kind: SideEffectEdge, From: "mod"}},
		{"default import", "import foo from 'mod'", Import{Kind: DefaultEdge, From: "mod", Symbols: []Symbol{{Name: "foo"}}}},
		{"default type import", "import type foo from 'mod'", Import{Kind: DefaultEdge, From: "mod", Symbols: []Symbol{{Name: "foo", TypeOnly: true}}}},
		{"named imports", "import { foo, bar } from 'mod'", Import{Kind: NamedEdge, From: "mod", Symbols: []Symbol{{Name: "foo"}, {Name: "bar"}}}},
		{"named type imports", "import { type foo, bar } from 'mod'", Import{Kind: NamedEdge, From: "mod", Symbols: []Symbol{{Name: "foo", TypeOnly: true}, {Name: "bar"}}}},
		{"named only type imports", "import type { foo, bar } from 'mod'", Import{Kind: NamedEdge, From: "mod", Symbols: []Symbol{{Name: "foo", TypeOnly: true}, {Name: "bar", TypeOnly: true}}}},
		{"namepace import", "import * as foo from 'mod'", Import{Kind: NamespaceEdge, From: "mod", Symbols: []Symbol{{Name: "foo"}}}},
		{"namepace type import", "import type * as foo from 'mod'", Import{Kind: NamespaceEdge, From: "mod", Symbols: []Symbol{{Name: "foo", TypeOnly: true}}}},
		{"dynamic import", "import('mod')", Import{Kind: DynamicEdge, From: "mod"}},
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
