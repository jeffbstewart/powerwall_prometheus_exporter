# Build a static, stdlib-only binary and ship it alone on scratch:
# no shell, no libc, no third-party code in the trust path.
FROM golang:1.23 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /powerwall_prometheus_exporter .

# The gateway serves a self-signed certificate and the client dials it
# with InsecureSkipVerify, so the image needs no ca-certificates.
FROM scratch
COPY --from=build /powerwall_prometheus_exporter /powerwall_prometheus_exporter
ENTRYPOINT ["/powerwall_prometheus_exporter"]
