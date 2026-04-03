package configclient

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Load fetches config values and unmarshals them into dst.
// dst must be a pointer to a struct with `config:"key_name"` tags.
// Use `config:"key_name,optional"` to skip missing keys without error.
//
// Example:
//
//	type MyConfig struct {
//	    BotUsername string   `config:"default_bot_username"`
//	    MaxRetries int      `config:"max_retries,optional"`
//	    Origins    []string `config:"allowed_origins"`
//	}
//	var cfg MyConfig
//	err := client.Load(ctx, &cfg)
func (c *Client) Load(ctx context.Context, dst interface{}) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("configclient: Load requires a pointer to a struct, got %T", dst)
	}

	rv = rv.Elem()
	rt := rv.Type()

	type fieldInfo struct {
		index    int
		key      string
		optional bool
	}

	var fields []fieldInfo
	var keys []string
	seen := make(map[string]bool)

	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("config")
		if tag == "" || tag == "-" {
			continue
		}

		parts := strings.SplitN(tag, ",", 2)
		key := parts[0]
		optional := len(parts) > 1 && parts[1] == "optional"

		fields = append(fields, fieldInfo{index: i, key: key, optional: optional})
		if !seen[key] {
			keys = append(keys, key)
			seen[key] = true
		}
	}

	if len(keys) == 0 {
		return nil
	}

	values, err := c.BatchGet(ctx, keys)
	if err != nil {
		return fmt.Errorf("configclient: load: %w", err)
	}

	var missing []string
	for _, f := range fields {
		raw, ok := values[f.key]
		if !ok {
			if !f.optional {
				missing = append(missing, f.key)
			}
			continue
		}

		fieldPtr := rv.Field(f.index).Addr().Interface()
		if err := json.Unmarshal(raw, fieldPtr); err != nil {
			return fmt.Errorf("configclient: unmarshal key %q into field %s (%s): %w",
				f.key, rt.Field(f.index).Name, rt.Field(f.index).Type, err)
		}
	}

	if len(missing) > 0 {
		return &MissingKeysError{Keys: missing}
	}

	return nil
}
