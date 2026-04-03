package configclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoad_Success(t *testing.T) {
	type testConfig struct {
		BotUsername string   `config:"bot_username"`
		MaxRetries int      `config:"max_retries"`
		Origins    []string `config:"origins"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"configs": map[string]any{
				"bot_username": map[string]any{
					"key":   "bot_username",
					"value": "stream_gpt_bot",
				},
				"max_retries": map[string]any{
					"key":   "max_retries",
					"value": 3,
				},
				"origins": map[string]any{
					"key":   "origins",
					"value": []string{"https://example.com"},
				},
			},
		})
	}))
	defer server.Close()

	client := New(WithBaseURL(server.URL + "/v1"))

	var cfg testConfig
	if err := client.Load(context.Background(), &cfg); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.BotUsername != "stream_gpt_bot" {
		t.Errorf("BotUsername = %q, want %q", cfg.BotUsername, "stream_gpt_bot")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if len(cfg.Origins) != 1 || cfg.Origins[0] != "https://example.com" {
		t.Errorf("Origins = %v, want [https://example.com]", cfg.Origins)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	type testConfig struct {
		Required string `config:"required_key"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"configs": map[string]any{},
		})
	}))
	defer server.Close()

	client := New(WithBaseURL(server.URL + "/v1"))

	var cfg testConfig
	err := client.Load(context.Background(), &cfg)
	if err == nil {
		t.Fatal("expected error for missing required key")
	}

	missingErr, ok := err.(*MissingKeysError)
	if !ok {
		t.Fatalf("expected MissingKeysError, got %T: %v", err, err)
	}
	if len(missingErr.Keys) != 1 || missingErr.Keys[0] != "required_key" {
		t.Errorf("missing keys = %v, want [required_key]", missingErr.Keys)
	}
}

func TestLoad_Optional(t *testing.T) {
	type testConfig struct {
		Optional string `config:"optional_key,optional"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"configs": map[string]any{},
		})
	}))
	defer server.Close()

	client := New(WithBaseURL(server.URL + "/v1"))

	var cfg testConfig
	if err := client.Load(context.Background(), &cfg); err != nil {
		t.Fatalf("Load should not fail for optional keys: %v", err)
	}

	if cfg.Optional != "" {
		t.Errorf("Optional = %q, want empty", cfg.Optional)
	}
}

func TestLoad_NotPointerToStruct(t *testing.T) {
	client := New(WithBaseURL("http://localhost/v1"))

	var s string
	if err := client.Load(context.Background(), &s); err == nil {
		t.Fatal("expected error for non-struct pointer")
	}
}
