package semdiff

import (
	"strings"

	"github.com/odvcencio/gotreesitter/grammars/bash"
	"github.com/odvcencio/gotreesitter/grammars/c"
	"github.com/odvcencio/gotreesitter/grammars/cmake"
	"github.com/odvcencio/gotreesitter/grammars/cpp"
	"github.com/odvcencio/gotreesitter/grammars/dart"
	"github.com/odvcencio/gotreesitter/grammars/dockerfile"
	"github.com/odvcencio/gotreesitter/grammars/elixir"
	"github.com/odvcencio/gotreesitter/grammars/git_config"
	"github.com/odvcencio/gotreesitter/grammars/gomod"
	"github.com/odvcencio/gotreesitter/grammars/graphql"
	"github.com/odvcencio/gotreesitter/grammars/groovy"
	"github.com/odvcencio/gotreesitter/grammars/haskell"
	"github.com/odvcencio/gotreesitter/grammars/hcl"
	"github.com/odvcencio/gotreesitter/grammars/ini"
	"github.com/odvcencio/gotreesitter/grammars/kotlin"
	"github.com/odvcencio/gotreesitter/grammars/less"
	makefile "github.com/odvcencio/gotreesitter/grammars/make"
	"github.com/odvcencio/gotreesitter/grammars/markdown"
	"github.com/odvcencio/gotreesitter/grammars/markdown_inline"
	"github.com/odvcencio/gotreesitter/grammars/nix"
	"github.com/odvcencio/gotreesitter/grammars/objc"
	"github.com/odvcencio/gotreesitter/grammars/perl"
	"github.com/odvcencio/gotreesitter/grammars/php"
	"github.com/odvcencio/gotreesitter/grammars/powershell"
	"github.com/odvcencio/gotreesitter/grammars/proto"
	"github.com/odvcencio/gotreesitter/grammars/r"
	"github.com/odvcencio/gotreesitter/grammars/regex"
	"github.com/odvcencio/gotreesitter/grammars/ruby"
	"github.com/odvcencio/gotreesitter/grammars/scala"
	"github.com/odvcencio/gotreesitter/grammars/sql"
	"github.com/odvcencio/gotreesitter/grammars/svelte"
	"github.com/odvcencio/gotreesitter/grammars/swift"
	"github.com/odvcencio/gotreesitter/grammars/toml"
	"github.com/odvcencio/gotreesitter/grammars/yaml"
	"github.com/odvcencio/gotreesitter/grammars/zig"
)

// The languages #965 added: the ones asked for (YAML, shell, PowerShell, Markdown and regular
// expressions), and the common ones from tree-sitter's list of parsers that gotreesitter
// ships under a permissive licence. Strings in these are compared as written unless a rule
// below reads them; whitespace inside one, a heredoc, a block scalar or a code block always
// counts. Node names come from each grammar, checked against the corpus.

var cFamily = rules{
	numbers:  []string{"number_literal"},
	comments: []string{"comment"},
	atoms:    []string{"string_literal", "char_literal", "system_lib_string", "raw_string_literal", "concatenated_string", "user_defined_literal"},
	idents:   []string{"identifier", "field_identifier", "type_identifier", "statement_identifier", "namespace_identifier", "method_identifier"},
	// Only lists and enums: in a macro call, F(a,) passes an empty second argument.
	commas: []string{"initializer_list", "enumerator_list"},
}

