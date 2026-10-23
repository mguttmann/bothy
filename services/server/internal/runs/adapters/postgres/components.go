// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// Compile-time check: Repository is the outgoing port of components and interactions (M2-15).
var _ app.ComponentRepository = (*Repository)(nil)

// clientInteractionConstraint is the unique key (principal_id, client_interaction_id) of 00061.
const clientInteractionConstraint = "interactions_principal_id_client_interaction_id_key"

// InsertComponents implements app.ComponentRepository: one INSERT per component inside the unit
// of work of the card (at most component.max_per_message rows).
func (r *Repository) InsertComponents(ctx context.Context, rows []app.ComponentRow) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	for _, c := range rows {
		if c.Position < 1 || c.Position > maxInt16 {
			return errs.New(errs.CodeInternalError, "runs: a component position outside smallint")
		}
		if err := q.InsertComponent(ctx, sqlcgen.InsertComponentParams{
			ID: uuid(c.ID), MessageID: uuid(c.MessageID), CustomID: c.CustomID, Type: c.Type, Definition: c.Definition,
			Position: int16(c.Position), SingleUse: c.SingleUse, RequiredCapability: optionalText(c.RequiredCapability),
			RestrictedTo: c.RestrictedTo, RunID: optionalUUID(c.RunID), ExpiresAt: optionalTS(c.ExpiresAt),
		}); err != nil {
			return mapError(err, "inserting a component")
		}
	}
	return nil
}

// maxInt16 bounds message_components.position.
const maxInt16 = 1<<15 - 1

// ComponentsOfMessages implements app.ComponentRepository; an empty list reads nothing.
func (r *Repository) ComponentsOfMessages(ctx context.Context, messageIDs []ids.ID) ([]app.ComponentRow, error) {
	if len(messageIDs) == 0 {
		return nil, nil
	}
	keys := make([]pgtype.UUID, len(messageIDs))
	for i, id := range messageIDs {
		keys[i] = uuid(id)
	}
	rows, err := r.read(ctx).ComponentsOfMessages(ctx, keys)
	return components(rows, err, "reading the components of messages")
}

// Component implements app.ComponentRepository.
func (r *Repository) Component(ctx context.Context, id ids.ID) (app.ComponentRow, bool, error) {
	row, err := r.read(ctx).GetComponent(ctx, uuid(id))
	return oneComponent(row, err, "reading a component")
}

// LockComponent implements app.ComponentRepository. Errors: internal outside a unit of work.
func (r *Repository) LockComponent(ctx context.Context, id ids.ID) (app.ComponentRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.ComponentRow{}, false, err
	}
	row, err := q.LockComponent(ctx, uuid(id))
	return oneComponent(row, err, "locking a component")
}

// LockSingleUseGroup implements app.ComponentRepository. Errors: internal outside a unit of work.
func (r *Repository) LockSingleUseGroup(ctx context.Context, messageID ids.ID) ([]app.ComponentRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockSingleUseGroup(ctx, uuid(messageID))
	return components(rows, err, "locking the single-use group")
}

// LockSingleUseSet implements app.ComponentRepository. Errors: internal outside a unit of work.
func (r *Repository) LockSingleUseSet(ctx context.Context, messageID ids.ID) ([]app.ComponentRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockSingleUseSet(ctx, uuid(messageID))
	return components(rows, err, "locking the single-use set")
}

// LockDueComponents implements app.ComponentRepository. Errors: internal outside a unit of work.
func (r *Repository) LockDueComponents(ctx context.Context, messageID ids.ID, now time.Time) ([]app.ComponentRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockDueComponents(ctx, sqlcgen.LockDueComponentsParams{MessageID: uuid(messageID), Now: ts(now)})
	return components(rows, err, "locking the due components")
}

// NextComponentExpiry implements app.ComponentRepository.
func (r *Repository) NextComponentExpiry(ctx context.Context, messageID ids.ID) (time.Time, bool, error) {
	at, err := r.read(ctx).NextComponentExpiry(ctx, uuid(messageID))
	if err != nil {
		return time.Time{}, false, mapError(err, "reading the next component expiry")
	}
	return timeOf(at), at.Valid, nil
}

