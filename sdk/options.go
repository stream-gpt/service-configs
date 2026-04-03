package configclient

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Option configures the Client.
type Option func(*Client)

// WithBaseURL overrides the SERVICE_CONFIGS_URL env var.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithTTL overrides the default cache TTL (5 minutes).
func WithTTL(d time.Duration) Option {
	return func(c *Client) {
		c.ttl = d
	}
}

// WithTracer sets a custom OpenTelemetry tracer.
func WithTracer(t trace.Tracer) Option {
	return func(c *Client) {
		c.tracer = t
	}
}
