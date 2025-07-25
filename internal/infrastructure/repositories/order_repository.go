package repositories

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
)

type OrderRepository interface {
	Create(ctx context.Context, order *models.Order) error
	GetByID(ctx context.Context, id string) (*models.Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (*models.Order, error)
	Update(ctx context.Context, order *models.Order) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int) ([]models.Order, error)
	GetByCustomerID(ctx context.Context, customerID string, limit, offset int) ([]models.Order, error)
	GetByStatus(ctx context.Context, status models.OrderStatus, limit, offset int) ([]models.Order, error)
}

type orderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

// Create creates a new order
func (r *orderRepository) Create(ctx context.Context, order *models.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

// GetByID gets an order by ID
func (r *orderRepository) GetByID(ctx context.Context, id string) (*models.Order, error) {
	var order models.Order
	err := r.db.WithContext(ctx).
		Preload("Items").
		Where("id = ?", id).
		First(&order).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("order not found with id: %s", id)
		}
		return nil, err
	}

	return &order, nil
}

// GetByOrderNumber gets an order by order number
func (r *orderRepository) GetByOrderNumber(ctx context.Context, orderNumber string) (*models.Order, error) {
	var order models.Order
	err := r.db.WithContext(ctx).
		Preload("Items").
		Where("order_number = ?", orderNumber).
		First(&order).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("order not found with order number: %s", orderNumber)
		}
		return nil, err
	}

	return &order, nil
}

// Update updates an existing order
func (r *orderRepository) Update(ctx context.Context, order *models.Order) error {
	return r.db.WithContext(ctx).Save(order).Error
}

// Delete removes an order
func (r *orderRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Order{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("order not found with id: %s", id)
	}

	return nil
}

// List lists orders with pagination
func (r *orderRepository) List(ctx context.Context, limit, offset int) ([]models.Order, error) {
	var orders []models.Order
	err := r.db.WithContext(ctx).
		Preload("Items").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error

	return orders, err
}

// GetByCustomerID gets orders by customer ID
func (r *orderRepository) GetByCustomerID(ctx context.Context, customerID string, limit, offset int) ([]models.Order, error) {
	var orders []models.Order
	err := r.db.WithContext(ctx).
		Preload("Items").
		Where("customer_id = ?", customerID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error

	return orders, err
}

// GetByStatus gets orders by status
func (r *orderRepository) GetByStatus(ctx context.Context, status models.OrderStatus, limit, offset int) ([]models.Order, error) {
	var orders []models.Order
	err := r.db.WithContext(ctx).
		Preload("Items").
		Where("status = ?", status).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error

	return orders, err
}
