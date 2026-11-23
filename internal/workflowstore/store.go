// Package workflowstore implements the workflow persistence operations the
// HTTP layer needs, over a pgx pool and the sqlc-generated queries.
package workflowstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/FlowForge/internal/db/dbgen"
)

// Sentinel errors mapped to HTTP statuses by the handler layer.
var (
	ErrNotFound = errors.New("workflow not found")
	ErrConflict = errors.New("workflow changed since it was read")
)

// WorkflowSummary is the list view of a workflow.
type WorkflowSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	RowVersion  int32  `json:"rowVersion"`
	UpdatedAt   string `json:"updatedAt"`
}

// DraftState is the mutable draft version of a workflow.
type DraftState struct {
	VersionID     string          `json:"versionId"`
	VersionNumber int32           `json:"versionNumber"`
	RowVersion    int32           `json:"rowVersion"`
	ContentHash   string          `json:"contentHash"`
	Graph         json.RawMessage `json:"graph"`
}

// WorkflowDetail is a workflow plus its draft version.
type WorkflowDetail struct {
	WorkflowSummary
	Draft *DraftState `json:"draft,omitempty"`
}

// Store wraps a pgx pool.
type Store struct {
	pool *pgxpool.Pool
}

// New builds a Store over pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func uuidOf(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", id.Bytes[0:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:16])
}

func parseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return id, fmt.Errorf("parse uuid %q: %w", s, err)
	}
	return id, nil
}

func summaryOf(w dbgen.Workflow) WorkflowSummary {
	updated := ""
	if w.UpdatedAt.Valid {
		updated = w.UpdatedAt.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	}
	return WorkflowSummary{
		ID:          uuidOf(w.ID),
		Name:        w.Name,
		Description: w.Description,
		Status:      w.Status,
		RowVersion:  w.RowVersion,
		UpdatedAt:   updated,
	}
}

func draftOf(v dbgen.WorkflowVersion) *DraftState {
	return &DraftState{
		VersionID:     uuidOf(v.ID),
		VersionNumber: v.VersionNumber,
		RowVersion:    v.RowVersion,
		ContentHash:   fmt.Sprintf("%x", v.ContentHash),
		Graph:         json.RawMessage(v.GraphJson),
	}
}

// EnsureUser returns the ID of the user with the given email, creating the
// row on first use (dev identity bootstrap).
func (s *Store) EnsureUser(ctx context.Context, email, displayName string) (string, error) {
	user, err := dbgen.New(s.pool).EnsureUser(ctx, dbgen.EnsureUserParams{
		Email:       email,
		DisplayName: displayName,
	})
	if err != nil {
		return "", fmt.Errorf("ensure user: %w", err)
	}
	return uuidOf(user.ID), nil
}

// CreateWorkflow inserts the workflow and its first draft version in one
// transaction.
func (s *Store) CreateWorkflow(ctx context.Context, ownerID, name, description string, graph json.RawMessage, contentHash []byte) (*WorkflowDetail, error) {
	owner, err := parseUUID(ownerID)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := dbgen.New(tx)
	workflow, err := q.CreateWorkflow(ctx, dbgen.CreateWorkflowParams{
		OwnerID:     owner,
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("create workflow: %w", err)
	}
	version, err := q.CreateWorkflowVersion(ctx, dbgen.CreateWorkflowVersionParams{
		WorkflowID:    workflow.ID,
		VersionNumber: 1,
		GraphJson:     graph,
		ContentHash:   contentHash,
		CreatedBy:     owner,
	})
	if err != nil {
		return nil, fmt.Errorf("create draft version: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	detail := &WorkflowDetail{WorkflowSummary: summaryOf(workflow), Draft: draftOf(version)}
	return detail, nil
}

// ListWorkflows returns the owner's non-archived workflows, newest first.
func (s *Store) ListWorkflows(ctx context.Context, ownerID string, limit, offset int32) ([]WorkflowSummary, error) {
	owner, err := parseUUID(ownerID)
	if err != nil {
		return nil, err
	}
	rows, err := dbgen.New(s.pool).ListWorkflowsByOwner(ctx, dbgen.ListWorkflowsByOwnerParams{
		OwnerID: owner,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	out := make([]WorkflowSummary, 0, len(rows))
	for _, w := range rows {
		out = append(out, summaryOf(w))
	}
	return out, nil
}

// GetWorkflow returns one workflow with its draft version.
func (s *Store) GetWorkflow(ctx context.Context, ownerID, id string) (*WorkflowDetail, error) {
	owner, err := parseUUID(ownerID)
	if err != nil {
		return nil, ErrNotFound
	}
	wfID, err := parseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	q := dbgen.New(s.pool)
	workflow, err := q.GetWorkflowForOwner(ctx, dbgen.GetWorkflowForOwnerParams{ID: wfID, OwnerID: owner})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get workflow: %w", err)
	}
	detail := &WorkflowDetail{WorkflowSummary: summaryOf(workflow)}
	version, err := q.GetDraftVersion(ctx, wfID)
	if err == nil {
		detail.Draft = draftOf(version)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get draft version: %w", err)
	}
	return detail, nil
}

// UpdateWorkflowMeta renames or re-describes a workflow.
func (s *Store) UpdateWorkflowMeta(ctx context.Context, ownerID, id, name, description string) (*WorkflowSummary, error) {
	detail, err := s.GetWorkflow(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	wfID, _ := parseUUID(detail.ID)
	workflow, err := dbgen.New(s.pool).UpdateWorkflowMeta(ctx, dbgen.UpdateWorkflowMetaParams{
		ID:          wfID,
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("update workflow: %w", err)
	}
	summary := summaryOf(workflow)
	return &summary, nil
}

// ArchiveWorkflow soft-deletes a workflow owned by ownerID.
func (s *Store) ArchiveWorkflow(ctx context.Context, ownerID, id string) error {
	detail, err := s.GetWorkflow(ctx, ownerID, id)
	if err != nil {
		return err
	}
	wfID, _ := parseUUID(detail.ID)
	_, err = dbgen.New(s.pool).ArchiveWorkflow(ctx, wfID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("archive workflow: %w", err)
	}
	return nil
}

// SaveDraft replaces the draft graph, guarded by the row_version the caller
// last read. A stale token yields ErrConflict.
func (s *Store) SaveDraft(ctx context.Context, ownerID, workflowID string, graph json.RawMessage, contentHash []byte, expectedRowVersion int32) (*DraftState, error) {
	detail, err := s.GetWorkflow(ctx, ownerID, workflowID)
	if err != nil {
		return nil, err
	}
	if detail.Draft == nil {
		return nil, ErrNotFound
	}
	versionID, _ := parseUUID(detail.Draft.VersionID)
	version, err := dbgen.New(s.pool).UpdateDraftGraph(ctx, dbgen.UpdateDraftGraphParams{
		ID:          versionID,
		RowVersion:  expectedRowVersion,
		GraphJson:   graph,
		ContentHash: contentHash,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("update draft graph: %w", err)
	}
	return draftOf(version), nil
}
