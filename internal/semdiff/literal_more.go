package semdiff

import "strings"

// String rules for the languages #965 added. Each reads only what its language spells out;
// anything else is compared as written.

// tomlString reads a one-line TOML string: a basic one with escapes, or a literal one as
// written. Multi-line strings are compared as written.
func tomlString(s string) (string, bool) {
	if strings.HasPrefix(s, `"""`) || strings.HasPrefix(s, "'''") || strings.ContainsAny(s, "\r\n") {
		return "", false
	}
	if body, ok := unquote(s, "'"); ok {
		return "s:" + body, true
	}
	body, ok := unquote(s, `"`)
	if !ok {
		return "", false
	}
	v, ok := unescape(body, escapes{simple: "btnfr\"\\e", u4: true, u8: true})
	return "s:" + v, ok
}

// yamlString reads a one-line quoted YAML scalar: single-quoted, where two quotes make one, or
// double-quoted, with escapes. A quoted scalar over several lines folds them, and is
// compared as written.
func yamlString(s string) (string, bool) {
	if strings.ContainsAny(s, "\r\n") {
		return "", false
	}
	if body, ok := unquote(s, "'"); ok {
		return "s:" + strings.ReplaceAll(body, "''", "'"), true
	}
	body, ok := unquote(s, `"`)
	if !ok {
		return "", false
	}
	for _, e := range []string{`\N`, `\_`, `\L`, `\P`} {
		if strings.Contains(body, e) {
			return "", false
		}
	}
	v, ok := unescape(body, escapes{simple: "0abtnvfre\"/\\ ", hex: 2, u4: true, u8: true})
	return "s:" + v, ok
}

// bashString reads a single-quoted shell string, or a double-quoted one with nothing to
// expand. In double quotes only \$, \`, \", \\ and a backslash before a line break are
// escapes; any other backslash stays.
func bashString(typ, s string) (string, bool) {
	switch typ {
	case "raw_string":
		body, ok := unquote(s, "'")
		return "s:" + body, ok
	case "string":
		body, ok := unquote(s, `"`)
		if !ok || strings.ContainsAny(body, "$`") {
			return "", false
		}
		var b strings.Builder
		for i := 0; i < len(body); i++ {
			if body[i] == '\\' && i+1 < len(body) {
				switch body[i+1] {
				case '$', '`', '"', '\\':
					b.WriteByte(body[i+1])
					i++
					continue
				case '\n':
					i++
					continue
				}
			}
			b.WriteByte(body[i])
		}
		return "s:" + b.String(), true
	}
	return "", false
}

// psString reads a single-quoted PowerShell string, where two quotes make one. Here-strings are
// compared as written.
func psString(s string) (string, bool) {
	if strings.HasPrefix(s, "@") {
		return "", false
	}
	body, ok := unquote(s, "'")
	if !ok {
		return "", false
	}
	return "s:" + strings.ReplaceAll(body, "''", "'"), true
}
