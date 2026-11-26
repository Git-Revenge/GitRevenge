package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestCheckRevs(t *testing.T) {
	t.Parallel()
	for _, ok := range [][]string{{}, {""}, {"main", "origin/main", "refs/tags/--output=x", "v1.0..v1.1", "HEAD~2", "a1b2c3"}} {
		if err := CheckRevs(ok...); err != nil {
			t.Errorf("CheckRevs(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"-x", "--output=x", "--exec=calc", "--upload-pack=x", "-"} {
		if err := CheckRevs("main", bad); !errors.Is(err, ErrInvalidName) {
			t.Errorf("CheckRevs(%q) = %v, want ErrInvalidName", bad, err)
		}
	}
}

// optionName is a tag a remote can hand over: a valid ref name that git reads as an option
// wherever it's passed as a plain argument. As an option it writes git's output to a file.
const optionName = "--output=planted.txt"

// plantedRepo is a clone of a repository whose owner made a tag named optionName, as it
// would arrive from a hostile remote.
func plantedRepo(t *testing.T) (*gittest.Repo, *Repo) {
	t.Helper()
	src := gittest.New(t)
	src.CommitFile("a.txt", "a\n", "one")
	src.CommitFile("b.txt", "b\n", "two")
	src.Git("update-ref", "refs/tags/"+optionName, "HEAD")
	src.Git("update-ref", "refs/tags/v1", "HEAD~1")
	fx := gittest.Clone(t, src)
	if tags := fx.Git("for-each-ref", "--format=%(refname:short)", "refs/tags"); !strings.Contains(tags, optionName) {
		t.Fatalf("the tag didn't arrive with the clone, so this checks nothing: %q", tags)
	}
	return fx, OpenRepo(newTestRunner(t), fx.Dir)
}

// planted lists files that git wrote because a name was read as --output.
func planted(t *testing.T, dir string) []string {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(dir, "planted.txt*"))
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// Every function that puts a name it was given on git's command line, called with a name
// that is an option. None may run git with it: each refuses, and no file appears.
func TestNamesThatAreOptionsNeverReachGit(t *testing.T) {
	t.Parallel()
	fx, repo := plantedRepo(t)
	ctx := context.Background()
	head := fx.Git("rev-parse", "HEAD")
	n := optionName
	o := NetOptions{}

	// The tag list is where the name comes from in the app (Insights > Shipped).
	tags, err := repo.Tags(ctx)
	if err != nil || len(tags) != 2 {
		t.Fatalf("Tags = %+v, %v", tags, err)
	}

	calls := map[string]func() error{
		"CommitsBetween exclude": func() error { _, _, err := repo.CommitsBetween(ctx, n, "HEAD", 10); return err },
		"CommitsBetween include": func() error { _, _, err := repo.CommitsBetween(ctx, "HEAD", n, 10); return err },
		"CommitsOnlyOn":          func() error { _, _, err := repo.CommitsOnlyOn(ctx, n, "HEAD", 10); return err },
		"BisectRange":            func() error { _, _, err := repo.BisectRange(ctx, n, "v1"); return err },
		"BisectStart bad":        func() error { return repo.BisectStart(ctx, n, "v1") },
		"BisectStart good":       func() error { return repo.BisectStart(ctx, "HEAD", n) },
		"CreateBranch start":     func() error { return repo.CreateBranch(ctx, "topic", n, false) },
		"RenameBranch from":      func() error { return repo.RenameBranch(ctx, n, "topic") },
		"DeleteBranch":           func() error { return repo.DeleteBranch(ctx, n, true) },
		"Checkout":               func() error { return repo.Checkout(ctx, n) },
		"SetUpstream":            func() error { return repo.SetUpstream(ctx, n, "origin/main") },
		"CreateTag target":       func() error { return repo.CreateTag(ctx, "v2", n, "") },
		"DeleteTag":              func() error { return repo.DeleteTag(ctx, n) },
		"SetBranchTracking": func() error {
			return repo.SetBranchTracking(ctx, "main", BranchTracking{Remote: n, Merge: "refs/heads/main"})
		},
		"ForecastMerge":  func() error { _, err := repo.ForecastMerge(ctx, "HEAD", n); return err },
		"ForecastRebase": func() error { _, err := repo.ForecastRebase(ctx, "main", n); return err },
		"ForecastPlan":   func() error { _, err := repo.ForecastPlan(ctx, n, nil); return err },
		"ResetKeep":      func() error { return repo.ResetKeep(ctx, n) },
		"Blame rev":      func() error { _, err := repo.Blame(ctx, "a.txt", n, false); return err },
		"RebasePlan":     func() error { _, err := repo.RebasePlan(ctx, n); return err },
		"Log revs": func() error {
			return repo.Log(ctx, LogOptions{Revs: []string{n}}, func(LogCommit) error { return nil })
		},
		"CountCommits":      func() error { _, err := repo.CountCommits(ctx, LogOptions{Revs: []string{n}}); return err },
		"Merge":             func() error { return repo.Merge(ctx, n, "") },
		"CherryPick":        func() error { return repo.CherryPick(ctx, head, n) },
		"RebaseOnto":        func() error { return repo.RebaseOnto(ctx, n, "v1") },
		"RebaseStack":       func() error { return repo.RebaseStack(ctx, "v1", n, "main") },
		"Rebase":            func() error { return repo.Rebase(ctx, n) },
		"RebaseAutosquash":  func() error { return repo.RebaseAutosquash(ctx, n) },
		"Notes":             func() error { _, err := repo.Notes(ctx, n); return err },
		"SetNote":           func() error { return repo.SetNote(ctx, "refs/notes/commits", n, "note") },
		"FetchNotes":        func() error { return repo.FetchNotes(ctx, n, o) },
		"IntegrateNotes":    func() error { _, err := repo.IntegrateNotes(ctx, n); return err },
		"PushNotes":         func() error { return repo.PushNotes(ctx, n, o) },
		"Reflog":            func() error { _, err := repo.Reflog(ctx, n, 10); return err },
		"RemoveRemote":      func() error { return repo.RemoveRemote(ctx, n) },
		"RenameRemote from": func() error { return repo.RenameRemote(ctx, n, "upstream") },
		"Fetch":             func() error { return repo.Fetch(ctx, n, o) },
		"PullBranch":        func() error { _, _, err := repo.PullBranch(ctx, n, o); return err },
		"MoveBranch target": func() error { return repo.MoveBranch(ctx, "main", head, n) },
		"Reset":             func() error { return repo.Reset(ctx, n, ResetMixed) },
		"Revert":            func() error { return repo.Revert(ctx, n, 0) },
		"ForkPoint":         func() error { _, err := repo.ForkPoint(ctx, n, "main"); return err },
		"Tip":               func() error { _, err := repo.Tip(ctx, n); return err },
		"Shipped range":     func() error { _, err := repo.Shipped(ctx, ShippedQuery{Target: "main", Range: n + "..v1"}); return err },
		"Shipped target":    func() error { _, err := repo.Shipped(ctx, ShippedQuery{Target: n}); return err },
		"HotFiles range": func() error {
			_, err := repo.HotFiles(ctx, ShippedQuery{Target: "main", Range: n + "..v1"})
			return err
		},
		"OpenBranches":     func() error { _, err := repo.OpenBranches(ctx, n, 10, nil); return err },
		"WouldConflict":    func() error { _, err := repo.WouldConflict(ctx, "main", n); return err },
		"VerifySignatures": func() error { _, err := repo.VerifySignatures(ctx, []string{head, n}); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, ErrInvalidName) {
			t.Errorf("%s: err = %v, want the name refused", name, err)
		}
	}

	// The ones that answer without an error answer "nothing".
	if up := repo.UpstreamOf(ctx, n); up != "" {
		t.Errorf("UpstreamOf = %q", up)
	}
	if repo.WasOnBranch(ctx, "main", n) {
		t.Error("WasOnBranch = true")
	}
	if got := repo.BisectSuggestGood(ctx, n); got != "" {
		t.Errorf("BisectSuggestGood = %q", got)
	}
	if m := repo.BranchMergeState(ctx, n, []string{"main"}); m.How != "" {
		t.Errorf("BranchMergeState = %+v", m)
	}
	if m := repo.MergedBranches(ctx, []string{"main"}, []string{n}); m != nil {
		t.Errorf("MergedBranches = %+v", m)
	}
	if m := repo.BranchStates(ctx, []string{n}, []string{"main"}); m != nil {
		t.Errorf("BranchStates = %+v", m)
	}

	if files := planted(t, fx.Dir); len(files) > 0 {
		t.Errorf("git wrote %v: a name was read as --output", files)
	}
	if now := fx.Git("rev-parse", "HEAD"); now != head {
		t.Errorf("HEAD moved from %s to %s", head, now)
	}
}

// The suggestion bisect makes is a tag's name, read back from git. One that is an option is
// never offered.
func TestBisectNeverSuggestsATagThatIsAnOption(t *testing.T) {
	t.Parallel()
	src := gittest.New(t)
	src.CommitFile("a.txt", "a\n", "one")
	src.Git("update-ref", "refs/tags/"+optionName, "HEAD")
	src.CommitFile("b.txt", "b\n", "two")
	repo := OpenRepo(newTestRunner(t), src.Dir)
	if got := repo.BisectSuggestGood(context.Background(), "HEAD"); got != "" {
		t.Errorf("suggested %q", got)
	}
}

// The object reader takes one request a line.
func TestObjectReaderRefusesALineBreakInAName(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "one")
	objects := NewObjectReader(newTestRunner(t), fx.Dir)
	defer func() { _ = objects.Close() }()
	if _, err := objects.Info("HEAD\ncontents HEAD"); err == nil {
		t.Error("a name with a line break in it was sent to git")
	}
	if _, err := objects.Info("HEAD"); err != nil {
		t.Errorf("a plain name stopped working: %v", err)
	}
	_ = os.Remove(filepath.Join(fx.Dir, "unused"))
}
