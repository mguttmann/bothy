// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the store of the webhook use cases.
var _ app.WebhookStore = (*Repository)(nil)

// InsertSubscription implements app.WebhookStore.
func (r *Repository) InsertSubscription(ctx context.Context, s app.SubscriptionRecord) (app.SubscriptionRecord, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.SubscriptionRecord{}, err
	}
	row, err := q.InsertSubscription(ctx, sqlcgen.InsertSubscriptionParams{
		ID: uuid(s.ID), AppInstallationID: uuid(s.InstallationID), Url: s.URL, EventTypes: nonNil(s.EventTypes),
		SigningKeyKid: s.SigningKeyKID, HmacEnabled: s.HMACEnabled, CreatedBy: uuid(s.CreatedBy),
	})
	if err != nil {
		return app.SubscriptionRecord{}, mapError(err, "inserting a subscription")
	}
	return toSubscription(row, s.AppID), nil
}

// Subscription implements app.WebhookStore.
func (r *Repository) Subscription(ctx context.Context, id ids.ID) (app.SubscriptionRecord, bool, error) {
	row, err := r.read(ctx).GetSubscription(ctx, uuid(id))
	return one(row, err, func(x sqlcgen.GetSubscriptionRow) app.SubscriptionRecord {
		return toSubscription(x.WebhookSubscription, ids.ID(x.AppID.Bytes))
	}, "reading a subscription")
}

// LockSubscription implements app.WebhookStore.
func (r *Repository) LockSubscription(ctx context.Context, id ids.ID) (app.SubscriptionRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.SubscriptionRecord{}, false, err
	}
	row, err := q.LockSubscription(ctx, uuid(id))
	return one(row, err, func(x sqlcgen.LockSubscriptionRow) app.SubscriptionRecord {
		return toSubscription(x.WebhookSubscription, ids.ID(x.AppID.Bytes))
	}, "locking a subscription")
}

// UpdateSubscription implements app.WebhookStore.
func (r *Repository) UpdateSubscription(ctx context.Context, s app.SubscriptionRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	gen, err := int32Of(s.HMACGeneration)
	if err != nil {
		return err
	}
	failures, err := int32Of(s.ConsecutiveFailures)
	if err != nil {
		return err
	}
	n, err := q.UpdateSubscription(ctx, sqlcgen.UpdateSubscriptionParams{
		ID: uuid(s.ID), Url: s.URL, EventTypes: nonNil(s.EventTypes), SigningKeyKid: s.SigningKeyKID, HmacGeneration: gen,
		HmacRotatedAt: timestamp(s.HMACRotatedAt), State: string(s.State), ConsecutiveFailures: failures,
		LastDeliveredSeq: s.LastDeliveredSeq, DisabledAt: timestamp(s.DisabledAt),
	})
	return affected(n, err, "updating a subscription")
}

// DeleteSubscription implements app.WebhookStore.
func (r *Repository) DeleteSubscription(ctx context.Context, id ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.DeleteSubscription(ctx, uuid(id))
	if err != nil {
		return false, mapError(err, "deleting a subscription")
	}
	return n == 1, nil
}

// AppSubscriptions implements app.WebhookStore.
func (r *Repository) AppSubscriptions(ctx context.Context, appID ids.ID) ([]app.SubscriptionRecord, error) {
	rows, err := r.read(ctx).ListAppSubscriptions(ctx, uuid(appID))
	if err != nil {
		return nil, mapError(err, "listing subscriptions")
	}
	out := make([]app.SubscriptionRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSubscription(row.WebhookSubscription, ids.ID(row.AppID.Bytes)))
	}
	return out, nil
}

// LockInstallationSubscriptions implements app.WebhookStore.
func (r *Repository) LockInstallationSubscriptions(ctx context.Context, installationID ids.ID) ([]app.SubscriptionRecord, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockInstallationSubscriptions(ctx, uuid(installationID))
	if err != nil {
		return nil, mapError(err, "locking the subscriptions of an installation")
	}
	out := make([]app.SubscriptionRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSubscription(row.WebhookSubscription, ids.ID(row.AppID.Bytes)))
	}
	return out, nil
}

