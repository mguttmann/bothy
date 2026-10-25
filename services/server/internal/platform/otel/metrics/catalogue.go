// SPDX-License-Identifier: Apache-2.0

package metrics

import "slices"

// The bucket rows of signals.md 4.7. Every SLO threshold is one of these bounds (R-OB-06).
var (
	// BucketsSeconds is the row "durations up to seconds", the recommendation of the semantic
	// conventions for http.server.request.duration.
	BucketsSeconds = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
	// BucketsLong is the row "long durations".
	BucketsLong = []float64{1, 2.5, 5, 10, 30, 60, 120, 300, 600, 1800, 3600, 7200}
	// BucketsSizes is the row "sizes" (bothy.file.upload.size, M3-03).
	BucketsSizes = []float64{1024, 16384, 131072, 1048576, 8388608, 67108864, 536870912}
)

// LibraryOtelHTTP is the instrumentation scope of otelhttp
// (go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp); TestViews_BucketBounds holds
// it equal to otelhttp.ScopeName.
const LibraryOtelHTTP = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// LibraryOtelPGX is the instrumentation scope of otelpgx. Its query metric carries the raw first
// word of a statement and pgx.operation.type, its pool metrics a pool name with host and port,
// so View drops every instrument of the scope and platform/pg records
// db.client.operation.duration itself (docs/DECISIONS.md M1-68).
const LibraryOtelPGX = "github.com/exaring/otelpgx"

// droppedScopes are the instrumentation scopes whose instruments never leave the process.
var droppedScopes = map[string]bool{LibraryOtelPGX: true}

// The values failed and cancelled of bothy.outcome and bothy.run.state.
const (
	valueFailed    = "failed"
	valueCancelled = "cancelled"
)

// The reasons access_revoked and user_disabled, which runs, invocations and terminal sessions
// share in bothy.reason.
const (
	reasonAccessRevoked = "access_revoked"
	reasonUserDisabled  = "user_disabled"
)

// Attribute keys and units several instruments share.
const (
	keyReason        = "bothy.reason"
	keyOutcome       = "bothy.outcome"
	keyPrincipalKind = "bothy.principal.kind"
	keyAgentHandle   = "bothy.agent.handle"
	keyBackupKind    = "bothy.backup.kind"
	keyVolume        = "bothy.volume"
	keyRunState      = "bothy.run.state"
	keyPushProvider  = "bothy.push.provider"
	unitNotification = "{notification}"
	unitConnection   = "{connection}"
	unitRun          = "{run}"
	unitSession      = "{session}"
	unitFile         = "{file}"
)

// Closed literal sets shared by several instruments.
var (
	// principalKinds are bothy.principal.kind of an identified connection.
	principalKinds = []string{"user", "agent", "app"}
	// httpMethods are http.request.method: the methods of the HTTP semantic conventions and
	// _OTHER for every other method (platform/otel Middleware).
	// scanOutcomes are bothy.outcome of bothy.file.scan and bothy.file.scan.duration.
	scanOutcomes = []string{"clean", "infected", "error", "skipped"}
	// searchOutcomes are bothy.outcome of bothy.search.duration (M3-14): ok, client_error for any
	// 4xx, timeout for search.statement_timeout (503 search_timeout), server_error otherwise.
	searchOutcomes = []string{"ok", "client_error", "timeout", "server_error"}
	httpMethods    = []string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH", "_OTHER"}
	// backupKinds are bothy.backup.kind of the last success: pg_full and pg_diff both record
	// physical (M7-02).
	backupKinds = []string{"physical", "logical", "objects"}
	// volumes are bothy.volume: the volumes bothy-backupd measures (signals.md 4.6).
	volumes = []string{"pgdata", "files", "pgbackrest-repo", "backup-scratch"}
	// pushProviders are bothy.push.provider: the values of push_registrations.provider (03,
	// signals.md 5.3).
	pushProviders = []string{"apns", "fcm", "unifiedpush", "wns"}
	// runEndStates are bothy.run.state of an ended run (09 5.1).
	runEndStates = []string{"completed", valueFailed, valueCancelled}
	// runReasons are bothy.reason of bothy.run.finished: the codes of runs.state_reason, the 18
	// rows of 09 5.2 (user_disabled included, 02 4.3, D-M2-05-7). The platform cannot import the runs
	// domain, so the adapter test holds this list equal to domain's reason table.
	runReasons = []string{
		"user_cancelled", reasonAccessRevoked, "cancel_timeout", "agent_disabled", "billing_mode_personal_not_holder",
		"channel_read_only", "billing_mode_mismatch", "tool_class_owner_only", "tool_class_mismatch",
		"invalid_invocation_token", "project_not_ready", "agent_unavailable", "interrupted", "stale",
		"runtime_error", "adapter_error", "runtime_not_authenticated", reasonUserDisabled,
	}
	// restoreChecks are bothy.reason of bothy.backup.restore_test.failures: the seven checks of
	// docs/spec/16-operations.md 8.3 as internal/status/domain names them (M7-03).
	restoreChecks = []string{"pgbackrest", "manifest", "event_log", "audit_chain", "thread_sample", "file_sample", "duration"}
)

