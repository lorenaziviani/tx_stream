package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
)

// CreateOrderRequest represents the request to create an order
type CreateOrderRequest struct {
	CustomerID      string         `json:"customer_id" validate:"required"`
	OrderNumber     string         `json:"order_number" validate:"required"`
	Items           []OrderItemDTO `json:"items" validate:"required,min=1"`
	ShippingAddress AddressDTO     `json:"shipping_address" validate:"required"`
	BillingAddress  AddressDTO     `json:"billing_address" validate:"required"`
}

// OrderItemDTO represents an item of an order in the request
type OrderItemDTO struct {
	ProductID   string  `json:"product_id" validate:"required"`
	ProductName string  `json:"product_name" validate:"required"`
	Quantity    int     `json:"quantity" validate:"required,min=1"`
	UnitPrice   float64 `json:"unit_price" validate:"required,min=0"`
}

// AddressDTO represents an address in the request
type AddressDTO struct {
	Street     string `json:"street" validate:"required"`
	Number     string `json:"number" validate:"required"`
	Complement string `json:"complement,omitempty"`
	City       string `json:"city" validate:"required"`
	State      string `json:"state" validate:"required"`
	ZipCode    string `json:"zip_code" validate:"required"`
	Country    string `json:"country" validate:"required"`
}

// OrderResponse represents an order response
type OrderResponse struct {
	ID              uuid.UUID           `json:"id"`
	CustomerID      string              `json:"customer_id"`
	OrderNumber     string              `json:"order_number"`
	Status          string              `json:"status"`
	TotalAmount     float64             `json:"total_amount"`
	Currency        string              `json:"currency"`
	Items           []OrderItemResponse `json:"items"`
	ShippingAddress AddressResponse     `json:"shipping_address"`
	BillingAddress  AddressResponse     `json:"billing_address"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

// OrderItemResponse represents an item of an order in the response
type OrderItemResponse struct {
	ID          uint    `json:"id"`
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	TotalPrice  float64 `json:"total_price"`
}

// AddressResponse represents an address in the response
type AddressResponse struct {
	Street     string `json:"street"`
	Number     string `json:"number"`
	Complement string `json:"complement,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	ZipCode    string `json:"zip_code"`
	Country    string `json:"country"`
}

// ToModel converts CreateOrderRequest to models.Order
func (req *CreateOrderRequest) ToModel() *models.Order {
	var totalAmount float64
	items := make([]models.OrderItem, len(req.Items))

	for i, item := range req.Items {
		totalPrice := item.UnitPrice * float64(item.Quantity)
		totalAmount += totalPrice

		items[i] = models.OrderItem{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TotalPrice:  totalPrice,
		}
	}

	shippingAddress := models.Address{
		Street:     req.ShippingAddress.Street,
		Number:     req.ShippingAddress.Number,
		Complement: req.ShippingAddress.Complement,
		City:       req.ShippingAddress.City,
		State:      req.ShippingAddress.State,
		ZipCode:    req.ShippingAddress.ZipCode,
		Country:    req.ShippingAddress.Country,
	}

	billingAddress := models.Address{
		Street:     req.BillingAddress.Street,
		Number:     req.BillingAddress.Number,
		Complement: req.BillingAddress.Complement,
		City:       req.BillingAddress.City,
		State:      req.BillingAddress.State,
		ZipCode:    req.BillingAddress.ZipCode,
		Country:    req.BillingAddress.Country,
	}

	return &models.Order{
		CustomerID:      req.CustomerID,
		OrderNumber:     req.OrderNumber,
		Status:          models.OrderStatusPending,
		TotalAmount:     totalAmount,
		Currency:        "BRL",
		Items:           items,
		ShippingAddress: shippingAddress,
		BillingAddress:  billingAddress,
	}
}

// FromModel converts models.Order to OrderResponse
func FromModel(order *models.Order) *OrderResponse {
	items := make([]OrderItemResponse, len(order.Items))
	for i, item := range order.Items {
		items[i] = OrderItemResponse{
			ID:          item.ID,
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TotalPrice:  item.TotalPrice,
		}
	}

	shippingAddress := AddressResponse{
		Street:     order.ShippingAddress.Street,
		Number:     order.ShippingAddress.Number,
		Complement: order.ShippingAddress.Complement,
		City:       order.ShippingAddress.City,
		State:      order.ShippingAddress.State,
		ZipCode:    order.ShippingAddress.ZipCode,
		Country:    order.ShippingAddress.Country,
	}

	billingAddress := AddressResponse{
		Street:     order.BillingAddress.Street,
		Number:     order.BillingAddress.Number,
		Complement: order.BillingAddress.Complement,
		City:       order.BillingAddress.City,
		State:      order.BillingAddress.State,
		ZipCode:    order.BillingAddress.ZipCode,
		Country:    order.BillingAddress.Country,
	}

	return &OrderResponse{
		ID:              order.ID,
		CustomerID:      order.CustomerID,
		OrderNumber:     order.OrderNumber,
		Status:          string(order.Status),
		TotalAmount:     order.TotalAmount,
		Currency:        order.Currency,
		Items:           items,
		ShippingAddress: shippingAddress,
		BillingAddress:  billingAddress,
		CreatedAt:       order.CreatedAt,
		UpdatedAt:       order.UpdatedAt,
	}
}
