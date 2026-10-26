// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// PersonalData is the participant of apps in the privacy requests (M7-06,
// docs/spec/20-privacy.md sections 2, 4.4 and 5.2). It exports the app installations the person
// made into the section apps_installed of access.json. It erases nothing: apps, installations,
// webhook subscriptions and incoming webhooks stay with the pseudonymous ID of the person who
// installed or created them (20 section 2: "stays", "on revocation"); they belong to the instance,
// not to the person, and keep working for the others.
type PersonalData struct {
	pool *pg.Pool
}

// Compile-time check: PersonalData is a participant of kernel/personaldata.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant over pool, the pool of the role bothy_app.
func NewPersonalData(pool *pg.Pool) *PersonalData { return &PersonalData{pool: pool} }

// Module returns "apps".
func (*PersonalData) Module() string { return "apps" }

// Tables returns the tables of apps with personal data of 20 section 2; webhook_deliveries has
// none.
func (*PersonalData) Tables() []string {
	return []string{"apps", "app_installations", "webhook_subscriptions", "incoming_webhooks"}
}

// installation is one entry of access.json apps_installed
// (docs/api/gdpr-export/v1/sections/access.apps_installed.schema.json).
type installation struct {
	ID            ids.ID    `json:"id"`
	AppID         ids.ID    `json:"app_id"`
	State         string    `json:"state"`
	InstalledAt   time.Time `json:"installed_at"`
	UninstalledAt time.Time `json:"uninstalled_at,omitzero"`
}

// Export implements personaldata.Participant: the section apps_installed of access.json, an
// empty array when the person installed no app. Errors: those of the storage and of w.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	var rows []sqlcgen.PersonalDataInstallationsRow
	err := pg.ReadSnapshot(ctx, p.pool, func(ctx context.Context, q pg.DBTX) error {
		var err error
		rows, err = sqlcgen.New(q).PersonalDataInstallations(ctx, pg.UUID(e.Subject))
		return err
	})
	if err != nil {
		return err
	}
	out := make([]installation, 0, len(rows))
	for _, r := range rows {
		out = append(out, installation{
			ID: pg.IDOf(r.ID), AppID: pg.IDOf(r.AppID), State: r.State, InstalledAt: pg.TimeOf(r.InstalledAt),
			UninstalledAt: pg.TimeOf(r.UninstalledAt),
		})
	}
	return w.Section(personaldata.FileAccess, "apps_installed", out)
}

// Erase implements personaldata.Participant and changes nothing in any step (see PersonalData).
func (*PersonalData) Erase(context.Context, personaldata.Step, personaldata.Erasure) error {
	return nil
}
