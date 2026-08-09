package expo

import (
	"strings"
	"testing"
)

func render(t *testing.T, r *Registry) string {
	t.Helper()
	var sb strings.Builder
	if err := r.Render(&sb); err != nil {
		t.Fatalf("Render(): %v", err)
	}
	return sb.String()
}

func TestGaugeRendersLastSet(t *testing.T) {
	r := NewRegistry()
	g, err := r.NewGauge("test_gauge", "a gauge")
	if err != nil {
		t.Fatalf("NewGauge(): %v", err)
	}
	g.Set(3)
	g.Set(1.5)
	want := "# HELP test_gauge a gauge\n# TYPE test_gauge gauge\ntest_gauge 1.5\n"
	if got := render(t, r); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUnsetGaugeRendersZero(t *testing.T) {
	r := NewRegistry()
	if _, err := r.NewGauge("test_gauge", "a gauge"); err != nil {
		t.Fatalf("NewGauge(): %v", err)
	}
	if got, want := render(t, r), "test_gauge 0\n"; !strings.HasSuffix(got, want) {
		t.Errorf("got:\n%s\nwant suffix:\n%s", got, want)
	}
}

func TestCounterVecAccumulates(t *testing.T) {
	r := NewRegistry()
	c, err := r.NewCounterVec("test_counter", "a counter", "meter", "direction")
	if err != nil {
		t.Fatalf("NewCounterVec(): %v", err)
	}
	c.Add(2.5, "site", "to")
	c.Add(1.25, "site", "to")
	c.Add(7, "site", "from")
	got := render(t, r)
	// label pairs render sorted by label name: direction before meter.
	for _, want := range []string{
		"# TYPE test_counter counter\n",
		`test_counter{direction="to",meter="site"} 3.75` + "\n",
		`test_counter{direction="from",meter="site"} 7` + "\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; got:\n%s", want, got)
		}
	}
}

func TestGaugeVecTracksSeriesIndependently(t *testing.T) {
	r := NewRegistry()
	g, err := r.NewGaugeVec("test_vec", "a vec", "interface")
	if err != nil {
		t.Fatalf("NewGaugeVec(): %v", err)
	}
	g.Set(1, "ethernet")
	g.Set(0, "wifi")
	g.Set(-13, "ethernet")
	got := render(t, r)
	for _, want := range []string{
		`test_vec{interface="ethernet"} -13` + "\n",
		`test_vec{interface="wifi"} 0` + "\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; got:\n%s", want, got)
		}
	}
}

func TestEscaping(t *testing.T) {
	r := NewRegistry()
	g, err := r.NewGaugeVec("test_escape", "help with \\ and\nnewline", "label")
	if err != nil {
		t.Fatalf("NewGaugeVec(): %v", err)
	}
	g.Set(1, "value with \\, \", and\nnewline")
	got := render(t, r)
	for _, want := range []string{
		`# HELP test_escape help with \\ and\nnewline` + "\n",
		`test_escape{label="value with \\, \", and\nnewline"} 1` + "\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; got:\n%s", want, got)
		}
	}
}

func TestDuplicateFamilyRejected(t *testing.T) {
	r := NewRegistry()
	if _, err := r.NewGauge("dup", "first"); err != nil {
		t.Fatalf("NewGauge(): %v", err)
	}
	if _, err := r.NewCounterVec("dup", "second", "l"); err == nil {
		t.Error("NewCounterVec() with duplicate name: got nil error, want error")
	}
}
