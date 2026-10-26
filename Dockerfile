# Build a static, stdlib-only binary and ship it alone on scratch:
# no shell, no libc, no third-party code in the trust path.
# Builder tracks the newest Go major: only the two most recent majors
# get security fixes, so this must not linger on an aged-out version.
# golang:1.26.10, pinned by digest, resolved 2026-10-26 for the 2026-10-21 Go security release
FROM golang:1.26.10@sha256:672c944cfd07599b5f1bb179869270cdc94c6254e41db4d431259693cb286b82 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /powerwall_prometheus_exporter .

# The gateway serves a self-signed certificate and the client dials it
# with InsecureSkipVerify, so the image needs no ca-certificates.
FROM scratch
COPY --from=build /powerwall_prometheus_exporter /powerwall_prometheus_exporter
ENTRYPOINT ["/powerwall_prometheus_exporter"]
