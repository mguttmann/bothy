// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the repository is the binding store of the use cases (M2-03).
var _ app.BindingStore = (*Repository)(nil)

// workspaceRefIndex is the unique index on lower(workspace_ref) of project_bindings.
const workspaceRefIndex = "project_bindings_workspace_ref_key"

// runnerConfigJSON is the stored form of project_runner_config.v1 (jsonb): absent members are left
// out, an empty tool list stays an empty list, so a stored document compares like the request.
type runnerConfigJSON struct {
	Model        *string   `json:"model,omitempty"`
	AllowedTools *[]string `json:"allowed_tools,omitempty"`
}

// encodeRunnerConfig converts the runner configuration into its jsonb document.
func encodeRunnerConfig(c domain.RunnerConfig) ([]byte, error) {
	doc := runnerConfigJSON{Model: c.Model}
	if c.AllowedTools != nil {
		tools := c.AllowedTools
		doc.AllowedTools = &tools
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "chat: encoding runner_config")
	}
	return b, nil
}

// decodeRunnerConfig converts a stored jsonb document; NULL and {} are the empty configuration.
func decodeRunnerConfig(raw []byte) (domain.RunnerConfig, error) {
	if len(raw) == 0 {
		return domain.RunnerConfig{}, nil
	}
	var doc runnerConfigJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		return domain.RunnerConfig{}, errs.Wrap(errs.CodeInternalError, err, "chat: a stored runner_config is not the schema")
	}
	out := domain.RunnerConfig{Model: doc.Model}
	if doc.AllowedTools != nil {
		out.AllowedTools = *doc.AllowedTools
		if out.AllowedTools == nil {
			out.AllowedTools = []string{}
		}
	}
	return out, nil
}

// bindingWriteError maps a write error of a binding: a taken workspace_ref is 409 conflict
// without a reason (design note 2 f of M2-03), a workspace_ref too large for the unique index is
// a field error of the request; any other error stays internal. No message repeats a value.
func bindingWriteError(err error, what string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == sqlstateUniqueViolation && pgErr.ConstraintName == workspaceRefIndex:
			return errs.Wrap(errs.CodeConflict, err, "the workspace_ref is bound to another forum")
		case pgErr.Code == sqlstateProgramLimitExceeded:
			return errs.Wrap(errs.CodeValidationFailed, err, "the project binding is not valid").
				WithFields(errs.FieldError{Pointer: "/workspace_ref", Code: domain.FieldTooLong, Detail: "workspace_ref is too long to be stored"})
		}
	}
	return mapError(err, what)
}

// LockBinding implements app.BindingStore.
func (r *Repository) LockBinding(ctx context.Context, channel ids.ID) (domain.ProjectBinding, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.ProjectBinding{}, false, err
	}
	row, err := q.LockProjectBinding(ctx, uuid(channel))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProjectBinding{}, false, nil
	}
	if err != nil {
		return domain.ProjectBinding{}, false, mapError(err, "locking a project binding")
	}
	rc, err := decodeRunnerConfig(row.RunnerConfig)
	if err != nil {
		return domain.ProjectBinding{}, false, err
	}
	return domain.ProjectBinding{
		ChannelID: idOf(row.ChannelID), WorkspaceRef: row.WorkspaceRef, RepositoryURL: row.RepositoryUrl.String,
		DefaultBranch: row.DefaultBranch.String, RunnerConfig: rc, Ready: row.Ready, CreatedBy: idOf(row.CreatedBy),
		CreatedAt: timeOf(row.CreatedAt), Version: row.Version,
	}, true, nil
}

// InsertBinding implements app.BindingStore.
func (r *Repository) InsertBinding(ctx context.Context, b domain.ProjectBinding) (time.Time, error) {
	q, err := r.write(ctx)
	if err != nil {
		return time.Time{}, err
	}
	rc, err := encodeRunnerConfig(b.RunnerConfig)
	if err != nil {
		return time.Time{}, err
	}
	at, err := q.InsertProjectBinding(ctx, sqlcgen.InsertProjectBindingParams{
		ChannelID: uuid(b.ChannelID), WorkspaceRef: b.WorkspaceRef, RepositoryUrl: optionalText(b.RepositoryURL),
		DefaultBranch: optionalText(b.DefaultBranch), RunnerConfig: rc, CreatedBy: uuid(b.CreatedBy),
	})
	if err != nil {
		return time.Time{}, bindingWriteError(err, "inserting a project binding")
	}
	return timeOf(at), nil
}

// ReplaceBinding implements app.BindingStore.
func (r *Repository) ReplaceBinding(ctx context.Context, b domain.ProjectBinding) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	rc, err := encodeRunnerConfig(b.RunnerConfig)
	if err != nil {
		return err
	}
	err = q.ReplaceProjectBinding(ctx, sqlcgen.ReplaceProjectBindingParams{
		ChannelID: uuid(b.ChannelID), WorkspaceRef: b.WorkspaceRef, RepositoryUrl: optionalText(b.RepositoryURL),
		DefaultBranch: optionalText(b.DefaultBranch), RunnerConfig: rc, Version: b.Version,
	})
	if err != nil {
		return bindingWriteError(err, "replacing a project binding")
	}
	return nil
}

// DeleteBinding implements app.BindingStore.
func (r *Repository) DeleteBinding(ctx context.Context, channel ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.DeleteProjectBinding(ctx, uuid(channel)), "deleting a project binding")
}

// LockReportedBindings implements app.BindingStore.
func (r *Repository) LockReportedBindings(ctx context.Context, channels []ids.ID) (map[ids.ID]bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockReportedBindings(ctx, uuids(channels))
	if err != nil {
		return nil, mapError(err, "locking reported project bindings")
	}
	out := make(map[ids.ID]bool, len(rows))
	for _, row := range rows {
		out[idOf(row.ChannelID)] = row.Ready
	}
	return out, nil
}

// SetProjectReady implements app.BindingStore.
func (r *Repository) SetProjectReady(ctx context.Context, channel ids.ID, ready bool) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.SetProjectReady(ctx, sqlcgen.SetProjectReadyParams{ChannelID: uuid(channel), Ready: ready}), "writing project readiness")
}

// ResetProjectReadiness implements app.BindingStore.
func (r *Repository) ResetProjectReadiness(ctx context.Context, channels []ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.ResetProjectReadiness(ctx, uuids(channels)), "resetting project readiness")
}
