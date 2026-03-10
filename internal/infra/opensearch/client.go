package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SpellSuggestion represents a single spelling suggestion from OpenSearch.
type SpellSuggestion struct {
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

// SpellChecker checks spelling of tokens against OpenSearch.
type SpellChecker interface {
	Suggest(ctx context.Context, text, locale string) ([]SpellSuggestion, error)
}

// LinguisticMatch represents a match from the linguistic dictionary index.
type LinguisticMatch struct {
	Term string `json:"term"`
	Type string `json:"type"` // SYN, HYP, SW
}

// LinguisticLookup queries the linguistic dictionary for synonyms/hypernyms.
type LinguisticLookup interface {
	Lookup(ctx context.Context, term, locale string) ([]LinguisticMatch, error)
}

// ConceptHit represents a concept match from the concept index.
type ConceptHit struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	Field  string  `json:"field"`
	Weight int     `json:"weight"`
	Score  float64 `json:"score"`
	Source string  `json:"source"` // exact, alias
}

// ConceptSearcher searches the concept index for matching concepts.
type ConceptSearcher interface {
	SearchConcepts(ctx context.Context, text, locale, market string) ([]ConceptHit, error)
}

// ClientConfig holds OpenSearch connection settings.
type ClientConfig struct {
	URL                    string
	Username               string
	Password               string
	ConceptIndexPrefix     string
	LinguisticIndexPrefix  string
	Timeout                time.Duration
}

// Client is a plain net/http-based OpenSearch client.
type Client struct {
	cfg    ClientConfig
	http   *http.Client
}

// NewClient creates a new OpenSearch client.
func NewClient(cfg ClientConfig) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.ConceptIndexPrefix == "" {
		cfg.ConceptIndexPrefix = "concepts"
	}
	if cfg.LinguisticIndexPrefix == "" {
		cfg.LinguisticIndexPrefix = "linguistic"
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// indexName builds a locale-specific index name, e.g. "concepts" + "en_US" → "concepts_en_us".
func indexName(prefix, locale string) string {
	return prefix + "_" + strings.ToLower(strings.ReplaceAll(locale, "-", "_"))
}

// Suggest queries the OpenSearch term suggester for spelling corrections.
func (c *Client) Suggest(ctx context.Context, text, locale string) ([]SpellSuggestion, error) {
	body := map[string]interface{}{
		"suggest": map[string]interface{}{
			"spell-check": map[string]interface{}{
				"text": text,
				"term": map[string]interface{}{
					"field":           "label",
					"suggest_mode":    "popular",
					"min_word_length": 3,
					"max_edits":       2,
				},
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling suggest request: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_search?size=0", c.cfg.URL, indexName(c.cfg.ConceptIndexPrefix, locale))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing suggest request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opensearch returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result suggestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding suggest response: %w", err)
	}

	return parseSuggestions(result), nil
}

type suggestResponse struct {
	Suggest map[string][]suggestEntry `json:"suggest"`
}

type suggestEntry struct {
	Options []suggestOption `json:"options"`
}

type suggestOption struct {
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

// Lookup queries the linguistic dictionary index for synonyms/hypernyms of a term.
func (c *Client) Lookup(ctx context.Context, term, locale string) ([]LinguisticMatch, error) {
	body := map[string]interface{}{
		"size": 50,
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"term": term,
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling linguistic lookup: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_search", c.cfg.URL, indexName(c.cfg.LinguisticIndexPrefix, locale))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing linguistic lookup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opensearch returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result linguisticSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding linguistic response: %w", err)
	}

	var matches []LinguisticMatch
	for _, hit := range result.Hits.Hits {
		matches = append(matches, LinguisticMatch{
			Term: hit.Source.Variant,
			Type: hit.Source.Type,
		})
	}
	return matches, nil
}

type linguisticSearchResponse struct {
	Hits linguisticHits `json:"hits"`
}

type linguisticHits struct {
	Hits []linguisticHit `json:"hits"`
}

type linguisticHit struct {
	Source linguisticSource `json:"_source"`
}

type linguisticSource struct {
	Term    string `json:"term"`
	Variant string `json:"variant"`
	Type    string `json:"type"`
	Locale  string `json:"locale"`
}

// SearchConcepts queries the concept index for matching concepts.
// It uses multi_match with cross_fields to search both label and aliases.
func (c *Client) SearchConcepts(ctx context.Context, text, locale, market string) ([]ConceptHit, error) {
	body := map[string]interface{}{
		"size": 10,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  text,
				"fields": []string{"label^2", "aliases"},
				"type":   "cross_fields",
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling concept search: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_search", c.cfg.URL, indexName(c.cfg.ConceptIndexPrefix, locale))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing concept search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opensearch returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result conceptSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding concept response: %w", err)
	}

	var hits []ConceptHit
	for _, hit := range result.Hits.Hits {
		source := "alias"
		if hit.Source.Label == text {
			source = "exact"
		}
		hits = append(hits, ConceptHit{
			ID:     hit.Source.ID,
			Label:  hit.Source.Label,
			Field:  hit.Source.Field,
			Weight: hit.Source.Weight,
			Score:  hit.Score,
			Source: source,
		})
	}
	return hits, nil
}

type conceptSearchResponse struct {
	Hits conceptHits `json:"hits"`
}

type conceptHits struct {
	Hits []conceptHit `json:"hits"`
}

type conceptHit struct {
	Score  float64       `json:"_score"`
	Source conceptSource `json:"_source"`
}

type conceptSource struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Field   string   `json:"field"`
	Aliases []string `json:"aliases"`
	Weight  int      `json:"weight"`
	Locale  string   `json:"locale"`
	Market  string   `json:"market"`
}

func parseSuggestions(resp suggestResponse) []SpellSuggestion {
	entries, ok := resp.Suggest["spell-check"]
	if !ok || len(entries) == 0 {
		return nil
	}

	var suggestions []SpellSuggestion
	for _, entry := range entries {
		for _, opt := range entry.Options {
			suggestions = append(suggestions, SpellSuggestion{
				Text:  opt.Text,
				Score: opt.Score,
			})
		}
	}
	return suggestions
}