// The instruments of signals.md that this build registers. Every instrument the server creates
// comes from one of these declarations; TestCatalogue_RegistryMatchesSignalsDoc holds them equal
// to the catalogue.
var (
	// HTTPServerRequestDuration is http.server.request.duration (4.1), created by otelhttp;
	// http.route is the route template or "unmatched".
	HTTPServerRequestDuration = Spec{
		Name: "http.server.request.duration", Kind: KindHistogram, Unit: "s", Library: LibraryOtelHTTP,
		Description: "Duration of HTTP server requests.",
		Attrs: []Attr{
			{Key: "http.request.method", Values: httpMethods},
			{Key: "http.route", Source: SourceRoutes},
			{Key: "http.response.status_code", Source: SourceHTTPStatus},
			{Key: "error.type", Source: SourceHTTPStatus, Optional: true},
		},
		Buckets: BucketsSeconds,
	}
	// AuthzDecisions is bothy.authz.decisions (4.1): decisions for requests and ops; checks in
	// filter mode (lists, delivery) are not counted.
	AuthzDecisions = Spec{
		Name: "bothy.authz.decisions", Kind: KindCounter, Unit: "{decision}",
		Description: "Authorization decisions for requests and ops.",
		Attrs: []Attr{
			{Key: "bothy.action", Source: SourceActions},
			{Key: "bothy.decision", Values: []string{"allow", "not_found", "forbidden", "step_up_required", "invariant_violation", "unauthenticated"}},
		},
	}
	// AuthzErrors is bothy.authz.errors (4.1): programming errors of the rights check.
	AuthzErrors = Spec{
		Name: "bothy.authz.errors", Kind: KindCounter, Unit: "{check}",
		Description: "Programming errors of the rights check.",
		Attrs:       []Attr{{Key: keyReason, Values: []string{"unknown_action", "authz_not_checked", "filter_mismatch"}}},
	}
	// RateLimitRejected is bothy.ratelimit.rejected (4.1).
	RateLimitRejected = Spec{
		Name: "bothy.ratelimit.rejected", Kind: KindCounter, Unit: "{request}",
		Description: "Requests refused because their rate-limit class was exhausted.",
		Attrs:       []Attr{{Key: "bothy.ratelimit.class", Source: SourceRateClasses}},
	}
	// ConsoleCSPViolations is bothy.console.csp_violations (4.1, M6-03): accepted violation
	// reports of the web console (submitCspReport). The directive set is the fixed list of the
	// report handler of the instance module; every other value records as other.
	ConsoleCSPViolations = Spec{
		Name: "bothy.console.csp_violations", Kind: KindCounter, Unit: "{report}",
		Description: "CSP violation reports of the web console the server accepted.",
		Attrs: []Attr{
			{Key: "bothy.csp.directive", Values: []string{
				"base-uri", "child-src", "connect-src", "default-src", "fenced-frame-src", "font-src", "form-action",
				"frame-ancestors", "frame-src", "img-src", "manifest-src", "media-src", "object-src",
				"require-trusted-types-for", "sandbox", "script-src", "script-src-attr", "script-src-elem", "style-src",
				"style-src-attr", "style-src-elem", "trusted-types", "webrtc", "worker-src"}},
			{Key: "bothy.csp.disposition", Values: []string{"enforce", "report"}},
		},
	}
	// GatewayConnectionsActive is bothy.gateway.connections.active (4.1): +1 at ready or
	// resumed, -1 when such a connection closes.
	GatewayConnectionsActive = Spec{
		Name: "bothy.gateway.connections.active", Kind: KindUpDownCounter, Unit: unitConnection,
		Description: "Gateway connections that reached ready or resumed and are still open.",
		Attrs:       []Attr{{Key: keyPrincipalKind, Values: principalKinds}},
	}
	// GatewayConnectDuration is bothy.gateway.connect.duration (4.1): upgrade to ready or
	// resumed, or to the close before it.
	GatewayConnectDuration = Spec{
		Name: "bothy.gateway.connect.duration", Kind: KindHistogram, Unit: "s",
		Description: "Time from the WebSocket upgrade to ready or resumed, or to the close before it.",
		Attrs: []Attr{
			{Key: keyOutcome, Values: []string{"ok", "client_error", "server_error"}},
			{Key: keyPrincipalKind, Values: append(slices.Clone(principalKinds), "anonymous")},
		},
		Buckets: BucketsSeconds,
	}
	// GatewayCatchupDuration is bothy.gateway.catchup.duration (4.1).
	GatewayCatchupDuration = Spec{
		Name: "bothy.gateway.catchup.duration", Kind: KindHistogram, Unit: "s",
		Description: "Time from ready or resumed to catchup.complete or resync_required.",
		Attrs:       []Attr{{Key: keyOutcome, Values: []string{"complete", "resync_required"}}},
		Buckets:     BucketsLong,
	}
	// GatewayResync is bothy.gateway.resync (4.1): the reasons of 04 "resync_required".
	GatewayResync = Spec{
		Name: "bothy.gateway.resync", Kind: KindCounter, Unit: unitConnection,
		Description: "Connections told to resynchronise instead of catching up.",
		Attrs:       []Attr{{Key: keyReason, Values: []string{"retention", "too_far_behind", "cursor_ahead", "epoch_changed"}}},
	}
	// GatewayClose is bothy.gateway.close (4.1): every close in both directions.
	GatewayClose = Spec{
		Name: "bothy.gateway.close", Kind: KindCounter, Unit: unitConnection,
		Description: "Closes of gateway connections by close code, in both directions.",
		Attrs:       []Attr{{Key: "bothy.close.code", Source: SourceCloseCodes}},
	}
	// GatewayOutboundDropped is bothy.gateway.outbound.dropped (4.1).
	GatewayOutboundDropped = Spec{
		Name: "bothy.gateway.outbound.dropped", Kind: KindCounter, Unit: "{frame}",
		Description: "Outbound frames the gateway discarded.",
		Attrs:       []Attr{{Key: keyReason, Values: []string{"patch_replaced_by_snapshot", "slow_consumer"}}},
	}
	// GatewayPresenceWriteFailures is bothy.gateway.presence.write_failures (4.1, M7-17): failed
	// writes of gateway_presence by an api node; the next flush writes again.
	GatewayPresenceWriteFailures = Spec{
		Name: "bothy.gateway.presence.write_failures", Kind: KindCounter, Unit: "{write}",
		Description: "Failed writes of the shared presence state gateway_presence.",
	}
	// MessageCreated is bothy.message.created (4.2), counted after commit.
	MessageCreated = Spec{
		Name: "bothy.message.created", Kind: KindCounter, Unit: "{message}",
		Description: "Messages created, counted after the commit.",
		Attrs: []Attr{
			{Key: "bothy.message.origin", Values: []string{"human", "agent", "app", "import", "system"}},
			{Key: "bothy.message.kind", Values: []string{"text", "voice", "run", "card", "system"}},
		},
	}
	// EventAppended is bothy.event.appended (4.2), counted after commit per durable type.
	EventAppended = Spec{
		Name: "bothy.event.appended", Kind: KindCounter, Unit: "{event}",
		Description: "Durable events appended to the event log, counted after the commit.",
		Attrs:       []Attr{{Key: "bothy.event.type", Source: SourceEventTypes}},
	}
	// EventSeqLockWait is bothy.db.event_seq.lock_wait (4.2): the UPDATE of the event_seq row.
	EventSeqLockWait = Spec{
		Name: "bothy.db.event_seq.lock_wait", Kind: KindHistogram, Unit: "s",
		Description: "Duration of the statement that locks and advances the event_seq counter row.",
		Buckets:     BucketsSeconds,
	}
	// EventFanoutLag is bothy.event.fanout.lag (4.2).
	EventFanoutLag = Spec{
		Name: "bothy.event.fanout.lag", Kind: KindGauge, Unit: "s", Float: true,
		Description: "Age of the oldest event the log tail knows of and the hub has not distributed yet; 0 when idle.",
	}
	// EventConsumerLag is bothy.event.consumer.lag (4.2, M2-01).
	EventConsumerLag = Spec{
		Name: "bothy.event.consumer.lag", Kind: KindGauge, Unit: "{event}",
		Description: "Events in the log after the cursor of a worker consumer.",
		Attrs:       []Attr{{Key: "bothy.consumer.name", Values: []string{"push", "webhooks"}}},
	}
	// EventConsumerPendingAge is bothy.event.consumer.pending_age (4.2, M2-01).
	EventConsumerPendingAge = Spec{
		Name: "bothy.event.consumer.pending_age", Kind: KindGauge, Unit: "s", Float: true,
		Description: "Age of the oldest event a worker consumer has not processed.",
		Attrs:       []Attr{{Key: "bothy.consumer.name", Values: []string{"push", "webhooks"}}},
	}
	// AgentHeartbeatAge is bothy.agent.heartbeat.age (4.3, M2-20): now minus the last heartbeat of
	// every active agent the tracker of an api process holds.
	AgentHeartbeatAge = Spec{
		Name: "bothy.agent.heartbeat.age", Kind: KindGauge, Unit: "s", Float: true,
		Description: "Seconds since the last heartbeat of an active agent.",
		Attrs:       []Attr{{Key: keyAgentHandle, Source: SourceAgentHandles}},
	}
	// AgentBillingModeMismatchAge is bothy.agent.billing_mode.mismatch_age (4.3, M2-20): seconds
	// since a difference between configured and reported mode began according to the heartbeat;
	// 0 without one. BothyBillingModeMismatch fires on it after billing.mismatch_alert_after.
	AgentBillingModeMismatchAge = Spec{
		Name: "bothy.agent.billing_mode.mismatch_age", Kind: KindGauge, Unit: "s", Float: true,
		Description: "Seconds since the start of a mismatch between configured and reported billing mode; 0 without one.",
		Attrs:       []Attr{{Key: keyAgentHandle, Source: SourceAgentHandles}},
	}
	// DeliveryDuration is bothy.delivery.duration (4.2): occurred_at to the successful write,
	// live delivery only.
	DeliveryDuration = Spec{
		Name: "bothy.delivery.duration", Kind: KindHistogram, Unit: "s",
		Description: "Time from occurred_at of an event to the write of its frame, live delivery only.",
		Attrs: []Attr{
			{Key: "bothy.event.durability", Values: []string{"durable", "ephemeral"}},
			{Key: keyPrincipalKind, Values: principalKinds},
		},
		Buckets: BucketsSeconds,
	}
	// FileUploadSize is bothy.file.upload.size (4.4): the declared size of every completed
	// upload, recorded after the commit of completeUpload; no attributes (M3-03).
	FileUploadSize = Spec{
		Name: "bothy.file.upload.size", Kind: KindHistogram, Unit: "By",
		Description: "Declared size of completed uploads.",
		Buckets:     BucketsSizes,
	}
	// FileScan is bothy.file.scan (4.4): one per final scan outcome, recorded after the commit of
	// the scan pass; failed files and retries are not counted (M3-04).
	FileScan = Spec{
		Name: "bothy.file.scan", Kind: KindCounter, Unit: unitFile,
		Description: "Final outcomes of the virus scan of uploaded files.",
		Attrs:       []Attr{{Key: keyOutcome, Values: scanOutcomes}},
	}
	// FileScanDuration is bothy.file.scan.duration (4.4): the whole read pass with the scan, per
	// outcome (docs/DECISIONS.md D-M3-04-12).
	FileScanDuration = Spec{
		Name: "bothy.file.scan.duration", Kind: KindHistogram, Unit: "s",
		Description: "Duration of the read pass with the virus scan of one file.",
		Attrs:       []Attr{{Key: keyOutcome, Values: scanOutcomes}},
		Buckets:     BucketsLong,
	}
	// FileScanQueue is bothy.file.scan.queue (4.4): files awaiting the scan pass, recorded by the
	// health monitor of the worker every file.scan.health_interval.
	FileScanQueue = Spec{
		Name: "bothy.file.scan.queue", Kind: KindGauge, Unit: unitFile,
		Description: "Files awaiting the read pass with the virus scan.",
	}
	// FilePurgeBacklog is bothy.file.purge.backlog (4.4, M3-08): the files marked deleted whose
	// objects and row are not purged yet, recorded at the end of every run of the job
	// files.cleanup_pending (12 section 14 "deletion backlog"); before the first run it reports
	// nothing.
	FilePurgeBacklog = Spec{
		Name: "bothy.file.purge.backlog", Kind: KindGauge, Unit: unitFile,
		Description: "Files marked deleted whose objects and row are not purged yet.",
	}
	// RetentionDeleted is bothy.retention.deleted (4.4, M7-05): the rows one run of a job
	// retention.<class> removed or emptied, by class; never an ID or a table name.
	RetentionDeleted = Spec{
		Name: "bothy.retention.deleted", Kind: KindCounter, Unit: "{row}",
		Description: "Rows the retention jobs removed or emptied, by retention class.",
		Attrs:       []Attr{{Key: "bothy.retention.class", Values: []string{"security", "content", "transient", "audit", "cost"}}},
	}
	// PushSent is bothy.push.sent (4.4, M3-19): one per provider answer of the job
	// notify.push_send, with the outcome of its result class (13 7.3, D-M3-19-12).
	PushSent = Spec{
		Name: "bothy.push.sent", Kind: KindCounter, Unit: unitNotification,
		Description: "Push requests answered by a provider, by outcome.",
		Attrs: []Attr{
			{Key: keyPushProvider, Values: pushProviders},
			{Key: keyOutcome, Values: []string{"accepted", "rejected_token", "rejected_other", "throttled", "provider_error"}},
		},
	}
	// PushSendDuration is bothy.push.send.duration (4.4, M3-19): one provider request of the job
	// notify.push_send, outside any transaction.
	PushSendDuration = Spec{
		Name: "bothy.push.send.duration", Kind: KindHistogram, Unit: "s",
		Description: "Duration of one push request to a provider.",
		Attrs:       []Attr{{Key: keyPushProvider, Values: pushProviders}},
		Buckets:     BucketsSeconds,
	}
	// SearchDuration is bothy.search.duration (4.4, M3-14): one full-text search (operation search)
	// from the start of the use case to its end, per outcome (docs/DECISIONS.md D-M3-14-13). The
	// prefix search of the quick switcher is measured by http.server.request.duration only.
	SearchDuration = Spec{
		Name: "bothy.search.duration", Kind: KindHistogram, Unit: "s",
		Description: "Duration of one full-text search.",
		Attrs:       []Attr{{Key: keyOutcome, Values: searchOutcomes}},
		Buckets:     BucketsSeconds,
	}
	// PushSkipped is bothy.push.skipped (4.4, M3-19): one per recipient and hint decided none and
	// one per job notify.push_send that ends without sending (D-M3-19-13). Quiet hours make a hint
	// quiet and never drop it (R-PN-05), so they are no reason here.
	PushSkipped = Spec{
		Name: "bothy.push.skipped", Kind: KindCounter, Unit: unitNotification,
		Description: "Hints and push jobs that ended without a push, by reason.",
		Attrs: []Attr{{Key: keyReason, Values: []string{
			"no_access", "preference", "active_in_thread", "no_registration", "read", "expired", "no_provider"}}},
	}
	// PushFetch is bothy.push.fetch (4.4, M3-22): one per preview request (getPushPreview) whose
	// preview token identified an active registration, whatever the answer after that; a malformed
	// or unknown token counts nothing. It is the receipt t2 of 13 section 9.
	PushFetch = Spec{
		Name: "bothy.push.fetch", Kind: KindCounter, Unit: unitNotification,
		Description: "Preview fetches with the token of an active push registration.",
		Attrs:       []Attr{{Key: keyPushProvider, Values: pushProviders}},
	}
	// PushRegistrationsActive is bothy.push.registrations.active (4.4, M3-19): the rows of
	// push_registrations with disabled_at IS NULL per provider, observed at each collection.
	PushRegistrationsActive = Spec{
		Name: "bothy.push.registrations.active", Kind: KindGauge, Unit: "{registration}",
		Description: "Active push registrations per provider.",
		Attrs:       []Attr{{Key: keyPushProvider, Values: pushProviders}},
	}
	// JobDuration is bothy.job.duration (4.4).
	JobDuration = Spec{
		Name: "bothy.job.duration", Kind: KindHistogram, Unit: "s",
		Description: "Duration of one job attempt.",
		Attrs: []Attr{
			{Key: "bothy.job.kind", Source: SourceJobKinds},
			{Key: keyOutcome, Values: []string{"completed", valueFailed, valueCancelled, "snoozed"}},
		},
		Buckets: BucketsLong,
	}
	// JobQueue is bothy.job.queue (4.4): jobs per kind in the states available, scheduled and
	// retryable.
	JobQueue = Spec{
		Name: "bothy.job.queue", Kind: KindGauge, Unit: "{job}",
		Description: "Jobs waiting to run: states available, scheduled and retryable.",
		Attrs:       []Attr{{Key: "bothy.job.kind", Source: SourceJobKinds}},
	}
	// EgressBlocked is bothy.egress.blocked (4.4).
	EgressBlocked = Spec{
		Name: "bothy.egress.blocked", Kind: KindCounter, Unit: "{request}",
		Description: "Connections the egress client refused.",
		Attrs: []Attr{{Key: keyReason, Values: []string{
			"url_not_allowed", "scheme_not_allowed", "host_not_allowed", "address_blocked", "redirect_not_allowed"}}},
	}
	// DBClientOperationDuration is db.client.operation.duration (4.5), recorded by the query
	// tracer of platform/pg.
	DBClientOperationDuration = Spec{
		Name: "db.client.operation.duration", Kind: KindHistogram, Unit: "s",
		Description: "Duration of database client operations.",
		Attrs: []Attr{
			{Key: "db.operation.name", Values: []string{
				"SELECT", "INSERT", "UPDATE", "DELETE", "MERGE", "COPY", "CALL", "BEGIN", "COMMIT", "ROLLBACK", "SET", "LISTEN", "UNLISTEN"}},
			{Key: "db.collection.name", Source: SourceTables, Optional: true},
		},
		Buckets: BucketsSeconds,
	}
	// DBPoolConnections is bothy.db.pool.connections (4.5).
	DBPoolConnections = Spec{
		Name: "bothy.db.pool.connections", Kind: KindGauge, Unit: unitConnection,
		Description: "Connections of the database pool by state.",
		Attrs:       []Attr{{Key: "bothy.pool.state", Values: []string{"idle", "acquired", "max"}}},
	}
	// AuditVerifyFailures is bothy.audit.verify.failures (4.6, M1-50).
	AuditVerifyFailures = Spec{
		Name: "bothy.audit.verify.failures", Kind: KindCounter, Unit: "{run}",
		Description: "Runs of the periodic audit chain verification that found a broken chain.",
	}
	// AuditVerifyLastSuccess is bothy.audit.verify.last_success.timestamp (4.6, M1-50).
	AuditVerifyLastSuccess = Spec{
		Name: "bothy.audit.verify.last_success.timestamp", Kind: KindGauge, Unit: "s", Float: true,
		Description: "Unix time of the last successful audit chain verification.",
	}
	// BackupLastSuccess is bothy.backup.last_success.timestamp (4.6, M7-02): the Unix time of
	// the last successful backup run per kind, recorded by bothy-backupd.
	BackupLastSuccess = Spec{
		Name: "bothy.backup.last_success.timestamp", Kind: KindGauge, Unit: "s", Float: true,
		Description: "Unix time of the last successful backup run of a kind.",
		Attrs:       []Attr{{Key: keyBackupKind, Values: backupKinds}},
	}
	// BackupWALLastArchived is bothy.backup.wal.last_archived.timestamp (4.6, M7-02):
	// pg_stat_archiver.last_archived_time as Unix time.
	BackupWALLastArchived = Spec{
		Name: "bothy.backup.wal.last_archived.timestamp", Kind: KindGauge, Unit: "s", Float: true,
		Description: "Unix time the last WAL segment was archived.",
	}
	// BackupDuration is bothy.backup.duration (4.6, M7-02): the duration of the last run per
	// kind. The value restore_test is recorded by the restore test (M7-03); the column lists no
	// values, so the closed set is the one of 4.6 bothy.backup.last_success.timestamp plus it.
	BackupDuration = Spec{
		Name: "bothy.backup.duration", Kind: KindGauge, Unit: "s", Float: true,
		Description: "Duration of the last backup run of a kind.",
		Attrs:       []Attr{{Key: keyBackupKind, Values: append(slices.Clone(backupKinds), "restore_test")}},
	}
	// BackupSize is bothy.backup.size (4.6, M7-02): the bytes the last run of a kind added.
	BackupSize = Spec{
		Name: "bothy.backup.size", Kind: KindGauge, Unit: "By",
		Description: "Bytes the last backup run of a kind added to its repository.",
		Attrs:       []Attr{{Key: keyBackupKind, Values: append(slices.Clone(backupKinds), "restore_test")}},
	}
	// BackupRestoreTestLastSuccess is bothy.backup.restore_test.last_success.timestamp (4.6,
	// M7-03): the Unix time of the last restore test in which no check failed; the alert
	// BothyRestoreTestStale reads it.
	BackupRestoreTestLastSuccess = Spec{
		Name: "bothy.backup.restore_test.last_success.timestamp", Kind: KindGauge, Unit: "s", Float: true,
		Description: "Unix time of the last successful automatic restore test.",
	}
	// BackupRestoreTestDuration is bothy.backup.restore_test.duration (4.6, M7-03): the duration
	// of the last restore test, the value check 7 compares with dr.rto_target.
	BackupRestoreTestDuration = Spec{
		Name: "bothy.backup.restore_test.duration", Kind: KindGauge, Unit: "s", Float: true,
		Description: "Duration of the last automatic restore test.",
	}
	// BackupRestoreTestFailures is bothy.backup.restore_test.failures (4.6, M7-03): one per failed
	// check of a restore test, bothy.reason the name of the check (docs/spec/16-operations.md 8.3).
	BackupRestoreTestFailures = Spec{
		Name: "bothy.backup.restore_test.failures", Kind: KindCounter, Unit: "{check}",
		Description: "Failed checks of automatic restore tests.",
		Attrs:       []Attr{{Key: keyReason, Values: restoreChecks}},
	}
	// StorageVolumeFree is bothy.storage.volume.free (4.6, M7-02): the bytes an unprivileged
	// user may still write on a volume.
	StorageVolumeFree = Spec{
		Name: "bothy.storage.volume.free", Kind: KindGauge, Unit: "By",
		Description: "Free bytes of a volume.",
		Attrs:       []Attr{{Key: keyVolume, Values: volumes}},
	}
	// StorageVolumeSize is bothy.storage.volume.size (4.6, M7-02): the total bytes of a volume.
	StorageVolumeSize = Spec{
		Name: "bothy.storage.volume.size", Kind: KindGauge, Unit: "By",
		Description: "Total bytes of a volume.",
		Attrs:       []Attr{{Key: keyVolume, Values: volumes}},
	}
	// TelemetryDropped is bothy.telemetry.dropped (4.6, M1-10).
	TelemetryDropped = Spec{
		Name: "bothy.telemetry.dropped", Kind: KindCounter, Unit: "{item}",
		Description: "Telemetry items that never reached the collector.",
		Attrs:       []Attr{{Key: "bothy.signal", Values: []string{"traces", "metrics", "logs"}}},
	}
	// RunActive is bothy.run.active (4.3, M2-05): the active runs per agent handle and state,
	// observed per collection from the runs table.
	RunActive = Spec{
		Name: "bothy.run.active", Kind: KindGauge, Unit: unitRun,
		Description: "Runs in the states queued, running and waiting_input.",
		Attrs: []Attr{
			{Key: keyAgentHandle, Source: SourceAgentHandles},
			{Key: keyRunState, Values: []string{"queued", "running", "waiting_input"}},
		},
	}
	// RunStartLatency is bothy.run.start.latency (4.3, M2-05): queued_at until the run left
	// queued, per outcome and billing mode (the reported mode, else the expected one).
	RunStartLatency = Spec{
		Name: "bothy.run.start.latency", Kind: KindHistogram, Unit: "s",
		Description: "Time from queued_at until a run leaves the state queued.",
		Attrs: []Attr{
			{Key: keyAgentHandle, Source: SourceAgentHandles},
			{Key: keyOutcome, Values: []string{"started", valueFailed, valueCancelled}},
			{Key: "bothy.billing_mode", Values: []string{"personal", "organization"}},
		},
		Buckets: BucketsSeconds,
	}
	// RunDuration is bothy.run.duration (4.3, M2-05): started_at until the end of a started run.
	RunDuration = Spec{
		Name: "bothy.run.duration", Kind: KindHistogram, Unit: "s",
		Description: "Time from started_at until a started run ends.",
		Attrs: []Attr{
			{Key: keyAgentHandle, Source: SourceAgentHandles},
			{Key: keyRunState, Values: runEndStates},
		},
		Buckets: BucketsLong,
	}
	// RunSteps is bothy.run.steps (4.3, M2-10): one per step an adapter added through a stored
	// live-box patch, after the commit; the summary steps of the server are not counted.
	RunSteps = Spec{
		Name: "bothy.run.steps", Kind: KindCounter, Unit: "{step}",
		Description: "Steps adapters added to runs.",
		Attrs: []Attr{
			{Key: keyAgentHandle, Source: SourceAgentHandles},
			{Key: "bothy.step.kind", Values: []string{"tool_call", "status", "subagent", "workflow", "background_job"}},
		},
	}
	// RunFinished is bothy.run.finished (4.3, M2-05): one per run that reached an end state.
	// bothy.reason is absent for completed, which has no state_reason.
	RunFinished = Spec{
		Name: "bothy.run.finished", Kind: KindCounter, Unit: unitRun,
		Description: "Runs that reached an end state.",
		Attrs: []Attr{
			{Key: keyAgentHandle, Source: SourceAgentHandles},
			{Key: keyRunState, Values: runEndStates},
			{Key: keyReason, Values: runReasons, Optional: true},
		},
	}
	// TerminalRelayUnpaired is bothy.terminal.relay.unpaired (4.3a, M2-44): a side of the relay
	// that found no counterpart before terminal.open_timeout, once per connection.
	TerminalRelayUnpaired = Spec{
		Name: "bothy.terminal.relay.unpaired", Kind: KindCounter, Unit: unitConnection,
		Description: "Relay connections whose counterpart did not arrive before terminal.open_timeout.",
		Attrs:       []Attr{{Key: "bothy.terminal.side", Values: []string{"client", "host"}}},
	}
	// RunCost is bothy.run.cost (4.3, M2-22): the amount of each booked cost entry in currency
	// units (amount_micros divided by one million), counted after the commit of the unit of work
	// that booked it. bothy.currency is the ISO 4217 code of the entry (SourceCurrencies).
	RunCost = Spec{
		Name: "bothy.run.cost", Kind: KindCounter, Unit: "{currency_unit}", Float: true,
		Description: "Booked run costs in units of the currency.",
		Attrs: []Attr{
			{Key: keyAgentHandle, Source: SourceAgentHandles},
			{Key: "bothy.billing_mode", Values: []string{"personal", "organization"}},
			{Key: "bothy.currency", Source: SourceCurrencies},
			{Key: "bothy.cost.estimate", Values: []string{"true", "false"}},
		},
	}
	// StatsFlushDropped is bothy.stats.flush.dropped (4.3b, M6-13): counter keys of the own
	// measurement a process discarded: the oldest beyond stats.flush_queue_max (queue_full),
	// counters of a failed write that could not be kept (db_error), or a Record call outside the
	// closed lists of docs/spec/25-visualization.md 3.2 (invalid_key).
	StatsFlushDropped = Spec{
		Name: "bothy.stats.flush.dropped", Kind: KindCounter, Unit: "{key}",
		Description: "Counter keys of the own measurement that were discarded.",
		Attrs:       []Attr{{Key: keyReason, Values: []string{"queue_full", "db_error", "invalid_key"}}},
	}
	// StatsFlushDuration is bothy.stats.flush.duration (4.3b, M6-13): one flush of the own
	// measurement to stats_minutes.
	StatsFlushDuration = Spec{
		Name: "bothy.stats.flush.duration", Kind: KindHistogram, Unit: "s",
		Description: "Duration of one flush of the own measurement to stats_minutes.",
		Attrs:       []Attr{{Key: keyOutcome, Values: []string{"ok", "error"}}},
		Buckets:     BucketsSeconds,
	}
	// StatsLiveSubscribers is bothy.stats.live.subscribers (4.3b, M6-15): the subscriptions to
	// stats.live this api node holds (at most stats.live_max_subscribers).
	StatsLiveSubscribers = Spec{
		Name: "bothy.stats.live.subscribers", Kind: KindUpDownCounter, Unit: "{subscription}",
		Description: "Subscriptions to stats.live on this node.",
	}
	// AgentBillingModeMismatch is bothy.agent.billing_mode_mismatch (4.3, M2-09): one per run that
	// an I3 mismatch failed, found by the server at the start or reported by the adapter.
	AgentBillingModeMismatch = Spec{
		Name: "bothy.agent.billing_mode_mismatch", Kind: KindCounter, Unit: "{run}",
		Description: "Runs failed by a mismatch of the billing mode or the tool class (I3).",
		Attrs:       []Attr{{Key: keyAgentHandle, Source: SourceAgentHandles}},
	}
	// AgentInvocationDenied is bothy.agent.invocation.denied (4.3, M2-08): one per denial of
	// agent.invoke immediately before a delivery to an agent (I2), with the run reason the denial
	// maps to (09 5.2; the adapter test holds the values equal to the runs domain).
	AgentInvocationDenied = Spec{
		Name: "bothy.agent.invocation.denied", Kind: KindCounter, Unit: "{invocation}",
		Description: "Denials of agent.invoke immediately before a delivery to an agent.",
		Attrs: []Attr{
			{Key: keyAgentHandle, Source: SourceAgentHandles},
			{Key: keyReason, Values: []string{
				"billing_mode_personal_not_holder", "tool_class_owner_only", "project_not_ready", "channel_read_only",
				reasonUserDisabled, "agent_disabled", reasonAccessRevoked,
			}},
		},
	}
	// TerminalSessionsActive is bothy.terminal.sessions.active (4.3a, M2-46): the sessions the
	// relay of this api process holds, by state.
	TerminalSessionsActive = Spec{
		Name: "bothy.terminal.sessions.active", Kind: KindUpDownCounter, Unit: unitSession,
		Description: "Terminal sessions the relay of this process holds.",
		Attrs:       []Attr{{Key: "bothy.terminal.state", Values: []string{"opening", "active", "detached"}}},
	}
	// TerminalSessionsClosed is bothy.terminal.sessions.closed (4.3a, M2-46): ended sessions by
	// close reason (the closed list of 24-terminal.md section 10.3), counted after the commit by
	// the process that committed the end.
	TerminalSessionsClosed = Spec{
		Name: "bothy.terminal.sessions.closed", Kind: KindCounter, Unit: unitSession,
		Description: "Ended terminal sessions by close reason.",
		Attrs: []Attr{{Key: keyReason, Values: []string{"client_closed", "closed_by_owner", "process_exited", "idle_timeout",
			"max_duration", "detach_expired", reasonAccessRevoked, "i6_violation", "host_disabled", "host_retired", "host_rekeyed",
			"host_restarted", reasonUserDisabled, "device_revoked", "session_revoked", "logout", "open_failed", "host_policy_violation"}}},
	}
	// TerminalHostsConnected is bothy.terminal.hosts.connected (4.3a, M2-46): the control
	// connections of this api process that are usable for sessions.
	TerminalHostsConnected = Spec{
		Name: "bothy.terminal.hosts.connected", Kind: KindUpDownCounter, Unit: "{host}",
		Description: "Control connections of host services in this process that are usable for sessions.",
	}
	// SecretsRefreshFailures is bothy.secrets.refresh.failures (4.6, M1-09).
	SecretsRefreshFailures = Spec{
		Name: "bothy.secrets.refresh.failures", Kind: KindCounter, Unit: "{refresh}",
		Description: "Failed re-reads of a cached secret; the previous value stays in use.",
	}
)

