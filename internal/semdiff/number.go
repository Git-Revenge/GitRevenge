package semdiff

import (
	"math/big"
	"strings"
)

// numKey is the key of a number literal in language id, or false to compare it as written.
// Integers key as "i:" and floats as "f:" with their exact value, so 0x30 matches 48 and 1.50
// matches 1.5, while 1 and 1.0 stay apart in the languages that tell them apart. A type
// suffix stays on the key.
func numKey(id, s string) (string, bool) {
	switch id {
	case "go":
		if strings.HasSuffix(s, "i") {
			return "", false // imaginary
		}
		return value(s, true)
	case "javascript", "typescript", "tsx":
		return jsNumber(s)
	case "python":
		return pyNumber(s)
	case "csharp":
		return suffixed(s, false, []string{"ul", "lu", "u", "l", "f", "d", "m"})
	case "java":
		return suffixed(s, true, []string{"l", "f", "d"})
	case "rust":
		return suffixed(s, false, []string{"i128", "u128", "isize", "usize", "i16", "i32", "i64", "u16", "u32", "u64", "i8", "u8", "f32", "f64"})
	case "lua":
		if strings.ContainsAny(s, "_oObB") {
			return "", false
		}
		return value(s, false)
	case "json":
		return jsonNumber(s)
	case "css", "scss":
		return cssNumber(s)
	}
	return "", false
}

// jsNumber reads JavaScript numbers, which are all one type, and BigInts.
func jsNumber(s string) (string, bool) {
	if strings.HasSuffix(s, "n") {
		k, ok := value(strings.TrimSuffix(s, "n"), false)
		if !ok || !strings.HasPrefix(k, "i:") {
			return "", false
		}
		return "big:" + k[2:], true
	}
	// A leading zero and a digit is legacy octal, or decimal when an 8 or 9 follows.
	if len(s) > 1 && s[0] == '0' && isDigit(s[1]) {
		return "", false
	}
	k, ok := value(s, false)
	if !ok {
		return "", false
	}
	return "n:" + k[2:], true
}

// pyNumber reads Python ints and floats. Only zeros may lead with 0: 00 is 0.
func pyNumber(s string) (string, bool) {
	if strings.HasSuffix(s, "j") || strings.HasSuffix(s, "J") {
		return "", false // imaginary
	}
	if len(s) > 1 && s[0] == '0' && isDigit(s[1]) && strings.Trim(s, "0_") != "" {
		return "", false
	}
	return value(s, false)
}

// suffixed reads a literal that may end in a type suffix, as C#, Java and Rust write them.
func suffixed(s string, legacyOctal bool, suffixes []string) (string, bool) {
	t := strings.ToLower(strings.ReplaceAll(s, "_", ""))
	hex := strings.HasPrefix(t, "0x")
	suffix := ""
	for _, sfx := range suffixes {
		// In hex, f and d are digits.
		if hex && (sfx[0] == 'f' || sfx[0] == 'd') {
			continue
		}
		if strings.HasSuffix(t, sfx) && len(t) > len(sfx) {
			suffix = sfx
			break
		}
	}
	switch suffix {
	case "m":
		return "", false // a C# decimal keeps its scale: 1.50m prints as 1.50
	case "lu":
		suffix = "ul"
	}
	k, ok := value(t[:len(t)-len(suffix)], legacyOctal)
	if !ok {
		return "", false
	}
	// A float suffix makes a float of an integer-looking literal: 1f is 1.0f.
	if suffix == "f" || suffix == "d" || suffix == "f32" || suffix == "f64" {
		k = "f:" + k[2:]
	}
	return k + suffix, true
}

