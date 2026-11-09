package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// IgnoreSource is one file of ignore rules that applies to a work tree.
type IgnoreSource struct {
	// Source names the file as check-ignore does (see IgnoreLine.Source).
	Source string `json:"source"`
	// Path is the file's absolute path.
	Path  string `json:"path"`
	Scope string `json:"scope"`
	// Dir is the folder a .gitignore sits in, "" at the top.
	Dir    string `json:"dir,omitempty"`
	Exists bool   `json:"exists"`
	// Rules counts the lines that are rules, not blanks or comments.
	Rules int `json:"rules"`
}

// IgnoreSources lists the ignore files for this work tree in the order git reads them,
// least specific last: .gitignore files (the top one first, then by path), info/exclude,
// then the global excludes file. The top .gitignore, info/exclude and the global file are
// listed even when they don't exist yet, so they can be created.
func (r *Repo) IgnoreSources(ctx context.Context) ([]IgnoreSource, error) {
	out, err := r.Read(ctx, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", ":(glob)**/.gitignore")
	if err != nil {
		return nil, err
	}
	var rels []string
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" && !slices.Contains(rels, p) {
			rels = append(rels, p)
		}
	}
	slices.SortFunc(rels, func(a, b string) int {
		if a == ".gitignore" || b == ".gitignore" {
			return boolCmp(b == ".gitignore", a == ".gitignore")
		}
		return strings.Compare(a, b)
	})
	if !slices.Contains(rels, ".gitignore") {
		rels = append([]string{".gitignore"}, rels...)
	}
	list := make([]IgnoreSource, 0, len(rels)+2)
	for _, rel := range rels {
		abs, err := r.inside(rel)
		if err != nil {
			continue
		}
		dir := path.Dir(rel)
		if dir == "." {
			dir = ""
		}
		scope := IgnoreDir
		if dir == "" {
			scope = IgnoreRoot
		}
		list = append(list, describeSource(rel, abs, scope, dir))
	}
	for _, scope := range []string{IgnoreExclude, IgnoreGlobal} {
		abs, err := r.ignoreFile(ctx, IgnoreTarget{Scope: scope})
		if err != nil {
			continue
		}
		list = append(list, describeSource(r.sourceName(abs), abs, scope, ""))
	}
	return list, nil
}

