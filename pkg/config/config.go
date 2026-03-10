package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all infrastructure configuration loaded from environment variables.
type Config struct {
	HTTP       HTTPConfig       `envconfig:"HTTP"`
	OpenSearch OpenSearchConfig `envconfig:"OPENSEARCH"`
	Otel       OtelConfig       `envconfig:"OTEL"`
	LLM        LLMConfig        `envconfig:"LLM"`
	ConfigDir  string           `envconfig:"CONFIG_DIR" default:"configs"`
}

// LLMConfig holds LLM provider connection settings.
type LLMConfig struct {
	Enabled       bool          `envconfig:"ENABLED" default:"false"`
	Provider      string        `envconfig:"PROVIDER" default:"ollama"` // ollama or bedrock
	Region        string        `envconfig:"REGION" default:"eu-west-1"`
	Model         string        `envconfig:"MODEL" default:"qwen2.5:3b"`
	URL           string        `envconfig:"URL" default:"http://localhost:11434"`
	Timeout       time.Duration `envconfig:"TIMEOUT" default:"30s"`
	MaxRetries    int           `envconfig:"MAX_RETRIES" default:"1"`
	MinConfidence float64       `envconfig:"MIN_CONFIDENCE" default:"0.65"`
	FailOpen      bool          `envconfig:"FAIL_OPEN" default:"true"`
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Port int `envconfig:"PORT" default:"8080"`
}

// OpenSearchConfig holds OpenSearch connection settings.
type OpenSearchConfig struct {
	URL                   string        `envconfig:"URL" default:"http://localhost:9200"`
	Username              string        `envconfig:"USERNAME"`
	Password              string        `envconfig:"PASSWORD"`
	ConceptIndexPrefix    string        `envconfig:"CONCEPT_INDEX_PREFIX" default:"concepts"`
	LinguisticIndexPrefix string        `envconfig:"LINGUISTIC_INDEX_PREFIX" default:"linguistic"`
	Timeout               time.Duration `envconfig:"TIMEOUT" default:"5s"`
}

// OtelConfig holds OpenTelemetry settings.
type OtelConfig struct {
	Enabled      bool   `envconfig:"ENABLED" default:"false"`
	ExporterAddr string `envconfig:"EXPORTER_ADDR" default:"localhost:4317"`
	ServiceName  string `envconfig:"SERVICE_NAME" default:"qus"`
}

// Load reads configuration from environment variables with the given prefix.
func Load(prefix string) (Config, error) {
	var cfg Config
	if err := envconfig.Process(prefix, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks that required configuration fields are set and valid.
func (c Config) Validate() error {
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.HTTP.Port)
	}
	if c.OpenSearch.URL == "" {
		return fmt.Errorf("OpenSearch URL is required")
	}
	if c.ConfigDir == "" {
		return fmt.Errorf("config directory is required")
	}
	return nil
}
