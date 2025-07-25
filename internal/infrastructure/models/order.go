package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Order struct {
	ID              uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CustomerID      string         `gorm:"not null;index" json:"customer_id"`
	OrderNumber     string         `gorm:"uniqueIndex;not null" json:"order_number"`
	Status          OrderStatus    `gorm:"type:varchar(50);not null;default:'pending';index" json:"status"`
	TotalAmount     float64        `gorm:"type:decimal(10,2);not null" json:"total_amount"`
	Currency        string         `gorm:"type:varchar(3);not null;default:'BRL'" json:"currency"`
	Items           []OrderItem    `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE" json:"items"`
	ShippingAddress Address        `gorm:"embedded" json:"shipping_address"`
	BillingAddress  Address        `gorm:"embedded" json:"billing_address"`
	CreatedAt       time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
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
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID     uuid.UUID `gorm:"type:uuid;not null;index" json:"order_id"`
	ProductID   string    `gorm:"not null" json:"product_id"`
	ProductName string    `gorm:"not null" json:"product_name"`
	Quantity    int       `gorm:"not null" json:"quantity"`
	UnitPrice   float64   `gorm:"type:decimal(10,2);not null" json:"unit_price"`
	TotalPrice  float64   `gorm:"type:decimal(10,2);not null" json:"total_price"`
	CreatedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

type Address struct {
	Street     string `gorm:"type:varchar(255);not null" json:"street"`
	Number     string `gorm:"type:varchar(20);not null" json:"number"`
	Complement string `gorm:"type:varchar(100)" json:"complement,omitempty"`
	City       string `gorm:"type:varchar(100);not null" json:"city"`
	State      string `gorm:"type:varchar(50);not null" json:"state"`
	ZipCode    string `gorm:"type:varchar(20);not null" json:"zip_code"`
	Country    string `gorm:"type:varchar(50);not null;default:'Brasil'" json:"country"`
}

var (
	ErrInvalidOrderStatus         = fmt.Errorf("invalid order status")
	ErrCannotCancelDeliveredOrder = fmt.Errorf("cannot cancel a delivered order")
)

func (Order) TableName() string {
	return "orders"
}

func (OrderItem) TableName() string {
	return "order_items"
}

// BeforeCreate hook to generate UUID if not provided
func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

// BeforeUpdate hook to update UpdatedAt
func (o *Order) BeforeUpdate(tx *gorm.DB) error {
	o.UpdatedAt = time.Now()
	return nil
}

// BeforeCreate hook for OrderItem
func (oi *OrderItem) BeforeCreate(tx *gorm.DB) error {
	oi.CreatedAt = time.Now()
	oi.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate hook for OrderItem
func (oi *OrderItem) BeforeUpdate(tx *gorm.DB) error {
	oi.UpdatedAt = time.Now()
	return nil
}

// NewOrder creates a new instance of Order
func NewOrder(customerID, orderNumber string, items []OrderItem, shippingAddress, billingAddress Address) *Order {
	totalAmount := calculateTotalAmount(items)

	return &Order{
		CustomerID:      customerID,
		OrderNumber:     orderNumber,
		Status:          OrderStatusPending,
		TotalAmount:     totalAmount,
		Currency:        "BRL",
		Items:           items,
		ShippingAddress: shippingAddress,
		BillingAddress:  billingAddress,
	}
}

// Confirm confirms the order
func (o *Order) Confirm() error {
	if o.Status != OrderStatusPending {
		return ErrInvalidOrderStatus
	}
	o.Status = OrderStatusConfirmed
	return nil
}

// Ship marks the order as shipped
func (o *Order) Ship() error {
	if o.Status != OrderStatusConfirmed {
		return ErrInvalidOrderStatus
	}
	o.Status = OrderStatusShipped
	return nil
}

// Deliver marks the order as delivered
func (o *Order) Deliver() error {
	if o.Status != OrderStatusShipped {
		return ErrInvalidOrderStatus
	}
	o.Status = OrderStatusDelivered
	return nil
}

// Cancel cancels the order
func (o *Order) Cancel() error {
	if o.Status == OrderStatusDelivered {
		return ErrCannotCancelDeliveredOrder
	}
	o.Status = OrderStatusCancelled
	return nil
}

// IsPending checks if the order is pending
func (o *Order) IsPending() bool {
	return o.Status == OrderStatusPending
}

// IsConfirmed checks if the order is confirmed
func (o *Order) IsConfirmed() bool {
	return o.Status == OrderStatusConfirmed
}

// IsShipped checks if the order is shipped
func (o *Order) IsShipped() bool {
	return o.Status == OrderStatusShipped
}

// IsDelivered checks if the order is delivered
func (o *Order) IsDelivered() bool {
	return o.Status == OrderStatusDelivered
}

// IsCancelled checks if the order is cancelled
func (o *Order) IsCancelled() bool {
	return o.Status == OrderStatusCancelled
}

// CanBeCancelled checks if the order can be cancelled
func (o *Order) CanBeCancelled() bool {
	return o.Status != OrderStatusDelivered
}

// calculateTotalAmount calculates the total amount of the items
func calculateTotalAmount(items []OrderItem) float64 {
	var total float64
	for _, item := range items {
		total += item.TotalPrice
	}
	return total
}
