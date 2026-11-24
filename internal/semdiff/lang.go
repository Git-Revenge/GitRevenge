package semdiff

import (
	"hash/fnv"
	"path"
	"slices"
	"strings"
	"sync"

	ts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars/c_sharp"
	"github.com/odvcencio/gotreesitter/grammars/css"
	golang "github.com/odvcencio/gotreesitter/grammars/go"
	"github.com/odvcencio/gotreesitter/grammars/html"
	"github.com/odvcencio/gotreesitter/grammars/java"
	"github.com/odvcencio/gotreesitter/grammars/javascript"
	"github.com/odvcencio/gotreesitter/grammars/json"
	"github.com/odvcencio/gotreesitter/grammars/lua"
	"github.com/odvcencio/gotreesitter/grammars/python"
	"github.com/odvcencio/gotreesitter/grammars/rust"
	"github.com/odvcencio/gotreesitter/grammars/scss"
	"github.com/odvcencio/gotreesitter/grammars/tsx"
	"github.com/odvcencio/gotreesitter/grammars/typescript"
	"github.com/odvcencio/gotreesitter/grammars/vue"
	"github.com/odvcencio/gotreesitter/grammars/xml"
)

// language is one language read as code: its grammar, the files it's for, and which of its
// grammar's nodes are literals, names and optional punctuation. Node names are the
// grammars' own, as gotreesitter builds them (see the corpus test).
type language struct {
	id      string // the key for Options.Language
	name    string // what the views call it
	grammar func() *ts.Language
	exts    []string
	rules   rules

	once  sync.Once
	lang  *ts.Language
	table []nodeRule // by symbol
	names []uint64   // a hash of each symbol's name, by symbol
}

// rules list a grammar's node types by what they are to the token diff.
type rules struct {
	numbers  []string // read as one token, keyed by value
	strings  []string // read as one token, keyed by value, unless they hold interpolations
	comments []string // read as one token, with runs of whitespace counting as one space
	atoms    []string // read as one token, keyed as written
	idents   []string // names, which a rename changes
	texts    []string // markup prose: its words are tokens, its whitespace counts only by presence
	tags     []string // markup tags, inside which whitespace doesn't count
	interps  []string // inside a string node, makes it code rather than one token

	parens         string   // the parenthesized expression
	parensAnywhere []string // nodes whose parenthesized children can lose their parentheses
	parensLast     []string // nodes whose last child can lose its parentheses (assignments)
	keepParens     []string // nodes whose parentheses always count, as JavaScript's comma operator's do

	commas []string // nodes whose last item can carry an optional trailing comma
	semis  bool     // a ; between statements is optional
	markup bool     // whitespace in content counts by its presence

	scripts []string // elements whose raw text is code in another language
	exprs   []string // markup nodes holding a JavaScript expression (Vue's {{ }} and v- values)
}

// nodeKind says how the token reader treats a node.
type nodeKind uint8

const (
	nodePlain nodeKind = iota
	nodeNumber
	nodeString
	nodeComment
	nodeAtom
	nodeIdent
	nodeText
	nodeTag
	nodeParens
	nodeScript
	nodeExpr
)

type nodeRule struct {
	kind       nodeKind
	interp     bool  // an interpolation inside a string
	commas     bool  // its trailing comma is optional
	parens     uint8 // 1: any parenthesized child can lose its parentheses; 2: only its last
	keepParens bool
}

var langJS = rules{
	numbers:        []string{"number"},
	strings:        []string{"string", "template_string"},
	comments:       []string{"comment", "html_comment"},
	atoms:          []string{"regex", "jsx_text", "hash_bang_line"},
	idents:         []string{"identifier", "property_identifier", "shorthand_property_identifier", "shorthand_property_identifier_pattern", "private_property_identifier", "type_identifier", "statement_identifier"},
	interps:        []string{"template_substitution"},
	parens:         "parenthesized_expression",
	parensAnywhere: []string{"arguments", "array", "return_statement", "variable_declarator", "if_statement", "while_statement", "do_statement", "switch_statement", "parenthesized_expression", "template_substitution", "jsx_expression", "spread_element"},
	parensLast:     []string{"assignment_expression", "pair"},
	keepParens:     []string{"sequence_expression"},
	commas:         []string{"arguments", "formal_parameters", "array", "object", "named_imports", "export_clause", "object_pattern", "array_pattern", "enum_body"},
	semis:          true,
}

