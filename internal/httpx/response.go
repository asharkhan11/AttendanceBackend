package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/school/attendance-backend/internal/apperror"
)

type ErrorRecorder interface {
	RecordError(err error)
}

type Meta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type Envelope struct {
	Data  any      `json:"data,omitempty"`
	Meta  *Meta    `json:"meta,omitempty"`
	Error *ErrBody `json:"error,omitempty"`
}

type ErrBody struct {
	Code    apperror.Code        `json:"code"`
	Message string               `json:"message"`
	Details []apperror.FieldError `json:"details,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Data: data})
}

func WriteList(w http.ResponseWriter, status int, data any, meta Meta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Data: data, Meta: &meta})
}

func WriteError(w http.ResponseWriter, err error) {
	ae, ok := apperror.AsAppError(err)
	if !ok {
		ae = apperror.Internal(err)
	}

	// Record error on response writer for Logger middleware context
	if rec, ok := w.(ErrorRecorder); ok {
		rec.RecordError(err)
	}

	// Log immediately via zerolog with detailed error cause
	if ae.HTTPStatus >= 500 {
		event := log.Error().
			Int("status", ae.HTTPStatus).
			Str("error_code", string(ae.Code)).
			Str("error_msg", ae.Message)
		if ae.Err != nil {
			event = event.Err(ae.Err)
		} else if err != nil {
			event = event.Err(err)
		}
		event.Msg("API internal error")
	} else if ae.HTTPStatus >= 400 {
		event := log.Warn().
			Int("status", ae.HTTPStatus).
			Str("error_code", string(ae.Code)).
			Str("error_msg", ae.Message)
		if ae.Err != nil {
			event = event.Err(ae.Err)
		}
		if len(ae.Details) > 0 {
			event = event.Interface("error_details", ae.Details)
		}
		event.Msg("API client error")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.HTTPStatus)
	_ = json.NewEncoder(w).Encode(Envelope{Error: &ErrBody{
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
