// Package workflowstore implements the workflow persistence operations the
// HTTP layer needs, over a pgx pool and the sqlc-generated queries.
package workflowstore

import (
	"context"
	"encoding/hex"
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
	ErrNoDraft  = errors.New("workflow has no draft to publish")
)

// PublishedVersion is the frozen result of a publish.
type PublishedVersion struct {
	VersionID     string `json:"versionId"`
	VersionNumber int32  `json:"versionNumber"`
	ContentHash   string `json:"contentHash"`
	PublishedAt   string `json:"publishedAt"`
}

// VersionSummary is the history-list view of one version. Node/edge counts
// and the content hash serve as cheap diff metadata.
type VersionSummary struct {
	VersionID     string `json:"versionId"`
	VersionNumber int32  `json:"versionNumber"`
	Status        string `json:"status"`
	ContentHash   string `json:"contentHash"`
	CreatedBy     string `json:"createdBy"`
	CreatedAt     string `json:"createdAt"`
	PublishedAt   string `json:"publishedAt,omitempty"`
	NodeCount     int    `json:"nodeCount"`
	EdgeCount     int    `json:"edgeCount"`
}

// VersionDetail adds the frozen graph document.
type VersionDetail struct {
	VersionSummary
	Graph json.RawMessage `json:"graph"`
}

func tsOf(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
}

func graphCounts(graphJSON []byte) (nodes, edges int) {
	var doc struct {
		Nodes []json.RawMessage `json:"nodes"`
		Edges []json.RawMessage `json:"edges"`
	}
	if err := json.Unmarshal(graphJSON, &doc); err != nil {
		return 0, 0
	}
	return len(doc.Nodes), len(doc.Edges)
}

func versionSummary(v dbgen.WorkflowVersion, author string) VersionSummary {
	nodes, edges := graphCounts(v.GraphJson)
	return VersionSummary{
		VersionID:     uuidOf(v.ID),
		VersionNumber: v.VersionNumber,
		Status:        v.Status,
		ContentHash:   fmt.Sprintf("%x", v.ContentHash),
		CreatedBy:     author,
		CreatedAt:     tsOf(v.CreatedAt),
		PublishedAt:   tsOf(v.PublishedAt),
		NodeCount:     nodes,
		EdgeCount:     edges,
	}
}

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

