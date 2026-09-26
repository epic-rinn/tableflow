// Package audit appends immutable audit events inside the caller's
// transaction (OPS-001). Details must never contain secrets or tokens.
package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

// Event is one audited action.
type Event struct {
	BranchID     string
	ActorStaffID *string
	Action       string
	ResourceType string
	ResourceID   string
	Reason       *string
	RequestID    string
	Details      map[string]any
}

const insert = `INSERT INTO audit_events (branch_id, actor_staff_id, action, resource_type, resource_id, reason, request_id, details)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

// Record inserts e in tx.
func Record(ctx context.Context, tx pgx.Tx, e Event) error {
	if e.Details == nil {
		e.Details = map[string]any{}
	}
	b, err := json.Marshal(e.Details)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, insert, e.BranchID, e.ActorStaffID, e.Action, e.ResourceType, e.ResourceID, e.Reason, e.RequestID, b)
	return err
}
