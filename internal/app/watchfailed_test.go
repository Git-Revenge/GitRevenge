package app

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

func TestWatchFailedMessage(t *testing.T) {
	for _, err := range []error{syscall.ENOSPC, syscall.EMFILE} {
		if got := watchFailedMessage(err); !strings.Contains(got, "fs.inotify.max_user_watches") {
			t.Errorf("%v: no way to fix it in %q", err, got)
		}
	}
	got := watchFailedMessage(errors.New("something else"))
	if !strings.Contains(got, "won't show up on their own") || strings.Contains(got, "inotify") {
		t.Errorf("other failure: %q", got)
	}
}
