package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/lorenaziviani/txstream/internal/application/dto"
	"github.com/lorenaziviani/txstream/internal/application/usecases"
)

type OrderHandler struct {
	orderUseCase usecases.OrderUseCase
}

func NewOrderHandler(orderUseCase usecases.OrderUseCase) *OrderHandler {
	return &OrderHandler{
		orderUseCase: orderUseCase,
	}
}

// CreateOrderHandler processes the POST /orders request
func (h *OrderHandler) CreateOrderHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var request dto.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error": "Invalid JSON payload"}`, http.StatusBadRequest)
		return
	}

	response, err := h.orderUseCase.CreateOrder(r.Context(), &request)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorMessage := err.Error()

		switch {
		case err.Error() == "validation error: customer_id is required" ||
			err.Error() == "validation error: order_number is required" ||
			err.Error() == "validation error: at least one item is required":
			statusCode = http.StatusBadRequest
		case err.Error() == "order with number ORD-001 already exists":
			statusCode = http.StatusConflict
		}

		errorResponse := map[string]string{
			"error": errorMessage,
		}

		errorJSON, _ := json.Marshal(errorResponse)
		http.Error(w, string(errorJSON), statusCode)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetOrderByIDHandler processes the GET /orders/{id} request
func (h *OrderHandler) GetOrderByIDHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	id := vars["id"]

	response, err := h.orderUseCase.GetOrderByID(r.Context(), id)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "failed to get order: order not found with id: "+id {
			statusCode = http.StatusNotFound
		}

		errorResponse := map[string]string{
			"error": err.Error(),
		}

		errorJSON, _ := json.Marshal(errorResponse)
		http.Error(w, string(errorJSON), statusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetOrderByNumberHandler processes the GET /orders/number/{orderNumber} request
func (h *OrderHandler) GetOrderByNumberHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	orderNumber := vars["orderNumber"]

	response, err := h.orderUseCase.GetOrderByNumber(r.Context(), orderNumber)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "failed to get order: order not found with order number: "+orderNumber {
			statusCode = http.StatusNotFound
		}

		errorResponse := map[string]string{
			"error": err.Error(),
		}

		errorJSON, _ := json.Marshal(errorResponse)
		http.Error(w, string(errorJSON), statusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ListOrdersHandler processes the GET /orders request
func (h *OrderHandler) ListOrdersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 10
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	if limit > 100 {
		limit = 100
	}

	response, err := h.orderUseCase.ListOrders(r.Context(), limit, offset)
	if err != nil {
		errorResponse := map[string]string{
			"error": err.Error(),
		}

		errorJSON, _ := json.Marshal(errorResponse)
		http.Error(w, string(errorJSON), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
