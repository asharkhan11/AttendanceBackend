package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/school/attendance-backend/internal/apperror"
	"github.com/school/attendance-backend/internal/auth"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(ww, r)

		// Skip logging for routine health checks unless they return an error
		if r.URL.Path == "/healthz" && ww.status < 400 {
			return
		}

		reqID := RequestIDFromContext(r.Context())
		claims := auth.ClaimsFromContext(r.Context())

		var event *zerolog.Event
		if ww.status >= 500 {
			event = log.Error()
		} else if ww.status >= 400 {
			event = log.Warn()
		} else {
			event = log.Info()
		}

		event = event.
			Str("request_id", reqID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.status).
			Dur("latency_ms", time.Since(start))

		if claims != nil {
			event = event.
				Str("school_id", claims.SchoolID.String()).
				Str("user_id", claims.Subject).
				Str("role", string(claims.Role))
		}

		if ww.err != nil {
			event = event.Err(ww.err)
			if ae, ok := apperror.AsAppError(ww.err); ok {
				event = event.Str("error_code", string(ae.Code))
				if ae.Err != nil {
					event = event.Str("cause", ae.Err.Error())
				}
				if len(ae.Details) > 0 {
					event = event.Interface("error_details", ae.Details)
				}
			}
		}

		event.Msg("request")
	})
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	err         error
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) RecordError(err error) {
	rw.err = err
}
