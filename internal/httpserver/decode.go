package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/YASSERRMD/FlowForge/internal/apierror"
)

// DecodeJSON strictly decodes a request body into dst:
//   - the body is capped at maxBytes (413 beyond it)
//   - malformed JSON is rejected (400 malformed_json)
//   - unknown fields are rejected and listed (400 unknown_fields)
//   - trailing data after the first JSON value is rejected
//   - an empty body is rejected when the endpoint requires one
func DecodeJSON(r *http.Request, dst any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return classifyDecodeError(err, maxBytes)
	}

	// A well-formed request contains exactly one JSON value.
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return apierror.New(apierror.CodeMalformedJSON,
			"request body must contain a single JSON value")
	}
	return nil
}

func classifyDecodeError(err error, maxBytes int64) error {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return apierror.New(apierror.CodeBodyTooLarge,
			fmt.Sprintf("request body exceeds %d bytes", maxBytes))
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return apierror.New(apierror.CodeMalformedJSON,
			fmt.Sprintf("malformed JSON at byte offset %d", syntaxErr.Offset))
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return apierror.Validation(apierror.FieldError{
			Field:   typeErr.Field,
			Message: fmt.Sprintf("expected %s", typeErr.Type),
		})
	}

	if errors.Is(err, io.EOF) {
		return apierror.New(apierror.CodeMalformedJSON, "request body is empty")
	}

	if unknown, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		field := strings.Trim(unknown, `"`)
		return &apierror.Error{
			Code:    apierror.CodeUnknownFields,
			Message: "request body contains unknown fields",
			Fields: []apierror.FieldError{
				{Field: field, Message: "unknown field"},
			},
		}
	}

	return apierror.New(apierror.CodeMalformedJSON, "could not decode request body")
}
