package graph

import (
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// SearchResult lists matching rows in order.
type SearchResult struct {
	// Rows holds matching row indices, at most the limit passed to Search.
	Rows []int `json:"rows"`
	// Total counts every match, including any beyond the limit.
	Total int `json:"total"`
}

// Search finds rows whose subject or author contains query (case-insensitive), or whose
// commit ID starts with query when it looks like hex. At most limit row indices are
// returned; Total still counts every match.
//
// An author matches by the name the mailmap gives them and by the name the commit has, so
// someone is found under any name they committed with. A query with an @ in it is an
// address, and matches their addresses the same way; any other query leaves addresses
// alone, or "github" would find everyone with a no-reply address there.
func (g *Graph) Search(query string, limit int) SearchResult {
	res := SearchResult{Rows: []int{}}
	q := strings.TrimSpace(query)
	if q == "" {
		return res
	}
	var prefix []byte
	if len(q) >= 4 {
		// An odd-length prefix compares on whole bytes plus the high nibble of the next.
		if b, err := hex.DecodeString(q[:len(q)&^1]); err == nil {
			prefix = b
		}
	}
	match := folder(q)

	address := strings.Contains(q, "@")

	g.mu.RLock()
	defer g.mu.RUnlock()
	// Asked once for each author, not once for each of their commits.
	byAuthor := make([]bool, len(g.authors))
	for i, a := range g.authors {
		byAuthor[i] = match(a.name) || (a.written != a.name && match(a.written)) ||
			(address && (match(a.email) || (a.writtenE != a.email && match(a.writtenE))))
	}
	for i := range g.ids {
		hit := byAuthor[g.authorIdx[i]] || match(g.subjects[i])
		if !hit && prefix != nil && hasHexPrefix(g.ids[i][:], prefix, q) {
			hit = true
		}
		if hit {
			res.Total++
			if len(res.Rows) < limit {
				res.Rows = append(res.Rows, i)
			}
		}
	}
	return res
}

func hasHexPrefix(id, prefix []byte, q string) bool {
	if string(id[:len(prefix)]) != string(prefix) {
		return false
	}
	if len(q)%2 == 1 {
		last := q[len(q)-1]
		nib, ok := fromHex(last)
		return ok && id[len(prefix)]>>4 == nib
	}
	return true
}

func fromHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// folder returns a case-insensitive "contains q" matcher. ASCII queries use an
// allocation-free scan, which matters when searching a million subjects.
func folder(q string) func(string) bool {
	ascii := true
	for i := range len(q) {
		if q[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if !ascii {
		lq := strings.ToLower(q)
		return func(s string) bool { return strings.Contains(strings.ToLower(s), lq) }
	}
	lq := []byte(strings.ToLower(q))
	return func(s string) bool {
		n := len(lq)
	outer:
		for i := 0; i+n <= len(s); i++ {
			for j := range n {
				c := s[i+j]
				if 'A' <= c && c <= 'Z' {
					c += 'a' - 'A'
				}
				if c != lq[j] {
					continue outer
				}
			}
			return true
		}
		return false
	}
}
