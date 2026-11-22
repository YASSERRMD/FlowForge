package httpserver

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/apierror"
)

type payload struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func decodeBody(t *testing.T, body string, maxBytes int64) error {
	t.Helper()
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	var dst payload
	return DecodeJSON(req, &dst, maxBytes)
}

func TestDecodeValid(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"flow","age":3}`))
	var dst payload
	if err := DecodeJSON(req, &dst, 1024); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if dst.Name != "flow" || dst.Age != 3 {
		t.Errorf("dst = %+v", dst)
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	err := decodeBody(t, `{"name": `, 1024)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.Code != apierror.CodeMalformedJSON {
		t.Fatalf("err = %v", err)
	}
}

func TestDecodeRejectsEmptyBody(t *testing.T) {
	err := decodeBody(t, ``, 1024)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.Code != apierror.CodeMalformedJSON {
		t.Fatalf("err = %v", err)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	err := decodeBody(t, `{"name":"flow","age":1,"evil":true}`, 1024)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.Code != apierror.CodeUnknownFields {
		t.Fatalf("err = %v", err)
	}
	if len(apiErr.Fields) != 1 || apiErr.Fields[0].Field != "evil" {
		t.Errorf("fields = %+v", apiErr.Fields)
	}
}

func TestDecodeRejectsOversizedBody(t *testing.T) {
	err := decodeBody(t, `{"name":"`+strings.Repeat("x", 2048)+`"}`, 64)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.Code != apierror.CodeBodyTooLarge {
		t.Fatalf("err = %v", err)
	}
}

func TestDecodeRejectsTrailingData(t *testing.T) {
	err := decodeBody(t, `{"name":"flow","age":1} {"name":"second"}`, 4096)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.Code != apierror.CodeMalformedJSON {
		t.Fatalf("err = %v", err)
	}
}

func TestDecodeRejectsWrongTypes(t *testing.T) {
	err := decodeBody(t, `{"name":"flow","age":"three"}`, 1024)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.Code != apierror.CodeValidation {
		t.Fatalf("err = %v", err)
	}
	if len(apiErr.Fields) != 1 || apiErr.Fields[0].Field != "age" {
		t.Errorf("fields = %+v", apiErr.Fields)
	}
}
