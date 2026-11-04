package app

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/version"
)

func TestCheckForUpdate(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"tag_name":"v99.0.0","html_url":"https://example.test/r"}`))
	}))
	defer srv.Close()
	a := newTestApp(t)
	a.updateURL = srv.URL

	info, err := a.CheckForUpdate(true)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Latest != "99.0.0" || info.URL != "https://example.test/r" || info.Current != version.Version {
		t.Fatalf("info = %+v", info)
	}

	// Turned off, the startup check makes no request; asking still works.
	if err := a.SetUpdateChecks(false); err != nil {
		t.Fatal(err)
	}
	if on, _ := a.UpdateChecks(); on {
		t.Fatal("update checks still on")
	}
	before := calls.Load()
	if info, err := a.CheckForUpdate(true); err != nil || info.Latest != "" {
		t.Fatalf("automatic check while off = %+v, %v", info, err)
	}
	if calls.Load() != before {
		t.Fatal("automatic check made a request while turned off")
	}
	if info, err := a.CheckForUpdate(false); err != nil || !info.Available {
		t.Fatalf("manual check = %+v, %v", info, err)
	}
}
