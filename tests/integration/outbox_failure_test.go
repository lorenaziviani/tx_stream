package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorenaziviani/txstream/internal/application/dto"
	"github.com/lorenaziviani/txstream/internal/application/usecases"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
	"github.com/lorenaziviani/txstream/internal/infrastructure/repositories"
	"github.com/lorenaziviani/txstream/tests"
	"gorm.io/gorm"
)

// TestOutboxFailureScenarios tests specific outbox failure scenarios
func TestOutboxFailureScenarios(t *testing.T) {
	db := tests.SetupTestDatabase(t)
	defer tests.TeardownTestDatabase(t)

	orderRepo := repositories.NewOrderRepository(db)
	outboxRepo := repositories.NewOutboxRepository(db)
	orderUseCase := usecases.NewOrderUseCase(orderRepo, outboxRepo, db)

	t.Run("database_constraint_violation_rollbacks_transaction", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		request1 := createValidOrderRequest()
		request1.OrderNumber = "ORD-CONSTRAINT-TEST"
		ctx := context.Background()

		response1, err := orderUseCase.CreateOrder(ctx, request1)
		require.NoError(t, err)
		assert.NotNil(t, response1)

		request2 := createValidOrderRequest()
		request2.OrderNumber = "ORD-CONSTRAINT-TEST"
		request2.CustomerID = "different-customer"

		response2, err := orderUseCase.CreateOrder(ctx, request2)

		assert.Error(t, err, "Should fail due to unique constraint violation")
		assert.Nil(t, response2, "Response should be nil on error")

		orders, err := orderRepo.List(context.Background(), 10, 0)
		require.NoError(t, err)

		orderCount := 0
		for _, order := range orders {
			if order.OrderNumber == "ORD-CONSTRAINT-TEST" {
				orderCount++
			}
		}
		assert.Equal(t, 1, orderCount, "Should have exactly one order with this number due to rollback")
	})

	t.Run("outbox_table_not_exists_rollbacks_transaction", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		err := db.Exec("ALTER TABLE outbox RENAME TO outbox_backup").Error
		require.NoError(t, err)

		defer func() {
			db.Exec("ALTER TABLE outbox_backup RENAME TO outbox")
		}()

		request := createValidOrderRequest()
		request.OrderNumber = fmt.Sprintf("ORD-TABLE-%d", time.Now().UnixNano())
		ctx := context.Background()

		response, err := orderUseCase.CreateOrder(ctx, request)

		assert.Error(t, err, "Should fail due to missing outbox table")
		assert.Nil(t, response, "Response should be nil on error")

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

	t.Run("outbox_json_invalid_rollbacks_transaction", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		invalidOutboxEvent := &models.OutboxEvent{
			AggregateID:   "test-id",
			AggregateType: "Order",
			EventType:     "OrderCreated",
			EventData:     models.JSON{"invalid": make(chan int)},
			Status:        models.OutboxStatusPending,
		}

		err := db.Create(invalidOutboxEvent).Error
		assert.Error(t, err, "Should fail due to invalid JSON data")

		request := createValidOrderRequest()
		request.OrderNumber = fmt.Sprintf("ORD-JSON-%d", time.Now().UnixNano())
		ctx := context.Background()

		response, err := orderUseCase.CreateOrder(ctx, request)

		assert.NoError(t, err, "Should succeed with valid order")
		assert.NotNil(t, response, "Response should not be nil")

		orders, err := orderRepo.List(context.Background(), 10, 0)
		require.NoError(t, err)

		orderFound := false
		for _, order := range orders {
			if order.OrderNumber == request.OrderNumber {
				orderFound = true
				break
			}
		}
		assert.True(t, orderFound, "Order should exist")
	})

	t.Run("empty_aggregate_id_is_not_allowed", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		invalidOutboxEvent := &models.OutboxEvent{
			AggregateID:   "", // Empty aggregate_id
			AggregateType: "Order",
			EventType:     "OrderCreated",
			EventData:     models.JSON{"test": "data"},
			Status:        models.OutboxStatusPending,
		}

		err := db.Create(invalidOutboxEvent).Error
		assert.Error(t, err, "Should fail due to empty aggregate_id validation")

		outboxEvents, err := outboxRepo.GetPendingEvents(context.Background(), 10)
		require.NoError(t, err)
		assert.Len(t, outboxEvents, 0, "No outbox events should exist")
	})

	t.Run("empty_aggregate_id_in_transaction_rolls_back", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		request := createValidOrderRequest()
		ctx := context.Background()

		order := &models.Order{
			CustomerID:  request.CustomerID,
			OrderNumber: request.OrderNumber,
			Status:      models.OrderStatusPending,
			TotalAmount: 100.0,
			Currency:    "BRL",
		}

		err := db.Create(order).Error
		require.NoError(t, err)

		err = db.Transaction(func(tx *gorm.DB) error {
			invalidOutboxEvent := &models.OutboxEvent{
				AggregateID:   "", // Empty aggregate_id
				AggregateType: "Order",
				EventType:     "OrderCreated",
				EventData:     models.JSON{"test": "data"},
				Status:        models.OutboxStatusPending,
			}

			return tx.Create(invalidOutboxEvent).Error
		})

		assert.Error(t, err, "Transaction should fail due to validation")

		outboxEvents, err := outboxRepo.GetPendingEvents(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, outboxEvents, 0, "No outbox events should exist after failed transaction")
	})
}

