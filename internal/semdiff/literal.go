package semdiff

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Literals are compared by what they mean, so 0x30 and 48 are the same token and so are
// 'a\tb' and "a	b". Each language's rules are the language's own; anything they don't
// cover exactly is compared as written, so a rule this file lacks can only cost a nicer
// view, never claim two different values are the same.

// strKey is the key of a string or character literal of node type typ in language id, or
// false to compare it as written. Strings of different kinds (bytes and text, characters and
// strings) never match each other.
func strKey(id, typ, s string) (string, bool) {
	switch id {
	case "go":
		v, err := strconv.Unquote(s)
		if err != nil {
			return "", false
		}
		if typ == "rune_literal" {
			return "c:" + v, true
		}
		return "s:" + v, true
	case "json":
		var v string
		if json.Unmarshal([]byte(s), &v) != nil {
			return "", false
		}
		return "s:" + v, true
	case "javascript", "typescript", "tsx":
		return jsString(s)
	case "python":
		return pyString(s)
	case "csharp":
		return csString(typ, s)
	case "java":
		return javaString(typ, s)
	case "rust":
		return rustString(typ, s)
	case "css", "scss":
		return cssString(s)
	case "lua":
		return luaString(s)
	case "html", "vue", "xml":
		return quotedValue(s)
	}
	return "", false
}

// unquote takes the quote q off both ends of s.
func unquote(s, q string) (string, bool) {
	if len(s) < 2*len(q) || !strings.HasPrefix(s, q) || !strings.HasSuffix(s, q) {
		return "", false
	}
	return s[len(q) : len(s)-len(q)], true
}

// quotedValue is a markup attribute's value, quoted either way.
func quotedValue(s string) (string, bool) {
	for _, q := range []string{`"`, `'`} {
		if v, ok := unquote(s, q); ok {
			return "a:" + v, true
		}
	}
	return "a:" + s, true
}

func jsString(s string) (string, bool) {
	for _, q := range []string{`'`, `"`, "`"} {
		body, ok := unquote(s, q)
		if !ok {
			continue
		}
		if q == "`" {
			if strings.Contains(body, "${") {
				return "", false
			}
			// A template's raw line breaks are LF, whatever the file uses.
			body = strings.ReplaceAll(body, "\r\n", "\n")
		}
		v, ok := unescape(body, escapes{simple: "bfnrtv0'\"\\`$", hex: 2, u4: true, uBrace: true, lineCont: true, identity: true, noOctal: true})
		if !ok {
			return "", false
		}
		return "s:" + v, true
	}
	return "", false
}

func pyString(s string) (string, bool) {
	i := 0
	for i < len(s) && strings.ContainsRune("rRbBuUfF", rune(s[i])) {
		i++
	}
	prefix := strings.ToLower(s[:i])
	body := s[i:]
	ok := false
	for _, q := range []string{`"""`, `'''`, `"`, `'`} {
		if b, k := unquote(body, q); k {
			body, ok = b, true
			break
		}
	}
	if !ok {
		return "", false
	}
	if strings.Contains(prefix, "f") && strings.ContainsAny(body, "{}") {
		return "", false
	}
	kind := "s:"
	if strings.Contains(prefix, "b") {
		kind = "b:"
	}
	if strings.Contains(prefix, "r") {
		return kind + body, true
	}
	e := escapes{simple: "abfnrtv'\"\\", hex: 2, octal: 3, lineCont: true, keepUnknown: true}
	if kind == "s:" {
		e.u4, e.u8 = true, true
	}
	if strings.Contains(body, `\N{`) {
		return "", false
	}
	v, ok := unescape(body, e)
	return kind + v, ok
}

func csString(typ, s string) (string, bool) {
	switch typ {
	case "character_literal":
		body, ok := unquote(s, "'")
		if !ok {
			return "", false
		}
		v, ok := unescape(body, escapes{simple: "'\"\\0abfnrtve", hexVar: 4, u4: true, u8: true})
		return "c:" + v, ok
	case "verbatim_string_literal":
		body, ok := unquote(strings.TrimPrefix(s, "@"), `"`)
		if !ok {
			return "", false
		}
		return "s:" + strings.ReplaceAll(body, `""`, `"`), true
	case "raw_string_literal":
		if strings.ContainsAny(s, "\r\n") {
			return "", false // a multi-line raw string loses its indentation by rules of its own
		}
		body := strings.Trim(s, `"`)
		return "s:" + body, true
	}
	kind := "s:"
	if strings.HasSuffix(s, "u8") {
		kind, s = "u8:", strings.TrimSuffix(s, "u8")
	}
	body, ok := unquote(s, `"`)
	if !ok {
		return "", false
	}
	v, ok := unescape(body, escapes{simple: "'\"\\0abfnrtve", hexVar: 4, u4: true, u8: true})
	return kind + v, ok
}

