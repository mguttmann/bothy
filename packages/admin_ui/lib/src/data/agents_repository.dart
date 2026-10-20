// SPDX-License-Identifier: Apache-2.0

/// Agents, their backend switch, their operated targets and their API tokens over REST
/// (docs/spec/01-product.md 5.2 A5; 10 5.2, 5.4, 8, 9, 11, 13; 08 12; brief M4-28 Design notes 1
/// to 4).
///
/// The types carry what the server answers, unchanged: the warnings of 10 8 are the server's
/// codes (`AgentRuntimeStatus.warnings`), never a comparison of the client (15 1 principle 1),
/// and whether a switch is allowed is the server's answer (`409 i1_violation`).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';

import 'access_repository.dart';
import 'self_repository.dart';

/// The backend of an agent (`BillingMode`, 10 1).
enum AgentBillingMode {
  /// The personal subscription of exactly one person (I1).
  personal('personal'),

  /// An organization API key of the adapter.
  organization('organization'),

  /// A value of a newer server.
  unknown('');

  const AgentBillingMode(this.wire);

  /// The name on the wire, for example `personal`.
  final String wire;

  /// The mode of [wire]; [unknown] for a value this build does not know.
  static AgentBillingMode of(String? wire) =>
      values.where((m) => m != unknown && m.wire == wire).firstOrNull ??
      unknown;
}

/// The state of the runtime as the heartbeat reports it (`AgentRuntimeState`, 09 4.8).
enum AgentLiveStateKind {
  /// Online.
  online,

  /// Online with problems.
  degraded,

  /// No heartbeat within `agent.presence_timeout`.
  offline,

  /// No heartbeat yet, or a value of a newer server.
  unknown,
}

/// The presence of an agent as a stream delivers it: the desktop app maps the engine's
/// `EphemeralStore` to it, the console reads it from `getAgent` (brief M4-28 Design note 1).
final class AgentLiveState {
  /// Creates the presence.
  const AgentLiveState({required this.state, this.reportedMode});

  /// The state.
  final AgentLiveStateKind state;

  /// The reported mode, or null when the adapter reports none.
  final AgentBillingMode? reportedMode;
}

/// How the runtime proves its mode (`BillingEvidence`); never a key value.
final class AgentEvidence {
  /// Creates the evidence.
  const AgentEvidence({required this.keyType, this.provider, this.method});

  /// `api_key`, `oauth`, `cloud`, `none`, `unknown`.
  final String keyType;

  /// The provider, for example `anthropic`, or null.
  final String? provider;

  /// The evidence source, for example `resolved_credentials`, or null.
  final String? method;
}

/// The reported state of the runtime (`AgentRuntimeStatus`, 10 8).
final class AgentRuntimeInfo {
  /// Creates the state.
  const AgentRuntimeInfo({
    required this.state,
    this.reportedMode,
    this.runtimeVersion,
    this.lastHeartbeatAt,
    this.modesSupported,
    this.modeMatches,
    this.evidence,
    this.warnings = const [],
  });

  /// The state.
  final AgentLiveStateKind state;

  /// The reported mode; null for an adapter that follows the configured mode per run (10 8).
  final AgentBillingMode? reportedMode;

  /// The version of the runtime, or null.
  final String? runtimeVersion;

  /// The last heartbeat, or null.
  final DateTime? lastHeartbeatAt;

  /// The modes the adapter can prove (`billing_modes_supported`), or null when not reported.
  final List<AgentBillingMode>? modesSupported;

  /// The server's comparison of reported and configured mode, or null.
  final bool? modeMatches;

  /// The evidence, or null.
  final AgentEvidence? evidence;

  /// The warning codes of the server (09 4.8): `billing_mode_mismatch`,
  /// `billing_mode_unsupported`, `billing_evidence_unknown`, `tool_class_mismatch`, or codes this
  /// build does not know.
  final List<String> warnings;
}

