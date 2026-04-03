package configclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultTTL     = 5 * time.Minute
	requestTimeout = 5 * time.Second
)

// Client fetches and caches configuration from the configs service.
type Client struct {
	baseURL    string
	httpClient *http.Client
	cache      *cache
	tracer     trace.Tracer
	ttl        time.Duration
}

// New creates a config client. Base URL is read from SERVICE_CONFIGS_URL.
// Panics if the env var is not set and WithBaseURL was not provided.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(os.Getenv("SERVICE_CONFIGS_URL"), "/"),
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		tracer: otel.Tracer("configclient"),
		ttl:    defaultTTL,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.baseURL == "" {
		panic("configclient: SERVICE_CONFIGS_URL is not set: add 'configs' service to dependencies in app.yaml")
	}

	// Ensure baseURL has /v1 suffix for the API
	if !strings.HasSuffix(c.baseURL, "/v1") {
		c.baseURL = strings.TrimRight(c.baseURL, "/") + "/v1"
	}

	c.cache = newCache(c.ttl)

	return c
}

// Get fetches a single config value by key.
func (c *Client) Get(ctx context.Context, key string) (json.RawMessage, error) {
	result, err := c.BatchGet(ctx, []string{key})
	if err != nil {
		return nil, err
	}
	val, ok := result[key]
	if !ok {
		return nil, &MissingKeysError{Keys: []string{key}}
	}
	return val, nil
}

// BatchGet fetches multiple config values in one call.
// Returns cached values for non-expired keys; fetches the rest from the service.
// On fetch failure, returns stale cached values if available.
func (c *Client) BatchGet(ctx context.Context, keys []string) (map[string]json.RawMessage, error) {
	ctx, span := c.tracer.Start(ctx, "configclient.batch_get",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()
	span.SetAttributes(attribute.Int("configclient.keys_requested", len(keys)))

	result := make(map[string]json.RawMessage, len(keys))

	// Collect keys that need fetching
	var keysToFetch []string
	cacheHits := 0
	for _, key := range keys {
		if val, found := c.cache.Get(key); found && !c.cache.IsExpired(key) {
			result[key] = val
			cacheHits++
		} else {
			keysToFetch = append(keysToFetch, key)
		}
	}

	span.SetAttributes(
		attribute.Int("configclient.cache_hits", cacheHits),
		attribute.Int("configclient.keys_to_fetch", len(keysToFetch)),
	)

	if len(keysToFetch) == 0 {
		return result, nil
	}

	// Fetch from service
	fetched, err := c.fetchBatch(ctx, keysToFetch)
	if err != nil {
		span.RecordError(err)

		// Graceful degradation: use stale cached values
		for _, key := range keysToFetch {
			if val, found := c.cache.Get(key); found {
				result[key] = val
			}
		}
		if len(result) < len(keys) {
			return result, fmt.Errorf("configclient: fetch failed and some keys have no cached value: %w", err)
		}
		return result, nil
	}

	// Update cache and result
	c.cache.SetBatch(fetched)
	for k, v := range fetched {
		result[k] = v
	}

	return result, nil
}

type batchRequest struct {
	Keys []string `json:"keys"`
}

type batchResponseEntry struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

type batchResponse struct {
	Configs map[string]batchResponseEntry `json:"configs"`
}

func (c *Client) fetchBatch(ctx context.Context, keys []string) (map[string]json.RawMessage, error) {
	body, err := json.Marshal(batchRequest{Keys: keys})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/internal/configs/batch", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from configs service", resp.StatusCode)
	}

	var batchResp batchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	result := make(map[string]json.RawMessage, len(batchResp.Configs))
	for key, entry := range batchResp.Configs {
		result[key] = entry.Value
	}

	return result, nil
}
