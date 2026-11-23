package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/YASSERRMD/FlowForge/internal/apierror"
	"github.com/YASSERRMD/FlowForge/internal/graph"
	"github.com/YASSERRMD/FlowForge/internal/workflowstore"
)

// WorkflowStore abstracts workflow persistence for the HTTP layer.
type WorkflowStore interface {
	EnsureUser(ctx context.Context, email, displayName string) (string, error)
	CreateWorkflow(ctx context.Context, ownerID, name, description string, graphJSON json.RawMessage, contentHash []byte) (*workflowstore.WorkflowDetail, error)
	ListWorkflows(ctx context.Context, ownerID string, limit, offset int32) ([]workflowstore.WorkflowSummary, error)
	GetWorkflow(ctx context.Context, ownerID, id string) (*workflowstore.WorkflowDetail, error)
	UpdateWorkflowMeta(ctx context.Context, ownerID, id, name, description string) (*workflowstore.WorkflowSummary, error)
	ArchiveWorkflow(ctx context.Context, ownerID, id string) error
	SaveDraft(ctx context.Context, ownerID, workflowID string, graphJSON json.RawMessage, contentHash []byte, expectedRowVersion int32) (*workflowstore.DraftState, error)
	Publish(ctx context.Context, ownerID, workflowID string) (*workflowstore.PublishedVersion, error)
	ListVersions(ctx context.Context, ownerID, workflowID string) ([]workflowstore.VersionSummary, error)
	GetVersion(ctx context.Context, ownerID, workflowID string, versionNumber int32) (*workflowstore.VersionDetail, error)
}

// maxWorkflowBody bounds workflow payloads (a 200-node graph with configs
// stays well under 256 KiB).
const maxWorkflowBody = 256 * 1024

// defaultDraftGraph is the minimal valid graph assigned to new workflows.
const defaultDraftGraph = `{"schemaVersion":"1","nodes":[` +
	`{"id":"start-1","kind":"start","label":"Start","position":{"x":0,"y":0}},` +
	`{"id":"end-1","kind":"end","label":"End","position":{"x":0,"y":240}}],` +
	`"edges":[{"id":"e-start-1-out-end-1-in","source":"start-1","sourceHandle":null,"target":"end-1","targetHandle":null}]}`

var devEmailSanitizer = regexp.MustCompile(`[^a-z0-9-]+`)

func fieldError(field, message string) *apierror.Error {
	return apierror.Validation(apierror.FieldError{Field: field, Message: message})
}

// requireWorkflowStore fails closed with 503 when persistence is not wired.
func (s *Server) requireWorkflowStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.workflows == nil {
			apierror.Write(w, r, s.log, apierror.New(apierror.CodeUnavailable,
				"workflow storage is not configured"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ownerID resolves the authenticated identity to a users row, creating it
// on first use in development.
func (s *Server) ownerID(ctx context.Context, identity *Identity) (string, error) {
	name := strings.ToLower(identity.DisplayName)
	name = devEmailSanitizer.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "user"
	}
	email := "dev-" + name + "@dev.flowforge.local"
	return s.workflows.EnsureUser(ctx, email, identity.DisplayName)
}

func (s *Server) writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, workflowstore.ErrNotFound):
		apierror.Write(w, r, s.log, apierror.New(apierror.CodeNotFound, "workflow not found"))
	case errors.Is(err, workflowstore.ErrNoDraft):
		apierror.Write(w, r, s.log, apierror.New(apierror.CodeConflict,
			"workflow has no draft to publish"))
	case errors.Is(err, workflowstore.ErrConflict):
		apierror.Write(w, r, s.log, apierror.New(apierror.CodeConflict,
			"the workflow changed since you loaded it; reload and try again"))
	default:
		apierror.Write(w, r, s.log, err)
	}
}

func (s *Server) ownerOrError(w http.ResponseWriter, r *http.Request) (string, bool) {
	identity := IdentityFrom(r.Context())
	if identity == nil {
		apierror.Write(w, r, s.log, apierror.New(apierror.CodeUnauthorized, "authentication required"))
		return "", false
	}
	ownerID, err := s.ownerID(r.Context(), identity)
	if err != nil {
		apierror.Write(w, r, s.log, err)
		return "", false
	}
	return ownerID, true
}

type createWorkflowRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Graph       json.RawMessage `json:"graph"`
}

func (s *Server) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.ownerOrError(w, r)
	if !ok {
		return
	}
	var req createWorkflowRequest
	if err := DecodeJSON(r, &req, maxWorkflowBody); err != nil {
		apierror.Write(w, r, s.log, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 120 {
		apierror.Write(w, r, s.log, fieldError("name", "name is required and must be at most 120 characters"))
		return
	}

	graphJSON := json.RawMessage(defaultDraftGraph)
	if len(req.Graph) > 0 {
		graphJSON = req.Graph
		if !s.validateGraphPayload(w, r, graphJSON) {
			return
		}
	}
	hash := sha256.Sum256(graphJSON)

	detail, err := s.workflows.CreateWorkflow(r.Context(), ownerID, req.Name, req.Description, graphJSON, hash[:])
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	w.Header().Set("ETag", draftETag(detail.Draft))
	apierror.JSON(w, http.StatusCreated, detail)
}

func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.ownerOrError(w, r)
	if !ok {
		return
	}
	limit, offset := int32(50), int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			apierror.Write(w, r, s.log, fieldError("limit", "limit must be an integer between 1 and 200"))
			return
		}
		limit = int32(n)
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			apierror.Write(w, r, s.log, fieldError("offset", "offset must be a non-negative integer"))
			return
		}
		offset = int32(n)
	}
	summaries, err := s.workflows.ListWorkflows(r.Context(), ownerID, limit, offset)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	apierror.JSON(w, http.StatusOK, map[string]any{"workflows": summaries})
}

