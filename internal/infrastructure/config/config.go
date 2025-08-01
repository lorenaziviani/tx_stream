package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Worker   WorkerConfig   `mapstructure:"worker"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"ssl_mode"`

	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`

	LogLevel      string `mapstructure:"log_level"`
	SlowQueryTime int    `mapstructure:"slow_query_time"`
}

type KafkaConfig struct {
	Brokers     []string `mapstructure:"brokers"`
	TopicEvents string   `mapstructure:"topic_events"`
	GroupID     string   `mapstructure:"group_id"`

	RequiredAcks int           `mapstructure:"required_acks"`
	Timeout      time.Duration `mapstructure:"timeout"`

	AutoOffsetReset string        `mapstructure:"auto_offset_reset"`
	SessionTimeout  time.Duration `mapstructure:"session_timeout"`

	MaxRetries int           `mapstructure:"max_retries"`
	RetryDelay time.Duration `mapstructure:"retry_delay"`

	ExponentialRetryEnabled bool          `mapstructure:"exponential_retry_enabled"`
	BaseDelay               time.Duration `mapstructure:"base_delay"`
	MaxDelay                time.Duration `mapstructure:"max_delay"`
	Multiplier              float64       `mapstructure:"multiplier"`

	CircuitBreakerEnabled bool          `mapstructure:"circuit_breaker_enabled"`
	FailureThreshold      int           `mapstructure:"failure_threshold"`
	SuccessThreshold      int           `mapstructure:"success_threshold"`
	TimeoutDuration       time.Duration `mapstructure:"timeout_duration"`
	ResetTimeout          time.Duration `mapstructure:"reset_timeout"`
}

type WorkerConfig struct {
	PoolSize   int           `mapstructure:"pool_size"`
	BatchSize  int           `mapstructure:"batch_size"`
	Interval   time.Duration `mapstructure:"interval"`
	MaxRetries int           `mapstructure:"max_retries"`
	RetryDelay time.Duration `mapstructure:"retry_delay"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	OutputPath string `mapstructure:"output_path"`
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values for configuration
func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8083)

	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "txstream_user")
	viper.SetDefault("database.password", "txstream_password")
	viper.SetDefault("database.name", "txstream_db")
	viper.SetDefault("database.ssl_mode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", "5m")
	viper.SetDefault("database.log_level", "info")
	viper.SetDefault("database.slow_query_time", 200)

	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.topic_events", "txstream.events")
	viper.SetDefault("kafka.group_id", "txstream-consumer-group")
	viper.SetDefault("kafka.required_acks", 1)
	viper.SetDefault("kafka.timeout", "30s")
	viper.SetDefault("kafka.auto_offset_reset", "earliest")
	viper.SetDefault("kafka.session_timeout", "30s")
	viper.SetDefault("kafka.max_retries", 3)
	viper.SetDefault("kafka.retry_delay", "1s")
	viper.SetDefault("kafka.exponential_retry_enabled", false)
	viper.SetDefault("kafka.base_delay", "1s")
	viper.SetDefault("kafka.max_delay", "30s")
	viper.SetDefault("kafka.multiplier", 2.0)
	viper.SetDefault("kafka.circuit_breaker_enabled", false)
	viper.SetDefault("kafka.failure_threshold", 5)
	viper.SetDefault("kafka.success_threshold", 3)
	viper.SetDefault("kafka.timeout_duration", "10s")
	viper.SetDefault("kafka.reset_timeout", "30s")

	viper.SetDefault("worker.pool_size", 3)
	viper.SetDefault("worker.batch_size", 10)
	viper.SetDefault("worker.interval", "5s")
	viper.SetDefault("worker.max_retries", 3)
	viper.SetDefault("worker.retry_delay", "1s")

	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.port", 9091)
	viper.SetDefault("metrics.path", "/metrics")

	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output_path", "")
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	if err := c.Database.Validate(); err != nil {
		return fmt.Errorf("database config: %w", err)
	}

	if err := c.Kafka.Validate(); err != nil {
		return fmt.Errorf("kafka config: %w", err)
	}

	if err := c.Worker.Validate(); err != nil {
		return fmt.Errorf("worker config: %w", err)
	}

	if err := c.Logging.Validate(); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	return nil
}

// Validate validates server configuration
func (c *ServerConfig) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	return nil
}

// Validate validates database configuration
func (c *DatabaseConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", c.Port)
	}
	if c.User == "" {
		return fmt.Errorf("database user is required")
	}
	if c.Name == "" {
		return fmt.Errorf("database name is required")
	}
	return nil
}

// Validate validates Kafka configuration
func (c *KafkaConfig) Validate() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("at least one Kafka broker is required")
	}
	if c.TopicEvents == "" {
		return fmt.Errorf("kafka topic events is required")
	}
	if c.GroupID == "" {
		return fmt.Errorf("kafka group ID is required")
	}
	return nil
}

// Validate validates worker configuration
func (c *WorkerConfig) Validate() error {
	if c.PoolSize <= 0 {
		return fmt.Errorf("worker pool size must be positive")
	}
	if c.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}
	if c.Interval <= 0 {
		return fmt.Errorf("polling interval must be positive")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}
	return nil
}

// Validate validates logging configuration
func (c *LoggingConfig) Validate() error {
	validLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true, "fatal": true,
	}
	if !validLevels[c.Level] {
		return fmt.Errorf("invalid log level: %s", c.Level)
	}

	validFormats := map[string]bool{
		"json": true, "text": true,
	}
	if !validFormats[c.Format] {
		return fmt.Errorf("invalid log format: %s", c.Format)
	}

	return nil
}

// GetDSN returns the database connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

// GetKafkaBrokers returns Kafka brokers as string slice
func (c *KafkaConfig) GetKafkaBrokers() []string {
	return c.Brokers
}

// IsKafkaEnabled returns true if Kafka is properly configured
func (c *KafkaConfig) IsKafkaEnabled() bool {
	return len(c.Brokers) > 0 && c.Brokers[0] != ""
}
