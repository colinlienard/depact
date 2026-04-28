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
		{"default import", "import foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "foo"}}}},
		{"default type import", "import type foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "foo", TypeOnly: true}}}},
		{"named imports", "import { foo, bar } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}, {Kind: NamedSymbol, Name: "bar"}}}},
		{"renamed imports", "import { foo as foo2 } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", Alias: "foo2"}}}},
		{"named type imports", "import { type foo, bar } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", TypeOnly: true}, {Kind: NamedSymbol, Name: "bar"}}}},
		{"named only type imports", "import type { foo, bar } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", TypeOnly: true}, {Kind: NamedSymbol, Name: "bar", TypeOnly: true}}}},
		{"namepace import", "import * as foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamespaceSymbol, Name: "foo"}}}},
		{"mixed default and named", "import type foo, { bar } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "foo", TypeOnly: true}, {Kind: NamedSymbol, Name: "bar", TypeOnly: true}}}},
		{"mixed default and namespace", "import foo, * as ns from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "foo"}, {Kind: NamespaceSymbol, Name: "ns"}}}},
		{"default keyword as named alias", "import { default as foo } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "default", Alias: "foo"}}}},
		{"empty named clause", "import {} from 'mod'", Import{From: "mod"}},
		{"dollar identifier", "import $ from 'jquery'", Import{From: "jquery", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "$"}}}},
		{"namepace type import", "import type * as foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamespaceSymbol, Name: "foo", TypeOnly: true}}}},
		{"dynamic import", "import('mod')", Import{From: "mod", Dynamic: true}},
		{"dynamic import with with whitespaces", "import ( 'mod' )", Import{From: "mod", Dynamic: true}},

		{"no spaces", "import{foo}from'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}}}},
		{"extra spaces", "import   {   foo  ,  bar   }   from   'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}, {Kind: NamedSymbol, Name: "bar"}}}},
		{"newlines between tokens", "import\n{\n\tfoo,\n\tbar,\n}\nfrom\n'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}, {Kind: NamedSymbol, Name: "bar"}}}},
		{"trailing comma", "import { foo, bar, } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}, {Kind: NamedSymbol, Name: "bar"}}}},
		{"alias with extra spaces", "import {  foo   as   bar  } from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", Alias: "bar"}}}},
		{"namespace tightly packed", "import*as foo from'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: NamespaceSymbol, Name: "foo"}}}},

		{"escaped backslash in double-quoted string", `const s = "\\"` + "\n" + `import foo from 'mod'`, Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "foo"}}}},
		{"escaped backslash in single-quoted string", `const s = '\\'` + "\n" + `import foo from 'mod'`, Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "foo"}}}},
		{"escaped quote in double-quoted string", `const s = "\""` + "\n" + `import foo from 'mod'`, Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "foo"}}}},
		{"dynamic import inside template interpolation", "const x = `prefix ${import('mod')} suffix`", Import{From: "mod", Dynamic: true}},
		{"nested template inside interpolation", "const x = `a${`b`}c`\nimport foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "foo"}}}},
		{"object literal in interpolation", "const x = `${ {a: 1} }`\nimport foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "foo"}}}},
		{"string in interpolation", "const x = `${\"}\"}`\nimport foo from 'mod'", Import{From: "mod", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "foo"}}}},
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

