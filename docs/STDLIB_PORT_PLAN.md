# Plan: strip to the Go standard library, deploy alongside the incumbent

Status: handoff plan (2026-08-09). Written by the homenet/monitoring
operator session; to be executed by a Claude instance spawned in this
repository. Downstream deployment steps (marked DOWNSTREAM) happen in
the `jeff/monitoring` gitea repo and are NOT this instance's job --
they are recorded here only so you understand the contract you must
preserve.

## Why

Two motivations, one hard constraint.

1. **Supply-chain surface.** A Python + pip dependency tree is the
   build-time injection surface we want gone (the class of risk behind
   recent training-sandbox escapes). A stdlib-only Go static binary
   collapses it to nothing: `FROM scratch` + one file, zero third-party
   code in the trust path.
2. **This repo is the canonical exporter but is NOT what's deployed.**
   Production currently runs an unrelated ~120-line Python
   reimplementation (`powerwall_exporter/exporter.py` in the monitoring
   repo) that exposes six flat metrics. This Go program exposes a far
   richer set. The homenet operator has chosen to make THIS program the
   deployed exporter (option A: "Go wins, align the consumers").

**Hard constraint — do not break the grid page.** The operator is
paged today when the grid drops, via a Prometheus alert on the Python
metric `powerwall_grid_up`. This program does not emit that name. The
rollout is therefore PARALLEL-RUN: your Go binary stands up on a new
port next to the still-running Python exporter; Prometheus scrapes both
for ~12h to accumulate history under the new metric names; only then
(DOWNSTREAM) are the dashboard and alerts migrated and the Python
exporter retired. Your job is the binary and its image. Preserve the
metric contract below EXACTLY so the downstream migration targets a
stable set.

## Current state of this repo

- No `go.mod` (repo predates modules; it cannot build in module mode as
  is -- a symptom, not a mystery).
- Two third-party dependencies, both to be removed:
  - `github.com/golang/glog` -- used in `main.go`, `controller/`,
    `http/`, `view/`, `powerwall/*`.
  - `github.com/prometheus/client_golang` (`prometheus` + `promhttp`)
    -- used ONLY in `view/counters.go` and `controller/controller.go`.
- Everything else is already stdlib: `powerwall/` (net/http,
  net/http/cookiejar, crypto/tls, encoding/json), `model/` (regexp,
  strconv, time).
- Polling is scrape-driven: `controller.PollEngine.ServeHTTP` polls the
  gateway fresh on each `/metrics` hit, returns 500 if the poll fails,
  else renders. A `ticker` field exists but is unused (dead code).
  Startup gates the listener behind one successful poll. PRESERVE these
  behaviors.

## The work

### 1. Add go.mod (stdlib-only)

```
module github.com/jeffbstewart/powerwall_prometheus_exporter
go 1.23
```

No `require` directives. If any appear after the edits below, a
dependency was missed -- that is the acceptance test for "stdlib-only."

### 2. Replace glog with stdlib log

Mechanical. `log` (or `log/slog`; `log` is simpler and sufficient):

| glog | stdlib |
|------|--------|
| `glog.Infof(f, …)` | `log.Printf(f, …)` |
| `glog.Errorf(f, …)` | `log.Printf("ERROR: "+f, …)` |
| `glog.Warningf(f, …)` | `log.Printf("WARN: "+f, …)` |
| `glog.Exit(a…)` | `log.Fatal(a…)` |
| `glog.Exitf(f, …)` | `log.Fatalf(f, …)` |

`log.Fatal*` calls `os.Exit(1)` after logging, matching `glog.Exit*`.

### 3. Replace client_golang with a hand-rolled text exposition

This is the substance. `view/counters.go` and the `promhttp.Handler()`
in `controller/controller.go` are the only call sites.

Write a small internal metrics layer (suggest `internal/expo` or fold
into `view`) providing gauge, labelled-gauge, and labelled-counter
types plus a renderer that emits the Prometheus text exposition format:

```
# HELP <fqname> <help text>
# TYPE <fqname> <gauge|counter>
<fqname>{label="v",…} <float>
```

Requirements to match client_golang's output byte-for-byte in spirit:

