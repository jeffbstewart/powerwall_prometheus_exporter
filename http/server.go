package http

import (
	"fmt"
	"github.com/golang/glog"
	"net/http"
)

// ServeMetrics does not return under normal operation.
func ServeMetrics(port int) error {
	http.Handle("/", http.RedirectHandler("/metrics", 302))
	addr := fmt.Sprintf(":%d", port)
	glog.Infof("Serving metrics on port %d at /metrics", port)
	return http.ListenAndServe(addr, nil) // blocks normally.
}
