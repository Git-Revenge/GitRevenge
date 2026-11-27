package git

import (
	"context"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestDescribeSaysWhichTagsACommitIsBetween(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	ids := fx.Linear(4)
	// A lightweight tag with dashes in it, then an annotated one: both kinds count.
	fx.Git("tag", "release-2026-01", ids[0])
	fx.Git("tag", "-a", "v1.1", "-m", "v1.1", ids[2])
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	d, err := repo.Describe(ctx, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if d.After != "release-2026-01" || d.Since != 1 || !strings.HasPrefix(d.Name, "release-2026-01-1-g") {
		t.Errorf("after: %+v", d)
	}
	if d.FirstIn != "v1.1" || d.Contains != "v1.1~1" {
		t.Errorf("first in: %+v", d)
	}

	// The tag's own commit is named by the tag alone, though --contains says "v1.1^0".
	if d, err = repo.Describe(ctx, ids[2]); err != nil || d.Name != "v1.1" || d.Since != 0 || d.FirstIn != "v1.1" || d.Contains != "v1.1" {
		t.Errorf("tagged: %+v, %v", d, err)
	}

	// Past the last tag, no tag contains it yet: that's an answer, not an error.
	if d, err = repo.Describe(ctx, ids[3]); err != nil || d.After != "v1.1" || d.Since != 1 || d.FirstIn != "" || d.Contains != "" {
		t.Errorf("untagged yet: %+v, %v", d, err)
	}
}

func TestDescribeWithoutTags(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	ids := fx.Linear(2)
	d, err := OpenRepo(newTestRunner(t), fx.Dir).Describe(context.Background(), ids[0])
	if err != nil || d != (Described{}) {
		t.Fatalf("no tags: %+v, %v", d, err)
	}
}
