# Powerwall Prometheus Exporter

This program extracts gauge data from the
Tesla Energy Gateway associated with your
household powerwalls for monitoring from
[Prometheus](https://prometheus.io).

It is not endorsed or authorized by Tesla.
It is not using published APIs, and thus
could break at any time.  Use at your
own risk.

Google may have an ownership interest in this
code.

It is based loosely on
[jrester's tesla_powerwall Python library](https://github.com/jrester/tesla_powerwall).

# Building and running

The program is stdlib-only Go: `go build` with
no dependencies to fetch.  A `Dockerfile` builds
a static binary on a `scratch` base image.

Connection parameters come from flags or the
environment: `--gateway` (`$TEG_ADDRESS`),
`--customer_username` (`$TEG_EMAIL`), and
`--password` (`$TEG_PASSWORD`).  Metrics are
served on `--port` (default 5678) at `/metrics`,
named `<namespace>_<subsystem>_*` per
`--prometheus_namespace` (default `tesla`) and
`--prometheus_subsystem` (default
`energy_gateway`).