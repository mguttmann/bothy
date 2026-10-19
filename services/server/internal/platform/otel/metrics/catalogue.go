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

// Attribute keys and units several instruments share.
const (
	keyReason        = "bothy.reason"
	keyOutcome       = "bothy.outcome"
	keyPrincipalKind = "bothy.principal.kind"
	unitConnection   = "{connection}"
)

// Closed literal sets shared by several instruments.
var (
	// principalKinds are bothy.principal.kind of an identified connection.
	principalKinds = []string{"user", "agent", "app"}
	// httpMethods are http.request.method: the methods of the HTTP semantic conventions and
	// _OTHER for every other method (platform/otel Middleware).
	httpMethods = []string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH", "_OTHER"}
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
	// JobDuration is bothy.job.duration (4.4).
	JobDuration = Spec{
		Name: "bothy.job.duration", Kind: KindHistogram, Unit: "s",
		Description: "Duration of one job attempt.",
		Attrs: []Attr{
			{Key: "bothy.job.kind", Source: SourceJobKinds},
			{Key: keyOutcome, Values: []string{"completed", "failed", "cancelled", "snoozed"}},
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
	// TelemetryDropped is bothy.telemetry.dropped (4.6, M1-10).
	TelemetryDropped = Spec{
		Name: "bothy.telemetry.dropped", Kind: KindCounter, Unit: "{item}",
		Description: "Telemetry items that never reached the collector.",
		Attrs:       []Attr{{Key: "bothy.signal", Values: []string{"traces", "metrics", "logs"}}},
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
		HTTPServerRequestDuration, AuthzDecisions, AuthzErrors, RateLimitRejected,
		GatewayConnectionsActive, GatewayConnectDuration, GatewayCatchupDuration, GatewayResync,
		GatewayClose, GatewayOutboundDropped,
		MessageCreated, EventAppended, EventSeqLockWait, EventFanoutLag, EventConsumerLag,
		EventConsumerPendingAge, DeliveryDuration,
		JobDuration, JobQueue, EgressBlocked,
		DBClientOperationDuration, DBPoolConnections,
		AuditVerifyFailures, AuditVerifyLastSuccess, TelemetryDropped, SecretsRefreshFailures,
		ClientMetricsDiscarded,
	}, Client()...)
}
