package git

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// ReflogEntry is one move of a ref: where it pointed, when, and what git recorded as the
// reason ("commit: Fix…", "checkout: moving from a to b", "reset: moving to HEAD~1").
type ReflogEntry struct {
	// Selector names the entry, e.g. "HEAD@{2}" or "main@{0}".
	Selector string `json:"selector"`
	Commit   string `json:"commit"`
	Action   string `json:"action"`
	// Time is a Unix timestamp in seconds.
	Time int64 `json:"time"`
}

// Reflog lists ref's reflog, newest first, at most limit entries. A ref without a reflog
// has no entries.
func (r *Repo) Reflog(ctx context.Context, ref string, limit int) ([]ReflogEntry, error) {
	if err := CheckRevs(ref); err != nil {
		return nil, err
	}
	out, err := r.ReadString(ctx, "reflog", "show", "--date=unix", "--format=%H%x1f%gd%x1f%gs%x1e", "-n", strconv.Itoa(limit), ref, "--")
	entries := []ReflogEntry{}
	if err != nil {
		var gerr *Error
		if errors.As(err, &gerr) && (strings.Contains(gerr.Stderr, "unknown revision") || strings.Contains(gerr.Stderr, "bad revision")) {
			return entries, nil
		}
		return nil, err
	}
	short := strings.TrimPrefix(strings.TrimPrefix(ref, "refs/heads/"), "refs/")
	for i, rec := range strings.Split(strings.TrimSpace(out), "\x1e") {
		f := strings.Split(strings.TrimLeft(rec, "\n"), "\x1f")
		if len(f) < 3 {
			continue
		}
		// With --date=unix the selector carries the entry's time: HEAD@{1700000000}.
		var t int64
		if from, to := strings.LastIndex(f[1], "@{"), strings.LastIndex(f[1], "}"); from >= 0 && to > from {
			t, _ = strconv.ParseInt(f[1][from+2:to], 10, 64)
		}
		entries = append(entries, ReflogEntry{Selector: short + "@{" + strconv.Itoa(i) + "}", Commit: f[0], Action: f[2], Time: t})
	}
	return entries, nil
}
