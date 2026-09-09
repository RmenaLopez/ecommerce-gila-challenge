package api

import (
	"net/http"

	"ecommerce-backend/internal/service"
)

type Server struct {
	products *service.ProductService
	imports  *service.ImportService
	orders   *service.OrderService
}

func NewServer(products *service.ProductService, imports *service.ImportService, orders *service.OrderService) *Server {
	return &Server{products: products, imports: imports, orders: orders}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)

	mux.HandleFunc("GET /products", s.handleSearchProducts)
	mux.HandleFunc("POST /products", s.handleCreateProduct)
	mux.HandleFunc("GET /products/sku/{sku}", s.handleGetProductBySKU)
	mux.HandleFunc("GET /products/{id}", s.handleGetProduct)
	mux.HandleFunc("PUT /products/{id}", s.handleUpdateProduct)
	mux.HandleFunc("DELETE /products/{id}", s.handleDeleteProduct)
	mux.HandleFunc("POST /products/import", s.handleImportProducts)

	mux.HandleFunc("POST /orders", s.handleCreateOrder)
	mux.HandleFunc("GET /orders/{id}", s.handleGetOrder)

	return mux
}
