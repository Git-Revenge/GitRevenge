package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Where an ignore rule is written.
const (
	// IgnoreRoot is the .gitignore at the top of the work tree, shared through commits.
	IgnoreRoot = "root"
	// IgnoreDir is a folder's own .gitignore; IgnoreTarget.Dir names the folder.
	IgnoreDir = "dir"
	// IgnoreExclude is .git/info/exclude: this clone only, never committed.
	IgnoreExclude = "exclude"
	// IgnoreGlobal is core.excludesFile: every repository for this user.
	IgnoreGlobal = "global"
)

// IgnoreTarget names an ignore file to write a rule to.
type IgnoreTarget struct {
	Scope string `json:"scope"`
	// Dir is the folder for IgnoreDir, relative to the work tree, with forward slashes.
	Dir string `json:"dir,omitempty"`
}

// IgnoreLine is one rule in an ignore file.
type IgnoreLine struct {
	// Source names the file as `git check-ignore -v` does: relative to the work tree for
	// files inside it, absolute otherwise.
	Source  string `json:"source"`
	Line    int    `json:"line"`
	Pattern string `json:"pattern"`
	// Created means adding this rule created the file, so removing it again deletes the
	// file once nothing else is in it.
	Created bool `json:"created,omitempty"`
}

// IgnorePreview is what a rule would do before it's written.
type IgnorePreview struct {
	// Hidden are the untracked files the rule would stop showing.
	Hidden []string `json:"hidden"`
	// Tracked are tracked files the rule matches. Git keeps tracking them regardless.
	Tracked []string `json:"tracked"`
}

// IgnoredFile is an ignored path and the rule that ignores it.
type IgnoredFile struct {
	// Path is relative to the work tree. A wholly ignored folder ends in "/".
	Path string     `json:"path"`
	Rule IgnoreLine `json:"rule"`
	// CanExcept means a "!" rule could bring this path back. It can't when a folder above
	// it is ignored: git never looks inside an ignored folder.
	CanExcept bool `json:"canExcept"`
}

// maxIgnored caps IgnoredFiles, since build output can hold a great many files.
const maxIgnored = 2000

// EscapeIgnorePath turns a path into a pattern that matches only that path: wildcards,
// backslashes and trailing spaces lose their meaning. The result is anchored with a
// leading "/", so a leading "#" or "!" is safe too.
func EscapeIgnorePath(p string) string {
	var b strings.Builder
	b.WriteByte('/')
	trimmed := strings.TrimRight(p, " ")
	for _, c := range trimmed {
		if strings.ContainsRune(`\*?[`, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	for range len(p) - len(trimmed) {
		b.WriteString(`\ `)
	}
	return b.String()
}

// GlobalExcludesFile returns core.excludesFile, or git's default when it isn't set.
func (r *Repo) GlobalExcludesFile(ctx context.Context) (string, error) {
	if p, err := r.ReadString(ctx, "config", "--path", "--get", "core.excludesFile"); err == nil && p != "" {
		return p, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "git", "ignore"), nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = h
	}
	return filepath.Join(home, ".config", "git", "ignore"), nil
}

// excludeFile is this clone's .git/info/exclude, which linked worktrees share.
func (r *Repo) excludeFile(ctx context.Context) (string, error) {
	p, err := r.ReadString(ctx, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.dir, p)
	}
	return filepath.Clean(p), nil
}

// ignoreFile returns the absolute path of an ignore file.
func (r *Repo) ignoreFile(ctx context.Context, t IgnoreTarget) (string, error) {
	switch t.Scope {
	case IgnoreRoot:
		return r.inside(".gitignore")
	case IgnoreDir:
		dir := strings.Trim(t.Dir, "/")
		if dir == "" {
			return r.inside(".gitignore")
		}
		return r.inside(dir + "/.gitignore")
	case IgnoreExclude:
		return r.excludeFile(ctx)
	case IgnoreGlobal:
		return r.GlobalExcludesFile(ctx)
	}
	return "", fmt.Errorf("unknown ignore file %q", t.Scope)
}

