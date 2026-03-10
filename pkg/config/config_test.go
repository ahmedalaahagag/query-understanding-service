package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any env vars that might interfere
	os.Clearenv()

	cfg, err := Load("QUS")
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.HTTP.Port)
	assert.Equal(t, "http://localhost:9200", cfg.OpenSearch.URL)
	assert.Equal(t, "concepts", cfg.OpenSearch.ConceptIndexPrefix)
	assert.Equal(t, false, cfg.Otel.Enabled)
	assert.Equal(t, "qus", cfg.Otel.ServiceName)
	assert.Equal(t, "configs", cfg.ConfigDir)
}

func TestLoad_FromEnv(t *testing.T) {
	os.Clearenv()
	t.Setenv("QUS_HTTP_PORT", "9090")
	t.Setenv("QUS_OPENSEARCH_URL", "http://os:9200")
	t.Setenv("QUS_CONFIG_DIR", "/etc/qus")

	cfg, err := Load("QUS")
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.HTTP.Port)
	assert.Equal(t, "http://os:9200", cfg.OpenSearch.URL)
	assert.Equal(t, "/etc/qus", cfg.ConfigDir)
}

func TestLoadPipelineConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qus.yaml")

	data := `
server:
  port: 8080
pipeline:
  enabled_steps:
    - normalize
    - tokenize
spell:
  enabled: true
  min_token_length: 4
  confidence_threshold: 0.85
concept:
  shingle_max_size: 4
  max_matches_per_span: 3
ambiguity:
  prefer_longest_span: true
  min_score_delta: 0.05
`
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))

	cfg, err := LoadPipelineConfig(path)
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, []string{"normalize", "tokenize"}, cfg.Pipeline.EnabledSteps)
	assert.True(t, cfg.Spell.Enabled)
	assert.Equal(t, 4, cfg.Spell.MinTokenLength)
	assert.Equal(t, 0.85, cfg.Spell.ConfidenceThreshold)
	assert.Equal(t, 4, cfg.Concept.ShingleMaxSize)
	assert.True(t, cfg.Ambiguity.PreferLongestSpan)
}

func TestLoadPipelineConfig_FileNotFound(t *testing.T) {
	_, err := LoadPipelineConfig("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestValidate_Valid(t *testing.T) {
	cfg := Config{
		HTTP:      HTTPConfig{Port: 8080},
		OpenSearch: OpenSearchConfig{URL: "http://localhost:9200"},
		ConfigDir: "configs",
	}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too high", 70000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				HTTP:       HTTPConfig{Port: tt.port},
				OpenSearch: OpenSearchConfig{URL: "http://localhost:9200"},
				ConfigDir:  "configs",
			}
			assert.ErrorContains(t, cfg.Validate(), "invalid HTTP port")
		})
	}
}

func TestValidate_MissingOpenSearchURL(t *testing.T) {
	cfg := Config{
		HTTP:      HTTPConfig{Port: 8080},
		OpenSearch: OpenSearchConfig{URL: ""},
		ConfigDir: "configs",
	}
	assert.ErrorContains(t, cfg.Validate(), "OpenSearch URL is required")
}

func TestValidate_MissingConfigDir(t *testing.T) {
	cfg := Config{
		HTTP:       HTTPConfig{Port: 8080},
		OpenSearch: OpenSearchConfig{URL: "http://localhost:9200"},
		ConfigDir:  "",
	}
	assert.ErrorContains(t, cfg.Validate(), "config directory is required")
}
