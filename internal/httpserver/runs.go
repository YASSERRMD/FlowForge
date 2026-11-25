package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/go-chi/chi/v5"

	"github.com/YASSERRMD/FlowForge/internal/apierror"
	"github.com/YASSERRMD/FlowForge/internal/engine"
)

// EngineService abstracts run initiation and inspection for the HTTP layer.
type EngineService interface {
	CreateRun(ctx context.Context, callerID, workflowID string,
		input json.RawMessage, idempotencyKey string) (*engine.CreatedRun, error)
	GetRunDetail(ctx context.Context, callerID, runID string) (*engine.RunDetail, error)
}

// createRunRequest is the body of POST /workflows/{id}/runs. Both fields
// are optional: absent input becomes an empty object.
type createRunRequest struct {
	Input          json.RawMessage `json:"input"`
	IdempotencyKey string          `json:"idempotencyKey"`
}

// runResponse is the API view of a freshly initiated run.
type runResponse struct {
	ID            string `json:"id"`
	WorkflowID    string `json:"workflowId"`
	VersionNumber int32  `json:"versionNumber"`
	Status        string `json:"status"`
	TriggerType   string `json:"triggerType"`
	Replayed      bool   `json:"replayed"`
	CreatedAt     string `json:"createdAt"`
}

// requireEngine fails closed with 503 when the engine is not wired.
func (s *Server) requireEngine(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.engine == nil {
			apierror.Write(w, r, s.log, apierror.New(apierror.CodeUnavailable,
				"run execution is not configured"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleCreateRun initiates a run of the workflow's published version.
func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.ownerOrError(w, r)
	if !ok {
		return
	}

	var req createRunRequest
	if r.ContentLength != 0 {
		if err := DecodeJSON(r, &req, maxWorkflowBody); err != nil {
			apierror.Write(w, r, s.log, err)
			return
		}
	}

	created, err := s.engine.CreateRun(r.Context(), ownerID,
		chi.URLParam(r, "workflowID"), req.Input, req.IdempotencyKey)
	if err != nil {
		s.writeEngineError(w, r, err)
		return
	}

	status := http.StatusCreated
	if created.ReplayedFrom {
		status = http.StatusOK
	}
	apierror.JSON(w, status, runResponse{
		ID:            engineUUID(created.Run.ID),
		WorkflowID:    engineUUID(created.Run.WorkflowID),
		VersionNumber: created.Version.VersionNumber,
		Status:        created.Run.Status,
		TriggerType:   created.Run.TriggerType,
		Replayed:      created.ReplayedFrom,
		CreatedAt:     created.Run.CreatedAt.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	})
}

// handleGetRun returns the read-only run detail: status, node states,
// duration and structured attempts.
func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.ownerOrError(w, r)
	if !ok {
		return
	}
	detail, err := s.engine.GetRunDetail(r.Context(), ownerID, chi.URLParam(r, "runID"))
	if err != nil {
		s.writeEngineError(w, r, err)
		return
	}
	apierror.JSON(w, http.StatusOK, detail)
}

func (s *Server) writeEngineError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, engine.ErrWorkflowNotFound):
		apierror.Write(w, r, s.log, apierror.New(apierror.CodeNotFound, "workflow not found"))
	case errors.Is(err, engine.ErrNoPublishedVersion):
		apierror.Write(w, r, s.log, apierror.New(apierror.CodeConflict,
			"workflow has no published version; publish before running"))
	case errors.Is(err, engine.ErrRunNotFound):
		apierror.Write(w, r, s.log, apierror.New(apierror.CodeNotFound, "run not found"))
	case errors.Is(err, engine.ErrInvalidInput):
		apierror.Write(w, r, s.log, apierror.Validation(
			apierror.FieldError{Field: "input", Message: err.Error()}))
	default:
		apierror.Write(w, r, s.log, err)
	}
}

func engineUUID(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		id.Bytes[0:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:16])
}
