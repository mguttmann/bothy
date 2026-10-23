// SPDX-License-Identifier: Apache-2.0

// The agent calls of the fake repository (brief M4-28 Tests), behaving like the server of
// 10 5.4, 8, 9.1 and 11: the warnings are the server's codes (computed here as the server
// does at each heartbeat, with `billing.mismatch_alert_after` treated as zero), a switch to
// organization clears the holder, personal needs a holder, "all to personal" needs the list,
// one violating agent changes nothing (BR-08), and `resolve_i1` switches the listed agents in
// the same step as the outer change, both or none (BR-07). Agent tokens show their secret
// once.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';

import 'access_fake.dart';

/// The agent calls and whether 05 gives their action step-up (`agent.billing_mode.set`,
/// `agent.billing_mode.set_all`, `agent.credential_holder.set`, `token.create`; the proposal
/// reads the audit with `audit.read`; `token.revoke` has none).
const Map<String, bool> agentsStepUpOf = {
  'setMode': true,
  'setAllModes': true,
  'setHolder': true,
  'lastGlobalSwitch': true,
  'createToken': true,
  'revokeToken': false,
};

/// The warning codes the server computes for an agent with these values (10 8, 09 4.8).
///
/// `billing_mode_mismatch` shows only when the mismatch already held at a heartbeat for longer
/// than `billing.mismatch_alert_after` ([mismatchElapsed]); right after a switch the server
/// reports `billing_mode_matches = false` without the code.
List<String> serverWarnings({
  required AgentBillingMode mode,
  AgentBillingMode? reported,
  List<AgentBillingMode>? supported,
  String keyType = 'api_key',
  AgentLiveStateKind state = AgentLiveStateKind.online,
  bool mismatchElapsed = true,
}) {
  if (state != AgentLiveStateKind.online &&
      state != AgentLiveStateKind.degraded) {
    return const [];
  }
  return [
    if (mismatchElapsed && reported != null && reported != mode)
      'billing_mode_mismatch',
    if (reported == null && supported != null && !supported.contains(mode))
      'billing_mode_unsupported',
    if (keyType == 'unknown') 'billing_evidence_unknown',
  ];
}

/// An agent record with the given parts; [warnings] defaults to [serverWarnings].
AgentRecord fakeAgentRecord(
  String id, {
  String? name,
  AgentBillingMode mode = AgentBillingMode.organization,
  String? holderId,
  AgentBillingMode? reported,
  List<AgentBillingMode>? supported = const [
    AgentBillingMode.organization,
    AgentBillingMode.personal,
  ],
  String keyType = 'api_key',
  AgentLiveStateKind state = AgentLiveStateKind.online,
  List<String>? warnings,
  bool active = true,
  int version = 1,
}) => AgentRecord(
  id: id,
  handle: id,
  displayName: name ?? id,
  active: active,
  adapterKind: 'hermes',
  description: '',
  mode: mode,
  holderId: mode == AgentBillingMode.personal ? holderId : null,
  toolClass: 'chat',
  runtime: AgentRuntimeInfo(
    state: state,
    reportedMode: reported,
    runtimeVersion: '2.3.1',
    lastHeartbeatAt: DateTime.utc(2026, 10, 17, 8, 58),
    modesSupported: supported,
    modeMatches: reported == null ? null : reported == mode,
    evidence: AgentEvidence(
      keyType: keyType,
      provider: 'anthropic',
      method: 'resolved_credentials',
    ),
    warnings:
        warnings ??
        serverWarnings(
          mode: mode,
          reported: reported,
          supported: supported,
          keyType: keyType,
          state: state,
        ),
  ),
  version: version,
);

/// The I1 conflict of [agentId] held by [holderId] with the other [principalIds] (10 5.4).
InvariantFinding i1Finding(
  String agentId, {
  String? holderId,
  List<String> principalIds = const ['sam'],
  bool? organizationReady,
}) => InvariantFinding(
  code: 'i1_violation',
  agentId: agentId,
  holderId: holderId,
  principalIds: principalIds,
  organizationReady: organizationReady,
);

/// The `409 i1_violation` of [conflicts], as the repository throws it.
SelfActionFailed i1Failure(List<InvariantFinding> conflicts) =>
    SelfActionFailed(
      const ApiProblem(
        status: 409,
        code: 'i1_violation',
        hint: 'Switch the agent to organization first',
      ),
      conflicts: conflicts,
    );

