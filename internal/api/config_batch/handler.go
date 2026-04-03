package config_batch

import (
	"encoding/json"
	"net/http"

	"github.com/Gen-Do/service-configs/internal/generated/server/api"
	"github.com/Gen-Do/service-configs/internal/service"
)

type Handler struct {
	svc *service.ConfigService
}

func NewHandler(svc *service.ConfigService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) BatchGet(w http.ResponseWriter, r *http.Request) {
	var req api.BatchGetConfigsJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Keys) == 0 {
		writeError(w, http.StatusBadRequest, "keys must not be empty")
		return
	}
	if len(req.Keys) > 100 {
		writeError(w, http.StatusBadRequest, "too many keys (max 100)")
		return
	}

	configs, err := h.svc.BatchGet(r.Context(), req.Keys)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	entries := make(map[string]api.ConfigEntry, len(configs))
	for k, cfg := range configs {
		var value interface{}
		_ = json.Unmarshal(cfg.Value, &value)

		entries[k] = api.ConfigEntry{
			Key:         cfg.Key,
			Value:       value,
			Description: cfg.Description,
			CreatedAt:   cfg.CreatedAt,
			UpdatedAt:   cfg.UpdatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.BatchGetResponse{Configs: entries})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{Message: &msg})
}
