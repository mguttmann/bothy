// SPDX-License-Identifier: Apache-2.0

package attr

import "github.com/OWNER/bothy/services/server/internal/platform/otel/genai"

// Semantic convention keys of HTTP, network, database, errors and code (span and log).
const (
	// HTTPRequestMethod is the request method; unknown methods are "_OTHER".
	HTTPRequestMethod = "http.request.method"
	// HTTPResponseStatusCode is the response status code.
	HTTPResponseStatusCode = "http.response.status_code"
	// HTTPRoute is the route pattern (/api/v1/threads/{id}/messages), never a raw path, or
	// "unmatched" (signals.md "Kardinalität" item 2).
	HTTPRoute = "http.route"
	// URLScheme is http or https.
	URLScheme = "url.scheme"
	// ServerAddress is the address of the server a client span called (webhook.deliver). Server
	// spans of platform/otel do not carry it, because it comes from the Host header there.
	ServerAddress = "server.address"
	// ServerPort is the port of the server a client span called; like ServerAddress.
	ServerPort = "server.port"
	// NetworkProtocolName is the application protocol, for example http.
	NetworkProtocolName = "network.protocol.name"
	// NetworkProtocolVersion is its version, for example 1.1 or 2.
	NetworkProtocolVersion = "network.protocol.version"
	// UserAgentOriginal is the User-Agent header, the one header the access log reads.
	UserAgentOriginal = "user_agent.original"
	// ErrorType is the class or code of an error, never its message (signals.md "Logs").
	ErrorType = "error.type"
	// DBSystemName is the database system, postgresql.
	DBSystemName = "db.system.name"
	// DBNamespace is the database name.
	DBNamespace = "db.namespace"
	// DBOperationName is the operation, for example SELECT.
	DBOperationName = "db.operation.name"
	// DBCollectionName is the table.
	DBCollectionName = "db.collection.name"
	// DBQueryText is the query with placeholders, never with parameters.
	DBQueryText = "db.query.text"
	// DBResponseStatusCode is the SQLSTATE of a failed query.
	DBResponseStatusCode = "db.response.status_code"
	// ExceptionType is the type of an exception event; its message is never kept.
	ExceptionType = "exception.type"
	// OTelStatusCode is the span status as an attribute, where an exporter writes it so.
	OTelStatusCode = "otel.status_code"
	// OTelStatusDescription is the span status description as an attribute; platform/otel clears
	// status descriptions of its spans, because they are free text.
	OTelStatusDescription = "otel.status_description"
	// CodeFunctionName is the function that emitted a record.
	CodeFunctionName = "code.function.name"
)

// Bothy keys of signals.md "Attribute in Spans und Logs" (span and log).
const (
	// Action is an action id of 05-aktionen.md.
	Action = "bothy.action"
	// AgentHandle is the handle of an agent; agents are no persons.
	AgentHandle = "bothy.agent.handle"
	// AgentAdapterKind is agents.adapter_kind.
	AgentAdapterKind = "bothy.agent.adapter_kind"
	// BackupKind is physical, logical, objects or restore_test.
	BackupKind = "bothy.backup.kind"
	// BillingMode is personal or organization.
	BillingMode = "bothy.billing_mode"
	// ChannelID is a channel UUID; never on metrics.
	ChannelID = "bothy.channel.id"
	// SpaceID is a space UUID; never on metrics.
	SpaceID = "bothy.space.id"
	// ThreadID is a thread UUID; never on metrics.
	ThreadID = "bothy.thread.id"
	// MessageID is a message UUID; never on metrics.
	MessageID = "bothy.message.id"
	// RunID is a run UUID; never on metrics.
	RunID = "bothy.run.id"
	// FileID is a file UUID; never on metrics.
	FileID = "bothy.file.id"
	// CloseCode is a WebSocket close code of 04-sync-protokoll.md 6.4.
	CloseCode = "bothy.close.code"
	// ConsumerName is the name of a worker consumer of the event log (push, webhooks; 03
	// event_consumer_cursors); also the only attribute of bothy.event.consumer.lag and
	// bothy.event.consumer.pending_age, a closed set.
	ConsumerName = "bothy.consumer.name"
	// CorrelationID is the trace id as UUID text; logs only, in spans it is the trace id.
	CorrelationID = "bothy.correlation_id"
	// Decision is an authorization decision of 05-aktionen.md 2.5.
	Decision = "bothy.decision"
	// EgressPurpose is the purpose of an egress client; logs of refused connections only.
	EgressPurpose = "bothy.egress.purpose"
	// EventDurability is durable or ephemeral.
	EventDurability = "bothy.event.durability"
	// EventSeq is a global event sequence number; never on metrics.
	EventSeq = "bothy.event.seq"
	// EventType is an event type of 05-aktionen.md 6.
	EventType = "bothy.event.type"
	// JobKind is a job kind from the code.
	JobKind = "bothy.job.kind"
	// Outcome is a value of the fixed list of a signal.
	Outcome = "bothy.outcome"
	// PrincipalKind is user, agent, app or anonymous.
	PrincipalKind = "bothy.principal.kind"
	// PrincipalPseudoID is the HMAC pseudonym of a human principal (Pseudonymizer.Attr); spans
	// and logs only, never on metrics (privacy.md "Pseudonyme").
	PrincipalPseudoID = "bothy.principal.pseudo_id"
	// PushProvider is apns, fcm, unifiedpush or wns.
	PushProvider = "bothy.push.provider"
	// RatelimitClass is a rate limit class of 05-aktionen.md 3.
	RatelimitClass = "bothy.ratelimit.class"
	// Reason is a value of the fixed list of a signal, never free text.
	Reason = "bothy.reason"
	// RunState is a run state.
	RunState = "bothy.run.state"
	// SigningKeyPurpose is the purpose of a server signing key (access_token, invocation,
	// webhook_out, audit_checkpoint); logs of key transitions only (M1-32).
	SigningKeyPurpose = "bothy.signing_key.purpose"
	// SigningKeyKID is the kid of a server signing key, the RFC 7638 thumbprint of its public key;
	// logs of key transitions only, never on metrics.
	SigningKeyKID = "bothy.signing_key.kid"
	// SigningKeyFromState is the state a signing key leaves (none, next, active, retiring).
	SigningKeyFromState = "bothy.signing_key.from_state"
	// SigningKeyToState is the state a signing key enters (next, active, retiring, retired).
	SigningKeyToState = "bothy.signing_key.to_state"
	// StepKind is tool_call, status, subagent, workflow or background_job.
	StepKind = "bothy.step.kind"
	// VersionOwn is the SemVer version of this server build in the start check of
	// instance.min_server_version (M1-15); the resource carries it as service.version, which the
	// stdout log drops.
	VersionOwn = "bothy.version.own"
	// VersionRequired is instance.min_server_version, the smallest server version the schema
	// allows, in the same start check.
	VersionRequired = "bothy.version.required"
)