// Publish freezes the current draft as a published version in one
// transaction: the draft becomes published, any previously published version
// is superseded, the workflow points at the new version, and a fresh draft
// (next version number, same graph) is opened for further editing.
func (s *Store) Publish(ctx context.Context, ownerID, workflowID string) (*PublishedVersion, error) {
	detail, err := s.GetWorkflow(ctx, ownerID, workflowID)
	if err != nil {
		return nil, err
	}
	if detail.Draft == nil {
		return nil, ErrNoDraft
	}
	wfID, _ := parseUUID(detail.ID)
	draftID, _ := parseUUID(detail.Draft.VersionID)
	owner, _ := parseUUID(ownerID)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := dbgen.New(tx)
	published, err := q.PublishVersion(ctx, draftID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoDraft
		}
		return nil, fmt.Errorf("publish version: %w", err)
	}
	if err := q.SupersedePublishedVersions(ctx, dbgen.SupersedePublishedVersionsParams{
		WorkflowID: wfID,
		ID:         published.ID,
	}); err != nil {
		return nil, fmt.Errorf("supersede previous versions: %w", err)
	}
	if err := q.SetWorkflowCurrentVersion(ctx, dbgen.SetWorkflowCurrentVersionParams{
		ID:               wfID,
		CurrentVersionID: published.ID,
	}); err != nil {
		return nil, fmt.Errorf("set current version: %w", err)
	}
	if err := q.ActivateWorkflow(ctx, wfID); err != nil {
		return nil, fmt.Errorf("activate workflow: %w", err)
	}

	next, err := q.NextVersionNumber(ctx, wfID)
	if err != nil {
		return nil, fmt.Errorf("next version number: %w", err)
	}
	if _, err := q.CreateWorkflowVersion(ctx, dbgen.CreateWorkflowVersionParams{
		WorkflowID:    wfID,
		VersionNumber: next,
		GraphJson:     published.GraphJson,
		ContentHash:   published.ContentHash,
		CreatedBy:     owner,
	}); err != nil {
		return nil, fmt.Errorf("open next draft: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	publishedAt := ""
	if published.PublishedAt.Valid {
		publishedAt = published.PublishedAt.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	}
	return &PublishedVersion{
		VersionID:     uuidOf(published.ID),
		VersionNumber: published.VersionNumber,
		ContentHash:   fmt.Sprintf("%x", published.ContentHash),
		PublishedAt:   publishedAt,
	}, nil
}

func listRowToVersion(r dbgen.ListVersionsWithAuthorRow) dbgen.WorkflowVersion {
	return dbgen.WorkflowVersion{
		ID: r.ID, WorkflowID: r.WorkflowID, VersionNumber: r.VersionNumber,
		Status: r.Status, GraphJson: r.GraphJson, ContentHash: r.ContentHash,
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, PublishedAt: r.PublishedAt,
		RowVersion: r.RowVersion,
	}
}

func getRowToVersion(r dbgen.GetVersionWithAuthorRow) dbgen.WorkflowVersion {
	return dbgen.WorkflowVersion{
		ID: r.ID, WorkflowID: r.WorkflowID, VersionNumber: r.VersionNumber,
		Status: r.Status, GraphJson: r.GraphJson, ContentHash: r.ContentHash,
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, PublishedAt: r.PublishedAt,
		RowVersion: r.RowVersion,
	}
}

// ListVersions returns every version of a workflow, newest first.
func (s *Store) ListVersions(ctx context.Context, ownerID, workflowID string) ([]VersionSummary, error) {
	detail, err := s.GetWorkflow(ctx, ownerID, workflowID)
	if err != nil {
		return nil, err
	}
	wfID, _ := parseUUID(detail.ID)
	rows, err := dbgen.New(s.pool).ListVersionsWithAuthor(ctx, wfID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	out := make([]VersionSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, versionSummary(listRowToVersion(row), row.CreatedByName))
	}
	return out, nil
}

// GetVersion returns one version with its frozen graph.
func (s *Store) GetVersion(ctx context.Context, ownerID, workflowID string, versionNumber int32) (*VersionDetail, error) {
	detail, err := s.GetWorkflow(ctx, ownerID, workflowID)
	if err != nil {
		return nil, err
	}
	wfID, _ := parseUUID(detail.ID)
	row, err := dbgen.New(s.pool).GetVersionWithAuthor(ctx, dbgen.GetVersionWithAuthorParams{
		WorkflowID:    wfID,
		VersionNumber: versionNumber,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get version: %w", err)
	}
	return &VersionDetail{
		VersionSummary: versionSummary(getRowToVersion(row), row.CreatedByName),
		Graph:          json.RawMessage(row.GraphJson),
	}, nil
}

// CopyVersionToDraft reinitializes the mutable draft from any frozen
// version, guarded by the draft's row_version like a normal save.
func (s *Store) CopyVersionToDraft(ctx context.Context, ownerID, workflowID string, versionNumber int32, expectedRowVersion int32) (*DraftState, error) {
	detail, err := s.GetWorkflow(ctx, ownerID, workflowID)
	if err != nil {
		return nil, err
	}
	if detail.Draft == nil {
		return nil, ErrNoDraft
	}
	wfID, _ := parseUUID(detail.ID)
	source, err := dbgen.New(s.pool).GetVersionWithAuthor(ctx, dbgen.GetVersionWithAuthorParams{
		WorkflowID:    wfID,
		VersionNumber: versionNumber,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get source version: %w", err)
	}
	draftID, _ := parseUUID(detail.Draft.VersionID)
	version, err := dbgen.New(s.pool).UpdateDraftGraph(ctx, dbgen.UpdateDraftGraphParams{
		ID:          draftID,
		RowVersion:  expectedRowVersion,
		GraphJson:   source.GraphJson,
		ContentHash: source.ContentHash,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("copy version to draft: %w", err)
	}
	return draftOf(version), nil
}

// CloneWorkflow duplicates a workflow for the calling owner: a brand-new
// workflow row and version rows (fresh entity IDs) whose draft starts from
// the source's published graph, falling back to its draft.
func (s *Store) CloneWorkflow(ctx context.Context, ownerID, sourceID, name string) (*WorkflowDetail, error) {
	source, err := s.GetWorkflow(ctx, ownerID, sourceID)
	if err != nil {
		return nil, err
	}

	var graph json.RawMessage
	var hash []byte
	if published, err := s.publishedVersion(ctx, source.ID); err == nil {
		graph = json.RawMessage(published.GraphJson)
		hash = published.ContentHash
	} else if source.Draft != nil {
		graph = source.Draft.Graph
		raw, decErr := hex.DecodeString(source.Draft.ContentHash)
		if decErr != nil {
			return nil, fmt.Errorf("decode content hash: %w", decErr)
		}
		hash = raw
	} else {
		return nil, ErrNoDraft
	}

	return s.CreateWorkflow(ctx, ownerID, name, "Clone of "+source.Name, graph, hash)
}

func (s *Store) publishedVersion(ctx context.Context, workflowID string) (dbgen.WorkflowVersion, error) {
	wfID, err := parseUUID(workflowID)
	if err != nil {
		return dbgen.WorkflowVersion{}, err
	}
	return dbgen.New(s.pool).GetPublishedVersion(ctx, wfID)
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