/// An agent as the agent administration shows it (`Agent`).
final class AgentRecord {
  /// Creates the agent.
  const AgentRecord({
    required this.id,
    required this.handle,
    required this.displayName,
    required this.active,
    required this.adapterKind,
    required this.description,
    required this.mode,
    required this.toolClass,
    required this.runtime,
    required this.version,
    this.holderId,
    this.modeChangedAt,
  });

  /// The agent ID.
  final String id;

  /// The handle.
  final String handle;

  /// The display name.
  final String displayName;

  /// Whether the agent is active (not disabled).
  final bool active;

  /// The adapter, for example `hermes`.
  final String adapterKind;

  /// The description.
  final String description;

  /// The configured mode.
  final AgentBillingMode mode;

  /// The credential holder with `personal`, else null.
  final String? holderId;

  /// When the mode last changed, or null.
  final DateTime? modeChangedAt;

  /// `chat`, `scoped` or `host`.
  final String toolClass;

  /// The reported state of the runtime.
  final AgentRuntimeInfo runtime;

  /// The version for `If-Match`.
  final int version;
}

/// A space or channel an agent operates (10 5.2), from the operator fields the API offers
/// (`Space.default_operator_agent_id`, `Channel.operator.effective_agent_id`).
final class OperatedTarget {
  /// Creates the target; [byDefault] marks a space whose default operator is the agent.
  const OperatedTarget(this.target, {this.byDefault = false});

  /// The space or channel.
  final AdminTarget target;

  /// Whether the agent is the default operator of this space.
  final bool byDefault;
}

/// What the switch of all agents changed (`BillingModeSetAllResult`).
final class SwitchAllResult {
  /// Creates the result.
  const SwitchAllResult(this.changedAgentIds);

  /// The agents that changed.
  final List<String> changedAgentIds;
}

/// The proposal for "All to Personal" from the last global switch (10 9.1): the agents it
/// switched from `personal` to `organization`, and their holder when it was one person.
final class SwitchAllProposal {
  /// Creates the proposal.
  const SwitchAllProposal(this.agentIds, {this.holderId});

  /// The proposed agents.
  final List<String> agentIds;

  /// The common former holder, or null.
  final String? holderId;
}

/// An API token of an agent (`ApiToken`, 08 12): only its prefix, never its secret.
final class AgentToken {
  /// Creates the token.
  const AgentToken({
    required this.id,
    required this.name,
    required this.prefix,
    required this.createdAt,
    this.expiresAt,
    this.lastUsedAt,
    this.revokedAt,
    this.dpopBound = false,
  });

  /// The token ID.
  final String id;

  /// The name.
  final String name;

  /// The prefix, for example `bothy_agt_EXAMPL`.
  final String prefix;

  /// When it was created.
  final DateTime createdAt;

  /// When it expires, or null.
  final DateTime? expiresAt;

  /// When it was last used, or null.
  final DateTime? lastUsedAt;

  /// When it was revoked, or null.
  final DateTime? revokedAt;

  /// Whether it is bound to a DPoP key.
  final bool dpopBound;
}

/// A created token with its secret (`ApiTokenCreated`): a one-time secret, shown once and never
/// stored by the client (08 12; brief Design note 4).
final class AgentTokenCreated {
  /// Creates the result.
  const AgentTokenCreated(this.token, this.secret);

  /// The token.
  final AgentToken token;

  /// The secret.
  final String secret;
}

/// The calls of the agent administration; a changing call carries the [purpose] text the
/// step-up dialog shows when the server answers `401 step_up_required` (05:
/// `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set` and
/// `token.create` have step-up, `token.revoke` has none).
///
/// Errors: [SelfActionFailed] with the server's problem (a `409 i1_violation` carries the
/// conflicts in [SelfActionFailed.conflicts]), [StepUpCancelled] when the person cancels the
/// step-up.
abstract interface class AgentsRepository {
  /// Every agent with its record (`listAgents`, all pages).
  Future<List<AgentRecord>> agentRecords();

  /// One agent (`getAgent`).
  Future<AgentRecord> agentRecord(String agentId);