// jsonNumber reads a JSON number. -0 is compared as written, since a float keeps its sign.
func jsonNumber(s string) (string, bool) {
	neg := strings.HasPrefix(s, "-")
	body := strings.TrimPrefix(s, "-")
	if body == "" || !isDigit(body[0]) || (len(body) > 1 && body[0] == '0' && isDigit(body[1])) {
		return "", false
	}
	k, ok := value(body, false)
	if !ok || strings.ContainsAny(body, "xXoObB_") {
		return "", false
	}
	if neg {
		if k[2:] == "0" {
			return "", false
		}
		k = k[:2] + "-" + k[2:]
	}
	return k, true
}

// cssNumber reads a CSS number with its unit: 0.5em and .5em are the same, 0 and 0px aren't.
func cssNumber(s string) (string, bool) {
	sign := ""
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		if s[0] == '-' {
			sign = "-"
		}
		s = s[1:]
	}
	end := 0
	for end < len(s) && (isDigit(s[end]) || s[end] == '.') {
		end++
	}
	// An exponent belongs to the number only when digits follow it.
	if end < len(s) && (s[end] == 'e' || s[end] == 'E') {
		e := end + 1
		if e < len(s) && (s[e] == '+' || s[e] == '-') {
			e++
		}
		if e < len(s) && isDigit(s[e]) {
			for e < len(s) && isDigit(s[e]) {
				e++
			}
			end = e
		}
	}
	num, unit := s[:end], strings.ToLower(s[end:])
	if num == "" {
		return "", false
	}
	k, ok := value(num, false)
	if !ok {
		return "", false
	}
	v := k[2:]
	if sign == "-" && v != "0" {
		v = "-" + v
	}
	return "n:" + v + unit, true
}

// value reads an unsigned literal: an integer in the base its 0x, 0o or 0b prefix names, or
// in octal after a lone leading 0 when legacyOctal is set, or a float, decimal or hex.
// Underscores between digits are dropped.
func value(s string, legacyOctal bool) (string, bool) {
	s = strings.ReplaceAll(s, "_", "")
	if s == "" {
		return "", false
	}
	lower := strings.ToLower(s)
	// An exact value of 1e999999999 would take gigabytes; no real literal needs one.
	expAt := strings.LastIndexByte(lower, 'e')
	if strings.HasPrefix(lower, "0x") {
		expAt = strings.LastIndexByte(lower, 'p')
	}
	if expAt >= 0 && len(strings.TrimLeft(lower[expAt+1:], "+-0")) > 3 {
		return "", false
	}
	base := 10
	digits := s
	switch {
	case strings.HasPrefix(lower, "0x"):
		if strings.ContainsAny(lower, ".p") {
			f, _, err := big.ParseFloat(s, 0, 512, big.ToNearestEven)
			if err != nil || f.IsInf() {
				return "", false
			}
			r, exact := f.Rat(nil)
			if exact != big.Exact {
				return "", false
			}
			return "f:" + r.RatString(), true
		}
		base, digits = 16, s[2:]
	case strings.HasPrefix(lower, "0o"):
		base, digits = 8, s[2:]
	case strings.HasPrefix(lower, "0b"):
		base, digits = 2, s[2:]
	case strings.ContainsAny(lower, ".e"):
		if !isDigit(lower[0]) && (lower[0] != '.' || len(lower) < 2 || !isDigit(lower[1])) {
			return "", false
		}
		// big.Rat reads "5." and ".5" as Go does; write them out so it never has to guess.
		if strings.HasSuffix(lower, ".") {
			lower += "0"
		}
		lower = strings.Replace(lower, ".e", ".0e", 1)
		if lower[0] == '.' {
			lower = "0" + lower
		}
		r, ok := new(big.Rat).SetString(lower)
		if !ok {
			return "", false
		}
		return "f:" + r.RatString(), true
	case legacyOctal && len(s) > 1 && s[0] == '0':
		base, digits = 8, s[1:]
	}
	if digits == "" || strings.HasPrefix(digits, "+") || strings.HasPrefix(digits, "-") {
		return "", false
	}
	v, ok := new(big.Int).SetString(digits, base)
	if !ok {
		return "", false
	}
	return "i:" + v.String(), true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
