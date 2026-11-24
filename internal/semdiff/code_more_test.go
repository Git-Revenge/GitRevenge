package semdiff

import "testing"

// The languages #965 added. Each case is a pair of versions and whether the change between
// them is formatting only. The ones that aren't are where whitespace or a spelling counts.
func TestTheNewLanguagesTellFormattingFromChange(t *testing.T) {
	for _, c := range []struct {
		name, id, before, after string
		format                  bool
	}{
		// Markdown: prose rewraps, tables realign, emphasis and bullets have two spellings.
		{"a paragraph rewrapped", "markdown", "# Tides\n\nSome text that\nwraps over lines.\n", "# Tides\n\nSome text that wraps\nover lines.\n", true},
		{"a word changed", "markdown", "Some text.\n", "Some words.\n", false},
		{"a table realigned", "markdown", "| A | Height |\n|---|---:|\n| Brest | 6.81 |\n", "| A     | Height |\n| ----- | -----: |\n| Brest |   6.81 |\n", true},
		{"a column's alignment", "markdown", "| A |\n|---|\n| b |\n", "| A |\n|--:|\n| b |\n", false},
		{"emphasis spelled another way", "markdown", "Some *text* and __bold__.\n", "Some _text_ and **bold**.\n", true},
		{"bullets spelled another way", "markdown", "- one\n- two\n", "* one\n* two\n", true},
		{"a hard break added", "markdown", "one\ntwo\n", "one  \ntwo\n", false},
		{"words run together", "markdown", "one two\n", "onetwo\n", false},
		{"code spacing", "markdown", "Run `a  b` now.\n", "Run `a b` now.\n", false},
		{"Go in a fence, same value", "markdown", "```go\nconst a = 0x30\n```\n", "```go\nconst a = 48\n```\n", true},
		{"Go in a fence, changed", "markdown", "```go\nconst a = 1\n```\n", "```go\nconst a = 2\n```\n", false},
		{"a fence without a language", "markdown", "```\na  b\n```\n", "```\na b\n```\n", false},
		// YAML: indentation is structure, plain and quoted scalars differ, block scalars are text.
		{"YAML quotes", "yaml", "a: \"x\"\n", "a: 'x'\n", true},
		{"YAML plain and quoted", "yaml", "a: yes\n", "a: 'yes'\n", false},
		{"YAML flow spacing", "yaml", "a: [x,y]\n", "a: [ x, y ]\n", true},
		{"YAML nesting", "yaml", "a:\n  b: 1\nc: 2\n", "a:\n  b: 1\n  c: 2\n", false},
		{"YAML block scalar spacing", "yaml", "a: |\n  x  y\n", "a: |\n  x y\n", false},
		// Shell: quotes, separators and spacing between words don't matter; heredocs and
		// spacing inside strings do.
		{"shell quotes", "bash", "echo 'a b'\n", "echo \"a b\"\n", true},
		{"shell word spacing", "bash", "echo a  b\n", "echo a b\n", true},
		{"shell semicolons", "bash", "a; b\n", "a\nb\n", true},
		{"shell string spacing", "bash", "echo \"$a $b\"\n", "echo \"$a  $b\"\n", false},
		{"shell heredoc indent", "bash", "cat <<EOF\n  x\nEOF\n", "cat <<EOF\n    x\nEOF\n", false},
		{"shell assignment", "bash", "a=1\n", "a = 1\n", false},
		// PowerShell: names have no case, environment variables can.
		{"PowerShell case", "powershell", "Get-ChildItem -Path $Home\n", "get-childitem -path $home\n", true},
		{"PowerShell env case", "powershell", "$env:PATH\n", "$env:Path\n", false},
		{"PowerShell quotes", "powershell", "$a = 'it''s'\n", "$a = 'it''s'  \n", true},
		// Regular expressions inside JavaScript: whitespace counts.
		{"regex spacing", "javascript", "const r = /a b/\n", "const r = /a  b/\n", false},
		// C: numbers by value, trailing commas in lists, never in calls.
		{"C number", "c", "int a = 0x30;\n", "int a = 48;\n", true},
		{"C list comma", "c", "int a[] = {1, 2,};\n", "int a[] = {1, 2};\n", true},
		{"C call comma", "c", "int a = F(1,);\n", "int a = F(1);\n", false},
		{"C++ digit separators", "cpp", "int a = 1'000;\n", "int a = 1000;\n", true},
		// Others.
		{"GraphQL commas", "graphql", "query { a(x: 1, y: 2) }\n", "query { a(x: 1 y: 2) }\n", true},
		{"TOML strings", "toml", "a = \"x\"\n", "a = 'x'\n", true},
		{"TOML numbers", "toml", "a = 1_000\n", "a = 1000\n", true},
		{"INI spacing", "ini", "[s]\na=1\n", "[s]\na = 1\n", true},
		{"Ruby trailing comma", "ruby", "a = [1, 2,]\n", "a = [1, 2]\n", true},
		{"Kotlin number", "kotlin", "val a = 0x30\n", "val a = 48\n", true},
		{"Makefile recipe quotes", "make", "a:\n\techo \"x  y\"\n", "a:\n\techo \"x y\"\n", false},
		{"HCL heredoc indent", "hcl", "a = <<EOT\n  x\nEOT\n", "a = <<EOT\n    x\nEOT\n", false},
		{"Perl heredoc indent", "perl", "my $a = <<\"EOT\";\n  x\nEOT\n", "my $a = <<\"EOT\";\n    x\nEOT\n", false},
		{"Ruby heredoc indent", "ruby", "a = <<~EOS\n  x  y\nEOS\n", "a = <<~EOS\n  x y\nEOS\n", false},
		{"Dockerfile string", "dockerfile", "ENV A=\"x  y\"\n", "ENV A=\"x y\"\n", false},
	} {
		res := readDiff(t, c.id, c.before, c.after)
		got := len(res.Blocks) > 0
		for _, b := range res.Blocks {
			got = got && b.Kind == KindFormat
		}
		if got != c.format {
			t.Errorf("%s (%s): formatting only %v, want %v; blocks %+v", c.name, c.id, got, c.format, res.Blocks)
		}
	}
}

