package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorenaziviani/txstream/internal/application/dto"
	"github.com/lorenaziviani/txstream/internal/application/usecases"
	"github.com/lorenaziviani/txstream/internal/infrastructure/handlers"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
	"github.com/lorenaziviani/txstream/internal/infrastructure/repositories"
	"github.com/lorenaziviani/txstream/tests"
)

// TestOrderTransactionWithOutbox test the order transaction with outbox
func TestOrderTransactionWithOutbox(t *testing.T) {
	db := tests.SetupTestDatabase(t)
	defer tests.TeardownTestDatabase(t)

	orderRepo := repositories.NewOrderRepository(db)
	outboxRepo := repositories.NewOutboxRepository(db)
	orderUseCase := usecases.NewOrderUseCase(orderRepo, outboxRepo, db)
	orderHandler := handlers.NewOrderHandler(orderUseCase)

	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	apiRouter.HandleFunc("/orders", orderHandler.CreateOrderHandler).Methods("POST")

	t.Run("successful_transaction_creates_order_and_outbox", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		request := createValidOrderRequest()
		requestBody, _ := json.Marshal(request)

		req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response dto.OrderResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotEmpty(t, response.ID)
		assert.Equal(t, request.OrderNumber, response.OrderNumber)

		outboxEvents, err := outboxRepo.GetPendingEvents(context.Background(), 10)
		require.NoError(t, err)

		var event *models.OutboxEvent
		for _, e := range outboxEvents {
			if e.AggregateID == response.ID.String() {
				event = &e
				break
			}
		}
		require.NotNil(t, event, "Event for order not found")

		assert.Equal(t, "Order", event.AggregateType)
		assert.Equal(t, "OrderCreated", event.EventType)
		assert.Equal(t, response.ID.String(), event.AggregateID)
	})

	t.Run("failed_outbox_creation_rollbacks_order", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		// This test simulates a scenario where the outbox table doesn't exist
		// We'll rename the table temporarily to force a failure
		err := db.Exec("ALTER TABLE outbox RENAME TO outbox_backup").Error
		require.NoError(t, err)

		defer func() {
			db.Exec("ALTER TABLE outbox_backup RENAME TO outbox")
		}()

		request := createValidOrderRequest()
		request.OrderNumber = fmt.Sprintf("ORD-TEST-%d", time.Now().UnixNano())
		requestBody, _ := json.Marshal(request)

		req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		orders, err := orderRepo.List(context.Background(), 10, 0)
		require.NoError(t, err)

		orderFound := false
		for _, order := range orders {
			if order.OrderNumber == request.OrderNumber {
				orderFound = true
				break
			}
		}
		assert.False(t, orderFound, "Order should not exist due to rollback")
	})

	t.Run("duplicate_order_number_returns_conflict", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		request1 := createValidOrderRequest()
		request1.OrderNumber = fmt.Sprintf("ORD-DUP-%d", time.Now().UnixNano())

		ctx := context.Background()
		response1, err := orderUseCase.CreateOrder(ctx, request1)
		require.NoError(t, err)
		assert.NotNil(t, response1)

		request2 := createValidOrderRequest()
		request2.OrderNumber = request1.OrderNumber
		requestBody2, _ := json.Marshal(request2)

		req2 := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBuffer(requestBody2))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusConflict, w2.Code)

		orders, err := orderRepo.List(context.Background(), 10, 0)
		require.NoError(t, err)

		orderCount := 0
		for _, order := range orders {
			if order.OrderNumber == request1.OrderNumber {
				orderCount++
			}
		}
		assert.Equal(t, 1, orderCount, "Should have exactly one order with this number")
	})

	t.Run("invalid_request_returns_bad_request", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		request := createValidOrderRequest()
		request.CustomerID = "" // Required field empty
		requestBody, _ := json.Marshal(request)

		req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		orders, err := orderRepo.List(context.Background(), 10, 0)
		require.NoError(t, err)

		orderFound := false
		for _, order := range orders {
			if order.OrderNumber == request.OrderNumber {
				orderFound = true
				break
			}
		}
		assert.False(t, orderFound, "Order should not exist due to validation failure")
	})
}

