package api

import "net/http"

func (s *Server) handleSearchProducts(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (s *Server) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (s *Server) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	product, err := s.products.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, product)
}

func (s *Server) handleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (s *Server) handleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}
