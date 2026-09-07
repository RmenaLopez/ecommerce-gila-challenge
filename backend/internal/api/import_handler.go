package api

import "net/http"

func (s *Server) handleImportProducts(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}
