package opensearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Suggest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/test-index_en_gb/_search")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		resp := suggestResponse{
			Suggest: map[string][]suggestEntry{
				"spell-check": {
					{
						Options: []suggestOption{
							{Text: "chicken", Score: 0.95},
							{Text: "chickens", Score: 0.80},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		URL:   server.URL,
		ConceptIndexPrefix: "test-index",
	})

	suggestions, err := client.Suggest(context.Background(), "chiken", "en_GB")
	require.NoError(t, err)
	require.Len(t, suggestions, 2)
	assert.Equal(t, "chicken", suggestions[0].Text)
	assert.Equal(t, 0.95, suggestions[0].Score)
	assert.Equal(t, "chickens", suggestions[1].Text)
}

func TestClient_Suggest_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := suggestResponse{
			Suggest: map[string][]suggestEntry{
				"spell-check": {
					{Options: []suggestOption{}},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		URL:   server.URL,
		ConceptIndexPrefix: "test-index",
	})

	suggestions, err := client.Suggest(context.Background(), "chicken", "en_GB")
	require.NoError(t, err)
	assert.Empty(t, suggestions)
}

func TestClient_Suggest_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		URL:   server.URL,
		ConceptIndexPrefix: "test-index",
	})

	_, err := client.Suggest(context.Background(), "chiken", "en_GB")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestClient_Suggest_WithBasicAuth(t *testing.T) {
	var gotUser, gotPass string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		resp := suggestResponse{
			Suggest: map[string][]suggestEntry{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		URL:                server.URL,
		ConceptIndexPrefix: "test-index",
		Username:           "admin",
		Password:           "secret",
	})

	_, err := client.Suggest(context.Background(), "test", "en_GB")
	require.NoError(t, err)
	assert.Equal(t, "admin", gotUser)
	assert.Equal(t, "secret", gotPass)
}