var moreLanguages = []*language{
	{id: "yaml", name: "YAML", grammar: yaml.Language, exts: []string{".yaml", ".yml"}, files: []string{".clang-format", ".clang-tidy"}, rules: rules{
		comments: []string{"comment"},
		// Plain scalars are compared as written: yes and 'yes' aren't the same value.
		strings: []string{"double_quote_scalar", "single_quote_scalar"},
		atoms:   []string{"block_scalar"},
	}},
	{id: "bash", name: "Shell", grammar: bash.Language,
		exts:    []string{".sh", ".bash", ".zsh", ".ksh", ".bats", ".command"},
		files:   []string{".bashrc", ".bash_profile", ".bash_login", ".bash_logout", ".bash_aliases", ".zshrc", ".zshenv", ".zprofile", ".zlogin", ".zlogout", ".profile", ".kshrc", "PKGBUILD", "APKBUILD"},
		interps: []string{"sh", "bash", "zsh", "ksh", "dash", "ash", "mksh"},
		rules: rules{
			comments: []string{"comment"},
			strings:  []string{"raw_string", "string", "ansi_c_string"},
			interps:  []string{"simple_expansion", "expansion", "command_substitution", "arithmetic_expansion"},
			// A heredoc's whitespace is its content, from the first line's indentation on.
			atoms:  []string{"heredoc_redirect"},
			idents: []string{"variable_name"},
			semis:  true,
		}},
	{id: "powershell", name: "PowerShell", grammar: powershell.Language, exts: []string{".ps1", ".psm1", ".psd1"}, interps: []string{"pwsh", "powershell"}, rules: rules{
		comments: []string{"comment"},
		numbers:  []string{"decimal_integer_literal", "hexadecimal_integer_literal", "real_literal"},
		strings:  []string{"verbatim_string_characters", "verbatim_here_string_characters"},
		atoms:    []string{"expandable_string_literal", "expandable_here_string_literal"},
		idents:   []string{"variable", "function_name", "simple_name"},
		semis:    true,
		fold:     true,
	}},
	{id: "markdown", name: "Markdown", grammar: markdown.Language, exts: []string{".md", ".markdown", ".mdown", ".mkd", ".mkdn"}, rules: rules{
		injects: map[string]string{"inline": "markdown_inline", "pipe_table_cell": "markdown_inline"},
		fences:  []string{"fenced_code_block"},
		atoms:   []string{"indented_code_block", "html_block", "link_destination", "link_title"},
		keys: map[string]func(string) string{
			// Any bullet makes the same list, any fence the same code block, and a table's
			// delimiter row says only how its columns align.
			"list_marker_minus":           func(string) string { return "bullet" },
			"list_marker_plus":            func(string) string { return "bullet" },
			"list_marker_star":            func(string) string { return "bullet" },
			"thematic_break":              func(string) string { return "rule" },
			"fenced_code_block_delimiter": func(string) string { return "fence" },
			"setext_h1_underline":         func(string) string { return "h1" },
			"setext_h2_underline":         func(string) string { return "h2" },
			"pipe_table_delimiter_cell":   tableAlignment,
		},
	}},
	{id: "markdown_inline", name: "Markdown", grammar: markdown_inline.Language, hidden: true, rules: rules{
		prose: true,
		atoms: []string{"code_span", "link_destination", "link_title", "uri_autolink", "email_autolink", "html_tag", "entity_reference", "numeric_character_reference", "latex_block"},
		keys: map[string]func(string) string{
			// *a* and _a_ are the same emphasis; a hard break is a hard break however written.
			"emphasis_delimiter": func(string) string { return "*" },
			"hard_line_break":    func(string) string { return "break" },
		},
	}},
	{id: "regex", name: "Regular expression", grammar: regex.Language, hidden: true, rules: rules{exact: true}},

	{id: "c", name: "C", grammar: c.Language, exts: []string{".c", ".h"}, rules: cFamily},
	{id: "cpp", name: "C++", grammar: cpp.Language, exts: []string{".cc", ".cpp", ".cxx", ".c++", ".hpp", ".hh", ".hxx", ".h++", ".ipp", ".tpp", ".inl"}, rules: cFamily},
	{id: "objc", name: "Objective-C", grammar: objc.Language, exts: []string{".m"}, rules: cFamily},
	{id: "php", name: "PHP", grammar: php.Language, exts: []string{".php", ".phtml", ".php5", ".php7", ".phpt"}, interps: []string{"php"}, rules: rules{
		numbers:        []string{"integer", "float"},
		comments:       []string{"comment"},
		atoms:          []string{"encapsed_string", "string", "heredoc", "nowdoc", "shell_command_expression"},
		idents:         []string{"name"},
		trailingCommas: true,
	}},
	{id: "ruby", name: "Ruby", grammar: ruby.Language, exts: []string{".rb", ".rake", ".gemspec", ".ru", ".rbw", ".jbuilder"}, files: []string{"Rakefile", "Gemfile", "Guardfile", "Podfile", "Fastfile", "Vagrantfile", "Brewfile", "Dangerfile"}, interps: []string{"ruby"}, rules: rules{
		numbers:        []string{"integer", "float"},
		comments:       []string{"comment"},
		atoms:          []string{"string", "heredoc_body", "regex", "string_array", "symbol_array", "character", "subshell", "simple_symbol", "delimited_symbol"},
		idents:         []string{"identifier", "constant", "instance_variable", "class_variable", "global_variable"},
		trailingCommas: true,
		semis:          true,
	}},
	{id: "kotlin", name: "Kotlin", grammar: kotlin.Language, exts: []string{".kt", ".kts"}, rules: rules{
		numbers:        []string{"integer_literal", "hex_literal", "bin_literal", "real_literal", "long_literal"},
		comments:       []string{"line_comment", "multiline_comment"},
		atoms:          []string{"string_literal", "character_literal", "unsigned_literal"},
		idents:         []string{"simple_identifier", "identifier", "type_identifier"},
		trailingCommas: true,
		semis:          true,
	}},
	{id: "swift", name: "Swift", grammar: swift.Language, exts: []string{".swift"}, rules: rules{
		numbers:        []string{"integer_literal", "hex_literal", "oct_literal", "bin_literal", "real_literal"},
		comments:       []string{"comment", "multiline_comment"},
		atoms:          []string{"line_string_literal", "multi_line_string_literal", "raw_string_literal", "regex_literal"},
		idents:         []string{"simple_identifier", "type_identifier"},
		trailingCommas: true,
		semis:          true,
	}},
	{id: "dart", name: "Dart", grammar: dart.Language, exts: []string{".dart"}, rules: rules{
		numbers:        []string{"decimal_integer_literal", "hex_integer_literal", "decimal_floating_point_literal"},
		comments:       []string{"comment", "documentation_comment"},
		atoms:          []string{"string_literal", "symbol_literal"},
		idents:         []string{"identifier", "type_identifier"},
		trailingCommas: true,
	}},
	{id: "scala", name: "Scala", grammar: scala.Language, exts: []string{".scala", ".sc", ".sbt"}, rules: rules{
		numbers:        []string{"integer_literal", "floating_point_literal"},
		comments:       []string{"comment", "block_comment"},
		atoms:          []string{"string", "interpolated_string_expression", "character_literal", "symbol_literal"},
		idents:         []string{"identifier", "type_identifier"},
		trailingCommas: true,
		semis:          true,
	}},
	{id: "perl", name: "Perl", grammar: perl.Language, exts: []string{".pl", ".pm", ".t"}, interps: []string{"perl"}, rules: rules{
		numbers:        []string{"number"},
		comments:       []string{"comment"},
		atoms:          []string{"string_literal", "interpolated_string_literal", "command_string", "heredoc_content", "heredoc_token", "pod", "quoted_regexp", "match_regexp", "substitution_regexp", "transliteration_expression"},
		idents:         []string{"varname", "identifier"},
		trailingCommas: true,
	}},
	{id: "r", name: "R", grammar: r.Language, exts: []string{".r"}, interps: []string{"rscript"}, rules: rules{
		comments: []string{"comment"},
		// 1L keeps its L outside the tree's leaves, so numbers are read whole, as written.
		atoms:  []string{"string", "integer", "float"},
		idents: []string{"identifier"},
		semis:  true,
	}},
	{id: "elixir", name: "Elixir", grammar: elixir.Language, exts: []string{".ex", ".exs"}, interps: []string{"elixir"}, rules: rules{
		numbers:        []string{"integer", "float"},
		comments:       []string{"comment"},
		atoms:          []string{"string", "charlist", "sigil", "char"},
		idents:         []string{"identifier"},
		trailingCommas: true,
		semis:          true,
	}},
	{id: "haskell", name: "Haskell", grammar: haskell.Language, exts: []string{".hs"}, interps: []string{"runghc", "runhaskell"}, rules: rules{
		numbers:  []string{"integer", "float"},
		comments: []string{"comment", "haddock", "pragma"},
		atoms:    []string{"string", "char", "quasiquote"},
		idents:   []string{"variable", "name", "constructor"},
	}},
	{id: "zig", name: "Zig", grammar: zig.Language, exts: []string{".zig", ".zon"}, rules: rules{
		numbers:        []string{"integer", "float"},
		comments:       []string{"comment"},
		atoms:          []string{"string", "multiline_string", "character"},
		idents:         []string{"identifier"},
		trailingCommas: true,
	}},
	{id: "groovy", name: "Groovy", grammar: groovy.Language, exts: []string{".groovy", ".gradle", ".gvy", ".gy"}, files: []string{"Jenkinsfile"}, interps: []string{"groovy"}, rules: rules{
		comments:       []string{"comment"},
		atoms:          []string{"string", "number_literal"},
		idents:         []string{"identifier"},
		trailingCommas: true,
		semis:          true,
	}},
	{id: "sql", name: "SQL", grammar: sql.Language, exts: []string{".sql"}, rules: rules{
		comments: []string{"comment", "marginalia"},
		atoms:    []string{"string", "literal"},
		idents:   []string{"identifier"},
	}},
	{id: "toml", name: "TOML", grammar: toml.Language, exts: []string{".toml"}, files: []string{"Cargo.lock", "Pipfile", "poetry.lock", "uv.lock"}, rules: rules{
		numbers:        []string{"integer", "float"},
		strings:        []string{"string"},
		comments:       []string{"comment"},
		trailingCommas: true,
	}},
	{id: "ini", name: "INI", grammar: ini.Language, exts: []string{".ini"}, files: []string{".editorconfig", "setup.cfg", ".npmrc", ".pylintrc", ".flake8"}, rules: rules{
		comments: []string{"comment"},
		// key = value and key=value say the same: a value is read without the spaces around it.
		keys: map[string]func(string) string{"setting_value": strings.TrimSpace},
	}},
	{id: "dockerfile", name: "Dockerfile", grammar: dockerfile.Language, exts: []string{".dockerfile", ".containerfile"}, files: []string{"Dockerfile", "Containerfile", "Dockerfile.*", "Containerfile.*"}, rules: rules{
		comments: []string{"comment"},
		atoms:    []string{"double_quoted_string", "single_quoted_string", "json_string", "param", "image_spec", "heredoc_block", "shell_command"},
	}},
	{id: "make", name: "Makefile", grammar: makefile.Language, exts: []string{".mk", ".mak", ".make"}, files: []string{"Makefile", "makefile", "GNUmakefile"}, interps: []string{"make"}, rules: rules{
		comments: []string{"comment"},
		// A recipe is shell, its text exact.
		atoms: []string{"recipe_line", "shell_text", "string", "raw_text"},
	}},
	{id: "cmake", name: "CMake", grammar: cmake.Language, exts: []string{".cmake"}, files: []string{"CMakeLists.txt"}, rules: rules{
		comments: []string{"line_comment", "bracket_comment"},
		atoms:    []string{"quoted_argument", "bracket_argument"},
		idents:   []string{"identifier"},
	}},
	{id: "hcl", name: "HCL", grammar: hcl.Language, exts: []string{".tf", ".tfvars", ".hcl", ".nomad"}, rules: rules{
		comments:       []string{"comment"},
		atoms:          []string{"quoted_template", "heredoc_template", "numeric_lit"},
		idents:         []string{"identifier"},
		trailingCommas: true,
	}},
	{id: "nix", name: "Nix", grammar: nix.Language, exts: []string{".nix"}, rules: rules{
		numbers:  []string{"integer_expression", "float_expression"},
		comments: []string{"comment"},
		atoms:    []string{"string_expression", "indented_string_expression", "path_expression", "uri_expression"},
		idents:   []string{"identifier"},
	}},
	{id: "proto", name: "Protocol Buffers", grammar: proto.Language, exts: []string{".proto"}, rules: rules{
		numbers:  []string{"decimal_lit", "hex_lit", "octal_lit", "float_lit"},
		comments: []string{"comment"},
		atoms:    []string{"string"},
		idents:   []string{"identifier"},
	}},
	{id: "graphql", name: "GraphQL", grammar: graphql.Language, exts: []string{".graphql", ".gql", ".graphqls"}, rules: rules{
		numbers:  []string{"int_value", "float_value"},
		comments: []string{"comment"},
		atoms:    []string{"string_value"},
		idents:   []string{"name"},
		// GraphQL reads commas as whitespace.
		optional: []string{","},
	}},
	{id: "less", name: "LESS", grammar: less.Language, exts: []string{".less"}, rules: rules{
		numbers:  []string{"integer_value", "float_value"},
		strings:  []string{"string_value"},
		comments: []string{"comment", "js_comment", "single_line_comment"},
		atoms:    []string{"color_value"},
		commas:   []string{"block"},
	}},
	{id: "svelte", name: "Svelte", grammar: svelte.Language, exts: []string{".svelte"}, rules: rules{
		strings:  []string{"quoted_attribute_value", "attribute_value"},
		comments: []string{"comment"},
		atoms:    []string{"entity", "raw_text"},
		texts:    []string{"text"},
		tags:     []string{"start_tag", "end_tag", "self_closing_tag"},
		markup:   true,
		scripts:  []string{"script_element", "style_element"},
		injects:  map[string]string{"svelte_raw_text": "javascript"},
	}},
	{id: "gomod", name: "Go module", grammar: gomod.Language, files: []string{"go.mod"}, rules: rules{
		comments: []string{"comment"},
		strings:  []string{"interpreted_string_literal", "raw_string_literal"},
	}},
	{id: "gitconfig", name: "Git config", grammar: git_config.Language, exts: []string{".gitconfig"}, files: []string{".gitconfig", ".gitmodules", ".lfsconfig"}, rules: rules{
		comments: []string{"comment"},
		atoms:    []string{"string"},
	}},
}

