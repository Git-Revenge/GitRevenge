package git

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// Described is where a commit stands among the repository's tags (#940): the tag it comes
// after, as git describe names it, and the first tag that contains it.
type Described struct {
	// Name is git describe's name for it: "v1.1.0-12-gabc1234", or the tag itself when the
	// commit is tagged. "" when no tag comes before it.
	Name string `json:"name"`
	// After is the tag it comes after, and Since how many commits it is past it: 0 when the
	// commit is the tag's own.
	After string `json:"after"`
	Since int    `json:"since"`
	// FirstIn is the first tag that contains it, by git describe --contains: "" when none
	// does yet. Contains is that command's own answer, such as "v1.2.0~3".
	FirstIn  string `json:"firstIn"`
	Contains string `json:"contains"`
}

// describeLong reads describe's --long answer: the tag, the commits since it and the id. A tag
// may hold dashes, so it's read from the right.
var describeLong = regexp.MustCompile(`^(.+)-(\d+)-g[0-9a-f]+$`)

// Describe says which tag a commit comes after and the first tag that contains it. Tags of
// either kind count, as bisect's do: plenty of projects tag releases without annotating them.
// A commit with no tag before it, or none after, leaves those parts empty: that's an answer,
// not an error. On the Linux kernel each part takes under a second (0.3 s and 0.7 to 1 s).
func (r *Repo) Describe(ctx context.Context, id string) (Described, error) {
	var d Described
	if err := CheckRevs(id); err != nil {
		return d, err
	}
	long, err := r.ReadString(ctx, "describe", "--tags", "--long", id)
	if err == nil {
		if m := describeLong.FindStringSubmatch(long); m != nil {
			d.After = m[1]
			d.Since, _ = strconv.Atoi(m[2])
			d.Name = long
			if d.Since == 0 {
				d.Name = d.After
			}
		}
	} else if ctx.Err() != nil {
		return d, ctx.Err()
	}
	contains, err := r.ReadString(ctx, "describe", "--contains", "--tags", id)
	if err == nil && contains != "" {
		d.Contains = contains
		// "v1.2.0~3" or "v3.19-rc5~10^2~4^2": the tag is what comes before the first ~ or ^,
		// neither of which a ref name may hold.
		d.FirstIn = contains
		if i := strings.IndexAny(contains, "~^"); i >= 0 {
			d.FirstIn = contains[:i]
		}
		// An annotated tag's own commit comes back as "v1.1.0^0": that's the tag itself.
		if contains == d.FirstIn+"^0" {
			d.Contains = d.FirstIn
		}
	} else if ctx.Err() != nil {
		return d, ctx.Err()
	}
	return d, nil
}
