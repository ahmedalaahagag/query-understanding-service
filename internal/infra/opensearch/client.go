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
	Term        string `json:"term"`
	Type        string `json:"type"` // SYN, HYP, SW
	IsCanonical bool   `json:"isCanonical"` // true if the input IS the canonical term (has its own entry)
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
// It searches the variant field to find canonical forms. Returns the canonical term
// and whether the input is itself a canonical form (IsCanonical flag).
func (c *Client) Lookup(ctx context.Context, term, locale string) ([]LinguisticMatch, error) {
	// Search both directions: is the input a variant of something (variant field),
	// OR is it a canonical term with variants (term field)?
	body := map[string]interface{}{
		"size": 50,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					map[string]interface{}{
						"term": map[string]interface{}{
							"variant": term,
						},
					},
					map[string]interface{}{
						"term": map[string]interface{}{
							"term": term,
						},
					},
				},
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

	// Separate hits into two groups:
	// 1. Entries where the input is the term (canonical) — means the input IS a canonical form.
	// 2. Entries where the input is a variant — these give us the canonical form to replace with.
	isCanonical := false
	var canonicals []LinguisticMatch
	for _, hit := range result.Hits.Hits {
		if strings.EqualFold(hit.Source.Term, term) {
			// The input is a canonical term — it has variants but is already the primary form.
			isCanonical = true
		} else if strings.EqualFold(hit.Source.Variant, term) {
			// The input is a variant of this canonical term.
			canonicals = append(canonicals, LinguisticMatch{
				Term: hit.Source.Term,
				Type: hit.Source.Type,
			})
		}
	}

	// If the input is already a canonical form, mark all results so the
	// synonym expander can skip replacement for bidirectional synonyms.
	for i := range canonicals {
		canonicals[i].IsCanonical = isCanonical
	}
	return canonicals, nil
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

// FetchStopwords returns the set of stopwords from the linguistic index for the
// given locale. Intended to be called once at init and cached.
func (c *Client) FetchStopwords(ctx context.Context, locale string) (map[string]bool, error) {
	body := map[string]interface{}{
		"size": 500,
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"type.keyword": "SW",
			},
		},
		"_source": []string{"term"},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling stopword query: %w", err)
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
		return nil, fmt.Errorf("stopword fetch: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading stopword response: %w", err)
	}

	var result linguisticSearchResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing stopword response: %w", err)
	}

	stopwords := make(map[string]bool, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		stopwords[strings.ToLower(hit.Source.Term)] = true
	}
	return stopwords, nil
}

// FetchAllStopwords loads stopwords for all given locales, returning a map
// of locale → stopword set. Errors for individual locales are logged and skipped.
func (c *Client) FetchAllStopwords(ctx context.Context, locales []string) map[string]map[string]bool {
	result := make(map[string]map[string]bool, len(locales))
	for _, locale := range locales {
		sw, err := c.FetchStopwords(ctx, locale)
		if err != nil {
			continue
		}
		if len(sw) > 0 {
			result[strings.ToLower(strings.ReplaceAll(locale, "-", "_"))] = sw
		}
	}
	return result
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

// CompoundEntry represents a compound word rule from the linguistic index.
type CompoundEntry struct {
	Compound string // joined form (e.g., "icecream")
	Parts    string // split form (e.g., "ice cream")
}

// CompoundLookup queries the linguistic dictionary for compound word rules.
type CompoundLookup interface {
	LookupCompounds(ctx context.Context, text, locale string) ([]CompoundEntry, error)
}

// LookupCompounds queries the linguistic index for compound word rules (type=CMP)
// matching the given text against both the term (joined) and variant (split) fields.
func (c *Client) LookupCompounds(ctx context.Context, text, locale string) ([]CompoundEntry, error) {
	body := map[string]interface{}{
		"size": 10,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"term": map[string]interface{}{
							"type.keyword": "CMP",
						},
					},
				},
				"should": []interface{}{
					map[string]interface{}{
						"term": map[string]interface{}{
							"term": text,
						},
					},
					map[string]interface{}{
						"term": map[string]interface{}{
							"variant": text,
						},
					},
				},
				"minimum_should_match": 1,
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling compound lookup: %w", err)
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
		return nil, fmt.Errorf("executing compound lookup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opensearch returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result linguisticSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding compound response: %w", err)
	}

	var entries []CompoundEntry
	for _, hit := range result.Hits.Hits {
		entries = append(entries, CompoundEntry{
			Compound: hit.Source.Term,
			Parts:    hit.Source.Variant,
		})
	}
	return entries, nil
}

// FuzzySearcher provides fuzzy-matching search capabilities for the native pipeline.
type FuzzySearcher interface {
	FuzzySuggest(ctx context.Context, token, locale string) (string, float64, error)
	FuzzySearchConcepts(ctx context.Context, text, locale, market string) ([]ConceptHit, error)
}

// FuzzySuggest finds the best fuzzy match for a token against the concept index.
// Returns the corrected term, its score, and any error. Returns empty string if no match.
func (c *Client) FuzzySuggest(ctx context.Context, token, locale string) (string, float64, error) {
	body := map[string]interface{}{
		"size": 1,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     token,
				"fields":    []string{"label^2", "aliases"},
				"fuzziness": "AUTO",
				"type":      "best_fields",
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", 0, fmt.Errorf("marshalling fuzzy suggest: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_search", c.cfg.URL, indexName(c.cfg.ConceptIndexPrefix, locale))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("executing fuzzy suggest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("opensearch returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result conceptSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, fmt.Errorf("decoding fuzzy suggest response: %w", err)
	}

	if len(result.Hits.Hits) == 0 {
		return "", 0, nil
	}

	hit := result.Hits.Hits[0]
	return hit.Source.Label, hit.Score, nil
}

// FuzzySearchConcepts searches the concept index with fuzzy matching enabled.
// Handles typos, synonym variants, and compound word matching natively in OS.
func (c *Client) FuzzySearchConcepts(ctx context.Context, text, locale, market string) ([]ConceptHit, error) {
	body := map[string]interface{}{
		"size": 10,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     text,
				"fields":    []string{"label^2", "aliases"},
				"type":      "best_fields",
				"fuzziness": "AUTO",
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling fuzzy concept search: %w", err)
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
		return nil, fmt.Errorf("executing fuzzy concept search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opensearch returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result conceptSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding fuzzy concept response: %w", err)
	}

	var hits []ConceptHit
	for _, hit := range result.Hits.Hits {
		source := "fuzzy"
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
