package http

import (
	"fmt"
	"log"
	"net/http"
)

// ServeMetrics does not return under normal operation.
func ServeMetrics(port int) error {
	http.Handle("/", http.RedirectHandler("/metrics", 302))
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Serving metrics on port %d at /metrics", port)
	return http.ListenAndServe(addr, nil) // blocks normally.
}
