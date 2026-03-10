package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/hybrid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Parse_Success(t *testing.T) {
	llmResult := hybrid.LLMParseResult{
		NormalizedQuery: "chicken burger",
		Rewrites:        []string{"chicken burger"},
		CandidateConcepts: []hybrid.LLMCandidateConcept{
			{Label: "burger", Field: "product_type", Confidence: 0.94},
		},
		Filters: []hybrid.LLMFilter{
			{Field: "price", Operator: "lt", Value: float64(20), Confidence: 0.96},
		},
		Sort:       &hybrid.LLMSort{Field: "price", Direction: "asc", Confidence: 0.72},
		Confidence: 0.89,
	}

	llmJSON, _ := json.Marshal(llmResult)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req chatRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "qwen2.5:3b", req.Model)
		assert.Equal(t, "json", req.Format)
		assert.False(t, req.Stream)
		assert.Len(t, req.Messages, 2)

		resp := chatResponse{
			Message: chatMessage{Role: "assistant", Content: string(llmJSON)},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{
		URL:     srv.URL,
		Model:   "qwen2.5:3b",
		Timeout: 5 * time.Second,
	})

	result, err := c.Parse(context.Background(), "system prompt", "cheap chicken burger under 20")
	require.NoError(t, err)
	assert.Equal(t, "chicken burger", result.NormalizedQuery)
	assert.Len(t, result.CandidateConcepts, 1)
	assert.Len(t, result.Filters, 1)
	assert.NotNil(t, result.Sort)
	assert.Equal(t, 0.89, result.Confidence)
}

func TestClient_Parse_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Message: chatMessage{Role: "assistant", Content: "not valid json at all"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{
		URL:        srv.URL,
		Model:      "qwen2.5:3b",
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})

	_, err := c.Parse(context.Background(), "system prompt", "test query")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing LLM JSON output")
}

func TestClient_Parse_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{
		URL:        srv.URL,
		Model:      "qwen2.5:3b",
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})

	_, err := c.Parse(context.Background(), "system prompt", "test query")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ollama returned 500")
}

func TestClient_Parse_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{
		URL:        srv.URL,
		Model:      "qwen2.5:3b",
		Timeout:    50 * time.Millisecond,
		MaxRetries: 0,
	})

	_, err := c.Parse(context.Background(), "system prompt", "test query")
	assert.Error(t, err)
}

func TestClient_Parse_RetryOnFailure(t *testing.T) {
	callCount := 0
	llmResult := hybrid.LLMParseResult{
		NormalizedQuery: "test",
		Confidence:      0.8,
	}
	llmJSON, _ := json.Marshal(llmResult)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("temporary error"))
			return
		}
		resp := chatResponse{
			Message: chatMessage{Role: "assistant", Content: string(llmJSON)},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{
		URL:        srv.URL,
		Model:      "qwen2.5:3b",
		Timeout:    5 * time.Second,
		MaxRetries: 1,
	})

	result, err := c.Parse(context.Background(), "system prompt", "test query")
	require.NoError(t, err)
	assert.Equal(t, "test", result.NormalizedQuery)
	assert.Equal(t, 2, callCount)
}