// All returns every instrument of the catalogue this build registers, in catalogue order; a
// fresh slice per call.
func All() []Spec {
	return append([]Spec{
		HTTPServerRequestDuration, AuthzDecisions, AuthzErrors, RateLimitRejected, ConsoleCSPViolations,
		GatewayConnectionsActive, GatewayConnectDuration, GatewayCatchupDuration, GatewayResync,
		GatewayClose, GatewayOutboundDropped, GatewayPresenceWriteFailures,
		MessageCreated, EventAppended, EventSeqLockWait, EventFanoutLag, EventConsumerLag,
		EventConsumerPendingAge, DeliveryDuration, AgentHeartbeatAge, AgentBillingModeMismatchAge,
		JobDuration, JobQueue, EgressBlocked, FileUploadSize, FileScan, FileScanDuration, FileScanQueue, FilePurgeBacklog, RetentionDeleted,
		PushSent, PushSendDuration, PushSkipped, PushRegistrationsActive, PushFetch, SearchDuration,
		RunActive, RunStartLatency, RunDuration, RunFinished, RunSteps, RunCost, AgentInvocationDenied, AgentBillingModeMismatch,
		StatsFlushDropped, StatsFlushDuration, StatsLiveSubscribers,
		DBClientOperationDuration, DBPoolConnections,
		BackupLastSuccess, BackupWALLastArchived, BackupDuration, BackupSize,
		BackupRestoreTestLastSuccess, BackupRestoreTestDuration, BackupRestoreTestFailures,
		StorageVolumeFree, StorageVolumeSize,
		AuditVerifyFailures, AuditVerifyLastSuccess, TelemetryDropped, SecretsRefreshFailures,
		ClientMetricsDiscarded,
		TerminalRelayUnpaired, TerminalSessionsActive, TerminalSessionsClosed, TerminalHostsConnected,
	}, Client()...)
}