// sourceName names an ignore file the way check-ignore does.
func (r *Repo) sourceName(abs string) string {
	root, err := filepath.Abs(r.dir)
	if err == nil {
		if rel, err := filepath.Rel(root, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return abs
}

// sourceFile resolves a source named by check-ignore or IgnoreLine, refusing anything
// that isn't an ignore file: a .gitignore in the work tree, info/exclude, or the global
// excludes file. It also says which folder a .gitignore's patterns are relative to.
func (r *Repo) sourceFile(ctx context.Context, source string) (abs, base string, err error) {
	if source == "" {
		return "", "", errors.New("no ignore file given")
	}
	if !filepath.IsAbs(source) {
		if path.Base(filepath.ToSlash(source)) == ".gitignore" {
			p, err := r.inside(source)
			if err != nil {
				return "", "", err
			}
			base = path.Dir(filepath.ToSlash(source))
			if base == "." {
				base = ""
			}
			return p, base, nil
		}
		source = filepath.Join(r.dir, source)
	}
	source = filepath.Clean(source)
	for _, scope := range []string{IgnoreExclude, IgnoreGlobal} {
		p, err := r.ignoreFile(ctx, IgnoreTarget{Scope: scope})
		if err == nil && sameIgnoreFile(p, source) {
			return p, "", nil
		}
	}
	return "", "", fmt.Errorf("%s is not an ignore file", source)
}

func sameIgnoreFile(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if a == b || strings.EqualFold(a, b) && filepath.Separator == '\\' {
		return true
	}
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}

// readLines splits a file into lines without their endings, and reports its line ending.
func readLines(p string) (lines []string, eol string, err error) {
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "\n", nil
	}
	if err != nil {
		return nil, "", err
	}
	eol = "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		eol = "\r\n"
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" && len(data) == 0 {
		return nil, eol, nil
	}
	return strings.Split(text, "\n"), eol, nil
}

func writeLines(p string, lines []string, eol string) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	text := ""
	if len(lines) > 0 {
		text = strings.Join(lines, eol) + eol
	}
	return os.WriteFile(p, []byte(text), 0o644)
}

// AddIgnoreRule appends a pattern to an ignore file, creating it when needed, and says
// where it landed. A pattern already in the file isn't added twice.
func (r *Repo) AddIgnoreRule(ctx context.Context, t IgnoreTarget, pattern string) (IgnoreLine, error) {
	pattern = strings.TrimRight(pattern, "\r\n")
	if strings.TrimSpace(pattern) == "" || strings.ContainsAny(pattern, "\r\n") {
		return IgnoreLine{}, errors.New("an ignore rule is one non-empty line")
	}
	p, err := r.ignoreFile(ctx, t)
	if err != nil {
		return IgnoreLine{}, err
	}
	lines, eol, err := readLines(p)
	if err != nil {
		return IgnoreLine{}, err
	}
	src := r.sourceName(p)
	for i, l := range lines {
		if l == pattern {
			return IgnoreLine{Source: src, Line: i + 1, Pattern: pattern}, nil
		}
	}
	_, statErr := os.Stat(p)
	lines = append(lines, pattern)
	if err := writeLines(p, lines, eol); err != nil {
		return IgnoreLine{}, err
	}
	return IgnoreLine{Source: src, Line: len(lines), Pattern: pattern, Created: errors.Is(statErr, os.ErrNotExist)}, nil
}

// ErrRuleMoved means an ignore file changed since its rule was read.
var ErrRuleMoved = errors.New("the ignore file changed since it was read")

// RemoveIgnoreRule deletes one line from an ignore file, after checking it still holds
// the pattern. If the line moved, the pattern is looked for once elsewhere in the file.
func (r *Repo) RemoveIgnoreRule(ctx context.Context, rule IgnoreLine) error {
	p, _, err := r.sourceFile(ctx, rule.Source)
	if err != nil {
		return err
	}
	lines, eol, err := readLines(p)
	if err != nil {
		return err
	}
	at := rule.Line - 1
	if at < 0 || at >= len(lines) || strings.TrimRight(lines[at], " \t") != strings.TrimRight(rule.Pattern, " \t") {
		at = -1
		for i, l := range lines {
			if l == rule.Pattern {
				if at >= 0 {
					return ErrRuleMoved
				}
				at = i
			}
		}
		if at < 0 {
			return ErrRuleMoved
		}
	}
	lines = append(lines[:at], lines[at+1:]...)
	if rule.Created && len(lines) == 0 {
		return os.Remove(p)
	}
	return writeLines(p, lines, eol)
}

