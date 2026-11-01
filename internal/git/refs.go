package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// RefKind classifies a ref.
type RefKind string

// Ref kinds.
const (
	LocalBranch  RefKind = "local"
	RemoteBranch RefKind = "remote"
	Tag          RefKind = "tag"
	Stash        RefKind = "stash"
)

// Ref is a branch, remote-tracking branch, tag or stash entry.
type Ref struct {
	// Name is the full ref name, e.g. refs/heads/main, or stash@{0} for stashes.
	Name string `json:"name"`
	// Short is the display name, e.g. main, origin/main, v1.0.
	Short string  `json:"short"`
	Kind  RefKind `json:"kind"`
	// Commit is the commit the ref points at (annotated tags are peeled).
	Commit string `json:"commit"`
	// Head marks the checked-out branch.
	Head bool `json:"head"`
	// Upstream is the tracked branch's short name, for local branches.
	Upstream string `json:"upstream"`
	Ahead    int    `json:"ahead"`
	Behind   int    `json:"behind"`
	// Gone means the upstream is configured but no longer exists.
	Gone bool `json:"gone"`
	// Hidden marks a branch the repository's hide patterns match (set by the session, not
	// by git).
	Hidden bool `json:"hidden,omitempty"`
	// BranchKind is the branch's kind from the repository's patterns ("trunk", "release",
	// "environment", "docs", "bot", "feature"), or "" for an ordinary branch (set by the session).
	BranchKind string `json:"branchKind,omitempty"`
	// StackParent is the local branch this one is stacked on, inferred or set by the user
	// (set by the session). StackDepth is its place in the stack, 1 for the branch at the
	// bottom, and StackSize the stack's height; both 0 when it isn't in a stack.
	StackParent string `json:"stackParent,omitempty"`
	StackDepth  int    `json:"stackDepth,omitempty"`
	StackSize   int    `json:"stackSize,omitempty"`
	// StackParentMerged means the parent's upstream is gone (merged and deleted on the
	// host), so this branch still carries the parent's old commits: time to restack.
	StackParentMerged bool `json:"stackParentMerged,omitempty"`
	// Subject is the stash message, for stashes.
	Subject string `json:"subject,omitempty"`
	// Updated is when the ref last changed, in Unix seconds: the tag date for an annotated
	// tag, otherwise the commit date. 0 for stashes.
	Updated int64 `json:"updated,omitempty"`
}

// refFormat deliberately avoids %(refname:short): git makes it unambiguous by comparing
// every ref against all others, which is quadratic (about 1.9s for 2,000 branches).
// Stripping the namespace prefix gives the same display name at no cost.
const refFormat = "%(refname)%00%(objectname)%00%(*objectname)%00%(HEAD)%00%(creatordate:unix)"

// upstreamFormat is separate because it's nearly all of the cost. Git spends about 125µs per ref
// per upstream field, even on a ref with no upstream: 5s of the 5.2s for 20,000 branches. So
// refFormat lists everything, and only branches that have an upstream are asked about it.
const upstreamFormat = "%(refname)%00%(upstream:short)%00%(upstream:track)"

// maxUpstreamPatternBytes keeps the upstream query's command line well under Windows' limit of
// about 32KB. Past it, the query covers all of refs/heads, as it did before the split.
var maxUpstreamPatternBytes = 16 * 1024

var trackRE = regexp.MustCompile(`(ahead|behind) (\d+)`)

// Refs lists branches, remote-tracking branches, tags and stashes. Symbolic refs such as
// refs/remotes/origin/HEAD are skipped.
func (r *Repo) Refs(ctx context.Context) ([]Ref, error) {
	var refs []Ref
	err := r.Stream(ctx, func(rd io.Reader) error {
		sc := bufio.NewScanner(rd)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			f := bytes.Split(sc.Bytes(), []byte{0})
			if len(f) != 5 {
				continue
			}
			ref := Ref{
				Name:   string(f[0]),
				Commit: string(f[1]),
				Head:   string(f[3]) == "*",
			}
			if len(f[2]) > 0 {
				ref.Commit = string(f[2]) // annotated tag: use the commit it points to
			}
			var ok bool
			switch {
			case strings.HasPrefix(ref.Name, "refs/heads/"):
				ref.Kind = LocalBranch
				ref.Short, ok = strings.CutPrefix(ref.Name, "refs/heads/")
			case strings.HasPrefix(ref.Name, "refs/remotes/"):
				if strings.HasSuffix(ref.Name, "/HEAD") {
					continue
				}
				ref.Kind = RemoteBranch
				ref.Short, ok = strings.CutPrefix(ref.Name, "refs/remotes/")
			case strings.HasPrefix(ref.Name, "refs/tags/"):
				ref.Kind = Tag
				ref.Short, ok = strings.CutPrefix(ref.Name, "refs/tags/")
			}
			if !ok {
				continue
			}
			// A ref whose object has no date (a tag of a tree, say) just sorts last.
			ref.Updated, _ = strconv.ParseInt(string(f[4]), 10, 64)
			refs = append(refs, ref)
		}
		return sc.Err()
	}, "for-each-ref", "--format="+refFormat, "refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return nil, err
	}
	if err := r.addUpstreams(ctx, refs); err != nil {
		return nil, err
	}

	stashes, err := r.stashes(ctx)
	if err != nil {
		return nil, err
	}
	return append(refs, stashes...), nil
}

