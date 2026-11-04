package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.7.0", "0.6.0", true},
		{"v0.6.1", "0.6.0", true},
		{"1.0.0", "0.99.99", true},
		{"0.6.0", "0.6.0", false},
		{"0.5.9", "0.6.0", false},
		{"0.6.0", "0.6.0-dev", true},
		{"0.6.0-rc.2", "0.6.0-rc.1", true},
		{"0.6.0-rc.10", "0.6.0-rc.9", true},
		{"0.6.0-rc.1", "0.6.0", false},
		{"0.6.0-rc.1", "0.6.0-dev", true},
		{"0.6.0+build.5", "0.6.0", false},
		{"garbage", "0.6.0", false},
		{"0.7.0", "garbage", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestLatest(t *testing.T) {
	var gotUA, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA, gotAccept = r.Header.Get("User-Agent"), r.Header.Get("Accept")
		_, _ = w.Write([]byte(`{"tag_name":"v0.7.0","html_url":"https://github.com/Git-Revenge/GitRevenge/releases/tag/v0.7.0","draft":false}`))
	}))
	defer srv.Close()

	rel, err := Latest(context.Background(), srv.Client(), srv.URL, "0.6.0")
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "0.7.0" || rel.URL != "https://github.com/Git-Revenge/GitRevenge/releases/tag/v0.7.0" {
		t.Errorf("release = %+v", rel)
	}
	if gotUA != "GitRevenge/0.6.0" || gotAccept != "application/vnd.github+json" {
		t.Errorf("headers: User-Agent %q, Accept %q", gotUA, gotAccept)
	}
}

func TestLatestReportsHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := Latest(context.Background(), srv.Client(), srv.URL, "0.6.0"); err == nil {
		t.Fatal("want an error for a 403")
	}
}