// SetComponent implements app.ComponentRepository. Errors: not_found for a component that is gone,
// internal outside a unit of work.
func (r *Repository) SetComponent(ctx context.Context, w app.ComponentWrite) (app.ComponentRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.ComponentRow{}, err
	}
	row, err := q.SetComponent(ctx, sqlcgen.SetComponentParams{
		State: w.State, Definition: w.Definition, UsedBy: optionalUUID(w.UsedBy), UsedAt: optionalTS(w.UsedAt), ID: uuid(w.ID),
	})
	out, found, err := oneComponent(row, err, "changing a component")
	if err == nil && !found {
		return app.ComponentRow{}, errs.New(errs.CodeNotFound, "runs: the component is gone")
	}
	return out, err
}

// ActiveComponentMessagesOfRun implements app.ComponentRepository.
func (r *Repository) ActiveComponentMessagesOfRun(ctx context.Context, runID ids.ID) ([]ids.ID, error) {
	rows, err := r.read(ctx).ActiveComponentMessagesOfRun(ctx, uuid(runID))
	if err != nil {
		return nil, mapError(err, "reading the cards of a run")
	}
	out := make([]ids.ID, len(rows))
	for i, u := range rows {
		out[i] = idOf(u)
	}
	return out, nil
}

// LockActiveRunComponents implements app.ComponentRepository. Errors: internal outside a unit of
// work.
func (r *Repository) LockActiveRunComponents(ctx context.Context, runID, messageID ids.ID) ([]app.ComponentRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockActiveRunComponents(ctx, sqlcgen.LockActiveRunComponentsParams{RunID: uuid(runID), MessageID: uuid(messageID)})
	return components(rows, err, "locking the active components of a run")
}

// DeleteComponentsOfMessage implements app.ComponentRepository. Errors: internal outside a unit
// of work.
func (r *Repository) DeleteComponentsOfMessage(ctx context.Context, messageID ids.ID) (int64, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteComponentsOfMessage(ctx, uuid(messageID))
	if err != nil {
		return 0, mapError(err, "deleting the components of a message")
	}
	return n, nil
}

// InsertInteraction implements app.ComponentRepository. The unique key of (principal,
// client_interaction_id) gives app.ErrInteractionTaken. Errors: internal outside a unit of work.
func (r *Repository) InsertInteraction(ctx context.Context, n app.NewInteraction) (app.InteractionRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.InteractionRow{}, err
	}
	values := n.Values
	if len(values) == 0 {
		values = []byte("{}")
	}
	row, err := q.InsertInteraction(ctx, sqlcgen.InsertInteractionParams{
		ID: uuid(n.ID), ComponentID: uuid(n.ComponentID), MessageID: uuid(n.MessageID), PrincipalID: uuid(n.PrincipalID),
		ClientInteractionID: uuid(n.ClientInteractionID), Values: values, At: ts(n.At),
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == clientInteractionConstraint {
		return app.InteractionRow{}, fmt.Errorf("%w", app.ErrInteractionTaken)
	}
	if err != nil {
		return app.InteractionRow{}, mapError(err, "inserting an interaction")
	}
	return toInteraction(row), nil
}

// InteractionByClientID implements app.ComponentRepository.
func (r *Repository) InteractionByClientID(ctx context.Context, principal, clientID ids.ID) (app.InteractionRow, bool, error) {
	row, err := r.read(ctx).InteractionByClientID(ctx, sqlcgen.InteractionByClientIDParams{
		PrincipalID: uuid(principal), ClientInteractionID: uuid(clientID),
	})
	return oneInteraction(row, err, "reading an interaction by its client id")
}

// Interaction implements app.ComponentRepository.
func (r *Repository) Interaction(ctx context.Context, id ids.ID) (app.InteractionRow, bool, error) {
	row, err := r.read(ctx).GetInteraction(ctx, uuid(id))
	return oneInteraction(row, err, "reading an interaction")
}

// LockInteraction implements app.ComponentRepository. Errors: internal outside a unit of work.
func (r *Repository) LockInteraction(ctx context.Context, id ids.ID) (app.InteractionRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.InteractionRow{}, false, err
	}
	row, err := q.LockInteraction(ctx, uuid(id))
	return oneInteraction(row, err, "locking an interaction")
}