// LockLiveSubscriptions implements app.WebhookStore.
func (r *Repository) LockLiveSubscriptions(ctx context.Context) ([]app.SubscriptionRecord, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockLiveSubscriptions(ctx)
	if err != nil {
		return nil, mapError(err, "locking the live subscriptions")
	}
	out := make([]app.SubscriptionRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSubscription(row.WebhookSubscription, ids.ID(row.AppID.Bytes)))
	}
	return out, nil
}

// ShareLiveInstallation implements app.WebhookStore.
func (r *Repository) ShareLiveInstallation(ctx context.Context, appID ids.ID) (app.InstallationRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.InstallationRecord{}, false, err
	}
	row, err := q.ShareLiveInstallation(ctx, uuid(appID))
	return one(row, err, func(x sqlcgen.ShareLiveInstallationRow) app.InstallationRecord {
		return toInstallation(x.AppInstallation)
	}, "locking the live installation")
}

// Installation implements app.WebhookStore.
func (r *Repository) Installation(ctx context.Context, id ids.ID) (app.InstallationRecord, bool, error) {
	row, err := r.read(ctx).GetInstallation(ctx, uuid(id))
	return one(row, err, func(x sqlcgen.GetInstallationRow) app.InstallationRecord { return toInstallation(x.AppInstallation) },
		"reading an installation")
}

// Candidates implements app.WebhookStore.
func (r *Repository) Candidates(ctx context.Context) ([]app.Candidate, error) {
	rows, err := r.read(ctx).Candidates(ctx)
	if err != nil {
		return nil, mapError(err, "reading the subscription candidates")
	}
	out := make([]app.Candidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.Candidate{
			SubscriptionID: ids.ID(row.ID.Bytes), InstallationID: ids.ID(row.InstallationID.Bytes), AppID: ids.ID(row.AppID.Bytes),
			EventTypes: nonNil(row.EventTypes), State: domain.SubscriptionState(row.State),
			InstallationState: domain.State(row.InstallationState), InstallationEvents: nonNil(row.EventSubscriptions),
		})
	}
	return out, nil
}

// InsertDelivery implements app.WebhookStore.
func (r *Repository) InsertDelivery(ctx context.Context, d app.DeliveryRecord) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.InsertDelivery(ctx, sqlcgen.InsertDeliveryParams{
		ID: uuid(d.ID), SubscriptionID: uuid(d.SubscriptionID), EventSeq: d.EventSeq, EventID: uuid(d.EventID),
		CreatedAt: pgtype.Timestamptz{Time: d.CreatedAt, Valid: true},
	})
	if err != nil {
		return false, mapError(err, "inserting a delivery")
	}
	return n == 1, nil
}

// CountPending implements app.WebhookStore.
func (r *Repository) CountPending(ctx context.Context, subscriptionID ids.ID, bound int) (int, error) {
	b, err := int32Of(bound)
	if err != nil {
		return 0, err
	}
	n, err := r.read(ctx).CountPending(ctx, sqlcgen.CountPendingParams{SubscriptionID: uuid(subscriptionID), Bound: b})
	if err != nil {
		return 0, mapError(err, "counting the pending deliveries")
	}
	return int(n), nil
}

// Head implements app.WebhookStore.
func (r *Repository) Head(ctx context.Context, subscriptionID ids.ID) (app.DeliveryRecord, bool, error) {
	row, err := r.read(ctx).HeadDelivery(ctx, uuid(subscriptionID))
	return one(row, err, toDelivery, "reading the head delivery")
}

// Delivery implements app.WebhookStore.
func (r *Repository) Delivery(ctx context.Context, id ids.ID) (app.DeliveryRecord, bool, error) {
	row, err := r.read(ctx).GetDelivery(ctx, uuid(id))
	return one(row, err, toDelivery, "reading a delivery")
}