// addUpstreams fills in the upstream fields of the local branches that have one.
func (r *Repo) addUpstreams(ctx context.Context, refs []Ref) error {
	configured, known, err := r.branchesWithUpstream(ctx)
	if err != nil {
		return err
	}
	if known && len(configured) == 0 {
		return nil
	}
	byName := make(map[string]*Ref, len(refs))
	for i := range refs {
		if refs[i].Kind == LocalBranch {
			byName[refs[i].Name] = &refs[i]
		}
	}
	args := []string{"for-each-ref", "--format=" + upstreamFormat}
	args = append(args, upstreamPatterns(configured, known, byName)...)
	if len(args) == 2 {
		return nil // Every configured branch has been deleted.
	}
	return r.Stream(ctx, func(rd io.Reader) error {
		sc := bufio.NewScanner(rd)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			f := bytes.Split(sc.Bytes(), []byte{0})
			if len(f) != 3 {
				continue
			}
			if ref := byName[string(f[0])]; ref != nil {
				ref.setUpstream(string(f[1]), string(f[2]))
			}
		}
		return sc.Err()
	}, args...)
}

// upstreamPatterns narrows the upstream query to the branches listed, when that's known and fits
// on a command line, and otherwise covers them all.
func upstreamPatterns(configured []string, known bool, byName map[string]*Ref) []string {
	if !known {
		return []string{"refs/heads"}
	}
	patterns := make([]string, 0, len(configured))
	size := 0
	for _, name := range configured {
		full := "refs/heads/" + name
		if byName[full] == nil {
			continue // The config outlives a deleted branch.
		}
		size += len(full) + 1
		if size > maxUpstreamPatternBytes {
			return []string{"refs/heads"}
		}
		patterns = append(patterns, full)
	}
	return patterns
}

// setUpstream records a branch's upstream from for-each-ref's short name and track fields.
func (ref *Ref) setUpstream(short, track string) {
	ref.Upstream = short
	ref.Gone = track == "[gone]"
	for _, m := range trackRE.FindAllStringSubmatch(track, -1) {
		n, _ := strconv.Atoi(m[2])
		if m[1] == "ahead" {
			ref.Ahead = n
		} else {
			ref.Behind = n
		}
	}
}

// branchesWithUpstream lists the branches whose config names an upstream. known is false if that
// couldn't be read, in which case the caller should check every branch.
func (r *Repo) branchesWithUpstream(ctx context.Context) (names []string, known bool, err error) {
	out, err := r.Read(ctx, "config", "-z", "--get-regexp", "^branch\\..*\\.merge$")
	if err != nil {
		var gerr *Error
		if errors.As(err, &gerr) && gerr.ExitCode == 1 && gerr.Stderr == "" {
			return nil, true, nil // Nothing matched: no branch has an upstream.
		}
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		return nil, false, nil
	}
	seen := map[string]bool{}
	for _, entry := range bytes.Split(out, []byte{0}) {
		// With -z, each entry is "key\nvalue".
		key, _, _ := bytes.Cut(entry, []byte{'\n'})
		name, ok := strings.CutPrefix(string(key), "branch.")
		if !ok {
			continue
		}
		if name, ok = strings.CutSuffix(name, ".merge"); ok && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names, true, nil
}

func (r *Repo) stashes(ctx context.Context) ([]Ref, error) {
	if _, err := r.Read(ctx, "rev-parse", "--verify", "-q", "refs/stash"); err != nil {
		return nil, nil // no stashes
	}
	out, err := r.Read(ctx, "log", "-g", "-z", "--format=%H%x1f%gd%x1f%s", "refs/stash", "--")
	if err != nil {
		return nil, err
	}
	var refs []Ref
	for rec := range strings.SplitSeq(string(out), "\x00") {
		f := strings.SplitN(strings.TrimLeft(rec, "\n"), "\x1f", 3)
		if len(f) != 3 {
			continue
		}
		refs = append(refs, Ref{Name: f[1], Short: f[1], Kind: Stash, Commit: f[0], Subject: f[2]})
	}
	return refs, nil
}
