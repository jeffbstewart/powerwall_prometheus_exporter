package main

import (
	"flag"
	"log"
	"os"
	"time"
	// The gateway reports its timezone by name; embed the tzdata so it
	// still decodes on machines and images without a zoneinfo database.
	_ "time/tzdata"

	"github.com/jeffbstewart/powerwall_prometheus_exporter/controller"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/powerwall"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/view"
)

// The connection flags default from the environment (the deployment's
// convention, and it keeps the password out of the process list).
var (
	gateway          = flag.String("gateway", os.Getenv("TEG_ADDRESS"), "hostname or IP address of the Tesla Energy Gateway (defaults to $TEG_ADDRESS)")
	customerUsername = flag.String("customer_username", os.Getenv("TEG_EMAIL"), "username to log in with (defaults to $TEG_EMAIL)")
	password         = flag.String("password", os.Getenv("TEG_PASSWORD"), "password to log in with (defaults to $TEG_PASSWORD)")
	namespace        = flag.String("prometheus_namespace", "tesla", "namespace to export stats into")
	subsystem        = flag.String("prometheus_subsystem", "energy_gateway", "subsystem to export stats into")
	port             = flag.Int("port", 5678, "TCP port to expose /metrics interface on.")
	pollInterval     = flag.Duration("poll_interval", 10*time.Second, "Inter-poll frequency")
)

func main() {
	flag.Parse()
	if *customerUsername == "" {
		log.Fatal("You must provide --customer_username or set TEG_EMAIL")
	}
	if *password == "" {
		log.Fatal("You must provide --password or set TEG_PASSWORD")
	}
	if *gateway == "" {
		log.Fatal("You must provide the address for --gateway or set TEG_ADDRESS")
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
		log.Fatalf("controller.Run(): %v", err)
	}
}
