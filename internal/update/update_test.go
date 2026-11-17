package update

import (
	"context"
	"encoding/json"
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

// The page to open is made from the version. Whatever address the response gives is ignored.
func TestLatestIgnoresTheResponsesAddress(t *testing.T) {
	for tag, want := range map[string]string{
		"v99.0.0":        "https://github.com/Git-Revenge/GitRevenge/releases/tag/v99.0.0",
		"v1.2.0-rc.1":    "https://github.com/Git-Revenge/GitRevenge/releases/tag/v1.2.0-rc.1",
		"v1.0.0-a/../..": "https://github.com/Git-Revenge/GitRevenge/releases/tag/v1.0.0-a%2F..%2F..",
		"nightly":        "https://github.com/Git-Revenge/GitRevenge/releases/latest",
		"":               "https://github.com/Git-Revenge/GitRevenge/releases/latest",
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": tag, "html_url": "https://attacker.example/download"})
		}))
		rel, err := Latest(context.Background(), srv.Client(), srv.URL, "0.6.0")
		srv.Close()
		if err != nil {
			t.Fatal(err)
		}
		if rel.URL != want {
			t.Errorf("tag %q: URL = %q, want %q", tag, rel.URL, want)
		}
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