var languages = []*language{
	{id: "go", name: "Go", grammar: golang.Language, exts: []string{".go"}, rules: rules{
		numbers:        []string{"int_literal", "float_literal", "imaginary_literal"},
		strings:        []string{"interpreted_string_literal", "raw_string_literal", "rune_literal"},
		comments:       []string{"comment"},
		idents:         []string{"identifier", "field_identifier", "type_identifier", "package_identifier", "label_name"},
		parens:         "parenthesized_expression",
		parensAnywhere: []string{"expression_list", "argument_list", "literal_element", "if_statement", "expression_switch_statement", "parenthesized_expression", "return_statement"},
		commas:         []string{"argument_list", "parameter_list", "literal_value", "type_parameter_list", "type_arguments"},
		semis:          true,
	}},
	{id: "javascript", name: "JavaScript", grammar: javascript.Language, exts: []string{".js", ".mjs", ".cjs", ".jsx"}, rules: langJS},
	{id: "typescript", name: "TypeScript", grammar: typescript.Language, exts: []string{".ts", ".mts", ".cts"}, rules: langJS},
	{id: "tsx", name: "TypeScript", grammar: tsx.Language, exts: []string{".tsx"}, rules: langJS},
	{id: "python", name: "Python", grammar: python.Language, exts: []string{".py", ".pyi", ".pyw"}, rules: rules{
		numbers:        []string{"integer", "float"},
		strings:        []string{"string"},
		comments:       []string{"comment"},
		idents:         []string{"identifier"},
		interps:        []string{"interpolation"},
		parens:         "parenthesized_expression",
		parensAnywhere: []string{"argument_list", "list", "set", "return_statement", "if_statement", "elif_clause", "while_statement", "parenthesized_expression"},
		parensLast:     []string{"assignment", "augmented_assignment", "keyword_argument", "pair"},
		keepParens:     []string{"yield", "named_expression"},
		commas:         []string{"argument_list", "parameters", "list", "set", "dictionary"},
		semis:          true,
	}},
	{id: "csharp", name: "C#", grammar: c_sharp.Language, exts: []string{".cs", ".csx"}, rules: rules{
		numbers:        []string{"integer_literal", "real_literal"},
		strings:        []string{"string_literal", "verbatim_string_literal", "raw_string_literal", "character_literal"},
		comments:       []string{"comment"},
		idents:         []string{"identifier"},
		parens:         "parenthesized_expression",
		parensAnywhere: []string{"argument", "return_statement", "if_statement", "while_statement", "parenthesized_expression", "equals_value_clause", "arrow_expression_clause"},
		parensLast:     []string{"assignment_expression"},
		commas:         []string{"initializer_expression", "enum_member_declaration_list", "collection_expression"},
	}},
	{id: "java", name: "Java", grammar: java.Language, exts: []string{".java"}, rules: rules{
		numbers:        []string{"decimal_integer_literal", "hex_integer_literal", "octal_integer_literal", "binary_integer_literal", "decimal_floating_point_literal", "hex_floating_point_literal"},
		strings:        []string{"string_literal", "character_literal"},
		comments:       []string{"line_comment", "block_comment"},
		idents:         []string{"identifier", "type_identifier"},
		parens:         "parenthesized_expression",
		parensAnywhere: []string{"argument_list", "return_statement", "if_statement", "while_statement", "parenthesized_expression", "array_initializer"},
		parensLast:     []string{"assignment_expression", "variable_declarator"},
		commas:         []string{"array_initializer", "element_value_array_initializer"},
	}},
	{id: "css", name: "CSS", grammar: css.Language, exts: []string{".css"}, rules: rules{
		numbers:  []string{"integer_value", "float_value"},
		strings:  []string{"string_value"},
		comments: []string{"comment"},
		atoms:    []string{"color_value"},
		// The last declaration's semicolon in a block is optional.
		commas: []string{"block"},
	}},
	{id: "scss", name: "SCSS", grammar: scss.Language, exts: []string{".scss"}, rules: rules{
		numbers:  []string{"integer_value", "float_value"},
		strings:  []string{"string_value"},
		comments: []string{"comment", "js_comment", "single_line_comment"},
		atoms:    []string{"color_value"},
		// The last declaration's semicolon in a block is optional.
		commas: []string{"block"},
	}},
	{id: "json", name: "JSON", grammar: json.Language, exts: []string{".json"}, rules: rules{
		numbers:  []string{"number"},
		strings:  []string{"string"},
		comments: []string{"comment"},
	}},
	{id: "rust", name: "Rust", grammar: rust.Language, exts: []string{".rs"}, rules: rules{
		numbers:        []string{"integer_literal", "float_literal"},
		strings:        []string{"string_literal", "raw_string_literal", "char_literal"},
		comments:       []string{"line_comment", "block_comment"},
		idents:         []string{"identifier", "field_identifier", "type_identifier", "shorthand_field_identifier"},
		parens:         "parenthesized_expression",
		parensAnywhere: []string{"arguments", "return_expression", "if_expression", "while_expression", "match_expression", "array_expression", "parenthesized_expression"},
		parensLast:     []string{"assignment_expression", "compound_assignment_expr", "field_initializer", "let_declaration"},
		commas:         []string{"arguments", "parameters", "array_expression", "field_initializer_list", "field_declaration_list", "enum_variant_list", "use_list", "type_arguments", "type_parameters"},
	}},
	{id: "vue", name: "Vue", grammar: vue.Language, exts: []string{".vue"}, rules: rules{
		strings:  []string{"quoted_attribute_value", "attribute_value"},
		comments: []string{"comment"},
		atoms:    []string{"entity", "raw_text"},
		texts:    []string{"text"},
		tags:     []string{"start_tag", "end_tag", "self_closing_tag"},
		markup:   true,
		scripts:  []string{"script_element", "style_element"},
		exprs:    []string{"interpolation", "directive_attribute"},
	}},
	{id: "html", name: "HTML", grammar: html.Language, exts: []string{".html", ".htm", ".xhtml"}, rules: rules{
		strings:  []string{"quoted_attribute_value", "attribute_value"},
		comments: []string{"comment"},
		atoms:    []string{"entity", "raw_text"},
		texts:    []string{"text"},
		tags:     []string{"start_tag", "end_tag", "self_closing_tag", "doctype"},
		markup:   true,
		scripts:  []string{"script_element", "style_element"},
	}},
	{id: "lua", name: "Lua", grammar: lua.Language, exts: []string{".lua"}, rules: rules{
		numbers:  []string{"number"},
		strings:  []string{"string"},
		comments: []string{"comment"},
		idents:   []string{"identifier"},
		// No optional parentheses: in Lua, (f()) keeps only f's first result.
		commas: []string{"table_constructor"},
		semis:  true,
	}},
	{id: "xml", name: "XML", grammar: xml.Language, exts: []string{".xml", ".xsd", ".xsl", ".xslt", ".svg", ".csproj", ".vbproj", ".fsproj", ".props", ".targets", ".resx", ".xaml", ".plist", ".nuspec", ".wsdl"}, rules: rules{
		strings:  []string{"AttValue"},
		comments: []string{"Comment"},
		atoms:    []string{"CDSect", "CharRef", "EntityRef", "PI"},
		texts:    []string{"CharData"},
		tags:     []string{"STag", "ETag", "EmptyElemTag", "XMLDecl", "doctypedecl"},
		markup:   true,
	}},
}

