package powerwall_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// sessionGateway wraps the fake gateway with session tracking: a login
// establishes a session, dropSession simulates the gateway rebooting
// (e.g. for a firmware update), and requests without a session get 403
// the way the real gateway answers them.
type sessionGateway struct {
	inner http.Handler

	mu           sync.Mutex
	logins       int
	sessionValid bool
	failLogins   bool
}

func (g *sessionGateway) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	g.mu.Lock()
	if req.URL.Path == "/api/login/Basic" {
		g.logins++
		if g.failLogins {
			g.mu.Unlock()
			rw.WriteHeader(http.StatusForbidden)
			return
		}
		g.sessionValid = true
		g.mu.Unlock()
		g.inner.ServeHTTP(rw, req)
		return
	}
	if !g.sessionValid {
		g.mu.Unlock()
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	g.mu.Unlock()
	g.inner.ServeHTTP(rw, req)
}

func (g *sessionGateway) dropSession(failLogins bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sessionValid = false
	g.failLogins = failLogins
}

func (g *sessionGateway) loginCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.logins
}

// TestReloginAfterSessionDrop covers the gateway invalidating every
// session when it reboots: the client must log in again and retry
// rather than fail every poll until the process restarts.
func TestReloginAfterSessionDrop(t *testing.T) {
	gw := &sessionGateway{inner: powerwall.NewFakeGatewayHandler()}
	srv := httptest.NewTLSServer(gw)
	defer srv.Close()

	mon, err := powerwall.New(powerwall.Options{
		Gateway:  strings.TrimPrefix(srv.URL, "https://"),
		Username: "fake@example.com",
		Password: "hunter2",
	})
	if err != nil {
		t.Fatalf("powerwall.New(): %v", err)
	}
	defer mon.Close()

	if _, err := mon.GetStatus(); err != nil {
		t.Fatalf("GetStatus() before session drop: %v", err)
	}

	gw.dropSession(false)
	if _, err := mon.GetStatus(); err != nil {
		t.Fatalf("GetStatus() after session drop: %v", err)
	}
	if got, want := gw.loginCount(), 2; got != want {
		t.Errorf("login count: got %d, want %d", got, want)
	}
}

// TestReloginBackoff covers a session drop where re-login itself fails
// (e.g. the password changed): the client must not hammer the login
// endpoint on every poll, because the gateway locks the account after
// repeated failed logins.
func TestReloginBackoff(t *testing.T) {
	gw := &sessionGateway{inner: powerwall.NewFakeGatewayHandler()}
	srv := httptest.NewTLSServer(gw)
	defer srv.Close()

	mon, err := powerwall.New(powerwall.Options{
		Gateway:  strings.TrimPrefix(srv.URL, "https://"),
		Username: "fake@example.com",
		Password: "hunter2",
	})
	if err != nil {
		t.Fatalf("powerwall.New(): %v", err)
	}
	defer mon.Close()

	gw.dropSession(true)
	if _, err := mon.GetStatus(); err == nil {
		t.Fatal("GetStatus() with failing logins: got nil error, want error")
	}
	if got, want := gw.loginCount(), 2; got != want {
		t.Fatalf("login count after failed re-login: got %d, want %d", got, want)
	}

	// The next poll must NOT try another login while the backoff holds.
	if _, err := mon.GetStatus(); err == nil {
		t.Fatal("GetStatus() during backoff: got nil error, want error")
	}
	if got, want := gw.loginCount(), 2; got != want {
		t.Errorf("login count during backoff: got %d, want %d", got, want)
	}
}
