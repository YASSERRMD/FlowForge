// Package apierror defines the uniform error contract for the FlowForge
// API: machine-readable codes, optional field-level errors, and a single
// JSON shape shared by every endpoint.
package apierror

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/YASSERRMD/FlowForge/internal/httpserver/middleware"
)

// Code is a stable, machine-readable domain error code.
type Code string

const (
	CodeInternal      Code = "internal_error"
	CodeBadRequest    Code = "bad_request"
	CodeMalformedJSON Code = "malformed_json"
	CodeBodyTooLarge  Code = "body_too_large"
	CodeUnknownFields Code = "unknown_fields"
	CodeValidation    Code = "validation_failed"
	CodeUnauthorized  Code = "unauthorized"
	CodeForbidden     Code = "forbidden"
	CodeNotFound      Code = "not_found"
	CodeConflict      Code = "conflict"
	CodeUnavailable   Code = "service_unavailable"
)

// FieldError describes one invalid request field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error is the canonical API error. It implements the error interface and
// carries everything needed to render the response body.
type Error struct {
	Code    Code         `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`

	status int
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// Status returns the HTTP status mapped to this error's code.
func (e *Error) Status() int {
	if e.status != 0 {
		return e.status
	}
	return StatusForCode(e.Code)
}

// StatusForCode maps a domain code to its HTTP status.
func StatusForCode(c Code) int {
	switch c {
	case CodeBadRequest, CodeMalformedJSON, CodeUnknownFields, CodeValidation:
		return http.StatusBadRequest
	case CodeBodyTooLarge:
		return http.StatusRequestEntityTooLarge
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// New builds an error with an explicit status override.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Validation builds a validation error with field-level detail.
func Validation(fields ...FieldError) *Error {
	return &Error{Code: CodeValidation, Message: "request validation failed", Fields: fields}
}

type errorBody struct {
	Error *Error `json:"error"`
}

// Write renders err as the uniform JSON error body. Non-API errors are
// logged server-side and collapsed into a generic internal_error so no
// internals leak to clients.
func Write(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		log.ErrorContext(r.Context(), "unhandled error",
			"request_id", middleware.FromContext(r.Context()),
			"error", err.Error(),
		)
		apiErr = New(CodeInternal, "internal server error")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.Status())
	_ = json.NewEncoder(w).Encode(errorBody{Error: apiErr})
}

// JSON writes a success payload with the given status.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
