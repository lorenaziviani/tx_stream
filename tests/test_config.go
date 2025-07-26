package tests

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/lorenaziviani/txstream/internal/infrastructure/database"
)

type TestConfig struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
}

func NewTestConfig() *TestConfig {
	return &TestConfig{
		DBHost:     getEnv("TEST_DB_HOST", "localhost"),
		DBPort:     getEnv("TEST_DB_PORT", "5432"),
		DBUser:     getEnv("TEST_DB_USER", "txstream_user"),
		DBPassword: getEnv("TEST_DB_PASSWORD", "txstream_password"),
		DBName:     getEnv("TEST_DB_NAME", "txstream_test"),
		DBSSLMode:  getEnv("TEST_DB_SSL_MODE", "disable"),
	}
}

func SetupTestDatabase(t *testing.T) *gorm.DB {
	config := NewTestConfig()

	os.Setenv("DB_HOST", config.DBHost)
	os.Setenv("DB_PORT", config.DBPort)
	os.Setenv("DB_USER", config.DBUser)
	os.Setenv("DB_PASSWORD", config.DBPassword)
	os.Setenv("DB_NAME", config.DBName)
	os.Setenv("DB_SSL_MODE", config.DBSSLMode)

	err := database.InitializeDatabase()
	require.NoError(t, err)

	db := database.GetDB()
	require.NotNil(t, db)

	CleanupTestDatabase(t, db)

	return db
}

func CleanupTestDatabase(t *testing.T, db *gorm.DB) {
	db.Exec("DELETE FROM outbox")
	db.Exec("DELETE FROM order_items")
	db.Exec("DELETE FROM orders")
	db.Exec("DELETE FROM events")
}

func TeardownTestDatabase(t *testing.T) {
	err := database.CloseDatabase()
	require.NoError(t, err)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
