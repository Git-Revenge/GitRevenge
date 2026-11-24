package app

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/semdiff"
)

// Diff modes for FileContents.
const (
	// DiffCommit compares a commit's file with its first parent.
	DiffCommit = "commit"
	// DiffStaged compares HEAD with the index.
	DiffStaged = "staged"
	// DiffUnstaged compares the index with the work tree.
	DiffUnstaged = "unstaged"
)

// FileContents returns both versions of a file for the diff viewer. commit is used in
// commit mode; oldPath is the source of a rename, if any. force loads files over the
// normal size limit.
func (a *App) FileContents(path, mode, commit, file, oldPath string, force bool) (git.FileContents, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.FileContents{}, err
	}
	if oldPath == "" {
		oldPath = file
	}
	ctx := context.Background()
	var from, to git.FileRef
	switch mode {
	case DiffCommit:
		c, err := git.ReadCommit(r.Objects, commit)
		if err != nil {
			return git.FileContents{}, err
		}
		if len(c.Parents) > 0 {
			from = git.FileRef{Rev: c.Parents[0], Path: oldPath}
		}
		to = git.FileRef{Rev: c.ID, Path: file}
	case DiffStaged:
		from, to = git.FileRef{Rev: "HEAD", Path: oldPath}, git.IndexVersion(file)
	case DiffUnstaged:
		from, to = git.IndexVersion(file), git.WorkTreeVersion(file)
	default:
		return git.FileContents{}, userError("Unknown diff mode " + mode + ".")
	}
	return r.Git.Contents(ctx, file, from, to, force)
}

// FileDiff is a file's two versions and how they differ, for the diff views.
type FileDiff struct {
	Contents git.FileContents `json:"contents"`
	// Diff compares the two texts. It's nil unless Contents.Kind is "text".
	Diff *semdiff.Result `json:"diff"`
}

// How FileDiff reads a file in a language it knows.
const (
	// ReadLines compares whole lines, as git does.
	ReadLines = "lines"
	// ReadCode reads the file as code, within the quick time budget (semdiff.QuickRead).
	ReadCode = "code"
	// ReadPatient reads it as code with longer to parse (semdiff.PatientRead): the second
	// try, in the background, for a file that missed the quick one.
	ReadPatient = "patient"
	// ReadAnyway reads it as code when asked to, with up to semdiff.LongRead to parse: long
	// enough for any file under the size limit, short of keeping a core busy for good.
	ReadAnyway = "anyway"
)

// FileDiff returns both versions of a file, as FileContents does, and compares them by the
// line diff chosen in settings, or else the repository's diff.algorithm. Unless read is
// ReadLines, a file in a language the app knows is read as code too; its language comes from
// its linguist-language or diff attribute, or else its name. The comparison stops if the
// repository closes first.
func (a *App) FileDiff(path, mode, commit, file, oldPath string, force bool, read string) (FileDiff, error) {
	timeouts := map[string]time.Duration{ReadLines: 0, ReadCode: 0, ReadPatient: semdiff.PatientRead, ReadAnyway: semdiff.LongRead}
	timeout, ok := timeouts[read]
	if !ok {
		return FileDiff{}, userError("Unknown way to read a diff: " + read + ".")
	}
	r, err := a.openRepo(path)
	if err != nil {
		return FileDiff{}, err
	}
	m, err := a.sessions()
	if err != nil {
		return FileDiff{}, err
	}
	s := m.Settings().Get()
	opts := semdiff.Options{Algorithm: s.DiffAlgorithm, Moves: !s.DiffMovesOff, ReadTime: timeout}
	// git's setting and the file's attributes are read while the file is, so they cost no
	// wait.
	fromGit := make(chan string, 1)
	if opts.Algorithm == "" {
		go func() { fromGit <- gitDiffAlgorithm(r.Git) }()
	}
	language := make(chan string, 1)
	go func() { language <- fileLanguage(r.Git, file) }()
	contents, err := a.FileContents(path, mode, commit, file, oldPath, force)
	if opts.Algorithm == "" {
		opts.Algorithm = <-fromGit
	}
	lang := <-language
	if err != nil || contents.Kind != "text" {
		return FileDiff{Contents: contents}, err
	}
	// A script without a name the app knows says what it is on its #! line.
	if lang == "" {
		lang = scriptLanguage(contents.New.Text)
		if lang == "" {
			lang = scriptLanguage(contents.Old.Text)
		}
	}
	if read != ReadLines {
		opts.Language = lang
	}
	res, err := a.diffs.get().Diff(r.Git.Context(), contents.Old.Text, contents.New.Text, opts)
	if err != nil {
		return FileDiff{}, err
	}
	// Compared by lines, the view still says what the file is, to offer reading it as code.
	if res.Language == "" {
		res.Language = semdiff.LanguageName(lang)
	}
	return FileDiff{Contents: contents, Diff: &res}, nil
}

