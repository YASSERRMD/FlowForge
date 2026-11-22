package apierror

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestStatusForCode(t *testing.T) {
	cases := map[Code]int{
		CodeValidation:    400,
		CodeMalformedJSON: 400,
		CodeUnknownFields: 400,
		CodeBodyTooLarge:  413,
		CodeUnauthorized:  401,
		CodeForbidden:     403,
		CodeNotFound:      404,
		CodeConflict:      409,
		CodeUnavailable:   503,
		CodeInternal:      500,
	}
	for code, want := range cases {
		if got := StatusForCode(code); got != want {
			t.Errorf("StatusForCode(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestWriteRendersUniformBody(t *testing.T) {
	err := Validation(FieldError{Field: "name", Message: "is required"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	Write(rec, req, discardLogger(), err)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Error struct {
			Code   string       `json:"code"`
			Fields []FieldError `json:"fields"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Error.Code != "validation_failed" {
		t.Errorf("code = %q", body.Error.Code)
	}
	if len(body.Error.Fields) != 1 || body.Error.Fields[0].Field != "name" {
		t.Errorf("fields = %+v", body.Error.Fields)
	}
}

func TestWriteCollapsesUnknownErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	Write(rec, req, discardLogger(), errors.New("postgres://user:pw@host leaked"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "postgres://") {
		t.Fatal("internal detail leaked to client")
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Error.Code != CodeInternal {
		t.Errorf("code = %q", body.Error.Code)
	}
}

func TestJSONHelper(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, http.StatusCreated, map[string]string{"id": "123"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
}