// rootPattern rewrites a pattern from a folder's .gitignore as the same pattern at the top
// of the work tree, for previewing it.
func rootPattern(dir, pattern string) string {
	dir = strings.Trim(dir, "/")
	if dir == "" {
		return pattern
	}
	neg := strings.HasPrefix(pattern, "!")
	body := strings.TrimPrefix(pattern, "!")
	if strings.Contains(strings.TrimSuffix(body, "/"), "/") {
		body = "/" + dir + "/" + strings.TrimPrefix(body, "/")
	} else {
		body = "/" + dir + "/**/" + body
	}
	if neg {
		return "!" + body
	}
	return body
}

// PreviewIgnore says which files a new rule would hide, without writing it. Visible
// untracked files are checked against the rule with git's own matching, behind every
// existing rule, so a "!" rule that brings a file back still wins.
func (r *Repo) PreviewIgnore(ctx context.Context, t IgnoreTarget, pattern string) (IgnorePreview, error) {
	pv := IgnorePreview{Hidden: []string{}, Tracked: []string{}}
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || strings.HasPrefix(pattern, "#") || strings.HasPrefix(pattern, "!") {
		return pv, nil
	}
	if t.Scope == IgnoreDir {
		pattern = rootPattern(t.Dir, pattern)
	}
	st, err := r.Status(ctx)
	if err != nil {
		return pv, err
	}
	var untracked []string
	for _, f := range st.Files {
		if f.Untracked {
			untracked = append(untracked, f.Path)
		}
	}
	if len(untracked) > 0 {
		tmp, err := os.CreateTemp("", "gitrevenge-ignore-*")
		if err != nil {
			return pv, err
		}
		defer func() { _ = os.Remove(tmp.Name()) }()
		_, werr := tmp.WriteString(pattern + "\n")
		if cerr := tmp.Close(); werr != nil || cerr != nil {
			return pv, errors.Join(werr, cerr)
		}
		// core.excludesFile is read last, so every existing rule keeps precedence.
		rules, err := r.checkIgnore(ctx, untracked, false, "-c", "core.excludesFile="+tmp.Name())
		if err != nil {
			return pv, err
		}
		for _, p := range untracked {
			if m, ok := rules[p]; ok && sameIgnoreFile(m.Source, tmp.Name()) && !strings.HasPrefix(m.Pattern, "!") {
				pv.Hidden = append(pv.Hidden, p)
			}
		}
	}
	out, err := r.Read(ctx, "ls-files", "-z", "--cached", "--ignored", "--exclude="+pattern)
	if err != nil {
		return pv, err
	}
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" {
			pv.Tracked = append(pv.Tracked, p)
		}
	}
	return pv, nil
}

