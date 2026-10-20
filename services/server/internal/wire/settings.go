// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/api/routes"
	authzflags "github.com/OWNER/bothy/services/server/internal/authz/adapters/flagsource"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	identityflags "github.com/OWNER/bothy/services/server/internal/identity/adapters/flagsource"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/loginoptions"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instancehttp "github.com/OWNER/bothy/services/server/internal/instance/adapters/httpapi"
	instancemetrics "github.com/OWNER/bothy/services/server/internal/instance/adapters/otelmetrics"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// keyLocalPasswordEnabled is the identity switch discovery reports (M1-46).
const keyLocalPasswordEnabled = "auth.local_password_enabled" //nolint:gosec // G101: a configuration key name, no credential

// LocalPasswordEnabled returns auth.local_password_enabled of v.
func LocalPasswordEnabled(v *config.Values) bool { return v.Bool(keyLocalPasswordEnabled) }

// InstanceSettings is the settings part of the instance module in a process (M1-46): the use
// cases of discovery, the instance view, the settings, the branding, the problem types and the
// feature flags (M1-47), which also serve the read ports instance.Settings and instance.Flags of
// the other modules, and their REST handlers.
type InstanceSettings struct {
	// Admin implements instance.Admin, instance.Settings and instance.Flags.
	Admin *instanceapp.Admin
	// Handlers are the REST handlers of the six settings operations and the two flag operations.
	Handlers *instancehttp.Admin
	// CSPReports is the handler of submitCspReport (M6-03); NewCSPReports builds it.
	CSPReports *instancehttp.CSPReports
}

// KeyCSPReportMaxRecords is console.csp_report_max_records, the most log records one request of
// submitCspReport writes (M6-03).
const KeyCSPReportMaxRecords = "console.csp_report_max_records"

// NewCSPReports returns the handler of submitCspReport over the Authorizer az, the logger log, the
// counter bothy.console.csp_violations on mp (nil means the global meter provider) and
// maxRecords (console.csp_report_max_records). Errors: the meter provider refuses the counter, az
// is nil or maxRecords is below 1.
func NewCSPReports(az authorize.Authorizer, log *slog.Logger, mp metric.MeterProvider, maxRecords int) (*instancehttp.CSPReports, error) {
	counter, err := instancemetrics.NewCSPViolations(mp)
	if err != nil {
		return nil, err
	}
	reports, err := instanceapp.NewCSPReports(az, log, counter, maxRecords)
	if err != nil {
		return nil, err
	}
	return instancehttp.NewCSPReports(reports), nil
}

// SettingsWrites are the collaborators of the writing settings use cases: the recorder of audit
// and the appender of sync.
type SettingsWrites struct {
	// Audit records the allow entries.
	Audit auditing.Recorder
	// Events appends instance.updated.
	Events eventlog.Appender
}

// Contract returns the values of the API contract the instance use cases report, from the
// generated layer api/routes: the spec version, the gateway protocol versions and the problem
// catalogue x-problem-codes (06 section 5).
func Contract() instanceapp.Contract {
	c := instanceapp.Contract{SpecVersion: routes.SpecVersion, GatewayVersions: routes.GatewayProtocolVersions()}
	for _, p := range routes.ProblemTypes() {
		c.Problems = append(c.Problems, instance.ProblemType{
			Code: p.Code, Status: p.Status, Title: p.Title, Description: p.Description, ClientAction: p.ClientAction,
		})
	}
	return c
}

// NewInstanceSettings builds the settings part over pool with the unit of work uow, the
// Authorizer az, the recorder and appender of w and the clock (nil means clock.System()).
// Discovery learns the sign-in ways from identity through adapters/loginoptions over the
// provider store and localPassword (auth.local_password_enabled); the flag evaluation reaches
// groups and people through authz/adapters/flagsource and identity/adapters/flagsource over their
// repositories. It opens no connection and
// starts nothing. Errors: a configuration value or collaborator the use cases refuse.
func NewInstanceSettings(cfg instanceapp.Config, localPassword bool, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer,
	w SettingsWrites, clk clock.Clock) (*InstanceSettings, error) {
	logins, err := loginoptions.New(identitypg.NewRepository(pool), localPassword)
	if err != nil {
		return nil, err
	}
	groups, err := authzflags.New(authzpg.NewRepository(pool))
	if err != nil {
		return nil, err
	}
	users, err := identityflags.New(identitypg.NewRepository(pool))
	if err != nil {
		return nil, err
	}
	repo := instancepg.NewRepository(pool)
	admin, err := instanceapp.NewAdmin(cfg, instanceapp.AdminDeps{
		Instance: repo, Settings: repo, Logins: logins, UoW: uow, Authorizer: az, Audit: w.Audit, Events: w.Events,
		Flags: repo, FlagGroups: groups, FlagUsers: users, Contract: Contract(), Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	return &InstanceSettings{Admin: admin, Handlers: instancehttp.NewAdmin(admin)}, nil
}

// DefaultInstanceConfig returns InstanceConfig of the schema defaults (no client versions set),
// for tests that build the module by hand instead of loading a configuration;
// TestInstanceConfig_Defaults holds it equal to the schema.
func DefaultInstanceConfig() instanceapp.Config {
	return instanceapp.Config{
		ProductNameMaxChars: 64, MessageMaxContentBytes: 262144, MessageMaxAttachments: 10, FileMaxUploadBytes: 104857600,
		ThreadTitleMaxChars: 200, VoiceMaxDuration: 15 * time.Minute,
	}
}