// byID finds a language by its id, or nil.
func byID(id string) *language {
	for _, l := range languages {
		if l.id == id {
			return l
		}
	}
	return nil
}

// Languages are the names of the languages read as code, for the settings to list.
func Languages() []string {
	var out []string
	for _, l := range languages {
		if !slices.Contains(out, l.name) {
			out = append(out, l.name)
		}
	}
	return out
}

// LanguageName is what the views call the language with id, or "".
func LanguageName(id string) string {
	if l := byID(id); l != nil {
		return l.name
	}
	return ""
}

// linguistNames are linguist-language values, lower-cased, and git's built-in diff drivers,
// by the language they mean.
var linguistNames = map[string]string{
	"go": "go", "golang": "go",
	"javascript": "javascript", "js": "javascript", "jsx": "javascript",
	"typescript": "typescript", "ts": "typescript", "tsx": "tsx",
	"python": "python",
	"c#":     "csharp", "csharp": "csharp",
	"java": "java",
	"css":  "css", "scss": "scss",
	"json": "json",
	"rust": "rust",
	"vue":  "vue",
	"html": "html",
	"lua":  "lua",
	"xml":  "xml",
}

// LanguageOf is the language a file is read as: the one its linguist-language attribute
// names, or else its diff attribute (git's own drivers, such as golang), or else its file
// name. It returns "" for a file not read as code; a linguist-language naming another
// language means the file isn't read as code, whatever its name.
func LanguageOf(file, linguist, driver string) string {
	if linguist != "" {
		return linguistNames[strings.ToLower(linguist)]
	}
	if id, ok := linguistNames[strings.ToLower(driver)]; ok {
		return id
	}
	ext := strings.ToLower(path.Ext(file))
	for _, l := range languages {
		if slices.Contains(l.exts, ext) {
			return l.id
		}
	}
	return ""
}

