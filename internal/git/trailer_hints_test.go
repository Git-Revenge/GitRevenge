package git

import (
	"context"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestTrailerHints(t *testing.T) {
	ctx := context.Background()
	fx := gittest.New(t)
	commit := func(msg string) {
		fx.Git("commit", "-q", "--allow-empty", "-m", msg)
	}
	commit("one\n\nCo-authored-by: Ana Cruz <ana@example.com>\nCloses: #1")
	commit("two\n\nCo-authored-by: Ana Cruz <ana@example.com>\nco-authored-by: Bo Lee <bo@example.com>")
	commit("three\n\nCloses: #2\nCloses: #3\nReviewed-by: Bo Lee <bo@example.com>")
	commit("no trailers at all")

	got := OpenRepo(newTestRunner(t), fx.Dir).TrailerHints(ctx)
	if !slices.Contains(got.Keys, "Closes") || !slices.Contains(got.Keys, "Reviewed-by") {
		t.Errorf("keys: %v", got.Keys)
	}
	if n := len(slices.DeleteFunc(slices.Clone(got.Keys), func(k string) bool { return k != "Co-authored-by" && k != "co-authored-by" })); n != 1 {
		t.Errorf("one key was counted as two spellings: %v", got.Keys)
	}
	if got.Keys[0] != "Closes" {
		t.Errorf("the most used key should be first: %v", got.Keys)
	}
	if want := []string{"Bo Lee <bo@example.com>", "Ana Cruz <ana@example.com>"}; !slices.Equal(got.People, want) {
		t.Errorf("people: got %v, want %v", got.People, want)
	}
}

func TestTrailerHintsIgnoresKeysWithoutPeople(t *testing.T) {
	fx := gittest.New(t)
	fx.Git("commit", "-q", "--allow-empty", "-m", "one\n\nCloses: Ana <ana@example.com>")
	got := OpenRepo(newTestRunner(t), fx.Dir).TrailerHints(context.Background())
	if len(got.People) != 0 {
		t.Errorf("people: %v", got.People)
	}
}
