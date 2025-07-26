package database

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/lorenaziviani/txstream/internal/infrastructure/config"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
)

var (
	db  *gorm.DB
	cfg *config.Config
)

// InitializeDatabase initializes the database connection using configuration
func InitializeDatabase() error {
	var err error

	cfg, err = config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	db, err = Connect(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := AutoMigrate(db); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// Connect establishes a database connection using the provided configuration
func Connect(dbConfig config.DatabaseConfig) (*gorm.DB, error) {
	dsn := dbConfig.GetDSN()

	gormLogger := logger.Default
	switch dbConfig.LogLevel {
	case "debug":
		gormLogger = logger.Default.LogMode(logger.Info)
	case "error":
		gormLogger = logger.Default.LogMode(logger.Error)
	case "warn":
		gormLogger = logger.Default.LogMode(logger.Warn)
	}

	gormConfig := &gorm.Config{
		Logger: gormLogger,
	}

	database, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConns)
	sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established successfully")
	return database, nil
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return db
}

// GetConfig returns the current configuration
func GetConfig() *config.Config {
	return cfg
}

// CloseDatabase closes the database connection
func CloseDatabase() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("failed to get underlying sql.DB: %w", err)
		}

		if err := sqlDB.Close(); err != nil {
			return fmt.Errorf("failed to close database connection: %w", err)
		}

		log.Println("Database connection closed")
	}
	return nil
}

// AutoMigrate runs database migrations
func AutoMigrate(database *gorm.DB) error {
	log.Println("Running database migrations...")

	models := []interface{}{
		&models.Order{},
		&models.OrderItem{},
		&models.OutboxEvent{},
		&models.Event{},
	}

	for _, model := range models {
		if err := database.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate model %T: %w", model, err)
		}
		log.Printf("Migrated model: %T", model)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// HealthCheck performs a database health check
func HealthCheck() error {
	if db == nil {
		return fmt.Errorf("database connection not initialized")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// GetConnectionStats returns database connection statistics
func GetConnectionStats() map[string]interface{} {
	if db == nil {
		return map[string]interface{}{
			"status": "not_initialized",
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
	}

	stats := sqlDB.Stats()
	return map[string]interface{}{
		"status":              "connected",
		"max_open_conns":      stats.MaxOpenConnections,
		"open_conns":          stats.OpenConnections,
		"in_use":              stats.InUse,
		"idle":                stats.Idle,
		"wait_count":          stats.WaitCount,
		"wait_duration":       stats.WaitDuration.String(),
		"max_idle_closed":     stats.MaxIdleClosed,
		"max_lifetime_closed": stats.MaxLifetimeClosed,
	}
}