func TestScannerEveryExport(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedImports []Import
		expectedExports []Export
	}{
		{"named export", "export { foo }", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}}}}},
		{"multiple named exports", "export { foo, bar }", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}, {Kind: NamedSymbol, Name: "bar"}}}}},
		{"renamed export", "export { foo as bar }", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", Alias: "bar"}}}}},
		{"named export with inline type", "export { type foo, bar }", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", TypeOnly: true}, {Kind: NamedSymbol, Name: "bar"}}}}},
		{"named only type exports", "export type { foo, bar }", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", TypeOnly: true}, {Kind: NamedSymbol, Name: "bar", TypeOnly: true}}}}},
		{"trailing comma", "export { foo, bar, }", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}, {Kind: NamedSymbol, Name: "bar"}}}}},

		{"re-export named", "export { foo } from 'mod'",
			[]Import{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}}}},
			[]Export{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}}}}},
		{"re-export renamed", "export { foo as bar } from 'mod'",
			[]Import{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", Alias: "bar"}}}},
			[]Export{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", Alias: "bar"}}}}},
		{"re-export type-only", "export type { foo } from 'mod'",
			[]Import{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", TypeOnly: true}}}},
			[]Export{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", TypeOnly: true}}}}},
		{"re-export star", "export * from 'mod'",
			[]Import{{From: "mod", Symbols: []Symbol{{Kind: NamespaceSymbol}}}},
			[]Export{{From: "mod", Symbols: []Symbol{{Kind: NamespaceSymbol}}}}},
		{"re-export star as namespace", "export * as ns from 'mod'",
			[]Import{{From: "mod", Symbols: []Symbol{{Kind: NamespaceSymbol, Name: "ns"}}}},
			[]Export{{From: "mod", Symbols: []Symbol{{Kind: NamespaceSymbol, Name: "ns"}}}}},
		{"re-export star as default", "export * as default from 'mod'",
			[]Import{{From: "mod", Symbols: []Symbol{{Kind: NamespaceSymbol, Name: "default"}}}},
			[]Export{{From: "mod", Symbols: []Symbol{{Kind: NamespaceSymbol, Name: "default"}}}}},
		{"empty named re-export", "export {} from 'mod'",
			[]Import{{From: "mod"}},
			[]Export{{From: "mod"}}},
		{"re-export type star", "export type * from 'mod'",
			[]Import{{From: "mod", Symbols: []Symbol{{Kind: NamespaceSymbol, TypeOnly: true}}}},
			[]Export{{From: "mod", Symbols: []Symbol{{Kind: NamespaceSymbol, TypeOnly: true}}}}},

		{"default export identifier", "export default foo", nil, []Export{{Symbols: []Symbol{{Kind: DefaultSymbol}}}}},
		{"default export named function", "export default function bar() {}", nil, []Export{{Symbols: []Symbol{{Kind: DefaultSymbol}}}}},
		{"default export named class", "export default class Bar {}", nil, []Export{{Symbols: []Symbol{{Kind: DefaultSymbol}}}}},
		{"default export anonymous function", "export default function() {}", nil, []Export{{Symbols: []Symbol{{Kind: DefaultSymbol}}}}},
		{"default export anonymous class", "export default class {}", nil, []Export{{Symbols: []Symbol{{Kind: DefaultSymbol}}}}},
		{"default export literal number", "export default 42", nil, []Export{{Symbols: []Symbol{{Kind: DefaultSymbol}}}}},
		{"default export object literal", "export default { a: 1 }", nil, []Export{{Symbols: []Symbol{{Kind: DefaultSymbol}}}}},
		{"default export async anonymous function", "export default async function() {}", nil, []Export{{Symbols: []Symbol{{Kind: DefaultSymbol}}}}},
		{"default export async named function", "export default async function bar() {}", nil, []Export{{Symbols: []Symbol{{Kind: DefaultSymbol}}}}},

		{"empty named clause", "export {}", nil, []Export{{}}},
		{"named export aliased to default", "export { foo as default }", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", Alias: "default"}}}}},
		{"re-export default as named", "export { default as foo } from 'mod'",
			[]Import{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "default", Alias: "foo"}}}},
			[]Export{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "default", Alias: "foo"}}}}},
		{"re-export bare default", "export { default } from 'mod'",
			[]Import{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "default"}}}},
			[]Export{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "default"}}}}},

		{"export abstract class", "export abstract class Foo {}", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "Foo"}}}}},
		{"export declare const", "export declare const x: string", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "x", TypeOnly: true}}}}},
		{"export declare class", "export declare class Foo {}", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "Foo", TypeOnly: true}}}}},
		{"export declare function", "export declare function foo(): void", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", TypeOnly: true}}}}},

		{"export const", "export const foo = 1", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}}}}},
		{"export let", "export let foo = 1", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}}}}},
		{"export var", "export var foo = 1", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}}}}},
		{"export function", "export function foo() {}", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}}}}},
		{"export async function", "export async function foo() {}", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}}}}},
		{"export class", "export class Foo {}", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "Foo"}}}}},
		{"export enum", "export enum Foo {}", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "Foo"}}}}},

		{"export type alias", "export type Foo = string", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "Foo", TypeOnly: true}}}}},
		{"export interface", "export interface Foo {}", nil, []Export{{Symbols: []Symbol{{Kind: NamedSymbol, Name: "Foo", TypeOnly: true}}}}},

		{"no spaces", "export{foo}from'mod'",
			[]Import{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}}}},
			[]Export{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}}}}},
		{"newlines between tokens", "export\n{\n\tfoo,\n\tbar,\n}\nfrom\n'mod'",
			[]Import{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}, {Kind: NamedSymbol, Name: "bar"}}}},
			[]Export{{From: "mod", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo"}, {Kind: NamedSymbol, Name: "bar"}}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := &scanner{src: []byte(tt.input)}
			imports, exports, err := scanner.scan()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(imports, tt.expectedImports) {
				t.Errorf("imports: expected %+v, got %+v", tt.expectedImports, imports)
			}
			if !reflect.DeepEqual(exports, tt.expectedExports) {
				t.Errorf("exports: expected %+v, got %+v", tt.expectedExports, exports)
			}
		})
	}
}

