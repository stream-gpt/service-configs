package config_crud

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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req api.CreateConfigJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	valueBytes, err := json.Marshal(req.Value)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid value")
		return
	}

	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}

	cfg, err := h.svc.Create(r.Context(), req.Key, valueBytes, desc)
	if err != nil {
		if isConflict(err) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		if isValidation(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toEntry(cfg))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	configs, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	entries := make([]api.ConfigEntry, 0, len(configs))
	for _, c := range configs {
		entries = append(entries, toEntry(c))
	}

	writeJSON(w, http.StatusOK, map[string]any{"configs": entries})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request, key api.ConfigKey) {
	cfg, err := h.svc.Get(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cfg == nil {
		writeError(w, http.StatusNotFound, "config not found")
		return
	}

	writeJSON(w, http.StatusOK, toEntry(cfg))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request, key api.ConfigKey) {
	var req api.UpdateConfigJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	valueBytes, err := json.Marshal(req.Value)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid value")
		return
	}

	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}

	cfg, err := h.svc.Update(r.Context(), key, valueBytes, desc)
	if err != nil {
		if isValidation(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toEntry(cfg))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request, key api.ConfigKey) {
	if err := h.svc.Delete(r.Context(), key); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
