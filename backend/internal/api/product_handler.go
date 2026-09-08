package api

import (
	"encoding/json"
	"net/http"

	"ecommerce-backend/internal/domain"
)

func (s *Server) handleSearchProducts(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (s *Server) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	var product domain.Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.products.Create(r.Context(), &product); err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Location", "/products/"+product.ID)
	writeJSON(w, http.StatusCreated, product)
}

func (s *Server) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}

	product, err := s.products.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, product)
}

func (s *Server) handleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}

	var product domain.Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	product.ID = id // the URL is authoritative for which resource is being updated, not the body

	if err := s.products.Update(r.Context(), &product); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, product)
}

func (s *Server) handleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}

	if err := s.products.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
