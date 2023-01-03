package http

import (
	"fmt"
	"net/http"
	"time"
)

// handleHealthCheck is a handler function for checking the health of the server.
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	time.Sleep(5 * time.Second)
	fmt.Fprint(w, "healthcheck")
}