func TestScriptsAreKnownByTheirFirstLine(t *testing.T) {
	for line, want := range map[string]string{
		"#!/bin/sh":                      "bash",
		"#!/usr/bin/env bash":            "bash",
		"#!/bin/zsh -f":                  "bash",
		"#!/usr/bin/env -S node --flags": "javascript",
		"#!/usr/bin/python3.12":          "python",
		"#!/usr/bin/env pwsh":            "powershell",
		"#!/usr/bin/env ruby":            "ruby",
		"#!/usr/bin/make -f":             "make",
		"# not a shebang":                "",
		"#!/usr/bin/env awk":             "",
	} {
		if got := LanguageOfScript(line); got != want {
			t.Errorf("LanguageOfScript(%q) = %q, want %q", line, got, want)
		}
	}
	for file, want := range map[string]string{
		"Dockerfile":           "dockerfile",
		"build/Dockerfile.dev": "dockerfile",
		"Makefile":             "make",
		"CMakeLists.txt":       "cmake",
		"go.mod":               "gomod",
		".gitmodules":          "gitconfig",
		"src/a.h":              "c",
		"README.md":            "markdown",
		"deploy.yml":           "yaml",
		"Cargo.lock":           "toml",
		"Jenkinsfile":          "groovy",
	} {
		if got := LanguageOf(file, "", ""); got != want {
			t.Errorf("LanguageOf(%q) = %q, want %q", file, got, want)
		}
	}
	if got := LanguageOf("a.c", "", "cpp"); got != "c" {
		t.Errorf("git's cpp driver on a .c file reads it as %q, want c", got)
	}
	for _, hidden := range []string{"Regular expression"} {
		for _, name := range Languages() {
			if name == hidden {
				t.Errorf("%s is read only inside other languages, but Languages lists it", hidden)
			}
		}
	}
}
