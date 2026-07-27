package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/school/attendance-backend/internal/apperror"
)

type Meta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type envelope struct {
	Data  any    `json:"data,omitempty"`
	Meta  *Meta  `json:"meta,omitempty"`
	Error *errBody `json:"error,omitempty"`
}

type errBody struct {
	Code    apperror.Code        `json:"code"`
	Message string               `json:"message"`
	Details []apperror.FieldError `json:"details,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Data: data})
}

func WriteList(w http.ResponseWriter, status int, data any, meta Meta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Data: data, Meta: &meta})
}

func WriteError(w http.ResponseWriter, err error) {
	ae, ok := apperror.AsAppError(err)
	if !ok {
		ae = apperror.Internal(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.HTTPStatus)
	_ = json.NewEncoder(w).Encode(envelope{Error: &errBody{
		Code:    ae.Code,
		Message: ae.Message,
		Details: ae.Details,
	}})
}

func WriteCreated(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusCreated, data)
}

func WriteOK(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, data)
}

func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