// load decodes the grammar and its rule table, once.
func (l *language) load() *ts.Language {
	l.once.Do(func() {
		l.lang = l.grammar()
		r := l.rules
		l.table = make([]nodeRule, len(l.lang.SymbolNames))
		l.names = make([]uint64, len(l.lang.SymbolNames))
		for sym, name := range l.lang.SymbolNames {
			var nr nodeRule
			switch {
			case slices.Contains(r.numbers, name):
				nr.kind = nodeNumber
			case slices.Contains(r.strings, name):
				nr.kind = nodeString
			case slices.Contains(r.comments, name):
				nr.kind = nodeComment
			case slices.Contains(r.atoms, name):
				nr.kind = nodeAtom
			case slices.Contains(r.texts, name):
				nr.kind = nodeText
			case slices.Contains(r.tags, name):
				nr.kind = nodeTag
			case slices.Contains(r.scripts, name):
				nr.kind = nodeScript
			case slices.Contains(r.exprs, name):
				nr.kind = nodeExpr
			case r.parens != "" && name == r.parens:
				nr.kind = nodeParens
			case slices.Contains(r.idents, name):
				nr.kind = nodeIdent
			}
			nr.interp = slices.Contains(r.interps, name)
			nr.commas = slices.Contains(r.commas, name)
			switch {
			case slices.Contains(r.parensAnywhere, name):
				nr.parens = 1
			case slices.Contains(r.parensLast, name):
				nr.parens = 2
			}
			nr.keepParens = slices.Contains(r.keepParens, name)
			l.table[sym] = nr
			h := fnv.New64a()
			_, _ = h.Write([]byte(l.id + "/" + name))
			l.names[sym] = h.Sum64()
		}
	})
	return l.lang
}

// rule is how the reader treats nodes of symbol sym.
func (l *language) rule(sym ts.Symbol) nodeRule {
	if int(sym) < len(l.table) {
		return l.table[sym]
	}
	return nodeRule{}
}

// nameHash is a hash of symbol sym's name, which tells syntax nodes apart in a path.
func (l *language) nameHash(sym ts.Symbol) uint64 {
	if int(sym) < len(l.names) {
		return l.names[sym]
	}
	return uint64(sym)
}