// checkIgnore runs `git check-ignore -v` on paths and returns the rule that decides each
// matched one, including "!" rules that re-include a path. With noIndex, paths git
// tracks (or folders holding them) are checked too. Extra args go before the subcommand,
// such as -c settings.
func (r *Repo) checkIgnore(ctx context.Context, paths []string, noIndex bool, pre ...string) (map[string]IgnoreLine, error) {
	res := map[string]IgnoreLine{}
	if len(paths) == 0 {
		return res, nil
	}
	args := append(append([]string{}, pre...), "check-ignore", "-v", "-z", "-n", "--stdin")
	if noIndex {
		args = append(args, "--no-index")
	}
	out, err := r.runner.Run(ctx, Cmd{
		Dir:      r.dir,
		Args:     args,
		Stdin:    strings.NewReader(strings.Join(paths, "\x00") + "\x00"),
		ReadOnly: true,
	})
	var gerr *Error
	if errors.As(err, &gerr) && gerr.ExitCode == 1 && gerr.Stderr == "" {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	// -z -v records: source NUL line NUL pattern NUL path NUL; -n prints unmatched paths
	// with empty fields.
	f := strings.Split(string(out), "\x00")
	for i := 0; i+3 < len(f); i += 4 {
		if f[i] == "" {
			continue
		}
		n, _ := strconv.Atoi(f[i+1])
		src := f[i]
		if filepath.IsAbs(src) {
			src = filepath.Clean(src)
		}
		res[f[i+3]] = IgnoreLine{Source: src, Line: n, Pattern: f[i+2]}
	}
	return res, nil
}

// IgnoredFiles lists ignored paths in the work tree with the rule behind each. A wholly
// ignored folder is one entry. truncated reports that the list was cut at maxIgnored.
func (r *Repo) IgnoredFiles(ctx context.Context) (files []IgnoredFile, truncated bool, err error) {
	out, err := r.Read(ctx, "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--directory")
	if err != nil {
		return nil, false, err
	}
	var paths []string
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	if len(paths) > maxIgnored {
		paths, truncated = paths[:maxIgnored], true
	}
	// A folder git reports whole is checked without its trailing slash, which check-ignore
	// would otherwise take as a request about the folder's contents.
	asked := make([]string, len(paths))
	for i, p := range paths {
		asked[i] = strings.TrimSuffix(p, "/")
	}
	ancestors := map[string]bool{}
	for _, p := range asked {
		for d := path.Dir(p); d != "." && d != "/" && d != ""; d = path.Dir(d) {
			ancestors[d] = false
		}
	}
	rules, err := r.checkIgnore(ctx, asked, false)
	if err != nil {
		return nil, false, err
	}
	// A folder that holds tracked files only reads as ignored without the index.
	dirs := make([]string, 0, len(ancestors))
	for d := range ancestors {
		dirs = append(dirs, d)
	}
	above, err := r.checkIgnore(ctx, dirs, true)
	if err != nil {
		return nil, false, err
	}
	for d := range ancestors {
		if m, ok := above[d]; ok && !strings.HasPrefix(m.Pattern, "!") {
			ancestors[d] = true
		}
	}
	files = make([]IgnoredFile, 0, len(paths))
	for i, p := range paths {
		m, ok := rules[asked[i]]
		if !ok {
			continue
		}
		can := true
		for d := path.Dir(asked[i]); d != "." && d != "/" && d != ""; d = path.Dir(d) {
			if ancestors[d] {
				can = false
				break
			}
		}
		files = append(files, IgnoredFile{Path: p, Rule: m, CanExcept: can})
	}
	return files, truncated, nil
}

// ExceptIgnored adds a "!" rule that brings one ignored path back. It goes in the file
// whose rule ignores the path, after that rule, so it wins; a rule from the global
// excludes file is answered in info/exclude instead, so other repositories are untouched.
func (r *Repo) ExceptIgnored(ctx context.Context, p string, rule IgnoreLine) (IgnoreLine, error) {
	_, base, err := r.sourceFile(ctx, rule.Source)
	if err != nil {
		return IgnoreLine{}, err
	}
	dir := strings.TrimSuffix(p, "/")
	isDir := strings.HasSuffix(p, "/")
	if !filepath.IsAbs(rule.Source) && path.Base(filepath.ToSlash(rule.Source)) == ".gitignore" {
		rel := dir
		if base != "" {
			if !strings.HasPrefix(dir, base+"/") {
				return IgnoreLine{}, fmt.Errorf("%s is outside the folder of %s", p, rule.Source)
			}
			rel = strings.TrimPrefix(dir, base+"/")
		}
		pat := "!" + EscapeIgnorePath(rel)
		if isDir {
			pat += "/"
		}
		return r.AddIgnoreRule(ctx, IgnoreTarget{Scope: IgnoreDir, Dir: base}, pat)
	}
	pat := "!" + EscapeIgnorePath(dir)
	if isDir {
		pat += "/"
	}
	return r.AddIgnoreRule(ctx, IgnoreTarget{Scope: IgnoreExclude}, pat)
}

// StopTracking removes paths from the index, keeping the files on disk.
func (r *Repo) StopTracking(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := r.Write(ctx, pathspecCmd(paths, "rm", "--cached", "-r", "-q", "--ignore-unmatch"))
	return err
}

// RestoreIgnoreRule puts a removed rule back on its old line, or at the end if the file
// has since become shorter. It undoes RemoveIgnoreRule.
func (r *Repo) RestoreIgnoreRule(ctx context.Context, rule IgnoreLine) error {
	if strings.TrimSpace(rule.Pattern) == "" || strings.ContainsAny(rule.Pattern, "\r\n") {
		return errors.New("an ignore rule is one non-empty line")
	}
	p, _, err := r.sourceFile(ctx, rule.Source)
	if err != nil {
		return err
	}
	lines, eol, err := readLines(p)
	if err != nil {
		return err
	}
	at := min(max(rule.Line-1, 0), len(lines))
	lines = append(lines[:at], append([]string{rule.Pattern}, lines[at:]...)...)
	return writeLines(p, lines, eol)
}
