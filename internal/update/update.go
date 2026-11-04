// Package update checks GitHub Releases for a newer version of the app. There is no
// auto-update: the app only says a release exists and links to it.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// LatestURL is the GitHub API endpoint for the newest published, non-pre-release release.
const LatestURL = "https://api.github.com/repos/Git-Revenge/GitRevenge/releases/latest"

// Release is the newest published release.
type Release struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

// Latest fetches the newest release from url. It sends no identifiers beyond the
// User-Agent GitHub's API requires.
func Latest(ctx context.Context, client *http.Client, url, current string) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "GitRevenge/"+current)
	res, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub returned %s", res.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return Release{}, err
	}
	return Release{Version: strings.TrimPrefix(body.TagName, "v"), URL: body.HTMLURL}, nil
}

// Newer reports whether version a is newer than b. Both are semver, with or without a
// leading "v"; a pre-release sorts before its release (0.6.0-dev < 0.6.0). Unparseable
// versions are never newer.
func Newer(a, b string) bool {
	va, okA := parse(a)
	vb, okB := parse(b)
	if !okA || !okB {
		return false
	}
	for i := range 3 {
		if va.core[i] != vb.core[i] {
			return va.core[i] > vb.core[i]
		}
	}
	switch {
	case va.pre == vb.pre:
		return false
	case va.pre == "":
		return true
	case vb.pre == "":
		return false
	}
	return comparePre(va.pre, vb.pre) > 0
}

type semver struct {
	core [3]int
	pre  string
}

func parse(s string) (semver, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s, _, _ = strings.Cut(s, "+")
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var v semver
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return semver{}, false
		}
		v.core[i] = n
	}
	v.pre = pre
	return v, true
}

// comparePre orders pre-release identifiers as semver does: numeric ones numerically and
// below alphanumeric ones, then by count.
func comparePre(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		na, errA := strconv.Atoi(as[i])
		nb, errB := strconv.Atoi(bs[i])
		switch {
		case errA == nil && errB == nil:
			if na != nb {
				return na - nb
			}
		case errA == nil:
			return -1
		case errB == nil:
			return 1
		default:
			if c := strings.Compare(as[i], bs[i]); c != 0 {
				return c
			}
		}
	}
	return len(as) - len(bs)
}
