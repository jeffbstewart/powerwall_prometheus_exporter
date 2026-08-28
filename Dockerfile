# Build a static, stdlib-only binary and ship it alone on scratch:
# no shell, no libc, no third-party code in the trust path.
# Builder tracks the newest Go major: only the two most recent majors
# get security fixes, so this must not linger on an aged-out version.
# golang:1.26 as of 2026-08-27
FROM golang:1.26@sha256:dc2521c2a906db43073b8b4d99f491b6341cf15610b6ebbab187c45153f9959e AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /powerwall_prometheus_exporter .

# The gateway serves a self-signed certificate and the client dials it
# with InsecureSkipVerify, so the image needs no ca-certificates.
FROM scratch
COPY --from=build /powerwall_prometheus_exporter /powerwall_prometheus_exporter
ENTRYPOINT ["/powerwall_prometheus_exporter"]
