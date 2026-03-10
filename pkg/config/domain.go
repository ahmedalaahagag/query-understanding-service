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

// SynonymConfig holds synonym expansion rules.
type SynonymConfig struct {
	Locale  string         `yaml:"locale"`
	Entries []SynonymEntry `yaml:"entries"`
}

type SynonymEntry struct {
	Canonical string   `yaml:"canonical"`
	Variants  []string `yaml:"variants"`
}

// CompoundConfig holds compound word split/join rules.
type CompoundConfig struct {
	Locale string         `yaml:"locale"`
	Split  []string       `yaml:"split"`
	Join   []CompoundJoin `yaml:"join"`
}

type CompoundJoin struct {
	Source []string `yaml:"source"`
	Target string   `yaml:"target"`
}

// ComprehensionConfig holds comprehension rule definitions.
type ComprehensionConfig struct {
	PriceRules []PriceRule `yaml:"price_rules"`
	SortRules  []SortRule  `yaml:"sort_rules"`
}

type PriceRule struct {
	Pattern  string `yaml:"pattern"`
	Field    string `yaml:"field"`
	Operator string `yaml:"operator"`
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

// LoadSynonymConfig loads synonym configuration from a YAML file.
func LoadSynonymConfig(path string) (SynonymConfig, error) {
	var cfg SynonymConfig
	if err := loadYAML(path, &cfg); err != nil {
		return SynonymConfig{}, fmt.Errorf("loading synonym config: %w", err)
	}
	return cfg, nil
}

// LoadCompoundConfig loads compound word configuration from a YAML file.
func LoadCompoundConfig(path string) (CompoundConfig, error) {
	var cfg CompoundConfig
	if err := loadYAML(path, &cfg); err != nil {
		return CompoundConfig{}, fmt.Errorf("loading compound config: %w", err)
	}
	return cfg, nil
}

// LoadComprehensionConfig loads comprehension rules from a YAML file.
func LoadComprehensionConfig(path string) (ComprehensionConfig, error) {
	var cfg ComprehensionConfig
	if err := loadYAML(path, &cfg); err != nil {
		return ComprehensionConfig{}, fmt.Errorf("loading comprehension config: %w", err)
	}
	return cfg, nil
}

// AllowedFiltersConfig holds the allowlist of valid filter fields and operators.
type AllowedFiltersConfig struct {
	Filters []AllowedFilter `yaml:"filters"`
}

// AllowedFilter defines a single allowed filter field with its operators and type.
type AllowedFilter struct {
	Field       string            `yaml:"field"`
	Operators   []string          `yaml:"operators"`
	Type        string            `yaml:"type"`
	Description string            `yaml:"description,omitempty"`
	Examples    []string          `yaml:"examples,omitempty"`
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