func init() {
	languages = append(languages, moreLanguages...)
	for name, id := range map[string]string{
		"shell": "bash", "bash": "bash", "sh": "bash", "zsh": "bash", "shellscript": "bash",
		"yaml": "yaml", "yml": "yaml",
		"powershell": "powershell", "ps1": "powershell", "pwsh": "powershell",
		"markdown": "markdown", "md": "markdown",
		"c": "c", "c++": "cpp", "cpp": "cpp", "objective-c": "objc", "objc": "objc",
		"php": "php", "ruby": "ruby", "rb": "ruby", "kotlin": "kotlin", "kt": "kotlin", "swift": "swift",
		"dart": "dart", "scala": "scala", "perl": "perl", "r": "r", "elixir": "elixir", "haskell": "haskell",
		"zig": "zig", "groovy": "groovy", "gradle": "groovy", "sql": "sql", "toml": "toml", "ini": "ini",
		"dockerfile": "dockerfile", "docker": "dockerfile", "makefile": "make", "make": "make", "cmake": "cmake",
		"hcl": "hcl", "terraform": "hcl", "nix": "nix", "protocol buffer": "proto", "protocol buffers": "proto", "proto": "proto",
		"graphql": "graphql", "less": "less", "svelte": "svelte", "go module": "gomod", "git config": "gitconfig",
	} {
		linguistNames[name] = id
	}
	// The first languages' programs, for #! lines.
	programs := map[string][]string{
		"python":     {"python", "pypy"},
		"javascript": {"node", "nodejs"},
		"typescript": {"ts-node", "deno"},
		"lua":        {"lua", "luajit"},
	}
	for _, l := range languages {
		l.interps = append(l.interps, programs[l.id]...)
	}
}

// tableAlignment is what a Markdown table's delimiter cell says: how its column aligns,
// whatever its dashes.
func tableAlignment(s string) string {
	s = strings.TrimSpace(s)
	left, right := strings.HasPrefix(s, ":"), strings.HasSuffix(s, ":")
	switch {
	case left && right:
		return "center"
	case left:
		return "left"
	case right:
		return "right"
	}
	return "none"
}

// fenceLanguage is the language a fenced code block's info string names, such as "go" or
// "python", or "".
func fenceLanguage(info string) string {
	f := strings.Fields(info)
	if len(f) == 0 {
		return ""
	}
	name := strings.ToLower(strings.Trim(f[0], "{}."))
	if id, ok := linguistNames[name]; ok {
		return id
	}
	return LanguageOf("code."+name, "", "")
}
