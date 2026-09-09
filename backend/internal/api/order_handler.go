package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"ecommerce-backend/internal/domain"
)

type createOrderRequest struct {
	Items []struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	} `json:"items"`
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	items := make([]domain.OrderItem, len(req.Items))
	for i, it := range req.Items {
		items[i] = domain.OrderItem{ProductID: it.ProductID, Quantity: it.Quantity}
	}

	order, err := s.orders.Purchase(r.Context(), items)
	if err != nil {
		var rejected *domain.PurchaseRejectedError
		if errors.As(err, &rejected) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":    "purchase could not be completed",
				"problems": rejected.Problems,
			})
			return
		}
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, order)
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}

	order, err := s.orders.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, order)
}