- **Name prefixing.** client_golang joins `namespace_subsystem_name`.
  With the deployed flags `--prometheus_namespace=tesla
  --prometheus_subsystem=energy_gateway`, `grid_connected` becomes
  `tesla_energy_gateway_grid_connected`. Reproduce this join (skip
  empty parts, underscore-separated).
- **One HELP and one TYPE line per metric family**, followed by one
  series line per label-set. Escape the help text (`\\`, `\n`, `"` per
  the exposition spec).
- **Counter semantics for `cumulative_power`.** client_golang's
  `CounterVec` accumulates via `Add(delta)`. The current `Update`
  tracks `priorCumulative[meter][direction]`, computes the delta from
  the gateway's running total, and `Add`s it only when non-negative
  (guarding against gateway counter resets; it logs a warning if the
  drop exceeds an epsilon). Your hand-rolled counter must keep the
  accumulated sum in memory across scrapes and render `# TYPE …
  counter`. Keep the reset-guard and the warning.
- **Gauge families** just hold and render their last `Set` value; the
  `*Vec` families hold a map keyed by ordered label values.
- **Float formatting.** Use `strconv.FormatFloat(v, 'g', -1, 64)`;
  integers-as-floats render without a decimal, which is fine for
  Prometheus.
- Concurrency: the metric store is written by the poll and read by the
  renderer. Poll-on-scrape makes these same-goroutine today, but guard
  with a mutex so a future background poll is safe.

Replace `promhttp.Handler()` with a `http.HandlerFunc` that writes
`Content-Type: text/plain; version=0.0.4; charset=utf-8` and calls the
renderer. Keep `PollEngine.ServeHTTP`'s poll-then-render-or-500 shape.

### 4. Accept connection params from env as well as flags

The binary currently requires flags (`--gateway`,
`--customer_username`, `--password`) and defaults `--port=5678`. The
deployment configures via environment variables (the Python exporter's
convention, and better than a password on the command line where `ps`
sees it). Make the three connection flags default from env, so both
work and the compose can keep passing env vars:

```go
gateway := flag.String("gateway", os.Getenv("TEG_ADDRESS"), "…")
username := flag.String("customer_username", os.Getenv("TEG_EMAIL"), "…")
password := flag.String("password", os.Getenv("TEG_PASSWORD"), "…")
```

Leave `--port`, `--prometheus_namespace`, `--prometheus_subsystem`,
`--poll_interval` as flags with their current defaults. The deploy will
pass `--port` (see DOWNSTREAM) and rely on the `tesla`/`energy_gateway`
namespace defaults.

### 5. Build a scratch image

Multi-stage: `golang:1.23` builder with
`CGO_ENABLED=0 go build -trimpath -ldflags=-s -w`, final stage
`FROM scratch` copying only the static binary. The TEG uses a
self-signed cert and the client already sets `InsecureSkipVerify`
(verify this in `powerwall/powerwall.go` and KEEP it -- do not add CA
bundles), so `scratch` needs no `ca-certificates`. `ENTRYPOINT` the
binary; connection params arrive via env/flags at run time.

### 6. Golden-output check (strongly recommended)

Before removing client_golang, build the repo ONCE with a temporary
`go.mod` that requires it, run it against the gateway (or a captured
JSON fixture -- see the `powerwall.Monitor` interface; a fake
implementation returning recorded responses makes this hermetic and CI-
able), and capture `/metrics` as `testdata/golden.txt`. After the port,
a test diffs your renderer's output against the golden file
(sorting-insensitive). This is how you prove the contract held. Keep
the fake + golden test in the repo; it is also the CI's real check.

## The metric contract (what your /metrics MUST expose)

All names carry the `tesla_energy_gateway_` prefix at runtime. Types
and labels below are the contract the downstream migration depends on.

Gauges (no labels): `powerwall_charge_percent`,
`nominal_system_energy_kWh`, `nominal_system_power_kW`,
`num_powerwalls`, `total_solar_rating_W`,
`operating_in_backup_only_mode`, `operating_in_self_consumption_mode`,
`backup_reserve_percent`, `uptime_seconds`, `major_version`,
`minor_version`, `release_version`, `flattened_version`,
`sitemaster_running`, `site_master_connected_to_tesla`,
`site_master_supplying_power`, `grid_connected`, `grid_active`.

