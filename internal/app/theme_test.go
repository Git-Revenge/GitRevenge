package app

import "testing"

func TestThemeRoundTrip(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)

	if got, err := a.Theme(); err != nil || got != "" {
		t.Fatalf("default theme = %q, %v", got, err)
	}
	if err := a.SetTheme("dark"); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.Theme(); got != "dark" {
		t.Fatalf("theme = %q", got)
	}
	if err := a.SetTheme("neon"); err == nil {
		t.Fatal("unknown theme accepted")
	}
	if got, _ := a.Theme(); got != "dark" {
		t.Fatalf("theme after bad set = %q", got)
	}
}

func TestGraphDensityRoundTrip(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)

	if got, err := a.GraphDensity(); err != nil || got != "" {
		t.Fatalf("default density = %q, %v", got, err)
	}
	if err := a.SetGraphDensity("compact"); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.GraphDensity(); got != "compact" {
		t.Fatalf("density = %q", got)
	}
	if err := a.SetGraphDensity("roomy"); err == nil {
		t.Fatal("unknown density accepted")
	}
}