// UpdateDelivery implements app.WebhookStore.
func (r *Repository) UpdateDelivery(ctx context.Context, d app.DeliveryRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	attempt, err := int32Of(d.Attempt)
	if err != nil {
		return err
	}
	if d.ResponseCode < 0 || d.ResponseCode > math.MaxInt16 {
		return errs.New(errs.CodeInternalError, "apps: response code outside smallint")
	}
	n, err := q.UpdateDelivery(ctx, sqlcgen.UpdateDeliveryParams{
		ID: uuid(d.ID), Attempt: attempt, Status: d.Status,
		ResponseCode:  pgtype.Int2{Int16: int16(d.ResponseCode), Valid: d.ResponseCode != 0}, // #nosec G115 -- checked above
		LastError:     optionalText(d.LastError),
		NextAttemptAt: timestamp(d.NextAttemptAt), CreatedAt: pgtype.Timestamptz{Time: d.CreatedAt, Valid: true},
		DeliveredAt: timestamp(d.DeliveredAt),
	})
	return affected(n, err, "updating a delivery")
}

// ExpirePending implements app.WebhookStore.
func (r *Repository) ExpirePending(ctx context.Context, subscriptionID ids.ID, belowSeq int64) ([]int64, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	seqs, err := q.ExpirePending(ctx, sqlcgen.ExpirePendingParams{SubscriptionID: uuid(subscriptionID), BelowSeq: belowSeq})
	if err != nil {
		return nil, mapError(err, "expiring pending deliveries")
	}
	return seqs, nil
}

// Deliveries implements app.WebhookStore.
func (r *Repository) Deliveries(ctx context.Context, subscriptionID ids.ID, status string, hasAfter bool, afterSeq int64, afterID ids.ID, limit int) ([]app.DeliveryRecord, error) {
	lim, err := int32Of(limit)
	if err != nil {
		return nil, err
	}
	rows, err := r.read(ctx).ListDeliveries(ctx, sqlcgen.ListDeliveriesParams{
		SubscriptionID: uuid(subscriptionID), Status: status, HasAfter: hasAfter, AfterSeq: afterSeq,
		AfterID: uuid(afterID), Lim: lim,
	})
	if err != nil {
		return nil, mapError(err, "listing deliveries")
	}
	out := make([]app.DeliveryRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDelivery(row))
	}
	return out, nil
}

// toSubscription converts a row of webhook_subscriptions of the app appID.
func toSubscription(s sqlcgen.WebhookSubscription, appID ids.ID) app.SubscriptionRecord {
	return app.SubscriptionRecord{
		ID: ids.ID(s.ID.Bytes), InstallationID: ids.ID(s.AppInstallationID.Bytes), AppID: appID, URL: s.Url,
		EventTypes: nonNil(s.EventTypes), SigningKeyKID: s.SigningKeyKid, HMACEnabled: s.HmacEnabled,
		HMACGeneration: int(s.HmacGeneration), HMACRotatedAt: timeOf(s.HmacRotatedAt), State: domain.SubscriptionState(s.State),
		ConsecutiveFailures: int(s.ConsecutiveFailures), LastDeliveredSeq: s.LastDeliveredSeq, CreatedBy: ids.ID(s.CreatedBy.Bytes),
		CreatedAt: s.CreatedAt.Time, DisabledAt: timeOf(s.DisabledAt),
	}
}

// toDelivery converts a row of webhook_deliveries.
func toDelivery(d sqlcgen.WebhookDelivery) app.DeliveryRecord {
	return app.DeliveryRecord{
		ID: ids.ID(d.ID.Bytes), SubscriptionID: ids.ID(d.SubscriptionID.Bytes), EventID: ids.ID(d.EventID.Bytes), EventSeq: d.EventSeq,
		Attempt: int(d.Attempt), Status: d.Status, ResponseCode: int(d.ResponseCode.Int16), LastError: d.LastError.String,
		NextAttemptAt: timeOf(d.NextAttemptAt), CreatedAt: d.CreatedAt.Time, DeliveredAt: timeOf(d.DeliveredAt),
	}
}

// timestamp is t as nullable timestamptz; the zero time is NULL.
func timestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// timeOf is the time of a nullable timestamptz; NULL is the zero time.
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// int32Of converts n for an integer column; a value outside int32 is internal.
func int32Of(n int) (int32, error) {
	if n < math.MinInt32 || n > math.MaxInt32 {
		return 0, errs.New(errs.CodeInternalError, "apps: value outside integer")
	}
	return int32(n), nil
}