// TestOutboxEventConsistency test the consistency of outbox events
func TestOutboxEventConsistency(t *testing.T) {
	db := tests.SetupTestDatabase(t)
	defer tests.TeardownTestDatabase(t)

	orderRepo := repositories.NewOrderRepository(db)
	outboxRepo := repositories.NewOutboxRepository(db)
	orderUseCase := usecases.NewOrderUseCase(orderRepo, outboxRepo, db)

	t.Run("outbox_event_contains_correct_data", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		request := createValidOrderRequest()
		ctx := context.Background()

		response, err := orderUseCase.CreateOrder(ctx, request)
		require.NoError(t, err)

		outboxEvents, err := outboxRepo.GetPendingEvents(ctx, 10)
		require.NoError(t, err)

		var event *models.OutboxEvent
		for _, e := range outboxEvents {
			if e.AggregateID == response.ID.String() {
				event = &e
				break
			}
		}
		require.NotNil(t, event, "Event for order not found")

		assert.Equal(t, "Order", event.AggregateType)
		assert.Equal(t, "OrderCreated", event.EventType)
		assert.Equal(t, response.ID.String(), event.AggregateID)
		assert.Equal(t, models.OutboxStatusPending, event.Status)
		assert.Equal(t, 0, event.RetryCount)

		eventData := event.GetEventData()
		assert.Equal(t, response.ID.String(), eventData["order_id"])
		assert.Equal(t, request.CustomerID, eventData["customer_id"])
		assert.Equal(t, request.OrderNumber, eventData["order_number"])
		assert.Equal(t, "pending", eventData["status"])

		totalAmount, ok := eventData["total_amount"].(float64)
		assert.True(t, ok)
		assert.Equal(t, response.TotalAmount, totalAmount)

		assert.Equal(t, "BRL", eventData["currency"])

		itemsCount, ok := eventData["items_count"].(float64)
		assert.True(t, ok)
		assert.Equal(t, float64(len(request.Items)), itemsCount)

		eventMetadata := event.GetEventMetadata()
		assert.Equal(t, "txstream-api", eventMetadata["source"])
		assert.Equal(t, "1.0", eventMetadata["version"])

		// Verify correlation_id is a string and matches response ID
		correlationID, ok := eventMetadata["correlation_id"].(string)
		assert.True(t, ok)
		assert.Equal(t, response.ID.String(), correlationID)
	})

	t.Run("outbox_event_has_correct_timestamps", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		request := createValidOrderRequest()
		ctx := context.Background()

		response, err := orderUseCase.CreateOrder(ctx, request)
		require.NoError(t, err)

		outboxEvents, err := outboxRepo.GetPendingEvents(ctx, 10)
		require.NoError(t, err)

		var event *models.OutboxEvent
		for _, e := range outboxEvents {
			if e.AggregateID == response.ID.String() {
				event = &e
				break
			}
		}
		require.NotNil(t, event, "Event for order not found")

		assert.NotZero(t, event.CreatedAt)
		assert.Nil(t, event.PublishedAt)
		assert.WithinDuration(t, time.Now(), event.CreatedAt, 5*time.Second)
	})
}

// Helper functions
func createValidOrderRequest() *dto.CreateOrderRequest {
	return &dto.CreateOrderRequest{
		CustomerID:  "customer-123",
		OrderNumber: fmt.Sprintf("ORD-%d", time.Now().UnixNano()),
		Items: []dto.OrderItemDTO{
			{
				ProductID:   "prod-1",
				ProductName: "Produto Teste 1",
				Quantity:    2,
				UnitPrice:   29.99,
			},
			{
				ProductID:   "prod-2",
				ProductName: "Produto Teste 2",
				Quantity:    1,
				UnitPrice:   49.99,
			},
		},
		ShippingAddress: dto.AddressDTO{
			Street:     "Rua Teste",
			Number:     "123",
			Complement: "Apto 45",
			City:       "São Paulo",
			State:      "SP",
			ZipCode:    "01234-567",
			Country:    "Brasil",
		},
		BillingAddress: dto.AddressDTO{
			Street:     "Rua Teste",
			Number:     "123",
			Complement: "Apto 45",
			City:       "São Paulo",
			State:      "SP",
			ZipCode:    "01234-567",
			Country:    "Brasil",
		},
	}
}