// ServerRole is the resource attribute bothy.server.role (api, worker, all; signals.md
// "Resource-Attribute"). It belongs to the resource of the process, never to a span, log record
// or metric point, so Allowed reports false for it.
const ServerRole = "bothy.server.role"

// Signal is the metric-only attribute bothy.signal of bothy.telemetry.dropped (traces, metrics,
// logs). It never appears on spans or logs, so Allowed reports false for it.
const Signal = "bothy.signal"

// scope says where a key may appear.
type scope int

const (
	scopeSpanLog  scope = iota + 1 // spans, span events, links and log records
	scopeMetric                    // metric points only
	scopeResource                  // the resource of the process only
)

// scopes registers every exported constant; TestEveryConstantRegistered keeps it complete.
var scopes = map[string]scope{
	HTTPRequestMethod: scopeSpanLog, HTTPResponseStatusCode: scopeSpanLog, HTTPRoute: scopeSpanLog,
	URLScheme: scopeSpanLog, ServerAddress: scopeSpanLog, ServerPort: scopeSpanLog,
	NetworkProtocolName: scopeSpanLog, NetworkProtocolVersion: scopeSpanLog,
	UserAgentOriginal: scopeSpanLog, ErrorType: scopeSpanLog, DBSystemName: scopeSpanLog,
	DBNamespace: scopeSpanLog, DBOperationName: scopeSpanLog, DBCollectionName: scopeSpanLog,
	DBQueryText: scopeSpanLog, DBResponseStatusCode: scopeSpanLog, ExceptionType: scopeSpanLog,
	OTelStatusCode: scopeSpanLog, OTelStatusDescription: scopeSpanLog,
	CodeFunctionName: scopeSpanLog,

	Action: scopeSpanLog, AgentHandle: scopeSpanLog, AgentAdapterKind: scopeSpanLog,
	BackupKind: scopeSpanLog, BillingMode: scopeSpanLog, ChannelID: scopeSpanLog,
	SpaceID: scopeSpanLog, ThreadID: scopeSpanLog, MessageID: scopeSpanLog, RunID: scopeSpanLog,
	FileID: scopeSpanLog, CloseCode: scopeSpanLog, ConsumerName: scopeSpanLog, CorrelationID: scopeSpanLog,
	Decision: scopeSpanLog, EgressPurpose: scopeSpanLog, EventDurability: scopeSpanLog,
	EventSeq: scopeSpanLog, EventType: scopeSpanLog, JobKind: scopeSpanLog,
	Outcome: scopeSpanLog, PrincipalKind: scopeSpanLog, PrincipalPseudoID: scopeSpanLog,
	PushProvider: scopeSpanLog, RatelimitClass: scopeSpanLog, Reason: scopeSpanLog,
	RunState: scopeSpanLog, StepKind: scopeSpanLog, VersionOwn: scopeSpanLog, VersionRequired: scopeSpanLog,
	SigningKeyPurpose: scopeSpanLog, SigningKeyKID: scopeSpanLog, SigningKeyFromState: scopeSpanLog,
	SigningKeyToState: scopeSpanLog,

	ServerRole: scopeResource,
	Signal:     scopeMetric,
}

// Allowed reports whether key may appear on a span, a span event, a span link or a log record:
// a span and log key of this package or a key of package genai. It is false for metric-only and
// resource keys and for every key outside the catalog. The log handlers and the span exporter of
// platform/otel drop every attribute for which it is false.
func Allowed(key string) bool {
	return scopes[key] == scopeSpanLog || genai.Is(key)
}