// SetInteractionState implements app.ComponentRepository. Errors: not_found for an interaction
// that is gone, internal outside a unit of work.
func (r *Repository) SetInteractionState(ctx context.Context, id ids.ID, state string, response []byte, at time.Time) (app.InteractionRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.InteractionRow{}, err
	}
	row, err := q.SetInteractionState(ctx, sqlcgen.SetInteractionStateParams{State: state, Response: response, At: ts(at), ID: uuid(id)})
	out, found, err := oneInteraction(row, err, "changing an interaction")
	if err == nil && !found {
		return app.InteractionRow{}, errs.New(errs.CodeNotFound, "runs: the interaction is gone")
	}
	return out, err
}

// RunOfInteraction implements app.ComponentRepository.
func (r *Repository) RunOfInteraction(ctx context.Context, id ids.ID) (ids.ID, bool, error) {
	run, err := r.read(ctx).RunOfInteraction(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ids.ID{}, false, nil
	}
	if err != nil {
		return ids.ID{}, false, mapError(err, "reading the run of an interaction")
	}
	return idOf(run), true, nil
}

// LockRunShared implements app.ComponentRepository. Errors: internal outside a unit of work.
func (r *Repository) LockRunShared(ctx context.Context, id ids.ID) (app.RunRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.RunRow{}, false, err
	}
	row, err := q.LockRunShared(ctx, uuid(id))
	return one(row, err, "share-locking a run")
}

// oneComponent converts a single-row result: no row is found == false.
func oneComponent(row sqlcgen.MessageComponent, err error, what string) (app.ComponentRow, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ComponentRow{}, false, nil
	}
	if err != nil {
		return app.ComponentRow{}, false, mapError(err, what)
	}
	return toComponent(row), true, nil
}

// components converts a list result.
func components(rows []sqlcgen.MessageComponent, err error, what string) ([]app.ComponentRow, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]app.ComponentRow, len(rows))
	for i, row := range rows {
		out[i] = toComponent(row)
	}
	return out, nil
}

// toComponent converts a row of message_components.
func toComponent(row sqlcgen.MessageComponent) app.ComponentRow {
	return app.ComponentRow{
		ID: idOf(row.ID), MessageID: idOf(row.MessageID), CustomID: row.CustomID, Type: row.Type, Definition: row.Definition,
		Position: int(row.Position), SingleUse: row.SingleUse, RequiredCapability: row.RequiredCapability.String,
		RestrictedTo: row.RestrictedTo, RunID: idOf(row.RunID), State: row.State, ExpiresAt: timeOf(row.ExpiresAt),
		UsedBy: idOf(row.UsedBy), UsedAt: timeOf(row.UsedAt), Version: row.Version,
	}
}

// oneInteraction converts a single-row result: no row is found == false.
func oneInteraction(row sqlcgen.Interaction, err error, what string) (app.InteractionRow, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.InteractionRow{}, false, nil
	}
	if err != nil {
		return app.InteractionRow{}, false, mapError(err, what)
	}
	return toInteraction(row), true, nil
}

// toInteraction converts a row of interactions.
func toInteraction(row sqlcgen.Interaction) app.InteractionRow {
	return app.InteractionRow{
		ID: idOf(row.ID), ComponentID: idOf(row.ComponentID), MessageID: idOf(row.MessageID), PrincipalID: idOf(row.PrincipalID),
		ClientInteractionID: idOf(row.ClientInteractionID), Values: row.Values, State: row.State, Response: row.Response,
		CreatedAt: timeOf(row.CreatedAt), UpdatedAt: timeOf(row.UpdatedAt),
	}
}
