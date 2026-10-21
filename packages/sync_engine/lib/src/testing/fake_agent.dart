// SPDX-License-Identifier: Apache-2.0

/// An agent of the fake server with its backend policy, its reported runtime state and its API
/// tokens (brief M4-28 Scope: the agent administration of screen A5; 10 8, 09 4.8, 08 12).
library;

/// One agent next to its `FakePerson` of kind `agent` (same ID, handle and display name).
///
/// The configured values (`billingMode`, `credentialHolderId`) change through the handlers of
/// `rest/agent_handlers.dart`; the reported values change through `FakeWorld.agentHeartbeat`,
/// the fake's stand-in for the heartbeat of 09 4.8. `FakeWorld.agentJson` builds the API
/// `Agent` from both.
///
/// Example:
/// ```dart
/// final agent = world.addAgent(handle: 'support', displayName: 'Support');
/// world.agentHeartbeat(agent.id, reportedBillingMode: 'personal');
/// world.agentWarnings(agent); // ['billing_mode_mismatch']
/// ```
final class FakeAgent {
  /// Creates the agent of the principal [id]; every other value has the default of a fresh
  /// agent on the organization key with a runtime that is online.
  FakeAgent({
    required this.id,
    this.billingMode = 'organization',
    this.credentialHolderId,
    this.reportedBillingMode,
    this.billingModesSupported,
    this.keyType = 'api_key',
    this.evidenceMethod = 'resolved_credentials',
    this.state = 'online',
    this.adapterKind = 'hermes',
    this.toolClass = 'chat',
    this.description = '',
  });

  /// The principal ID (the ID of the `FakePerson` of kind `agent`).
  final String id;

  /// The configured backend policy (10 8 "Soll"): `organization` or `personal`.
  String billingMode;

  /// The person whose subscription a `personal` agent uses; null with `organization`.
  String? credentialHolderId;

  /// The mode the runtime reports (10 8 "Ist"); null for an adapter that follows the configured
  /// mode per run, so the API omits it.
  String? reportedBillingMode;

  /// The modes the adapter can prove (`billing_modes_supported`); null when not reported.
  List<String>? billingModesSupported;

  /// `billing_evidence.key_type`: `api_key`, `oauth`, `cloud`, `none` or `unknown`.
  String keyType;

  /// `billing_evidence.method`, or null to omit it.
  String? evidenceMethod;

  /// The runtime state of the heartbeat: `online`, `degraded`, `offline` or `unknown`.
  String state;

  /// The adapter kind, for example `hermes`.
  String adapterKind;

  /// The configured tool class: `chat`, `scoped` or `host`.
  String toolClass;

  /// The description of the agent card.
  String description;

  /// The version of the agent row; every change of the configured values raises it, and
  /// `If-Match` compares against it.
  int version = 1;

  /// When the backend policy last changed, or null before the first change.
  String? billingModeChangedAt;

  /// Whether the last heartbeat saw the reported mode differ from the configured one; the
  /// server warns `billing_mode_mismatch` only for a mismatch that lasted longer than
  /// `billing.mismatch_alert_after` (10 8), which the fake counts as one heartbeat. A change of
  /// the configured mode clears it.
  bool mismatchAtHeartbeat = false;

  /// When the last heartbeat arrived, or null before the first one.
  String? lastHeartbeatAt;

  /// The API tokens of the agent (`ApiToken` JSON) by token ID, oldest first; a revoked token
  /// keeps its row with `revoked_at`.
  final Map<String, Map<String, Object?>> tokens = {};
}
