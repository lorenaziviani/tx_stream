package domain

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID              uuid.UUID   `json:"id"`
	CustomerID      string      `json:"customer_id"`
	OrderNumber     string      `json:"order_number"`
	Status          OrderStatus `json:"status"`
	TotalAmount     float64     `json:"total_amount"`
	Currency        string      `json:"currency"`
	Items           []OrderItem `json:"items"`
	ShippingAddress Address     `json:"shipping_address"`
	BillingAddress  Address     `json:"billing_address"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type OrderItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	TotalPrice  float64 `json:"total_price"`
}

type Address struct {
	Street     string `json:"street"`
	Number     string `json:"number"`
	Complement string `json:"complement,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	ZipCode    string `json:"zip_code"`
	Country    string `json:"country"`
}

// NewOrder creates a new instance of Order
func NewOrder(customerID, orderNumber string, items []OrderItem, shippingAddress, billingAddress Address) *Order {
	totalAmount := calculateTotalAmount(items)

	return &Order{
		ID:              uuid.New(),
		CustomerID:      customerID,
		OrderNumber:     orderNumber,
		Status:          OrderStatusPending,
		TotalAmount:     totalAmount,
		Currency:        "BRL",
		Items:           items,
		ShippingAddress: shippingAddress,
		BillingAddress:  billingAddress,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// Confirm confirms the order
func (o *Order) Confirm() error {
	if o.Status != OrderStatusPending {
		return ErrInvalidOrderStatus
	}

	o.Status = OrderStatusConfirmed
	o.UpdatedAt = time.Now()
	return nil
}

// Ship marks the order as shipped
func (o *Order) Ship() error {
	if o.Status != OrderStatusConfirmed {
		return ErrInvalidOrderStatus
	}

	o.Status = OrderStatusShipped
	o.UpdatedAt = time.Now()
	return nil
}

// Deliver marks the order as delivered
func (o *Order) Deliver() error {
	if o.Status != OrderStatusShipped {
		return ErrInvalidOrderStatus
	}

	o.Status = OrderStatusDelivered
	o.UpdatedAt = time.Now()
	return nil
}

// Cancel cancels the order
func (o *Order) Cancel() error {
	if o.Status == OrderStatusDelivered {
		return ErrCannotCancelDeliveredOrder
	}

	o.Status = OrderStatusCancelled
	o.UpdatedAt = time.Now()
	return nil
}

// calculateTotalAmount calculates the total amount of the items
func calculateTotalAmount(items []OrderItem) float64 {
	total := 0.0
	for _, item := range items {
		total += item.TotalPrice
	}
	return total
}

var (
	ErrInvalidOrderStatus         = &DomainError{Message: "invalid order status"}
	ErrCannotCancelDeliveredOrder = &DomainError{Message: "cannot cancel a delivered order"}
)

// DomainError represents a domain error
type DomainError struct {
	Message string
}

func (e *DomainError) Error() string {
	return e.Message
}
