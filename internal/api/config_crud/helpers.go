package config_crud

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Gen-Do/service-configs/internal/generated/server/api"
	"github.com/Gen-Do/service-configs/internal/model"
)

func toEntry(cfg *model.Config) api.ConfigEntry {
	var value interface{}
	_ = json.Unmarshal(cfg.Value, &value)

	return api.ConfigEntry{
		Key:         cfg.Key,
		Value:       value,
		Description: cfg.Description,
		CreatedAt:   cfg.CreatedAt,
		UpdatedAt:   cfg.UpdatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, api.ErrorResponse{Message: &msg})
}

func isConflict(err error) bool {
	return strings.Contains(err.Error(), "already exists")
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found")
}

func isValidation(err error) bool {
	return strings.Contains(err.Error(), "invalid")
}