  /// The spaces and channels [agentId] operates (`listSpaces`, `listSpaceChannels`).
  Future<List<OperatedTarget>> operatedTargets(String agentId);

  /// Switches [agent] to [mode] (`setAgentBillingMode`); [holderId] is required for
  /// [AgentBillingMode.personal] and absent for organization (the server clears the holder).
  Future<AgentRecord> setMode(
    AgentRecord agent,
    AgentBillingMode mode, {
    String? holderId,
    required String reason,
    required String purpose,
  });

  /// Switches several agents in one transaction (`setAllAgentsBillingMode`, all or nothing,
  /// BR-08); [agentIds] is required for personal.
  Future<SwitchAllResult> setAllModes(
    AgentBillingMode mode, {
    String? holderId,
    List<String>? agentIds,
    required String reason,
    required String purpose,
  });

  /// Changes the holder of the personal [agent] (`setCredentialHolder`).
  Future<AgentRecord> setHolder(
    AgentRecord agent,
    String holderId, {
    required String reason,
    required String purpose,
  });

  /// The proposal of "All to Personal" from the newest `agent.billing_mode.set_all` to
  /// organization in the audit (`listAuditEvents` filtered by action; step-up of `audit.read`),
  /// or null when there is none.
  Future<SwitchAllProposal?> lastGlobalSwitch({required String purpose});

  /// The API tokens of [principalId] (`listTokens`, all pages).
  Future<List<AgentToken>> tokens(String principalId);

  /// Creates a token for [principalId] (`createToken`, step-up).
  Future<AgentTokenCreated> createToken(
    String principalId, {
    required String name,
    required DateTime expiresAt,
    String? dpopJkt,
    required String purpose,
  });

  /// Revokes [tokenId] (`revokeToken`, no step-up).
  Future<void> revokeToken(String tokenId);
}

/// [AgentsRepository] over the generated client of `api_client`.
///
/// Example:
/// ```dart
/// final agents = ApiAgentsRepository(session.api, newIdempotencyKey: ids.next);
/// final list = await agents.agentRecords();
/// ```
final class ApiAgentsRepository implements AgentsRepository {
  /// Creates the repository over `api` (the client of the signed-in session, so DPoP, refresh
  /// and the step-up interceptor apply); [newIdempotencyKey] returns a fresh UUIDv7.
  ApiAgentsRepository(this._api, {required String Function() newIdempotencyKey})
    : _newKey = newIdempotencyKey;

  final ApiClient _api;
  final String Function() _newKey;

  AgentsApi get _agents => _api.getAgentsApi();
  TokensApi get _tokens => _api.getTokensApi();

  Map<String, Object?> _purpose(String text) => {stepUpPurposeExtra: text};

  static String _ifMatch(AgentRecord a) => '"${a.version}"';

  @override
  Future<List<AgentRecord>> agentRecords() => adminCall(() async {
    final out = <AgentRecord>[];
    String? cursor;
    do {
      final page = (await _agents.listAgents(cursor: cursor)).data!;
      out.addAll(page.items.map(agentRecordOf));
      cursor = page.nextCursor;
    } while (cursor != null);
    return out;
  });

  @override
  Future<AgentRecord> agentRecord(String agentId) => adminCall(
    () async => agentRecordOf((await _agents.getAgent(agentId: agentId)).data!),
  );

  @override
  Future<List<OperatedTarget>> operatedTargets(String agentId) =>
      adminCall(() async {
        final out = <OperatedTarget>[];
        final spaces = (await _api.getSpacesApi().listSpaces()).data!.items;
        for (final s in spaces) {
          final space = AdminTarget(
            AdminTargetKind.space,
            id: s.space.id,
            name: s.space.name,
          );
          if (s.space.defaultOperatorAgentId == agentId) {
            out.add(OperatedTarget(space, byDefault: true));
          }
          final channels = (await _api.getChannelsApi().listSpaceChannels(
            spaceId: s.space.id,
          )).data!.items;
          for (final c in channels) {
            if (c.channel.operator_.effectiveAgentId != agentId) continue;
            out.add(
              OperatedTarget(
                AdminTarget(
                  AdminTargetKind.channel,
                  id: c.channel.id,
                  name: c.channel.name,
                  spaceName: s.space.name,
                ),
              ),
            );
          }
        }
        return out;
      });

