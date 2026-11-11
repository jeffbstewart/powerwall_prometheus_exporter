# Build a static, stdlib-only binary and ship it alone on scratch:
# no shell, no libc, no third-party code in the trust path.
# Builder tracks the newest Go major: only the two most recent majors
# get security fixes, so this must not linger on an aged-out version.
# golang:1.26.10, pinned by digest, resolved 2026-10-26 for the 2026-11 Go security release, GO-2026-6859
FROM golang:1.26.11@sha256:6327cb290853a5d13091c436b88259b4f35ff70e49b1f8d9e5d1dd1757e9c6ba AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /powerwall_prometheus_exporter .

# The gateway serves a self-signed certificate and the client dials it
# with InsecureSkipVerify, so the image needs no ca-certificates.
FROM scratch
COPY --from=build /powerwall_prometheus_exporter /powerwall_prometheus_exporter
ENTRYPOINT ["/powerwall_prometheus_exporter"]