// TestTransactionIsolation tests the transaction isolation
func TestTransactionIsolation(t *testing.T) {
	db := tests.SetupTestDatabase(t)
	defer tests.TeardownTestDatabase(t)

	orderRepo := repositories.NewOrderRepository(db)
	outboxRepo := repositories.NewOutboxRepository(db)
	orderUseCase := usecases.NewOrderUseCase(orderRepo, outboxRepo, db)

	t.Run("concurrent_transactions_are_isolated", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		request1 := createValidOrderRequest()
		request2 := createValidOrderRequest()
		ctx := context.Background()

		// Execute two concurrent transactions
		done := make(chan bool, 2)
		var response1, response2 *dto.OrderResponse
		var err1, err2 error

		go func() {
			response1, err1 = orderUseCase.CreateOrder(ctx, request1)
			done <- true
		}()

		go func() {
			response2, err2 = orderUseCase.CreateOrder(ctx, request2)
			done <- true
		}()

		// Wait for both transactions
		<-done
		<-done

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NotNil(t, response1)
		assert.NotNil(t, response2)
		assert.NotEqual(t, response1.ID, response2.ID)

		// Verify that both orders were created
		orders, err := orderRepo.List(ctx, 10, 0)
		require.NoError(t, err)
		assert.Len(t, orders, 2)

		// Verify that both outbox events were created
		outboxEvents, err := outboxRepo.GetPendingEvents(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, outboxEvents, 2)
	})

	t.Run("failed_transaction_does_not_affect_successful_one", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		validRequest := createValidOrderRequest()
		invalidRequest := createValidOrderRequest()
		invalidRequest.CustomerID = "" // Invalid request
		ctx := context.Background()

		validResponse, validErr := orderUseCase.CreateOrder(ctx, validRequest)

		invalidResponse, invalidErr := orderUseCase.CreateOrder(ctx, invalidRequest)

		assert.NoError(t, validErr)
		assert.NotNil(t, validResponse)
		assert.Error(t, invalidErr)
		assert.Nil(t, invalidResponse)

		// Verify that only the valid order was created
		orders, err := orderRepo.List(ctx, 10, 0)
		require.NoError(t, err)
		assert.Len(t, orders, 1)
		assert.Equal(t, validRequest.OrderNumber, orders[0].OrderNumber)

		// Verify that only the valid event was created
		outboxEvents, err := outboxRepo.GetPendingEvents(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, outboxEvents, 1)
		assert.Equal(t, validResponse.ID.String(), outboxEvents[0].AggregateID)
	})
}

// TestOutboxEventDataIntegrity tests the integrity of outbox event data
func TestOutboxEventDataIntegrity(t *testing.T) {
	db := tests.SetupTestDatabase(t)
	defer tests.TeardownTestDatabase(t)

	orderRepo := repositories.NewOrderRepository(db)
	outboxRepo := repositories.NewOutboxRepository(db)
	orderUseCase := usecases.NewOrderUseCase(orderRepo, outboxRepo, db)

	t.Run("outbox_event_data_matches_order_data", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		request := createValidOrderRequest()
		ctx := context.Background()

		response, err := orderUseCase.CreateOrder(ctx, request)
		require.NoError(t, err)

		order, err := orderRepo.GetByID(ctx, response.ID.String())
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

		eventData := event.GetEventData()

		assert.Equal(t, order.ID.String(), eventData["order_id"])
		assert.Equal(t, order.CustomerID, eventData["customer_id"])
		assert.Equal(t, order.OrderNumber, eventData["order_number"])
		assert.Equal(t, string(order.Status), eventData["status"])

		totalAmount, ok := eventData["total_amount"].(float64)
		assert.True(t, ok)
		assert.Equal(t, order.TotalAmount, totalAmount)

		assert.Equal(t, order.Currency, eventData["currency"])

		itemsCount, ok := eventData["items_count"].(float64)
		assert.True(t, ok)
		assert.Equal(t, float64(len(order.Items)), itemsCount)

		// Verify timestamps
		createdAt, ok := eventData["created_at"].(string)
		assert.True(t, ok)
		parsedTime, err := time.Parse(time.RFC3339, createdAt)
		assert.NoError(t, err)
		assert.WithinDuration(t, order.CreatedAt, parsedTime, time.Second)
	})

	t.Run("outbox_event_metadata_is_consistent", func(t *testing.T) {
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

		eventMetadata := event.GetEventMetadata()

		// Verify default metadata
		assert.Equal(t, "txstream-api", eventMetadata["source"])
		assert.Equal(t, "1.0", eventMetadata["version"])

		// Verify correlation_id is a string and matches response ID
		correlationID, ok := eventMetadata["correlation_id"].(string)
		assert.True(t, ok)
		assert.NotEmpty(t, correlationID)
		assert.Equal(t, response.ID.String(), correlationID)
	})
}