func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.ownerOrError(w, r)
	if !ok {
		return
	}
	detail, err := s.workflows.GetWorkflow(r.Context(), ownerID, chi.URLParam(r, "workflowID"))
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	if detail.Draft != nil {
		w.Header().Set("ETag", draftETag(detail.Draft))
	}
	apierror.JSON(w, http.StatusOK, detail)
}

type updateWorkflowRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.ownerOrError(w, r)
	if !ok {
		return
	}
	var req updateWorkflowRequest
	if err := DecodeJSON(r, &req, maxWorkflowBody); err != nil {
		apierror.Write(w, r, s.log, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 120 {
		apierror.Write(w, r, s.log, fieldError("name", "name is required and must be at most 120 characters"))
		return
	}
	summary, err := s.workflows.UpdateWorkflowMeta(r.Context(), ownerID, chi.URLParam(r, "workflowID"), req.Name, req.Description)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	apierror.JSON(w, http.StatusOK, summary)
}

func (s *Server) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.ownerOrError(w, r)
	if !ok {
		return
	}
	if err := s.workflows.ArchiveWorkflow(r.Context(), ownerID, chi.URLParam(r, "workflowID")); err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type saveDraftRequest struct {
	Graph json.RawMessage `json:"graph"`
}

// handleSaveDraft replaces the draft graph. The caller must send If-Match
// with the draft row_version it last read; a stale token yields 409.
func (s *Server) handleSaveDraft(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.ownerOrError(w, r)
	if !ok {
		return
	}
	expected, err := parseIfMatch(r.Header.Get("If-Match"))
	if err != nil {
		apierror.Write(w, r, s.log, fieldError("If-Match", `If-Match must carry the draft rowVersion, e.g. "3"`))
		return
	}
	var req saveDraftRequest
	if err := DecodeJSON(r, &req, maxWorkflowBody); err != nil {
		apierror.Write(w, r, s.log, err)
		return
	}
	if len(req.Graph) == 0 {
		apierror.Write(w, r, s.log, fieldError("graph", "graph is required"))
		return
	}
	if !s.validateGraphPayload(w, r, req.Graph) {
		return
	}
	hash := sha256.Sum256(req.Graph)

	draft, err := s.workflows.SaveDraft(r.Context(), ownerID, chi.URLParam(r, "workflowID"), req.Graph, hash[:], expected)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	w.Header().Set("ETag", draftETag(draft))
	apierror.JSON(w, http.StatusOK, draft)
}

// handlePublish freezes the draft as an immutable published version. The
// draft graph is re-validated before publishing so a published snapshot is
// always structurally sound.
func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.ownerOrError(w, r)
	if !ok {
		return
	}
	workflowID := chi.URLParam(r, "workflowID")

	detail, err := s.workflows.GetWorkflow(r.Context(), ownerID, workflowID)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	if detail.Draft == nil {
		apierror.Write(w, r, s.log, apierror.New(apierror.CodeConflict,
			"workflow has no draft to publish"))
		return
	}
	if !s.validateGraphPayload(w, r, detail.Draft.Graph) {
		return
	}

	published, err := s.workflows.Publish(r.Context(), ownerID, workflowID)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	apierror.JSON(w, http.StatusOK, published)
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.ownerOrError(w, r)
	if !ok {
		return
	}
	versions, err := s.workflows.ListVersions(r.Context(), ownerID, chi.URLParam(r, "workflowID"))
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	apierror.JSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.ownerOrError(w, r)
	if !ok {
		return
	}
	n, err := strconv.ParseInt(chi.URLParam(r, "versionNumber"), 10, 32)
	if err != nil || n < 1 {
		apierror.Write(w, r, s.log, fieldError("versionNumber", "versionNumber must be a positive integer"))
		return
	}
	detail, err := s.workflows.GetVersion(r.Context(), ownerID, chi.URLParam(r, "workflowID"), int32(n))
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	apierror.JSON(w, http.StatusOK, detail)
}

// validateGraphPayload parses and structurally validates a graph document,
// writing a 422 with every issue on failure. Returns true when valid.
func (s *Server) validateGraphPayload(w http.ResponseWriter, r *http.Request, raw json.RawMessage) bool {
	var doc graph.Graph
	decErr := json.Unmarshal(raw, &doc)
	if decErr != nil {
		apierror.Write(w, r, s.log, apierror.New(apierror.CodeValidation,
			"graph must be a JSON object matching the graph contract"))
		return false
	}
	issues := graph.Validate(&doc)
	errorIssues := make([]graph.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.Severity == "error" {
			errorIssues = append(errorIssues, issue)
		}
	}
	if len(errorIssues) == 0 {
		return true
	}
	apierror.JSON(w, http.StatusUnprocessableEntity, map[string]any{
		"error": map[string]any{
			"code":    apierror.CodeValidation,
			"message": fmt.Sprintf("graph validation failed with %d issue(s)", len(errorIssues)),
			"issues":  errorIssues,
		},
	})
	return false
}

func draftETag(draft *workflowstore.DraftState) string {
	if draft == nil {
		return ""
	}
	return fmt.Sprintf(`"%d"`, draft.RowVersion)
}

func parseIfMatch(header string) (int32, error) {
	trimmed := strings.Trim(strings.TrimSpace(header), `"`)
	if trimmed == "" {
		return 0, errors.New("missing If-Match")
	}
	n, err := strconv.ParseInt(trimmed, 10, 32)
	if err != nil || n < 1 {
		return 0, errors.New("invalid If-Match")
	}
	return int32(n), nil
}