func javaString(typ, s string) (string, bool) {
	q, kind := `"`, "s:"
	if typ == "character_literal" {
		q, kind = "'", "c:"
	}
	if strings.HasPrefix(s, `"""`) {
		return "", false // a text block loses its indentation by rules of its own
	}
	body, ok := unquote(s, q)
	if !ok {
		return "", false
	}
	v, ok := unescape(body, escapes{simple: "btnfrs\"'\\", octal: 3, octalMax: 0o377, u4: true, manyU: true})
	return kind + v, ok
}

func rustString(typ, s string) (string, bool) {
	kind := "s:"
	if typ == "char_literal" {
		kind = "c:"
	}
	switch {
	case strings.HasPrefix(s, "b"):
		kind, s = "b"+kind, s[1:]
	case strings.HasPrefix(s, "c"):
		kind, s = "cs"+kind, s[1:]
	}
	if typ == "raw_string_literal" || strings.HasPrefix(s, "r") {
		s = strings.TrimPrefix(s, "r")
		hashes := len(s) - len(strings.TrimLeft(s, "#"))
		body, ok := unquote(s[hashes:len(s)-hashes], `"`)
		if !ok {
			return "", false
		}
		return kind + body, true
	}
	q := `"`
	if typ == "char_literal" {
		q = "'"
	}
	body, ok := unquote(s, q)
	if !ok {
		return "", false
	}
	v, ok := unescape(body, escapes{simple: "nrt\\0'\"", hex: 2, uBrace: true, lineCont: true, skipSpace: true})
	return kind + v, ok
}

func cssString(s string) (string, bool) {
	for _, q := range []string{`"`, `'`} {
		body, ok := unquote(s, q)
		if !ok {
			continue
		}
		var b strings.Builder
		for i := 0; i < len(body); {
			c := body[i]
			if c != '\\' {
				b.WriteByte(c)
				i++
				continue
			}
			i++
			if i >= len(body) {
				return "", false
			}
			if body[i] == '\n' {
				i++
				continue
			}
			if body[i] == '\r' {
				i++
				if i < len(body) && body[i] == '\n' {
					i++
				}
				continue
			}
			j := i
			for j < len(body) && j-i < 6 && isHex(body[j]) {
				j++
			}
			if j > i {
				v, _ := strconv.ParseUint(body[i:j], 16, 32)
				if v == 0 || v > utf8.MaxRune || (v >= 0xd800 && v <= 0xdfff) {
					v = utf8.RuneError
				}
				b.WriteRune(rune(v))
				if j < len(body) && (body[j] == ' ' || body[j] == '\t' || body[j] == '\n') {
					j++
				}
				i = j
				continue
			}
			r, size := utf8.DecodeRuneInString(body[i:])
			b.WriteRune(r)
			i += size
		}
		return "s:" + b.String(), true
	}
	return "", false
}

func luaString(s string) (string, bool) {
	if strings.HasPrefix(s, "[") {
		level := len(s) - len(strings.TrimLeft(s[1:], "=")) - 1
		open := "[" + strings.Repeat("=", level) + "["
		closing := "]" + strings.Repeat("=", level) + "]"
		if !strings.HasPrefix(s, open) || !strings.HasSuffix(s, closing) {
			return "", false
		}
		body := s[len(open) : len(s)-len(closing)]
		// A long string drops a line break straight after its opening bracket.
		body = strings.TrimPrefix(body, "\r\n")
		body = strings.TrimPrefix(body, "\n")
		return "s:" + body, true
	}
	for _, q := range []string{`"`, `'`} {
		body, ok := unquote(s, q)
		if !ok {
			continue
		}
		v, ok := unescape(body, escapes{simple: "abfnrtv\\\"'", hex: 2, decimal: 3, uBrace: true, lineNewline: true, zSkip: true})
		return "s:" + v, ok
	}
	return "", false
}

// escapes says which backslash escapes a language's strings have.
type escapes struct {
	simple      string // letters with a fixed meaning, as in \n
	hex         int    // \x and exactly this many hex digits
	hexVar      int    // \x and one to this many hex digits (C#)
	octal       int    // \ and up to this many octal digits
	octalMax    int    // the largest octal escape, when octal is set (Java's is \377)
	decimal     int    // \ and up to this many decimal digits (Lua)
	u4          bool   // \uXXXX
	manyU       bool   // \uuuuXXXX (Java)
	u8          bool   // \UXXXXXXXX
	uBrace      bool   // \u{X...}
	lineCont    bool   // a backslash before a line break joins the lines
	lineNewline bool   // a backslash before a line break is a line break (Lua)
	skipSpace   bool   // a joined line drops its leading whitespace (Rust)
	zSkip       bool   // \z skips the whitespace after it (Lua)
	identity    bool   // any other escaped character is itself (JavaScript)
	keepUnknown bool   // any other escape keeps its backslash (Python)
	noOctal     bool   // \ and a digit other than \0 alone isn't read (JavaScript's legacy octal)
}

