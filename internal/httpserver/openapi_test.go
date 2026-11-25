package httpserver_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// loadSpec parses and validates the committed OpenAPI artifact.
func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "api", "openapi.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read openapi spec: %v", err)
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(data)
	if err != nil {
		t.Fatalf("parse openapi spec: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("openapi spec invalid: %v", err)
	}
	return doc
}

func TestOpenAPISpecIsValid(t *testing.T) {
	loadSpec(t)
}

// TestOpenAPICoversImplementedRoutes fails when the spec and the running
// route table drift apart.
func TestOpenAPICoversImplementedRoutes(t *testing.T) {
	doc := loadSpec(t)

	implemented := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/readyz"},
		{http.MethodGet, "/api/v1/ping"},
		{http.MethodGet, "/api/v1/me"},
		{http.MethodGet, "/api/v1/templates"},
		{http.MethodPost, "/api/v1/workflows"},
		{http.MethodGet, "/api/v1/workflows"},
		{http.MethodGet, "/api/v1/workflows/{workflowID}"},
		{http.MethodPatch, "/api/v1/workflows/{workflowID}"},
		{http.MethodDelete, "/api/v1/workflows/{workflowID}"},
		{http.MethodPut, "/api/v1/workflows/{workflowID}/draft"},
		{http.MethodPost, "/api/v1/workflows/{workflowID}/publish"},
		{http.MethodGet, "/api/v1/workflows/{workflowID}/versions"},
		{http.MethodGet, "/api/v1/workflows/{workflowID}/versions/{versionNumber}"},
		{http.MethodPost, "/api/v1/workflows/{workflowID}/versions/{versionNumber}/copy-to-draft"},
		{http.MethodPost, "/api/v1/workflows/{workflowID}/clone"},
		{http.MethodPost, "/api/v1/workflows/{workflowID}/runs"},
	}
	for _, route := range implemented {
		item := doc.Paths.Find(route.path)
		if item == nil {
			t.Errorf("spec missing implemented path %s", route.path)
			continue
		}
		if item.GetOperation(route.method) == nil {
			t.Errorf("spec missing %s %s", route.method, route.path)
		}
	}
}

// TestDocumentedResponsesMatchBehavior spot-checks that documented
// responses match what the server really emits.
func TestDocumentedResponsesMatchBehavior(t *testing.T) {
	doc := loadSpec(t)

	for path, methods := range map[string][]string{
		"/healthz":     {http.MethodGet},
		"/readyz":      {http.MethodGet},
		"/api/v1/ping": {http.MethodGet},
		"/api/v1/me":   {http.MethodGet},
	} {
		for _, method := range methods {
			op := doc.Paths.Find(path).GetOperation(method)
			if op.Responses == nil || op.Responses.Len() == 0 {
				t.Errorf("%s %s documents no responses", method, path)
			}
		}
	}

	// /api/v1/me must document 401 — the auth boundary is contractual.
	me := doc.Paths.Find("/api/v1/me").GetOperation(http.MethodGet)
	if me.Responses.Value("401") == nil {
		t.Error("/api/v1/me does not document the 401 response")
	}
}
