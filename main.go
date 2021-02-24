package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/controller"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/powerwall"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/view"
	"time"
)

var (
	gateway          = flag.String("gateway", "", "hostname or IP address of the Tesla Energy Gateway")
	customerUsername = flag.String("customer_username", "", "username to log in with")
	password         = flag.String("password", "", "password to log in with")
	namespace        = flag.String("prometheus_namespace", "tesla", "namespace to export stats into")
	subsystem        = flag.String("prometheus_subsystem", "energy_gateway", "subsystem to export stats into")
	port             = flag.Int("port", 5678, "TCP port to expose /metrics interface on.")
	pollInterval     = flag.Duration("poll_interval", 10*time.Second, "Inter-poll frequency")
)

func main() {
	flag.Parse()
	if *customerUsername == "" {
		glog.Exit("You must provide --customer_username")
	}
	if *password == "" {
		glog.Exit("You must provide --password")
	}
	if *gateway == "" {
		glog.Exit("You must provide the address for --gateway")
	}
	opts := controller.Options{
		Powerwall: powerwall.Options{
			Gateway:  *gateway,
			Username: *customerUsername,
			Password: *password,
		},
		View: view.Options{
			Namespace: *namespace,
			Subsystem: *subsystem,
		},
		HTTPPort:     *port,
		PollInterval: *pollInterval,
	}
	if err := controller.Run(opts); err != nil {
		glog.Exitf("controller.Run(): %v", err)
	}
}