// fileLanguage is the language file is read as code in, or "": its linguist-language or diff
// attribute decides, or else its name (semdiff.LanguageOf).
func fileLanguage(g *git.Repo, file string) string {
	return fileLanguages(g, []string{file})[file]
}

// scriptLanguage is the language text's #! line names, or "".
func scriptLanguage(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return semdiff.LanguageOfScript(line)
}

// fileLanguages is fileLanguage for several files, with one git call.
func fileLanguages(g *git.Repo, files []string) map[string]string {
	all, _ := g.AttributesOf(context.Background(), files, "linguist-language", "diff")
	out := make(map[string]string, len(files))
	for _, f := range files {
		value := func(name string) string {
			switch v := all[f][name]; v {
			case "set", "unset":
				return "" // an attribute without a value names no language
			default:
				return v
			}
		}
		out[f] = semdiff.LanguageOf(f, value("linguist-language"), value("diff"))
	}
	return out
}

// maxFormattingFiles is the most files FormattingOnly reads; a bigger list goes unmarked past
// them.
const maxFormattingFiles = 100

// FormattingOnly returns which of files change only formatting, read as code, for a list to
// mark them. mode and commit are as FileContents takes them. It reads at most
// maxFormattingFiles of them, and never marks a file it can't read as code.
func (a *App) FormattingOnly(path, mode, commit string, files []string) ([]string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	m, err := a.sessions()
	if err != nil {
		return nil, err
	}
	s := m.Settings().Get()
	if s.DiffLinesOnly {
		return []string{}, nil
	}
	files = files[:min(len(files), maxFormattingFiles)]
	langs := fileLanguages(r.Git, files)
	// The options the diff view uses, so a file opened from the list is already compared.
	opts := semdiff.Options{Algorithm: s.DiffAlgorithm, Moves: !s.DiffMovesOff}
	if opts.Algorithm == "" {
		opts.Algorithm = gitDiffAlgorithm(r.Git)
	}
	var (
		mu   sync.Mutex
		out  = []string{}
		wg   sync.WaitGroup
		gate = make(chan struct{}, 4)
	)
	for _, f := range files {
		if langs[f] == "" {
			continue
		}
		wg.Go(func() {
			gate <- struct{}{}
			defer func() { <-gate }()
			c, err := a.FileContents(path, mode, commit, f, "", false)
			if err != nil || c.Kind != "text" {
				return
			}
			o := opts
			o.Language = langs[f]
			res, err := a.diffs.get().Diff(r.Git.Context(), c.Old.Text, c.New.Text, o)
			if err != nil || res.Engine != semdiff.EngineCode || len(res.Blocks) == 0 {
				return
			}
			for _, b := range res.Blocks {
				if b.Kind != semdiff.KindFormat {
					return
				}
			}
			mu.Lock()
			out = append(out, f)
			mu.Unlock()
		})
	}
	wg.Wait()
	slices.Sort(out)
	return out, nil
}

