package view_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jeffbstewart/powerwall_prometheus_exporter/model"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/powerwall"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/view"
)

// TestGolden proves the hand-rolled exposition renderer preserves the
// metric contract: testdata/golden.txt was captured from this program
// built against prometheus/client_golang, fed by the same recorded
// gateway responses (powerwall.NewFake).  The comparison is
// sorting-insensitive; each line must match byte for byte.
func TestGolden(t *testing.T) {
	mon := powerwall.NewFake()
	fixed, err := model.New(mon)
	if err != nil {
		t.Fatalf("model.New(): %v", err)
	}
	v, err := view.New(fixed, view.Options{Namespace: "tesla", Subsystem: "energy_gateway"})
	if err != nil {
		t.Fatalf("view.New(): %v", err)
	}
	stats, err := model.Poll(mon, fixed)
	if err != nil {
		t.Fatalf("model.Poll(): %v", err)
	}
	if err := v.Update(stats); err != nil {
		t.Fatalf("Update(): %v", err)
	}

	rec := httptest.NewRecorder()
	v.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	if got, want := rec.Code, 200; got != want {
		t.Fatalf("status code: got %d, want %d", got, want)
	}
	if got, want := rec.Header().Get("Content-Type"), "text/plain; version=0.0.4; charset=utf-8"; got != want {
		t.Errorf("Content-Type: got %q, want %q", got, want)
	}

	goldenBytes, err := os.ReadFile(filepath.Join("testdata", "golden.txt"))
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}
	got := sortedNonEmptyLines(rec.Body.String())
	want := sortedNonEmptyLines(string(goldenBytes))
	for i := 0; i < len(got) || i < len(want); i++ {
		switch {
		case i >= len(got):
			t.Errorf("missing line: %s", want[i])
		case i >= len(want):
			t.Errorf("extra line: %s", got[i])
		case got[i] != want[i]:
			t.Errorf("line mismatch:\n  got:  %s\n  want: %s", got[i], want[i])
		}
	}
}

func sortedNonEmptyLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	sort.Strings(lines)
	return lines
}
