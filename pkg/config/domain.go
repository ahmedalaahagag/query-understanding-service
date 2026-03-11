package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PipelineConfig holds the main pipeline configuration.
type PipelineConfig struct {
	Server   ServerConfig   `yaml:"server"`
	Pipeline PipelineSteps  `yaml:"pipeline"`
	Spell    SpellConfig    `yaml:"spell"`
	Concept  ConceptConfig  `yaml:"concept"`
	Ambiguity AmbiguityConfig `yaml:"ambiguity"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type PipelineSteps struct {
	EnabledSteps []string `yaml:"enabled_steps"`
}

type SpellConfig struct {
	Enabled             bool    `yaml:"enabled"`
	MinTokenLength      int     `yaml:"min_token_length"`
	ConfidenceThreshold float64 `yaml:"confidence_threshold"`
}

type ConceptConfig struct {
	ShingleMaxSize   int `yaml:"shingle_max_size"`
	MaxMatchesPerSpan int `yaml:"max_matches_per_span"`
}

type AmbiguityConfig struct {
	PreferLongestSpan bool    `yaml:"prefer_longest_span"`
	MinScoreDelta     float64 `yaml:"min_score_delta"`
}

// ComprehensionConfig holds comprehension rule definitions, keyed by language prefix.
// The top-level map key is a language code (e.g. "en", "de", "fr").
type ComprehensionConfig map[string]LocaleComprehensionRules

// LocaleComprehensionRules holds all comprehension rules for a single language.
type LocaleComprehensionRules struct {
	FilterRules []FilterRule `yaml:"filter_rules"`
	SortRules   []SortRule   `yaml:"sort_rules"`
}

// FilterRule extracts a filter from the query via regex.
// For numeric filters: the pattern must have a capture group for the number.
// For keyword filters: Value is used directly (no capture group needed).
type FilterRule struct {
	Pattern    string  `yaml:"pattern"`
	Field      string  `yaml:"field"`
	Operator   string  `yaml:"operator"`
	Value      string  `yaml:"value,omitempty"`      // static value for keyword filters
	Multiplier float64 `yaml:"multiplier,omitempty"` // numeric multiplier (default 1)
}

type SortRule struct {
	Pattern   string `yaml:"pattern"`
	Field     string `yaml:"field"`
	Direction string `yaml:"direction"`
}

// LoadPipelineConfig loads the main pipeline configuration from a YAML file.
func LoadPipelineConfig(path string) (PipelineConfig, error) {
	var cfg PipelineConfig
	if err := loadYAML(path, &cfg); err != nil {
		return PipelineConfig{}, fmt.Errorf("loading pipeline config: %w", err)
	}
	return cfg, nil
}

// LoadComprehensionConfig loads locale-keyed comprehension rules from a YAML file.
func LoadComprehensionConfig(path string) (ComprehensionConfig, error) {
	cfg := make(ComprehensionConfig)
	if err := loadYAML(path, &cfg); err != nil {
		return nil, fmt.Errorf("loading comprehension config: %w", err)
	}
	return cfg, nil
}

// AllowedFiltersConfig holds the allowlist of valid filter fields and operators.
type AllowedFiltersConfig struct {
	Filters []AllowedFilter `yaml:"filters"`
}

// AllowedFilter defines a single allowed filter field with its operators and type.
type AllowedFilter struct {
	Field       string             `yaml:"field"`
	Operators   []string           `yaml:"operators"`
	Type        string             `yaml:"type"`
	Multiplier  float64            `yaml:"multiplier,omitempty"`
	Description string             `yaml:"description,omitempty"`
	Examples    []string           `yaml:"examples,omitempty"`
	WordValues  map[string]float64 `yaml:"word_values,omitempty"`
}

// AllowedSortsConfig holds the allowlist of valid sort keys.
type AllowedSortsConfig struct {
	Sorts []AllowedSort `yaml:"sorts"`
}

// AllowedSort defines a single allowed sort key.
type AllowedSort struct {
	Key string `yaml:"key"`
}

// LoadAllowedFiltersConfig loads the allowed filters configuration from a YAML file.
func LoadAllowedFiltersConfig(path string) (AllowedFiltersConfig, error) {
	var cfg AllowedFiltersConfig
	if err := loadYAML(path, &cfg); err != nil {
		return AllowedFiltersConfig{}, fmt.Errorf("loading allowed filters config: %w", err)
	}
	return cfg, nil
}

// LoadAllowedSortsConfig loads the allowed sorts configuration from a YAML file.
func LoadAllowedSortsConfig(path string) (AllowedSortsConfig, error) {
	var cfg AllowedSortsConfig
	if err := loadYAML(path, &cfg); err != nil {
		return AllowedSortsConfig{}, fmt.Errorf("loading allowed sorts config: %w", err)
	}
	return cfg, nil
}

func loadYAML(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	return nil
}