// GitDiffAlgorithm returns the line diff the repository's diff.algorithm names, or "" when it
// names none git knows, for the setting that follows it.
func (a *App) GitDiffAlgorithm(path string) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	return gitDiffAlgorithm(r.Git), nil
}

func gitDiffAlgorithm(g *git.Repo) string {
	name, _ := g.ReadString(context.Background(), "config", "--get", "diff.algorithm")
	return semdiff.Algorithm(strings.ToLower(name))
}

// diffCache holds recent comparisons, made on first use.
type diffCache struct {
	once  sync.Once
	cache *semdiff.Cache
}

// recentDiffs is how many comparisons are kept: enough to go back and forth through a
// commit's files without comparing them again.
const recentDiffs = 64

func (d *diffCache) get() *semdiff.Cache {
	d.once.Do(func() { d.cache = semdiff.NewCache(recentDiffs) })
	return d.cache
}

// FileImage returns an image in the repository as a data URL, at the version a diff of
// mode shows (a commit, the index or the working tree). Rendered markdown uses it for
// relative images. It returns "" when the file isn't an image or doesn't exist there.
func (a *App) FileImage(path, mode, commit, file string) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	// The path comes from a document's link: keep it inside the repository.
	clean := filepath.ToSlash(filepath.Clean(file))
	if file == "" || filepath.IsAbs(file) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", nil
	}
	at, ok := versionRef(mode, commit, clean)
	if !ok {
		return "", userError("Unknown diff mode " + mode + ".")
	}
	fc, err := r.Git.Contents(context.Background(), clean, git.FileRef{}, at, false)
	if err != nil || fc.Kind != "image" || !fc.New.Exists {
		return "", nil // a broken image in a document isn't an error worth a banner
	}
	return "data:" + fc.Mime + ";base64," + fc.New.Image, nil
}

// FileData returns a file in the repository as base64, at the version a diff of mode shows,
// for previews of binary formats such as PDF. It returns "" when the file doesn't exist there.
func (a *App) FileData(path, mode, commit, file string) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	at, ok := versionRef(mode, commit, file)
	if !ok {
		return "", userError("Unknown diff mode " + mode + ".")
	}
	data, exists, err := r.Git.FileBytes(context.Background(), at)
	if err != nil {
		return "", userError(err.Error())
	}
	if !exists {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// versionRef names the version of file a diff of mode shows: the commit's, the index's or
// the working tree's.
func versionRef(mode, commit, file string) (git.FileRef, bool) {
	switch mode {
	case DiffCommit:
		return git.FileRef{Rev: commit, Path: file}, true
	case DiffStaged:
		return git.IndexVersion(file), true
	case DiffUnstaged:
		return git.WorkTreeVersion(file), true
	}
	return git.FileRef{}, false
}

// errChanged is returned when a file changed after it was displayed.
const errChanged = userError("The file changed since it was shown. The view has been refreshed; please try again.")

// SetStagedContent replaces a file's staged content with content (in work-tree form). The
// UI computes it from the displayed diff to stage or unstage hunks and lines. expected is
// the staged content the UI was showing; if the index has changed since, nothing is written.
func (a *App) SetStagedContent(path, file, expected, content string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	cur, err := r.Git.Contents(ctx, file, git.FileRef{}, git.IndexVersion(file), true)
	if err != nil {
		return err
	}
	// A file not in the index (untracked) is shown as empty.
	if cur.New.Text != expected {
		return errChanged
	}
	return writeErr(r.Git.SetIndexContent(ctx, file, content))
}

// SetWorkTreeContent overwrites a file on disk to discard hunks or lines. expected is the
// content the UI was showing; if the file has changed since, nothing is written, so newer
// edits are never lost.
func (a *App) SetWorkTreeContent(path, file, expected, content string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	cur, err := r.Git.Contents(context.Background(), file, git.FileRef{}, git.WorkTreeVersion(file), true)
	if err != nil {
		return err
	}
	if cur.New.Text != expected {
		return errChanged
	}
	return r.Git.SetWorkTreeContent(file, content)
}
