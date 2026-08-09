// Package expo is a minimal in-memory metric store that renders the
// Prometheus text exposition format (version 0.0.4).  It provides the
// small subset of client_golang this program used: gauges, labelled
// gauges, and labelled counters.
package expo

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Registry holds metric families and renders them as text exposition.
// A single mutex guards all values: the poll path writes and the
// renderer reads.  Poll-on-scrape keeps those on one goroutine today,
// but the lock makes a future background poll safe.
type Registry struct {
	mu       sync.Mutex
	families map[string]*family
}

func NewRegistry() *Registry {
	return &Registry{families: make(map[string]*family)}
}

type family struct {
	name       string
	help       string
	metricType string
	labelNames []string
	series     map[string]*series
}

type series struct {
	labelValues []string
	value       float64
}

// seriesKeySep joins label values into a map key.  0xFF cannot appear
// in valid UTF-8, so joined values cannot collide.
const seriesKeySep = "\xff"

func (r *Registry) newFamily(name, help, metricType string, labelNames []string) (*family, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.families[name]; ok {
		return nil, fmt.Errorf("duplicate metric family %q", name)
	}
	f := &family{
		name:       name,
		help:       help,
		metricType: metricType,
		labelNames: labelNames,
		series:     make(map[string]*series),
	}
	r.families[name] = f
	return f, nil
}

func (f *family) getOrCreate(labelValues []string) *series {
	if len(labelValues) != len(f.labelNames) {
		panic(fmt.Sprintf("metric %q: got %d label values, want %d",
			f.name, len(labelValues), len(f.labelNames)))
	}
	key := strings.Join(labelValues, seriesKeySep)
	s, ok := f.series[key]
	if !ok {
		s = &series{labelValues: append([]string(nil), labelValues...)}
		f.series[key] = s
	}
	return s
}

// Gauge is an unlabelled metric holding its last Set value.
type Gauge struct {
	r *Registry
	s *series
}

func (r *Registry) NewGauge(name, help string) (*Gauge, error) {
	f, err := r.newFamily(name, help, "gauge", nil)
	if err != nil {
		return nil, err
	}
	return &Gauge{r: r, s: f.getOrCreate(nil)}, nil
}

func (g *Gauge) Set(v float64) {
	g.r.mu.Lock()
	defer g.r.mu.Unlock()
	g.s.value = v
}

// GaugeVec is a labelled gauge family; each label-value combination
// holds its last Set value.
type GaugeVec struct {
	r *Registry
	f *family
}

func (r *Registry) NewGaugeVec(name, help string, labelNames ...string) (*GaugeVec, error) {
	f, err := r.newFamily(name, help, "gauge", labelNames)
	if err != nil {
		return nil, err
	}
	return &GaugeVec{r: r, f: f}, nil
}

func (g *GaugeVec) Set(v float64, labelValues ...string) {
	g.r.mu.Lock()
	defer g.r.mu.Unlock()
	g.f.getOrCreate(labelValues).value = v
}

// CounterVec is a labelled counter family; each label-value
// combination accumulates deltas passed to Add.
type CounterVec struct {
	r *Registry
	f *family
}

func (r *Registry) NewCounterVec(name, help string, labelNames ...string) (*CounterVec, error) {
	f, err := r.newFamily(name, help, "counter", labelNames)
	if err != nil {
		return nil, err
	}
	return &CounterVec{r: r, f: f}, nil
}

func (c *CounterVec) Add(delta float64, labelValues ...string) {
	c.r.mu.Lock()
	defer c.r.mu.Unlock()
	c.f.getOrCreate(labelValues).value += delta
}

var (
	helpEscaper  = strings.NewReplacer(`\`, `\\`, "\n", `\n`)
	labelEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
)

// Render writes every family in the text exposition format: one HELP
// and one TYPE line, then one line per series.  Families sort by name
// and series by their rendered label pairs, with label pairs ordered
// by label name, matching client_golang's output.
func (r *Registry) Render(w io.Writer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.families))
	for name := range r.families {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f := r.families[name]
		if _, err := fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n",
			f.name, helpEscaper.Replace(f.help), f.name, f.metricType); err != nil {
			return err
		}
		labelOrder := make([]int, len(f.labelNames))
		for i := range labelOrder {
			labelOrder[i] = i
		}
		sort.Slice(labelOrder, func(a, b int) bool {
			return f.labelNames[labelOrder[a]] < f.labelNames[labelOrder[b]]
		})
		lines := make([]string, 0, len(f.series))
		for _, s := range f.series {
			var sb strings.Builder
			sb.WriteString(f.name)
			if len(f.labelNames) > 0 {
				sb.WriteByte('{')
				for i, li := range labelOrder {
					if i > 0 {
						sb.WriteByte(',')
					}
					sb.WriteString(f.labelNames[li])
					sb.WriteString(`="`)
					sb.WriteString(labelEscaper.Replace(s.labelValues[li]))
					sb.WriteByte('"')
				}
				sb.WriteByte('}')
			}
			sb.WriteByte(' ')
			sb.WriteString(strconv.FormatFloat(s.value, 'g', -1, 64))
			lines = append(lines, sb.String())
		}
		sort.Strings(lines)
		for _, line := range lines {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
	}
	return nil
}