/// The agent calls of the fake; [change], [read] and [refused] are those of the repository.
mixin FakeAgents {
  /// Records the change [op] and asks for the step-up as the server would.
  Future<void> change(String op, String args, String purpose);

  /// Asks for the step-up of the reading call [op] as the server would.
  Future<void> read(String op, String purpose);

  /// Throws the refusal of [op], if any.
  void refused(String op);

  /// The agents by ID: Support runs on Alex's personal subscription and operates the space
  /// Marketing by default; Texter runs on the organization key.
  final Map<String, AgentRecord> agentRecordsById = {
    'support': fakeAgentRecord(
      'support',
      name: 'Support',
      mode: AgentBillingMode.personal,
      holderId: 'alex',
      reported: AgentBillingMode.personal,
    ),
    'texter': fakeAgentRecord('texter', name: 'Texter'),
  };

  /// The targets per agent.
  final Map<String, List<OperatedTarget>> operated = {
    'support': const [OperatedTarget(marketing, byDefault: true)],
    'texter': const [OperatedTarget(campaigns)],
  };

  /// What [lastGlobalSwitch] answers.
  SwitchAllProposal? proposal;

  /// The tokens per principal, oldest first.
  final Map<String, List<AgentToken>> tokensById = {};

  /// The I1 conflicts the server answers for a call, by call name (`setMode`, `setAllModes`,
  /// `setHolder`, `createGrant`, `addMember`, `addOwner`). A `resolve_i1` that lists an
  /// agent removes its conflict.
  final Map<String, List<InvariantFinding>> i1On = {};

  /// A problem the call answers when it carries `resolve_i1`; nothing changes then (the
  /// server's transaction, BR-07).
  final Map<String, ApiProblem> guidedFails = {};

  /// The `resolve_i1` of every call that carried one, as `<call> <agents> <reason>`.
  final List<String> resolutions = [];

  int _tokens = 0;

  /// [a] with [mode] and [holderId], the version raised and the warnings recomputed as the
  /// server would; the reported values stay (the adapter changes them with its heartbeat).
  AgentRecord switched(
    AgentRecord a,
    AgentBillingMode mode, {
    String? holderId,
  }) {
    final r = a.runtime;
    return AgentRecord(
      id: a.id,
      handle: a.handle,
      displayName: a.displayName,
      active: a.active,
      adapterKind: a.adapterKind,
      description: a.description,
      mode: mode,
      holderId: mode == AgentBillingMode.personal ? holderId : null,
      modeChangedAt: DateTime.utc(2026, 10, 17, 9),
      toolClass: a.toolClass,
      runtime: AgentRuntimeInfo(
        state: r.state,
        reportedMode: r.reportedMode,
        runtimeVersion: r.runtimeVersion,
        lastHeartbeatAt: r.lastHeartbeatAt,
        modesSupported: r.modesSupported,
        modeMatches: r.reportedMode == null ? null : r.reportedMode == mode,
        evidence: r.evidence,
        warnings: serverWarnings(
          mode: mode,
          reported: r.reportedMode,
          supported: r.modesSupported,
          keyType: r.evidence?.keyType ?? 'api_key',
          state: r.state,
          // No heartbeat has seen the new mode yet, so the alert delay has not passed.
          mismatchElapsed: false,
        ),
      ),
      version: a.version + 1,
    );
  }

  /// The I1 guard of the change [op] (10 5.4, 11): with [resolve] the listed agents switch to
  /// organization first and their conflicts disappear; any other conflict of [i1On] restores
  /// the agents and throws the `409`. Call it after the step-up and before the change.
  void i1Guard(String op, I1Resolution? resolve) {
    final conflicts = i1On[op] ?? const [];
    if (resolve == null) {
      if (conflicts.isNotEmpty) throw i1Failure(conflicts);
      return;
    }
    resolutions.add('$op ${resolve.agentIds.join(',')} ${resolve.reason}');
    if (guidedFails[op] case final problem?) throw SelfActionFailed(problem);
    final left = [
      for (final c in conflicts)
        if (!resolve.agentIds.contains(c.agentId)) c,
    ];
    if (left.isNotEmpty) throw i1Failure(left);
    for (final id in resolve.agentIds) {
      final a = agentRecordsById[id];
      if (a == null) {
        throw const SelfActionFailed(
          ApiProblem(status: 404, code: 'not_found'),
        );
      }
      if (a.mode != AgentBillingMode.organization) {
        agentRecordsById[id] = switched(a, AgentBillingMode.organization);
      }
    }
    i1On.remove(op);
  }

  AgentRecord _agent(String id) =>
      agentRecordsById[id] ??
      (throw const SelfActionFailed(
        ApiProblem(status: 404, code: 'not_found'),
      ));

  /// Every agent.
  Future<List<AgentRecord>> agentRecords() async {
    refused('agentRecords');
    return agentRecordsById.values.toList();
  }

  /// One agent.
  Future<AgentRecord> agentRecord(String agentId) async {
    refused('agentRecord');
    return _agent(agentId);
  }

  /// The targets of [agentId].
  Future<List<OperatedTarget>> operatedTargets(String agentId) async {
    refused('operatedTargets');
    return List.of(operated[agentId] ?? const []);
  }

  /// Switches [agent]; `If-Match` against the stored version (`412` on a mismatch).
  Future<AgentRecord> setMode(
    AgentRecord agent,
    AgentBillingMode mode, {
    String? holderId,
    required String reason,
    required String purpose,
  }) async {
    await change(
      'setMode',
      '${agent.id} ${mode.wire} ${holderId ?? '-'} $reason',
      purpose,
    );
    final current = _agent(agent.id);
    if (current.version != agent.version) {
      throw const SelfActionFailed(
        ApiProblem(status: 412, code: 'version_conflict'),
      );
    }
    if (mode == AgentBillingMode.personal && holderId == null) {
      throw const SelfActionFailed(
        ApiProblem(status: 400, code: 'validation_failed'),
      );
    }
    if (mode == AgentBillingMode.personal) i1Guard('setMode', null);
    return agentRecordsById[agent.id] = switched(
      current,
      mode,
      holderId: holderId,
    );
  }

  /// Switches several agents, all or nothing (BR-08).
  Future<SwitchAllResult> setAllModes(
    AgentBillingMode mode, {
    String? holderId,
    List<String>? agentIds,
    required String reason,
    required String purpose,
  }) async {
    await change(
      'setAllModes',
      '${mode.wire} ${holderId ?? '-'} ${agentIds?.join(',') ?? '-'} $reason',
      purpose,
    );
    if (mode == AgentBillingMode.personal &&
        (holderId == null || agentIds == null || agentIds.isEmpty)) {
      throw const SelfActionFailed(
        ApiProblem(status: 400, code: 'validation_failed'),
      );
    }
    i1Guard('setAllModes', null);
    final ids =
        agentIds ??
        [
          for (final a in agentRecordsById.values)
            if (a.mode == AgentBillingMode.personal) a.id,
        ];
    final changed = <String>[];
    for (final id in ids) {
      final a = _agent(id);
      if (a.mode == mode && a.holderId == holderId) continue;
      agentRecordsById[id] = switched(a, mode, holderId: holderId);
      changed.add(id);
    }
    return SwitchAllResult(changed);
  }

  /// Changes the holder of a personal agent.
  Future<AgentRecord> setHolder(
    AgentRecord agent,
    String holderId, {
    required String reason,
    required String purpose,
  }) async {
    await change('setHolder', '${agent.id} $holderId $reason', purpose);
    i1Guard('setHolder', null);
    final current = _agent(agent.id);
    return agentRecordsById[agent.id] = switched(
      current,
      AgentBillingMode.personal,
      holderId: holderId,
    );
  }

  /// The [proposal], after the step-up of `audit.read`.
  Future<SwitchAllProposal?> lastGlobalSwitch({required String purpose}) async {
    await read('lastGlobalSwitch', purpose);
    return proposal;
  }

  /// The tokens of [principalId].
  Future<List<AgentToken>> tokens(String principalId) async {
    refused('tokens');
    return List.of(tokensById[principalId] ?? const []);
  }

  /// Creates a token; the secret is in the answer only.
  Future<AgentTokenCreated> createToken(
    String principalId, {
    required String name,
    required DateTime expiresAt,
    String? dpopJkt,
    required String purpose,
  }) async {
    await change(
      'createToken',
      '$principalId $name ${expiresAt.toUtc().toIso8601String()}',
      purpose,
    );
    // 08 12: `auth.dpop_required_for` contains `agent` by default, so an agent token
    // without a key thumbprint is refused; an app token needs none (M6-08).
    if (dpopJkt == null && !principalId.startsWith('app-')) {
      throw const SelfActionFailed(
        ApiProblem(status: 400, code: 'validation_failed'),
      );
    }
    final n = _tokens++;
    final token = AgentToken(
      id: 'token-$n',
      name: name,
      prefix: 'bothy_agt_EXAMP$n',
      createdAt: DateTime.utc(2026, 10, 17, 9),
      expiresAt: expiresAt,
      dpopBound: true,
    );
    tokensById.putIfAbsent(principalId, () => []).add(token);
    return AgentTokenCreated(
      token,
      'bothy_agt_EXAMP${n}EXAMPLESecretShownOnce000000000000000_000000',
    );
  }

  /// Revokes [tokenId] (no step-up).
  Future<void> revokeToken(String tokenId) async {
    await change('revokeToken', tokenId, '');
    for (final list in tokensById.values) {
      for (final (i, t) in list.indexed) {
        if (t.id != tokenId) continue;
        list[i] = AgentToken(
          id: t.id,
          name: t.name,
          prefix: t.prefix,
          createdAt: t.createdAt,
          expiresAt: t.expiresAt,
          lastUsedAt: t.lastUsedAt,
          revokedAt: DateTime.utc(2026, 10, 17, 9),
          dpopBound: t.dpopBound,
        );
      }
    }
  }
}
