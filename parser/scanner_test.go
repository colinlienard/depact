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
		{"side-effect single quote", "import 'mod'", Import{From: "mod"}},
		{"side-effect double quote", "import \"mod\"", Import{From: "mod"}},
		{"default import", "import foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSym, Name: "foo"}}}},
		{"default type import", "import type foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSym, Name: "foo", TypeOnly: true}}}},
		{"named imports", "import { foo, bar } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSym, Name: "foo"}, {Kind: NamedSym, Name: "bar"}}}},
		{"renamed imports", "import { foo as foo2 } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSym, Name: "foo", Alias: "foo2"}}}},
		{"named type imports", "import { type foo, bar } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSym, Name: "foo", TypeOnly: true}, {Kind: NamedSym, Name: "bar"}}}},
		{"named only type imports", "import type { foo, bar } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSym, Name: "foo", TypeOnly: true}, {Kind: NamedSym, Name: "bar", TypeOnly: true}}}},
		{"namepace import", "import * as foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamespaceSym, Name: "foo"}}}},
		{"mixed default and named", "import type foo, { bar } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSym, Name: "foo", TypeOnly: true}, {Kind: NamedSym, Name: "bar", TypeOnly: true}}}},
		{"namepace type import", "import type * as foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamespaceSym, Name: "foo", TypeOnly: true}}}},
		{"dynamic import", "import('mod')", Import{From: "mod", Dynamic: true}},
		{"dynamic import with with whitespaces", "import ( 'mod' )", Import{From: "mod", Dynamic: true}},

		{"no spaces", "import{foo}from'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSym, Name: "foo"}}}},
		{"extra spaces", "import   {   foo  ,  bar   }   from   'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSym, Name: "foo"}, {Kind: NamedSym, Name: "bar"}}}},
		{"newlines between tokens", "import\n{\n\tfoo,\n\tbar,\n}\nfrom\n'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSym, Name: "foo"}, {Kind: NamedSym, Name: "bar"}}}},
		{"trailing comma", "import { foo, bar, } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSym, Name: "foo"}, {Kind: NamedSym, Name: "bar"}}}},
		{"alias with extra spaces", "import {  foo   as   bar  } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSym, Name: "foo", Alias: "bar"}}}},
		{"namespace tightly packed", "import*as foo from'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamespaceSym, Name: "foo"}}}},

		{"escaped backslash in double-quoted string", `const s = "\\"` + "\n" + `import foo from 'mod'`, Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSym, Name: "foo"}}}},
		{"escaped backslash in single-quoted string", `const s = '\\'` + "\n" + `import foo from 'mod'`, Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSym, Name: "foo"}}}},
		{"escaped quote in double-quoted string", `const s = "\""` + "\n" + `import foo from 'mod'`, Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSym, Name: "foo"}}}},
		{"dynamic import inside template interpolation", "const x = `prefix ${import('mod')} suffix`", Import{From: "mod", Dynamic: true}},
		{"nested template inside interpolation", "const x = `a${`b`}c`\nimport foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSym, Name: "foo"}}}},
		{"object literal in interpolation", "const x = `${ {a: 1} }`\nimport foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSym, Name: "foo"}}}},
		{"string in interpolation", "const x = `${\"}\"}`\nimport foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSym, Name: "foo"}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := &scanner{src: []byte(tt.input)}
			imports, _, _ := scanner.scan()

			if len(imports) != 1 {
				t.Fatalf("expected 1 import, got %d", len(imports))
			}

			if !reflect.DeepEqual(imports[0], tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, imports[0])
			}
		})
	}
}

func TestScannerNonImports(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"line comment", "// import foo from 'mod'"},
		{"block comment", "/* import foo from 'mod' */"},
		{"trailing block comment", "/* import { x } from 'a' */\nconst x = 1"},
		{"import in double-quoted string", `const s = "import foo from 'mod'"`},
		{"import in single-quoted string", `const s = 'import foo from "mod"'`},
		{"import in template literal", "const s = `import foo from 'mod'`"},
		{"important identifier", "let important = 1"},
		{"imports identifier", "let imports = []"},
		{"importer identifier", "function importer() {}"},
		{"reimport identifier", "const reimport = 1"},
		{"property access import", "obj.import('mod')"},
		{"property access import with chain", "foo.import.bar"},
		{"import.meta", "console.log(import.meta)"},
		{"import.meta.ENV", "const e = import.meta.ENV.MODE"},
		{"jsdoc block comment", "/** @type {string} */\nconst x = 1"},
		{"dynamic import with non-literal argument", "import(somePath)"},
		{"dynamic import with template literal argument", "import(`mod`)"},
		{"export-like identifier", "let exported = 1"},
		{"empty source", ""},
		{"no imports at all", "const x = 1\nfunction y() { return x }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := &scanner{src: []byte(tt.input)}
			imports, _, _ := scanner.scan()

			if len(imports) != 0 {
				t.Errorf("expected 0 imports, got %d: %+v", len(imports), imports)
			}
		})
	}
}

func TestScannerErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing as in namespace import", "import * foo from 'mod'"},
		{"missing from after named import", "import { foo } 'mod'"},
		{"missing from after default import", "import foo 'mod'"},
		{"unterminated string", "import foo from 'mod"},
		{"unterminated dynamic import string", "import('mod"},
		{"namespace import missing identifier after as", "import * as from 'mod'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := &scanner{src: []byte(tt.input)}
			_, _, err := scanner.scan()
			if err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

func TestScannerMultipleImports(t *testing.T) {
	src := "" +
		"// header comment with import 'fake'\n" +
		"import 'side-effect'\n" +
		"import foo from \"./default\"\n" +
		"import * as ns from 'namespace'\n" +
		"import { a, b as bAlias, type c } from './named'\n" +
		"import type { D, E } from 'types'\n" +
		"import type Def, { Mixed } from 'mixed'\n" +
		"\n" +
		"const msg = `template ${import('dynamic')} ${`nested ${1}`} end`\n" +
		"const s1 = \"import fake from 'nope'\"\n" +
		"const s2 = 'import fake from \"nope\"'\n" +
		"/* import fake from 'nope' */\n" +
		"obj.import('not-an-import')\n" +
		"\n" +
		"import { last1, last2, } from 'trailing'\n"

	expected := []Import{
		{From: "side-effect"},
		{From: "./default", Symbols: []Symbol{{Kind: DefaultSym, Name: "foo"}}},
		{From: "namespace", Symbols: []Symbol{{Kind: NamespaceSym, Name: "ns"}}},
		{From: "./named", Symbols: []Symbol{
			{Kind: NamedSym, Name: "a"},
			{Kind: NamedSym, Name: "b", Alias: "bAlias"},
			{Kind: NamedSym, Name: "c", TypeOnly: true},
		}},
		{From: "types", Symbols: []Symbol{
			{Kind: NamedSym, Name: "D", TypeOnly: true},
			{Kind: NamedSym, Name: "E", TypeOnly: true},
		}},
		{From: "mixed", Symbols: []Symbol{
			{Kind: DefaultSym, Name: "Def", TypeOnly: true},
			{Kind: NamedSym, Name: "Mixed", TypeOnly: true},
		}},
		{From: "dynamic", Dynamic: true},
		{From: "trailing", Symbols: []Symbol{
			{Kind: NamedSym, Name: "last1"},
			{Kind: NamedSym, Name: "last2"},
		}},
	}

	scanner := &scanner{src: []byte(src)}
	imports, _, err := scanner.scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(imports) != len(expected) {
		t.Fatalf("expected %d imports, got %d: %+v", len(expected), len(imports), imports)
	}

	for i, want := range expected {
		if !reflect.DeepEqual(imports[i], want) {
			t.Errorf("import %d:\n  expected %+v\n  got      %+v", i, want, imports[i])
		}
	}
}