func boolCmp(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

func describeSource(source, abs, scope, dir string) IgnoreSource {
	s := IgnoreSource{Source: source, Path: abs, Scope: scope, Dir: dir}
	lines, _, err := readLines(abs)
	if _, statErr := os.Stat(abs); statErr == nil {
		s.Exists = true
	}
	if err == nil {
		for _, l := range lines {
			if isRuleLine(l) {
				s.Rules++
			}
		}
	}
	return s
}

// isRuleLine reports whether a line of an ignore file is a rule rather than a blank or a
// comment. A leading "\#" is a rule for a name starting with #.
func isRuleLine(l string) bool {
	t := strings.TrimRight(l, " \t")
	return t != "" && !strings.HasPrefix(t, "#")
}

// ReadIgnoreFile returns an ignore file's text with "\n" line endings, or "" when it
// doesn't exist yet.
func (r *Repo) ReadIgnoreFile(ctx context.Context, source string) (string, error) {
	p, _, err := r.sourceFile(ctx, source)
	if err != nil {
		return "", err
	}
	lines, _, err := readLines(p)
	if err != nil || len(lines) == 0 {
		return "", err
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// ErrIgnoreFileChanged means an ignore file changed on disk after it was read for editing.
var ErrIgnoreFileChanged = errors.New("the ignore file changed on disk")

// WriteIgnoreFile replaces an ignore file's text. base is the text ReadIgnoreFile gave
// when editing began; if the file no longer holds it, nothing is written and
// ErrIgnoreFileChanged is returned. The file keeps its line endings.
func (r *Repo) WriteIgnoreFile(ctx context.Context, source, base, text string) error {
	p, _, err := r.sourceFile(ctx, source)
	if err != nil {
		return err
	}
	now, err := r.ReadIgnoreFile(ctx, source)
	if err != nil {
		return err
	}
	if now != normalizeIgnoreText(base) {
		return ErrIgnoreFileChanged
	}
	_, eol, err := readLines(p)
	if err != nil {
		return err
	}
	text = normalizeIgnoreText(text)
	if text == "" {
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return writeLines(p, nil, eol)
	}
	return writeLines(p, strings.Split(strings.TrimSuffix(text, "\n"), "\n"), eol)
}

// normalizeIgnoreText gives text "\n" line endings and one final newline, or "" for none.
func normalizeIgnoreText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return ""
	}
	return text + "\n"
}

// IgnoreRuleStats says what one rule decides right now.
type IgnoreRuleStats struct {
	Source string `json:"source"`
	Line   int    `json:"line"`
	// Ignores counts the paths this rule ignores; a wholly ignored folder counts once.
	Ignores int `json:"ignores"`
	// Restores counts untracked files a "!" rule brings back.
	Restores int `json:"restores"`
	// Tracked counts tracked files the rule matches, which git keeps tracking anyway.
	Tracked int `json:"tracked"`
	// Paths is a sample of what the rule ignores, restores or matches.
	Paths []string `json:"paths"`
}

// IgnoreStats is what every rule in the work tree's ignore files decides.
type IgnoreStats struct {
	Rules []IgnoreRuleStats `json:"rules"`
	// Truncated means there were too many ignored paths to count them all.
	Truncated bool `json:"truncated"`
}

// maxStatPaths caps the sample of paths kept for each rule.
const maxStatPaths = 20

// IgnoreStats counts, for every rule, what it decides: the paths it ignores, the files a
// "!" rule brings back, and tracked files it matches. Each path counts toward the last
// rule that matches it, the one git goes by.
func (r *Repo) IgnoreStats(ctx context.Context) (IgnoreStats, error) {
	sources, err := r.IgnoreSources(ctx)
	if err != nil {
		return IgnoreStats{}, err
	}
	canon := func(src string) string {
		for _, s := range sources {
			if s.Source == src || sameIgnoreFile(s.Path, src) || !filepath.IsAbs(src) && sameIgnoreFile(s.Path, filepath.Join(r.dir, src)) {
				return s.Source
			}
		}
		return src
	}
	by := map[string]*IgnoreRuleStats{}
	var order []string
	at := func(rule IgnoreLine) *IgnoreRuleStats {
		src := canon(rule.Source)
		key := fmt.Sprintf("%s\x00%d", src, rule.Line)
		if s, ok := by[key]; ok {
			return s
		}
		s := &IgnoreRuleStats{Source: src, Line: rule.Line, Paths: []string{}}
		by[key] = s
		order = append(order, key)
		return s
	}
	sample := func(s *IgnoreRuleStats, p string) {
		if len(s.Paths) < maxStatPaths {
			s.Paths = append(s.Paths, p)
		}
	}

	ignored, truncated, err := r.IgnoredFiles(ctx)
	if err != nil {
		return IgnoreStats{}, err
	}
	for _, f := range ignored {
		s := at(f.Rule)
		s.Ignores++
		sample(s, f.Path)
	}

	st, err := r.Status(ctx)
	if err != nil {
		return IgnoreStats{}, err
	}
	var untracked []string
	for _, f := range st.Files {
		if f.Untracked {
			untracked = append(untracked, f.Path)
		}
	}
	back, err := r.checkIgnore(ctx, untracked, false)
	if err != nil {
		return IgnoreStats{}, err
	}
	for _, p := range untracked {
		if m, ok := back[p]; ok && strings.HasPrefix(m.Pattern, "!") {
			s := at(m)
			s.Restores++
			sample(s, p)
		}
	}

	out, err := r.Read(ctx, "ls-files", "-z", "--cached", "--ignored", "--exclude-standard")
	if err != nil {
		return IgnoreStats{}, err
	}
	var tracked []string
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" {
			tracked = append(tracked, p)
		}
	}
	matched, err := r.checkIgnore(ctx, tracked, true)
	if err != nil {
		return IgnoreStats{}, err
	}
	for _, p := range tracked {
		if m, ok := matched[p]; ok && !strings.HasPrefix(m.Pattern, "!") {
			s := at(m)
			s.Tracked++
			sample(s, p)
		}
	}

	res := IgnoreStats{Rules: make([]IgnoreRuleStats, 0, len(order)), Truncated: truncated}
	for _, k := range order {
		res.Rules = append(res.Rules, *by[k])
	}
	return res, nil
}

// IgnoreCheck is the answer to whether a path is ignored.
type IgnoreCheck struct {
	// Matched means some rule matches the path, either to ignore it or, with a "!" rule,
	// to bring it back.
	Matched bool       `json:"matched"`
	Ignored bool       `json:"ignored"`
	Rule    IgnoreLine `json:"rule"`
	// Tracked means git tracks the path, so rules don't hide it.
	Tracked bool `json:"tracked"`
}

// CheckIgnorePath says whether the saved rules ignore a path, and which rule decides. The
// path needn't exist.
func (r *Repo) CheckIgnorePath(ctx context.Context, p string) (IgnoreCheck, error) {
	p = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(p)), "/")
	if p == "" {
		return IgnoreCheck{}, nil
	}
	if _, err := r.inside(p); err != nil {
		return IgnoreCheck{}, err
	}
	res, err := r.checkIgnore(ctx, []string{p}, true)
	if err != nil {
		return IgnoreCheck{}, err
	}
	var c IgnoreCheck
	if m, ok := res[p]; ok {
		c.Matched = true
		c.Ignored = !strings.HasPrefix(m.Pattern, "!")
		c.Rule = m
		if sources, err := r.IgnoreSources(ctx); err == nil {
			for _, s := range sources {
				if s.Source == m.Source || sameIgnoreFile(s.Path, m.Source) {
					c.Rule.Source = s.Source
				}
			}
		}
	}
	if out, err := r.Read(ctx, "ls-files", "-z", "--cached", "--", ":(literal)"+strings.TrimSuffix(p, "/")); err == nil && len(out) > 0 {
		c.Tracked = true
	}
	return c, nil
}