func TestScannerNonExports(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"line comment", "// export { foo }"},
		{"block comment", "/* export { foo } from 'mod' */"},
		{"export in double-quoted string", `const s = "export { foo }"`},
		{"export in single-quoted string", `const s = 'export { foo }'`},
		{"export in template literal", "const s = `export { foo }`"},
		{"exported identifier", "let exported = 1"},
		{"exporter identifier", "function exporter() {}"},
		{"exports identifier", "let exports = {}"},
		{"reexport identifier", "const reexport = 1"},
		{"property access export", "obj.export = 1"},
		{"property access export with chain", "foo.export.bar"},
		{"empty source", ""},
		{"no exports at all", "const x = 1\nfunction y() { return x }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := &scanner{src: []byte(tt.input)}
			imports, exports, err := scanner.scan()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(exports) != 0 {
				t.Errorf("expected 0 exports, got %d: %+v", len(exports), exports)
			}
			if len(imports) != 0 {
				t.Errorf("expected 0 imports, got %d: %+v", len(imports), imports)
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
		{"unterminated named brace", "import { foo"},

		{"export missing from after star", "export * 'mod'"},
		{"export star as missing identifier", "export * as from 'mod'"},
		{"export unterminated named brace", "export { foo"},
		{"export unterminated string", "export { foo } from 'mod"},
		{"export star unterminated string", "export * from 'mod"},
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
		"export const localConst = 1\n" +
		"export function helper() {}\n" +
		"export default function main() {}\n" +
		"export { foo as bar } from './default'\n" +
		"export * from './all'\n" +
		"export type { Foo } from './types'\n" +
		"\n" +
		"const msg = `template ${import('dynamic')} ${`nested ${1}`} end`\n" +
		"const s1 = \"import fake from 'nope'\"\n" +
		"const s2 = 'import fake from \"nope\"'\n" +
		"const s3 = \"export { fake } from 'nope'\"\n" +
		"/* import fake from 'nope' */\n" +
		"/* export { fake } from 'nope' */\n" +
		"obj.import('not-an-import')\n" +
		"\n" +
		"import { last1, last2, } from 'trailing'\n" +
		"export { localConst, helper }\n"

	expectedImports := []Import{
		{From: "side-effect"},
		{From: "./default", Symbols: []Symbol{{Kind: DefaultSymbol, Name: "foo"}}},
		{From: "namespace", Symbols: []Symbol{{Kind: NamespaceSymbol, Name: "ns"}}},
		{From: "./named", Symbols: []Symbol{
			{Kind: NamedSymbol, Name: "a"},
			{Kind: NamedSymbol, Name: "b", Alias: "bAlias"},
			{Kind: NamedSymbol, Name: "c", TypeOnly: true},
		}},
		{From: "types", Symbols: []Symbol{
			{Kind: NamedSymbol, Name: "D", TypeOnly: true},
			{Kind: NamedSymbol, Name: "E", TypeOnly: true},
		}},
		{From: "mixed", Symbols: []Symbol{
			{Kind: DefaultSymbol, Name: "Def", TypeOnly: true},
			{Kind: NamedSymbol, Name: "Mixed", TypeOnly: true},
		}},
		{From: "./default", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", Alias: "bar"}}},
		{From: "./all", Symbols: []Symbol{{Kind: NamespaceSymbol}}},
		{From: "./types", Symbols: []Symbol{{Kind: NamedSymbol, Name: "Foo", TypeOnly: true}}},
		{From: "dynamic", Dynamic: true},
		{From: "trailing", Symbols: []Symbol{
			{Kind: NamedSymbol, Name: "last1"},
			{Kind: NamedSymbol, Name: "last2"},
		}},
	}

	expectedExports := []Export{
		{Symbols: []Symbol{{Kind: NamedSymbol, Name: "localConst"}}},
		{Symbols: []Symbol{{Kind: NamedSymbol, Name: "helper"}}},
		{Symbols: []Symbol{{Kind: DefaultSymbol}}},
		{From: "./default", Symbols: []Symbol{{Kind: NamedSymbol, Name: "foo", Alias: "bar"}}},
		{From: "./all", Symbols: []Symbol{{Kind: NamespaceSymbol}}},
		{From: "./types", Symbols: []Symbol{{Kind: NamedSymbol, Name: "Foo", TypeOnly: true}}},
		{Symbols: []Symbol{
			{Kind: NamedSymbol, Name: "localConst"},
			{Kind: NamedSymbol, Name: "helper"},
		}},
	}

	scanner := &scanner{src: []byte(src)}
	imports, exports, err := scanner.scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(imports) != len(expectedImports) {
		t.Fatalf("expected %d imports, got %d: %+v", len(expectedImports), len(imports), imports)
	}
	for i, want := range expectedImports {
		if !reflect.DeepEqual(imports[i], want) {
			t.Errorf("import %d:\n  expected %+v\n  got      %+v", i, want, imports[i])
		}
	}

	if len(exports) != len(expectedExports) {
		t.Fatalf("expected %d exports, got %d: %+v", len(expectedExports), len(exports), exports)
	}
	for i, want := range expectedExports {
		if !reflect.DeepEqual(exports[i], want) {
			t.Errorf("export %d:\n  expected %+v\n  got      %+v", i, want, exports[i])
		}
	}
}
