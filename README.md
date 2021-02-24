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

# Known Issues

The timezone reported from GetSiteInfo()
might be nil if the go development environment
is not installed on the local machine.