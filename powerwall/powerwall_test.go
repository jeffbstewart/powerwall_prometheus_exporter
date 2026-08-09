package powerwall_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeffbstewart/powerwall_prometheus_exporter/powerwall"
)

// TestClientAgainstFakeGateway drives the real HTTP client (login,
// TLS with a certificate that fails verification, JSON decoding)
// against the recorded responses served by NewFakeGatewayHandler.
func TestClientAgainstFakeGateway(t *testing.T) {
	srv := httptest.NewTLSServer(powerwall.NewFakeGatewayHandler())
	defer srv.Close()
	gateway := strings.TrimPrefix(srv.URL, "https://")

	mon, err := powerwall.New(powerwall.Options{
		Gateway:  gateway,
		Username: "fake@example.com",
		Password: "hunter2",
	})
	if err != nil {
		t.Fatalf("powerwall.New(): %v", err)
	}
	defer mon.Close()

	si, err := mon.GetSiteInfo()
	if err != nil {
		t.Fatalf("GetSiteInfo(): %v", err)
	}
	// Regression: newer firmware reports the capacity fields with
	// fractional parts; they must decode as floats.
	if got, want := si.MaxSystemEnergykWh, 40.5; got != want {
		t.Errorf("si.MaxSystemEnergykWh: got %v, want %v", got, want)
	}

	status, err := mon.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus(): %v", err)
	}
	if got, want := status.Version, "23.44.3 eb113390"; got != want {
		t.Errorf("status.Version: got %q, want %q", got, want)
	}

	agg, err := mon.GetAggregates()
	if err != nil {
		t.Fatalf("GetAggregates(): %v", err)
	}
	if got, want := agg.Site.InstantPower, 743.5; got != want {
		t.Errorf("agg.Site.InstantPower: got %v, want %v", got, want)
	}
	if got, want := agg.Battery.InstantPower, -2350.0; got != want {
		t.Errorf("agg.Battery.InstantPower: got %v, want %v", got, want)
	}

	grid, err := mon.GetGridStatus()
	if err != nil {
		t.Fatalf("GetGridStatus(): %v", err)
	}
	if got, want := grid.Status, powerwall.GridConnected; got != want {
		t.Errorf("grid.Status: got %v, want %v", got, want)
	}

	networks, err := mon.GetNetworks()
	if err != nil {
		t.Fatalf("GetNetworks(): %v", err)
	}
	if got, want := len(networks), 3; got != want {
		t.Errorf("len(networks): got %d, want %d", got, want)
	}
}
