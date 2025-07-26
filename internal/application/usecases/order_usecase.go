package usecases

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/lorenaziviani/txstream/internal/application/dto"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
	"github.com/lorenaziviani/txstream/internal/infrastructure/repositories"
)

// OrderUseCase defines the operations of the order use case
type OrderUseCase interface {
	CreateOrder(ctx context.Context, request *dto.CreateOrderRequest) (*dto.OrderResponse, error)
	GetOrderByID(ctx context.Context, id string) (*dto.OrderResponse, error)
	GetOrderByNumber(ctx context.Context, orderNumber string) (*dto.OrderResponse, error)
	ListOrders(ctx context.Context, limit, offset int) ([]dto.OrderResponse, error)
}

type orderUseCase struct {
	orderRepo  repositories.OrderRepository
	outboxRepo repositories.OutboxRepository
	db         *gorm.DB
}

func NewOrderUseCase(
	orderRepo repositories.OrderRepository,
	outboxRepo repositories.OutboxRepository,
	db *gorm.DB,
) OrderUseCase {
	return &orderUseCase{
		orderRepo:  orderRepo,
		outboxRepo: outboxRepo,
		db:         db,
	}
}

// CreateOrder creates a new order with ACID transaction
func (uc *orderUseCase) CreateOrder(ctx context.Context, request *dto.CreateOrderRequest) (*dto.OrderResponse, error) {
	if err := uc.validateCreateOrderRequest(request); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	order := uc.createOrderFromRequest(request)

	var orderResponse *dto.OrderResponse
	err := uc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("failed to create order: %w", err)
		}

		outboxEvent := uc.createOrderCreatedEvent(order)
		if err := tx.Create(outboxEvent).Error; err != nil {
			return fmt.Errorf("failed to create outbox event: %w", err)
		}

		orderResponse = dto.FromModel(order)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return orderResponse, nil
}

// GetOrderByID gets an order by ID
func (uc *orderUseCase) GetOrderByID(ctx context.Context, id string) (*dto.OrderResponse, error) {
	order, err := uc.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return dto.FromModel(order), nil
}

// GetOrderByNumber gets an order by order number
func (uc *orderUseCase) GetOrderByNumber(ctx context.Context, orderNumber string) (*dto.OrderResponse, error) {
	order, err := uc.orderRepo.GetByOrderNumber(ctx, orderNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return dto.FromModel(order), nil
}

// ListOrders lists orders with pagination
func (uc *orderUseCase) ListOrders(ctx context.Context, limit, offset int) ([]dto.OrderResponse, error) {
	orders, err := uc.orderRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}

	responses := make([]dto.OrderResponse, len(orders))
	for i, order := range orders {
		responses[i] = *dto.FromModel(&order)
	}

	return responses, nil
}

// validateCreateOrderRequest validates the request to create an order
func (uc *orderUseCase) validateCreateOrderRequest(request *dto.CreateOrderRequest) error {
	if request.CustomerID == "" {
		return fmt.Errorf("customer_id is required")
	}

	if request.OrderNumber == "" {
		return fmt.Errorf("order_number is required")
	}

	if len(request.Items) == 0 {
		return fmt.Errorf("at least one item is required")
	}

	for i, item := range request.Items {
		if item.ProductID == "" {
			return fmt.Errorf("item[%d]: product_id is required", i)
		}
		if item.ProductName == "" {
			return fmt.Errorf("item[%d]: product_name is required", i)
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("item[%d]: quantity must be greater than 0", i)
		}
		if item.UnitPrice < 0 {
			return fmt.Errorf("item[%d]: unit_price cannot be negative", i)
		}
	}

	if err := uc.validateAddress(request.ShippingAddress, "shipping_address"); err != nil {
		return err
	}

	if err := uc.validateAddress(request.BillingAddress, "billing_address"); err != nil {
		return err
	}

	return nil
}

// validateAddress validates an address
func (uc *orderUseCase) validateAddress(address dto.AddressDTO, fieldName string) error {
	if address.Street == "" {
		return fmt.Errorf("%s: street is required", fieldName)
	}
	if address.Number == "" {
		return fmt.Errorf("%s: number is required", fieldName)
	}
	if address.City == "" {
		return fmt.Errorf("%s: city is required", fieldName)
	}
	if address.State == "" {
		return fmt.Errorf("%s: state is required", fieldName)
	}
	if address.ZipCode == "" {
		return fmt.Errorf("%s: zip_code is required", fieldName)
	}
	if address.Country == "" {
		return fmt.Errorf("%s: country is required", fieldName)
	}
	return nil
}

// createOrderFromRequest creates an Order from the DTO using the domain constructor
func (uc *orderUseCase) createOrderFromRequest(request *dto.CreateOrderRequest) *models.Order {
	items := make([]models.OrderItem, len(request.Items))
	for i, item := range request.Items {
		totalPrice := item.UnitPrice * float64(item.Quantity)
		items[i] = models.OrderItem{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TotalPrice:  totalPrice,
		}
	}

	shippingAddress := models.Address{
		Street:     request.ShippingAddress.Street,
		Number:     request.ShippingAddress.Number,
		Complement: request.ShippingAddress.Complement,
		City:       request.ShippingAddress.City,
		State:      request.ShippingAddress.State,
		ZipCode:    request.ShippingAddress.ZipCode,
		Country:    request.ShippingAddress.Country,
	}

	billingAddress := models.Address{
		Street:     request.BillingAddress.Street,
		Number:     request.BillingAddress.Number,
		Complement: request.BillingAddress.Complement,
		City:       request.BillingAddress.City,
		State:      request.BillingAddress.State,
		ZipCode:    request.BillingAddress.ZipCode,
		Country:    request.BillingAddress.Country,
	}

	return models.NewOrder(request.CustomerID, request.OrderNumber, items, shippingAddress, billingAddress)
}

// createOrderCreatedEvent creates the OrderCreated event
func (uc *orderUseCase) createOrderCreatedEvent(order *models.Order) *models.OutboxEvent {
	eventData := map[string]interface{}{
		"order_id":     order.ID.String(),
		"customer_id":  order.CustomerID,
		"order_number": order.OrderNumber,
		"status":       string(order.Status),
		"total_amount": order.TotalAmount,
		"currency":     order.Currency,
		"items_count":  len(order.Items),
		"created_at":   order.CreatedAt,
	}

	eventMetadata := map[string]interface{}{
		"source":         "txstream-api",
		"version":        "1.0",
		"correlation_id": order.ID.String(),
	}

	return &models.OutboxEvent{
		AggregateID:   order.ID.String(),
		AggregateType: "Order",
		EventType:     "OrderCreated",
		EventData:     models.JSON(eventData),
		EventMetadata: models.JSON(eventMetadata),
		Status:        models.OutboxStatusPending,
		CreatedAt:     time.Now(),
	}
}
