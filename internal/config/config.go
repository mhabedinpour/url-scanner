package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config represents the application configuration structure.
// It contains settings for the environment, HTTP server, database connection,
// and graceful shutdown behavior.
type Config struct {
	// Environment specifies the current running environment (development, production, etc.)
	Environment string `env:"ENVIRONMENT" env-default:"development" yaml:"environment"`

	// HTTP contains all HTTP server related configurations
	HTTP struct {
		// Addr is the address and port the HTTP server will listen on
		Addr string `env:"HTTP_ADDR" env-default:":8080" yaml:"addr"`
		// ReadTimeout is the maximum duration for reading the entire request, including the body
		ReadTimeout time.Duration `env:"HTTP_READ_TIMEOUT" env-default:"1m" yaml:"readTimeout"`
		// ReadHeaderTimeout is the amount of time allowed to read request headers
		ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" env-default:"10s" yaml:"readHeaderTimeout"`
		// WriteTimeout is the maximum duration before timing out writes of the response
		WriteTimeout time.Duration `env:"HTTP_WRITE_TIMEOUT" env-default:"2m" yaml:"writeTimeout"`
		// IdleTimeout is the maximum amount of time to wait for the next request when keep-alives are enabled
		IdleTimeout time.Duration `env:"HTTP_IDLE_TIMEOUT" env-default:"2m" yaml:"idleTimeout"`
		// RequestTimeout is the maximum time allowed for processing a single request
		RequestTimeout time.Duration `env:"HTTP_REQUEST_TIMEOUT" env-default:"10s" yaml:"requestTimeout"`
		// MaxHeaderBytes controls the maximum number of bytes the server will read parsing the request header
		MaxHeaderBytes int `env:"HTTP_MAX_HEADER_BYTES" env-default:"0" yaml:"maxHeaderBytes"`
		// MetricsPath defines the URL path where metrics are exposed
		MetricsPath string `env:"HTTP_METRICS_PATH" env-default:"/metrics" yaml:"metricsPath"`
	} `yaml:"http"`

	// Database contains all database connection related configurations
	Database struct {
		// Username for database authentication
		Username string `env:"DATABASE_USERNAME" env-default:"myuser" yaml:"username"`
		// Password for database authentication
		Password string `env:"DATABASE_PASSWORD" env-default:"mypassword" yaml:"password"`
		// Host is the database server hostname or IP address
		Host string `env:"DATABASE_HOST" env-default:"localhost" yaml:"host"`
		// Port is the database server port number
		Port int `env:"DATABASE_PORT" env-default:"5432" yaml:"port"`
		// SslMode defines the SSL mode for the database connection
		SslMode string `env:"DATABASE_SSL_MODE" env-default:"disable" yaml:"sslMode"`
		// DatabaseName is the name of the database to connect to
		DatabaseName string `env:"DATABASE_NAME" env-default:"scanner" yaml:"name"`
		// MaxOpenConnections limits the number of open connections to the database
		MaxOpenConnections int `env:"DATABASE_MAX_OPEN_CONNECTIONS" env-default:"10" yaml:"maxOpenConnections"`
		// MaxIdleConnections limits the number of connections in the idle connection pool
		MaxIdleConnections int `env:"DATABASE_MAX_IDLE_CONNECTIONS" env-default:"8" yaml:"maxIdleConnections"`
		// ConnMaxLifetime is the maximum amount of time a connection may be reused
		ConnMaxLifetime time.Duration `env:"DATABASE_CONNECTION_MAX_LIFETIME" env-default:"3m" yaml:"connMaxLifetime"`
		// ConnMaxIdleTime is the maximum amount of time a connection may be idle
		ConnMaxIdleTime time.Duration `env:"DATABASE_CONNECTION_MAX_IDLE_TIME" env-default:"3m" yaml:"connMaxIdleTime"`
	} `yaml:"database"`

	// JWT contains keys used for signing and verifying JSON Web Tokens
	JWT struct {
		// PublicKey is the PEM-encoded public key used to verify JWT signatures
		PublicKey string `env:"JWT_PUBLIC_KEY" yaml:"publicKey"`
		// PrivateKey is the PEM-encoded private key used to sign JWTs
		PrivateKey string `env:"JWT_PRIVATE_KEY" yaml:"privateKey"`
	} `yaml:"jwt"`

	// Scanner contains configuration for the URL scanning subsystem
	Scanner struct {
		// MaxAttempts is the maximum number of attempts for a single scan job before giving up
		MaxAttempts int `env:"SCANNER_MAX_ATTEMPTS" env-default:"5" yaml:"maxAttempts"`
		// ResultCacheTTL is the duration for which scan results are cached and reused
		ResultCacheTTL time.Duration `env:"SCANNER_RESULT_CACHE_TTL" env-default:"1h" yaml:"resultCacheTtl"`
		// UrlscanioAPIKey is the API key used to authenticate with urlscan.io
		UrlscanioAPIKey string `env:"SCANNER_URLSCAN_IO_API_KEY" yaml:"urlscanioApiKey"`
	} `yaml:"scanner"`

	// Worker contains configuration for background job processing
	Worker struct {
		// JobTimeout is the maximum duration allowed for a single job execution
		JobTimeout time.Duration `env:"WORKER_JOB_TIMEOUT" env-default:"1m" yaml:"jobTimeout"`
		// JobConcurrency is the number of jobs that can be processed concurrently
		JobConcurrency int `env:"WORKER_JOB_CONCURRENCY" env-default:"10" yaml:"jobConcurrency"`
	} `yaml:"worker"`

	// GracefulShutdownTimeout is the maximum duration to wait for ongoing requests to complete during shutdown
	GracefulShutdownTimeout time.Duration `env:"GRACEFUL_SHUTDOWN_TIMEOUT" env-default:"10s" yaml:"gracefulShutdownTimeout"` //nolint: lll
}

// Load receives the path for yaml config file and returns a filled Config struct.
func Load(configPath string) (*Config, error) {
	var cfg Config
	err := cleanenv.ReadConfig(configPath, &cfg)
	if err != nil {
		return nil, fmt.Errorf("could not read config: %w", err)
	}

	return &cfg, nil
}
