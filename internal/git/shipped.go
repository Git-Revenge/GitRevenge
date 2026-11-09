package git

import (
	"bufio"
	"context"
	"errors"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Insights queries (ADR-0008) are a pure function of the repository and their inputs: they
// pin every setting that could make two people's results differ, and measure time from the
// commits themselves, never from the clock.

// pinned returns the -c overrides every insights query runs with. Names come only from the
// .mailmap committed at target, never from the user's mailmap.file.
func pinned(target string) []string {
	return []string{
		"-c", "log.mailmap=true",
		"-c", "mailmap.file=",
		"-c", "mailmap.blob=" + target + ":.mailmap",
		"-c", "diff.renames=true",
		"-c", "diff.renameLimit=1000",
		"-c", "log.showSignature=false",
		"-c", "i18n.logOutputEncoding=UTF-8",
	}
}

// InsightsTarget returns the branch insights measure against: the one origin/HEAD points
// at, else main, else master. It returns ErrNoTarget when there's none of these.
func (r *Repo) InsightsTarget(ctx context.Context) (string, error) {
	if ref, err := r.ReadString(ctx, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil && ref != "" {
		return ref, nil
	}
	for _, b := range []string{"main", "master"} {
		if _, err := r.ReadString(ctx, "rev-parse", "--verify", "--quiet", "refs/heads/"+b+"^{commit}"); err == nil {
			return b, nil
		}
	}
	return "", ErrNoTarget
}

// ErrNoTarget means a repository has no branch to measure insights against.
var ErrNoTarget = errors.New("no origin/HEAD, main or master branch")

// TipInfo is a ref's commit and its committer date, the default as-of point.
type TipInfo struct {
	SHA string `json:"sha"`
	// Time is the committer date, Unix seconds.
	Time int64 `json:"time"`
}

// ForkPoint returns where branch left target: their merge base, with its committer date.
func (r *Repo) ForkPoint(ctx context.Context, branch, target string) (TipInfo, error) {
	base, err := r.ReadString(ctx, "merge-base", branch, target)
	if err != nil {
		return TipInfo{}, err
	}
	return r.Tip(ctx, strings.TrimSpace(base))
}

// Tip resolves rev to its commit and committer date.
func (r *Repo) Tip(ctx context.Context, rev string) (TipInfo, error) {
	out, err := r.ReadString(ctx, "log", "-1", "--format=%H %ct", rev+"^{commit}", "--")
	if err != nil {
		return TipInfo{}, err
	}
	sha, at, _ := strings.Cut(out, " ")
	t, _ := strconv.ParseInt(at, 10, 64)
	return TipInfo{SHA: sha, Time: t}, nil
}

// TagInfo is a tag, for choosing a release window.
type TagInfo struct {
	Name string `json:"name"`
	// Time is the tag's creation date (the tagger date, or the commit's for a lightweight
	// tag), Unix seconds.
	Time int64 `json:"time"`
}

// Tags lists tags newest first, ties broken by name.
func (r *Repo) Tags(ctx context.Context) ([]TagInfo, error) {
	out, err := r.ReadString(ctx, "for-each-ref", "--format=%(refname:short)%00%(creatordate:unix)", "refs/tags")
	if err != nil {
		return nil, err
	}
	tags := []TagInfo{}
	for line := range strings.SplitSeq(out, "\n") {
		name, at, ok := strings.Cut(line, "\x00")
		if !ok {
			continue
		}
		t, _ := strconv.ParseInt(at, 10, 64)
		tags = append(tags, TagInfo{Name: name, Time: t})
	}
	slices.SortFunc(tags, func(a, b TagInfo) int {
		if a.Time != b.Time {
			return int(b.Time - a.Time)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return tags, nil
}

// ShippedQuery picks the shipped items: first-parent commits on Target, either committed
// in [Since, Until] (Unix seconds, inclusive) or, with Range set (such as "v1.0..v1.1"),
// the commits in that range.
type ShippedQuery struct {
	Target       string
	Since, Until int64
	Range        string
	// Match finds ticket references in a subject (place "message") or a reference trailer's
	// value (place "trailers"). Nil uses the built-in PROJ-123 and #123.
	Match func(text, place string) []string
}

// ShippedItem is one first-parent commit on the target branch.
type ShippedItem struct {
	SHA   string `json:"sha"`
	Title string `json:"title"`
	// Author and Email are as the committed .mailmap resolves them.
	Author string `json:"author"`
	Email  string `json:"email"`
	// Committed is the committer date, Unix seconds: when the item landed.
	Committed int64 `json:"committed"`
	// Merge is true for a merge commit, which stands for the branch it brought in.
	Merge   bool `json:"merge"`
	Files   int  `json:"files"`
	Added   int  `json:"added"`
	Removed int  `json:"removed"`
	// Refs are ticket references from the subject and trailers, in order of appearance.
	Refs []string `json:"refs"`
	// Cycle is seconds from the merged branch's first commit to the merge; 0 when unknown,
	// as for squash merges, which don't keep that history.
	Cycle  int64 `json:"cycle"`
	Revert bool  `json:"revert"`
	Fixup  bool  `json:"fixup"`
	// Bot marks a dependency update opened by a bot (set by the app from the repository's
	// Bot branch patterns, not by git).
	Bot bool `json:"bot"`
}

// Shipped lists the items of q, newest first, as they appear on the first-parent line.
func (r *Repo) Shipped(ctx context.Context, q ShippedQuery) ([]ShippedItem, error) {
	args := append(pinned(q.Target),
		"log", "--first-parent", "--diff-merges=first-parent", "--numstat", "-M",
		"--no-textconv", "--no-ext-diff",
		"--format=%x1e%H%x1f%P%x1f%aN%x1f%aE%x1f%ct%x1f%B%x1d")
	if q.Range != "" {
		args = append(args, q.Range, "--")
	} else {
		args = append(args, q.Target, "--")
	}

	items := []ShippedItem{}
	var parents [][]string
	err := r.Stream(ctx, func(rd io.Reader) error {
		br := bufio.NewReaderSize(rd, 64*1024)
		for {
			rec, err := br.ReadString('\x1e')
			rec = strings.TrimSuffix(rec, "\x1e")
			if head, numstat, ok := strings.Cut(rec, "\x1d"); ok {
				item, ps, stop := parseShipped(head, numstat, q)
				if stop {
					return errStopWalk
				}
				if item != nil {
					items = append(items, *item)
					parents = append(parents, ps)
				}
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}, args...)
	if err != nil && !errors.Is(err, errStopWalk) && !isEmptyRepoLogError(err) {
		return nil, err
	}
	for i := range items {
		if items[i].Merge {
			items[i].Cycle = r.cycle(ctx, q.Target, items[i].Committed, parents[i])
		}
	}
	return items, nil
}

// errStopWalk ends a first-parent walk once it has passed the start of the window.
var errStopWalk = errors.New("stop")

// parseShipped reads one commit record. stop is true once the walk is older than the
// window: the first-parent line is walked newest first, so nothing after it can be in it.
func parseShipped(head, numstat string, q ShippedQuery) (item *ShippedItem, parents []string, stop bool) {
	f := strings.SplitN(strings.TrimLeft(head, "\n"), "\x1f", 6)
	if len(f) != 6 {
		return nil, nil, false
	}
	at, _ := strconv.ParseInt(f[4], 10, 64)
	if q.Range == "" {
		if at < q.Since {
			return nil, nil, true
		}
		if at > q.Until {
			return nil, nil, false
		}
	}
	parents = strings.Fields(f[1])
	body := strings.TrimSpace(f[5])
	title, _, _ := strings.Cut(body, "\n")
	it := &ShippedItem{
		SHA: f[0], Title: title, Author: f[2], Email: f[3], Committed: at,
		Merge: len(parents) > 1, Refs: ReferencesWith(body, q.Match),
		Revert: strings.HasPrefix(title, "Revert \""),
		Fixup:  strings.HasPrefix(title, "fixup! ") || strings.HasPrefix(title, "squash! ") || strings.HasPrefix(title, "amend! "),
	}
	for line := range strings.SplitSeq(numstat, "\n") {
		cols := strings.SplitN(line, "\t", 3)
		if len(cols) != 3 {
			continue
		}
		it.Files++
		// Binary files show "-" and add no lines.
		if a, err := strconv.Atoi(cols[0]); err == nil {
			it.Added += a
		}
		if d, err := strconv.Atoi(cols[1]); err == nil {
			it.Removed += d
		}
	}
	return it, parents, false
}

// cycle measures a merge from the oldest commit it brought in (by author date) to the merge
// itself, or 0 if that can't be read.
func (r *Repo) cycle(ctx context.Context, target string, mergedAt int64, parents []string) int64 {
	if len(parents) < 2 {
		return 0
	}
	args := append(pinned(target), "log", "--format=%at", parents[0]+".."+parents[1], "--")
	out, err := r.ReadString(ctx, args...)
	if err != nil {
		return 0
	}
	first := int64(0)
	for line := range strings.SplitSeq(out, "\n") {
		if t, err := strconv.ParseInt(line, 10, 64); err == nil && (first == 0 || t < first) {
			first = t
		}
	}
	if first == 0 || mergedAt < first {
		return 0
	}
	return mergedAt - first
}

// referenceTrailers are the trailer keys whose values name tickets.
var referenceTrailers = []string{"refs", "references", "closes", "close", "closed", "fixes", "fix", "fixed", "resolves", "resolve", "resolved", "issue", "issues", "ticket", "see"}

var referenceRE = regexp.MustCompile(`\b[A-Z][A-Z0-9_]*-[0-9]+\b|#[0-9]+\b`)

// References finds ticket references (PROJ-123 or #123) in a message's subject and in its
// reference trailers, in order of first appearance.
func References(message string) []string { return ReferencesWith(message, nil) }

// ReferencesWith is References with the matching given: match finds references in a subject
// ("message") or a reference trailer's value ("trailers"). Nil means the built-in matching.
func ReferencesWith(message string, match func(text, place string) []string) []string {
	if match == nil {
		match = func(text, _ string) []string { return referenceRE.FindAllString(text, -1) }
	}
	subject, body, _ := strings.Cut(strings.TrimSpace(message), "\n")
	type text struct{ s, place string }
	texts := []text{{subject, "message"}}
	_, trailers := SplitTrailers(body)
	for _, t := range trailers {
		if slices.Contains(referenceTrailers, strings.ToLower(t.Key)) {
			texts = append(texts, text{t.Value, "trailers"})
		}
	}
	refs := []string{}
	for _, s := range texts {
		for _, m := range match(s.s, s.place) {
			if !slices.Contains(refs, m) {
				refs = append(refs, m)
			}
		}
	}
	return refs
}
