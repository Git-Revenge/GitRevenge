package git

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Pushing to a remote with several push URLs (#926). Git pushes to each in turn. A rejection
// (the remote has work this repository doesn't) is noted and git goes on to the next URL; a
// failure to reach one (no repository there, no network, a refused sign-in) ends the push, so
// the URLs after it are never tried. The report says which, so none of that is hidden.

// Push states, in words.
const (
	PushPushed   = "pushed"
	PushUpToDate = "up to date"
	PushRejected = "rejected"
	PushFailed   = "failed"
	PushNotTried = "not tried"
)

// PushDest is what one push URL did.
type PushDest struct {
	URL   string `json:"url"`
	State string `json:"state"`
	// Detail is git's reason for a rejection or failure.
	Detail string `json:"detail,omitempty"`
}

// porcelainBlocks reads `git push --porcelain` output: for each "To <url>" block, its refs'
// flags and summaries.
func porcelainBlocks(out string) (urls []string, refs map[string][][2]string) {
	refs = map[string][][2]string{}
	cur := ""
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "To "):
			cur = strings.TrimPrefix(line, "To ")
			urls = append(urls, cur)
		case line == "Done" || cur == "" || len(line) < 2:
		default:
			// "<flag>\t<from>:<to>\t<summary>"
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) == 3 {
				refs[cur] = append(refs[cur], [2]string{parts[0], parts[2]})
			}
		}
	}
	return urls, refs
}

// PushDests reads what each of a remote's push URLs did from `git push --porcelain` output and
// the push's error. Git goes through the URLs in order and stops at one it can't reach: that one
// failed, and the ones after it weren't tried.
func PushDests(pushURLs []string, out string, err error) []PushDest {
	_, refs := porcelainBlocks(out)
	blockFor := func(u string) ([][2]string, bool) {
		for got, rs := range refs {
			if SameRemoteURL(got, u) || got == u {
				return rs, true
			}
		}
		return nil, false
	}
	dests := make([]PushDest, 0, len(pushURLs))
	stopped := false
	for _, u := range pushURLs {
		d := PushDest{URL: u}
		rs, ok := blockFor(u)
		switch {
		case stopped:
			d.State = PushNotTried
		case !ok:
			if err == nil {
				d.State = PushUpToDate
				break
			}
			d.State, d.Detail = PushFailed, pushFailure(err)
			stopped = true
		default:
			d.State = PushUpToDate
			for _, r := range rs {
				switch r[0] {
				case "!":
					d.State, d.Detail = PushRejected, rejectionReason(r[1])
				case "=":
				default:
					if d.State != PushRejected {
						d.State = PushPushed
					}
				}
			}
		}
		dests = append(dests, d)
	}
	return dests
}

// rejectionReason reads git's summary of a refused ref, "[rejected] (fetch first)", as the
// reason alone: "fetch first".
func rejectionReason(summary string) string {
	s := strings.TrimSpace(summary)
	if strings.HasPrefix(s, "[") {
		if i := strings.Index(s, "]"); i >= 0 {
			if rest := strings.TrimSpace(s[i+1:]); rest != "" {
				return strings.Trim(rest, "()")
			}
			return strings.Trim(s, "[]")
		}
	}
	return strings.Trim(s, "()")
}

// pushFailure is git's reason a push couldn't reach a URL, its first "fatal:" line.
func pushFailure(err error) string {
	var gerr *Error
	if !errors.As(err, &gerr) {
		return err.Error()
	}
	for line := range strings.SplitSeq(gerr.Stderr, "\n") {
		if s, ok := strings.CutPrefix(strings.TrimSpace(line), "fatal: "); ok {
			return s
		}
	}
	return strings.TrimSpace(gerr.Stderr)
}

// PushWithReport pushes as Push does and says what each of the remote's push URLs did.
func (r *Repo) PushWithReport(ctx context.Context, p PushOptions, o NetOptions) ([]PushDest, error) {
	args := []string{"push", "--progress", "--porcelain"}
	if p.SetUpstream {
		args = append(args, "--set-upstream")
	}
	if p.ForceWithLease {
		args = append(args, "--force-with-lease")
	}
	args = append(args, "--", p.Remote, p.Refspec)
	c := o.cmd(args...)
	c.Dir = r.dir
	out, err := r.runner.Run(ctx, c)
	var urls []string
	if remotes, rerr := r.Remotes(ctx); rerr == nil {
		for _, rm := range remotes {
			if rm.Name == p.Remote {
				urls = rm.PushURLs
			}
		}
	}
	return PushDests(urls, string(out), err), err
}

// PushToURL pushes one refspec to one URL, for going on with the URLs a push didn't reach. It
// says what that URL did; the remote-tracking branch already moved with the push that did.
func (r *Repo) PushToURL(ctx context.Context, url, refspec string, o NetOptions) PushDest {
	if err := CheckRevs(url, refspec); err != nil {
		return PushDest{URL: url, State: PushFailed, Detail: err.Error()}
	}
	c := o.cmd("push", "--progress", "--porcelain", "--", url, refspec)
	c.Dir = r.dir
	out, err := r.runner.Run(ctx, c)
	return PushDests([]string{url}, string(out), err)[0]
}

// SetPushURLs makes a remote push to urls, in order (remote.<name>.pushurl, once each). Git
// needs the remote's own URL written among them once there are others; pushing only to its
// own URL needs none, so that leaves no pushurl behind.
func (r *Repo) SetPushURLs(ctx context.Context, name string, urls []string) error {
	if err := CheckRevs(append([]string{name}, urls...)...); err != nil {
		return err
	}
	remotes, err := r.Remotes(ctx)
	if err != nil {
		return err
	}
	fetchURL := ""
	for _, rm := range remotes {
		if rm.Name == name {
			fetchURL = rm.FetchURL
		}
	}
	if fetchURL == "" {
		return fmt.Errorf("no remote %q: %w", name, ErrInvalidName)
	}
	var keep []string
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u != "" && !slices.ContainsFunc(keep, func(k string) bool { return SameRemoteURL(k, u) }) {
			keep = append(keep, u)
		}
	}
	key := "remote." + name + ".pushurl"
	_, err = r.Write(ctx, Cmd{Args: []string{"config", "--local", "--unset-all", key}})
	var gerr *Error
	if err != nil && (!errors.As(err, &gerr) || gerr.ExitCode != 5) {
		return err
	}
	if len(keep) == 0 || (len(keep) == 1 && SameRemoteURL(keep[0], fetchURL)) {
		return nil
	}
	for _, u := range keep {
		if _, err := r.Write(ctx, Cmd{Args: []string{"config", "--local", "--add", key, u}}); err != nil {
			return err
		}
	}
	return nil
}