  @override
  Future<AgentRecord> setMode(
    AgentRecord agent,
    AgentBillingMode mode, {
    String? holderId,
    required String reason,
    required String purpose,
  }) => adminCall(
    () async => agentRecordOf(
      (await _agents.setAgentBillingMode(
        agentId: agent.id,
        ifMatch: _ifMatch(agent),
        billingModeSet: BillingModeSet(
          (b) => b
            ..billingMode = _wireMode(mode)
            ..credentialHolderId = mode == AgentBillingMode.personal
                ? holderId
                : null
            ..reason = reason,
        ),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<SwitchAllResult> setAllModes(
    AgentBillingMode mode, {
    String? holderId,
    List<String>? agentIds,
    required String reason,
    required String purpose,
  }) => adminCall(() async {
    final result = (await _agents.setAllAgentsBillingMode(
      billingModeSetAll: BillingModeSetAll((b) {
        b
          ..billingMode = _wireMode(mode)
          ..credentialHolderId = mode == AgentBillingMode.personal
              ? holderId
              : null
          ..reason = reason;
        if (agentIds != null) b.agentIds.replace(agentIds);
      }),
      extra: _purpose(purpose),
    )).data!;
    return SwitchAllResult(result.changedAgentIds.toList());
  });

  @override
  Future<AgentRecord> setHolder(
    AgentRecord agent,
    String holderId, {
    required String reason,
    required String purpose,
  }) => adminCall(
    () async => agentRecordOf(
      (await _agents.setCredentialHolder(
        agentId: agent.id,
        ifMatch: _ifMatch(agent),
        credentialHolderSet: CredentialHolderSet(
          (b) => b
            ..credentialHolderId = holderId
            ..reason = reason,
        ),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<SwitchAllProposal?> lastGlobalSwitch({required String purpose}) =>
      adminCall(() async {
        String? cursor;
        do {
          final page = (await _api.getAuditApi().listAuditEvents(
            action: 'agent.billing_mode.set_all',
            cursor: cursor,
            extra: _purpose(purpose),
          )).data!;
          for (final e in page.items) {
            final proposal = switchAllProposalOf({
              for (final MapEntry(:key, :value) in e.details.entries)
                key: value?.value,
            });
            if (proposal != null) return proposal;
          }
          cursor = page.nextCursor;
        } while (cursor != null);
        return null;
      });

  @override
  Future<List<AgentToken>> tokens(String principalId) => adminCall(() async {
    final out = <AgentToken>[];
    String? cursor;
    do {
      final page = (await _tokens.listTokens(
        principalId: principalId,
        cursor: cursor,
      )).data!;
      out.addAll(page.items.map(_token));
      cursor = page.nextCursor;
    } while (cursor != null);
    return out;
  });

  @override
  Future<AgentTokenCreated> createToken(
    String principalId, {
    required String name,
    required DateTime expiresAt,
    String? dpopJkt,
    required String purpose,
  }) => adminCall(() async {
    final created = (await _tokens.createToken(
      principalId: principalId,
      apiTokenCreate: ApiTokenCreate(
        (b) => b
          ..name = name
          ..expiresAt = expiresAt.toUtc()
          ..dpopJkt = dpopJkt,
      ),
      idempotencyKey: _newKey(),
      extra: _purpose(purpose),
    )).data!;
    return AgentTokenCreated(_token(created.token), created.secret);
  });

  @override
  Future<void> revokeToken(String tokenId) =>
      adminCall(() => _tokens.revokeToken(tokenId: tokenId));

  static BillingMode _wireMode(AgentBillingMode m) => switch (m) {
    AgentBillingMode.personal => BillingMode.personal,
    _ => BillingMode.organization,
  };

  static AgentToken _token(ApiToken t) => AgentToken(
    id: t.id,
    name: t.name,
    prefix: t.tokenPrefix,
    createdAt: t.createdAt,
    expiresAt: t.expiresAt,
    lastUsedAt: t.lastUsedAt,
    revokedAt: t.revokedAt,
    dpopBound: t.dpopJkt != null,
  );
}

AgentBillingMode _mode(BillingMode? m) => switch (m) {
  BillingMode.personal => AgentBillingMode.personal,
  BillingMode.organization => AgentBillingMode.organization,
  _ => AgentBillingMode.unknown,
};

AgentLiveStateKind _state(AgentRuntimeState s) => switch (s) {
  AgentRuntimeState.online => AgentLiveStateKind.online,
  AgentRuntimeState.degraded => AgentLiveStateKind.degraded,
  AgentRuntimeState.offline => AgentLiveStateKind.offline,
  _ => AgentLiveStateKind.unknown,
};

String _keyType(BillingEvidenceKeyTypeEnum k) => switch (k) {
  BillingEvidenceKeyTypeEnum.apiKey => 'api_key',
  BillingEvidenceKeyTypeEnum.oauth => 'oauth',
  BillingEvidenceKeyTypeEnum.cloud => 'cloud',
  BillingEvidenceKeyTypeEnum.none => 'none',
  _ => 'unknown',
};

/// The [AgentRecord] of the API's [a].
AgentRecord agentRecordOf(Agent a) {
  final p = a.presence;
  final evidence = p.billingEvidence;
  return AgentRecord(
    id: a.id,
    handle: a.handle,
    displayName: a.displayName,
    active: a.status == AgentStatusEnum.active,
    adapterKind: a.adapterKind,
    description: a.description,
    mode: _mode(a.billingMode),
    holderId: a.credentialHolderId,
    modeChangedAt: a.billingModeChangedAt,
    toolClass: switch (a.toolClass) {
      ToolClass.scoped => 'scoped',
      ToolClass.host => 'host',
      _ => 'chat',
    },
    runtime: AgentRuntimeInfo(
      state: _state(p.state),
      reportedMode: p.reportedBillingMode == null
          ? null
          : _mode(p.reportedBillingMode),
      runtimeVersion: p.runtimeVersion,
      lastHeartbeatAt: p.lastHeartbeatAt,
      modesSupported: p.billingModesSupported?.map(_mode).toList(),
      modeMatches: p.billingModeMatches,
      evidence: evidence == null
          ? null
          : AgentEvidence(
              keyType: _keyType(evidence.keyType),
              provider: evidence.provider,
              method: evidence.method,
            ),
      warnings: p.warnings?.toList() ?? const [],
    ),
    version: a.version,
  );
}

/// The proposal of an audit entry of `agent.billing_mode.set_all` with [details] (the server's
/// attributes: `to_billing_mode`, `agent_ids`, `from_billing_modes`, `held_agent_ids`,
/// `from_credential_holder_ids`): the agents that went from `personal` to `organization`, or
/// null for an entry of another direction or without a change.
///
/// Example: `{to_billing_mode: organization, agent_ids: [a, b], from_billing_modes: [personal,
/// organization]}` proposes `[a]`.
SwitchAllProposal? switchAllProposalOf(Map<String, Object?> details) {
  if (details['to_billing_mode'] != 'organization') return null;
  final ids = details['agent_ids'];
  if (ids is! List || ids.isEmpty) return null;
  final from = details['from_billing_modes'];
  final proposed = [
    for (final (i, id) in ids.indexed)
      if (id is String &&
          (from is! List || i >= from.length || from[i] == 'personal'))
        id,
  ];
  if (proposed.isEmpty) return null;
  final holders = details['from_credential_holder_ids'];
  final distinct = <String>{
    if (holders is List) ...holders.whereType<String>(),
  };
  return SwitchAllProposal(
    proposed,
    holderId: distinct.length == 1 ? distinct.single : null,
  );
}
