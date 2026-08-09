// Package controller manages polling of stats.
package controller

import (
	"fmt"
	"log"
	gohttp "net/http"
	"time"

	"github.com/jeffbstewart/powerwall_prometheus_exporter/http"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/model"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/powerwall"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/view"
)

type Options struct {
	Powerwall powerwall.Options
	View      view.Options
	// PollInterval is accepted for command-line compatibility but is
	// currently unused: polling is scrape-driven, so every /metrics
	// request polls the gateway fresh.
	PollInterval time.Duration
	HTTPPort     int
}

type PollEngine struct {
	mon            powerwall.Monitor
	fixed          *model.FixedInfo
	view           *view.PrometheusCounters
	metricsHandler gohttp.Handler
}

func (p *PollEngine) ServeHTTP(rw gohttp.ResponseWriter, req *gohttp.Request) {
	before := time.Now()
	if err := p.poll(); err != nil {
		log.Printf("ERROR: PollEngine.poll(): %v", err)
		rw.WriteHeader(500)
		return
	}
	elapsed := time.Since(before)
	log.Printf("Successfully polled the gateway stats in %s", elapsed)
	p.metricsHandler.ServeHTTP(rw, req)
}

// Run starts the controller loop.  Normally it does not return.
func Run(opts Options) error {
	mon, err := powerwall.New(opts.Powerwall)
	if err != nil {
		return fmt.Errorf("powerwall.New(): %v", err)
	}
	fixed, err := model.New(mon)
	if err != nil {
		return fmt.Errorf("model.New(): %v", err)
	}
	v, err := view.New(fixed, opts.View)
	if err != nil {
		return fmt.Errorf("view.New(): %v", err)
	}
	r := &PollEngine{
		mon:            mon,
		fixed:          fixed,
		view:           v,
		metricsHandler: v.Handler(),
	}

	// don't bring up the web interface until we've populated the metrics.
	if err := r.poll(); err != nil {
		return fmt.Errorf("poll(): %v", err)
	}
	gohttp.Handle("/metrics", r)
	if err := http.ServeMetrics(opts.HTTPPort); err != nil { // blocks normally.
		return fmt.Errorf("http.ServeMetrics: %v", err)
	}
	return nil
}

func (p *PollEngine) poll() error {
	stats, err := model.Poll(p.mon, p.fixed)
	if err != nil {
		return err
	}
	return p.view.Update(stats)
}