var simpleValue = map[byte]string{
	'a': "\a", 'b': "\b", 'f': "\f", 'n': "\n", 'r': "\r", 't': "\t", 'v': "\v", 'e': "\x1b",
	's': " ", '0': "\x00", '\\': "\\", '\'': "'", '"': "\"", '`': "`", '$': "$",
}

// unescape reads s's escapes by e, or returns false for one it can't read exactly.
func unescape(s string, e escapes) (string, bool) {
	if !strings.Contains(s, `\`) {
		return s, true
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if c != '\\' {
			b.WriteByte(c)
			i++
			continue
		}
		i++
		if i >= len(s) {
			return "", false
		}
		c = s[i]
		switch {
		case c == '\n' || c == '\r':
			next := i + 1
			if c == '\r' && next < len(s) && s[next] == '\n' {
				next++
			}
			switch {
			case e.lineCont:
				i = next
				if e.skipSpace {
					for i < len(s) && strings.IndexByte(" \t\n\r", s[i]) >= 0 {
						i++
					}
				}
			case e.lineNewline:
				b.WriteByte('\n')
				i = next
			default:
				return "", false
			}
		case c == 'z' && e.zSkip:
			i++
			for i < len(s) && strings.IndexByte(" \t\n\r\f\v", s[i]) >= 0 {
				i++
			}
		case c == '0' && e.noOctal && (i+1 >= len(s) || s[i+1] < '0' || s[i+1] > '9'):
			b.WriteByte(0)
			i++
		case c >= '0' && c <= '9' && e.noOctal:
			return "", false
		case c >= '0' && c <= '7' && e.octal > 0:
			j := i
			for j < len(s) && j-i < e.octal && s[j] >= '0' && s[j] <= '7' {
				j++
			}
			v, _ := strconv.ParseUint(s[i:j], 8, 32)
			if e.octalMax > 0 && int(v) > e.octalMax {
				j--
				v, _ = strconv.ParseUint(s[i:j], 8, 32)
			}
			if v > 0xff {
				return "", false
			}
			b.WriteByte(byte(v))
			i = j
		case c >= '0' && c <= '9' && e.decimal > 0:
			j := i
			for j < len(s) && j-i < e.decimal && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			v, _ := strconv.Atoi(s[i:j])
			if v > 255 {
				return "", false
			}
			b.WriteByte(byte(v))
			i = j
		case c == 'x' && (e.hex > 0 || e.hexVar > 0):
			j := i + 1
			n := e.hex
			if e.hexVar > 0 {
				n = e.hexVar
			}
			for j < len(s) && j-i-1 < n && isHex(s[j]) {
				j++
			}
			if j == i+1 || (e.hex > 0 && j-i-1 != e.hex) {
				return "", false
			}
			v, _ := strconv.ParseUint(s[i+1:j], 16, 32)
			if e.hexVar > 0 {
				b.WriteRune(rune(v))
			} else {
				b.WriteByte(byte(v))
			}
			i = j
		case c == 'u' && e.uBrace && i+1 < len(s) && s[i+1] == '{':
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				return "", false
			}
			v, err := strconv.ParseUint(strings.ReplaceAll(s[i+2:i+end], "_", ""), 16, 32)
			if err != nil || v > utf8.MaxRune {
				return "", false
			}
			b.WriteRune(rune(v))
			i += end + 1
		case c == 'u' && e.u4:
			j := i + 1
			if e.manyU {
				for j < len(s) && s[j] == 'u' {
					j++
				}
			}
			if j+4 > len(s) {
				return "", false
			}
			v, err := strconv.ParseUint(s[j:j+4], 16, 32)
			if err != nil {
				return "", false
			}
			// Surrogate pairs stay as written: they only make sense together.
			if v >= 0xd800 && v <= 0xdfff {
				return "", false
			}
			b.WriteRune(rune(v))
			i = j + 4
		case c == 'U' && e.u8:
			if i+9 > len(s) {
				return "", false
			}
			v, err := strconv.ParseUint(s[i+1:i+9], 16, 32)
			if err != nil || v > utf8.MaxRune {
				return "", false
			}
			b.WriteRune(rune(v))
			i += 9
		case strings.IndexByte(e.simple, c) >= 0:
			b.WriteString(simpleValue[c])
			i++
		case e.keepUnknown:
			b.WriteByte('\\')
		case e.identity:
			r, size := utf8.DecodeRuneInString(s[i:])
			b.WriteRune(r)
			i += size
		default:
			return "", false
		}
	}
	return b.String(), true
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