Gauge vectors: `network_active`, `network_enabled`, `network_primary`,
`network_signal_strength` labelled `{interface}`; `instant_power`
labelled `{meter, powerType}`; `instant_average_voltage`,
`instant_total_current_amps` labelled `{meter}`.

Counter vector: `cumulative_power` labelled `{meter, direction}`.

Label domains (exact values -- note the surprising one):
- `meter`: **`site`** (this is `MeterType.Total.String()` -- NOT
  "total"), `load`, `solar`, `battery`.
- `powerType`: `truePower`, `reactivePower`, `apparentPower`.
- `direction`: `to`, `from`.
- `interface`: `powerwall.NetworkInterface.String()` (see
  `powerwall/enums.go`).

## Boundary: what happens after you (DOWNSTREAM, monitoring repo)

You produce a pushed image `localhost:15000/powerwall-exporter:<tag>`
(coordinate the tag + digest with the operator; the registry is
digest-pinned). The operator then, in `jeff/monitoring`:

1. Adds a SECOND service `powerwall-exporter-go` on host port 9873,
   `--port=9873`, same `TEG_*` env, digest-pinned. Python stays on
   9872.
2. Adds a Prometheus scrape job for `:9873`. Both run in parallel.
3. Waits ~12h so the new `tesla_energy_gateway_*` series have history.
4. Migrates the consumers, then retires the Python exporter:
   - Dashboard panels and the two alerts move to the new names.
   - `PowerwallGridDown`: `powerwall_grid_up == 0` becomes
     `tesla_energy_gateway_grid_connected == 0`. (Nuance to verify:
     this program distinguishes `grid_connected` "available to supply"
     from `grid_active` "actively supplying"; `grid_connected` is the
     faithful analog of the Python `grid_up`. Confirm against a real
     islanding event or the 12h history before trusting the page.)
   - `PowerwallBatteryLow`: `powerwall_battery_percentage < 20` becomes
     `tesla_energy_gateway_powerwall_charge_percent < 20`.

### Consumer mapping (for the downstream migration)

Current Python name -> this program's metric:

| Python (today) | This program |
|----------------|--------------|
| `powerwall_grid_up` | `tesla_energy_gateway_grid_connected` |
| `powerwall_battery_percentage` | `tesla_energy_gateway_powerwall_charge_percent` |
| `powerwall_site_power_watts` | `tesla_energy_gateway_instant_power{meter="site",powerType="truePower"}` |
| `powerwall_battery_power_watts` | `tesla_energy_gateway_instant_power{meter="battery",powerType="truePower"}` |
| `powerwall_solar_power_watts` | `tesla_energy_gateway_instant_power{meter="solar",powerType="truePower"}` |
| `powerwall_load_power_watts` | `tesla_energy_gateway_instant_power{meter="load",powerType="truePower"}` |
| `uptime_seconds` | `tesla_energy_gateway_uptime_seconds` |

Both exporters read the same gateway `/api/meters/aggregates
instant_power`, so the sign convention (+import/-export, +discharge/
-charge) is preserved through this program unchanged.

### New metrics worth new panels (the "richer" half of option A)

Beyond the six the current dashboard draws, this program also offers:
per-meter reactive & apparent power (power-quality), per-meter voltage
and current (`instant_average_voltage`, `instant_total_current_amps`),
lifetime energy counters (`cumulative_power{meter,direction}` -> daily
kWh via `increase()`), `backup_reserve_percent` (a threshold line under
charge), the operating-mode booleans, capacity stat tiles
(`num_powerwalls`, `nominal_system_*`, `total_solar_rating_W`),
software version, per-interface network health, and the three
sitemaster health booleans. The operator will add panels for the
appropriate subset downstream.

## Definition of done (this instance)

- `go.mod` present, zero `require`s; `go build ./...`, `go vet ./...`
  clean; the golden test passes.
- No import of `glog` or `client_golang` anywhere (`grep -r` clean).
- Scratch image builds and, run against the gateway (or fake),
  serves `tesla_energy_gateway_*` on the chosen port matching the
  golden contract.
- Proposed via the repo's normal flow (jbstewart-agent on an `agent/`
  branch, operator merges; CI green).
- Image tag + digest handed to the operator for the DOWNSTREAM deploy.
