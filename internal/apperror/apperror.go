package apperror

import (
	"errors"
	"net/http"
)

type Code string

const (
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeNotFound     Code = "NOT_FOUND"
	CodeConflict     Code = "CONFLICT"
	CodeInternal     Code = "INTERNAL_ERROR"
)

type FieldError struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

type AppError struct {
	HTTPStatus int
	Code       Code
	Message    string
	Details    []FieldError
	Err        error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(status int, code Code, message string) *AppError {
	return &AppError{HTTPStatus: status, Code: code, Message: message}
}

func NewWithCause(status int, code Code, message string, err error) *AppError {
	return &AppError{HTTPStatus: status, Code: code, Message: message, Err: err}
}

func Validation(msg string, details ...FieldError) *AppError {
	return &AppError{HTTPStatus: http.StatusBadRequest, Code: CodeValidation, Message: msg, Details: details}
}

func Unauthorized(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusUnauthorized, Code: CodeUnauthorized, Message: msg}
}

func Forbidden(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusForbidden, Code: CodeForbidden, Message: msg}
}

func NotFound(resource string) *AppError {
	return &AppError{HTTPStatus: http.StatusNotFound, Code: CodeNotFound, Message: resource + " not found"}
}

func Conflict(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusConflict, Code: CodeConflict, Message: msg}
}

func Internal(err error) *AppError {
	return &AppError{HTTPStatus: http.StatusInternalServerError, Code: CodeInternal, Message: "internal server error", Err: err}
}

// Is allows errors.Is to match by Code
func Is(err error, code Code) bool {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae.Code == code
	}
	return false
}

func AsAppError(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
