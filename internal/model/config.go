package model

import (
	"encoding/json"
	"time"
)

// Config — запись конфигурации с произвольным JSON-значением.
type Config struct {
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Description string          `json:"description"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
