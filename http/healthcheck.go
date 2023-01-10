package http

import (
	"fmt"
	"net/http"
)

// handleHealthCheck is a handler function for checking the health of the server.
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "healthcheck")
}
