package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/pipeline"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRouter() http.Handler {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	p := pipeline.New(logger, nil, pipeline.Normalizer{}, pipeline.Tokenizer{})
	return New(logger, p, nil)
}

func TestHealthz(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"])
}

func TestAnalyze_NormalizesAndTokenizes(t *testing.T) {
	router := newTestRouter()

	body := `{"query":"Cheap  CHIKEN  Burger","locale":"en-GB","market":"uk"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/analyze", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.AnalyzeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Cheap  CHIKEN  Burger", resp.OriginalQuery)
	assert.Equal(t, "cheap chiken burger", resp.NormalizedQuery)
	assert.Equal(t, []string{"cheap chiken burger"}, resp.Rewrites)
	require.Len(t, resp.Tokens, 3)
	assert.Equal(t, "cheap", resp.Tokens[0].Value)
	assert.Equal(t, 0, resp.Tokens[0].Position)
	assert.Equal(t, "chiken", resp.Tokens[1].Value)
	assert.Equal(t, 1, resp.Tokens[1].Position)
	assert.Equal(t, "burger", resp.Tokens[2].Value)
	assert.Equal(t, 2, resp.Tokens[2].Position)
	assert.NotNil(t, resp.Concepts)
	assert.NotNil(t, resp.Filters)
}

func TestAnalyze_EmptyQuery(t *testing.T) {
	router := newTestRouter()

	body := `{"query":"","locale":"en-GB","market":"uk"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/analyze", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyze_InvalidJSON(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/v1/analyze", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
