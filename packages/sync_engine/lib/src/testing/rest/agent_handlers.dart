// SPDX-License-Identifier: Apache-2.0

/// Handlers of the agent administration of screen A5 (brief M4-28 Scope; 10 5.4, 8, 9.1, 11;
/// 08 12; 05 `agent.*`, `token.*`) and the I1 guard that the grant, group and owner handlers
/// share ([FakeI1Guard]).
///
/// A small model of the server: the agent card is [FakeWorld.agentJson]; the backend policy
/// changes per agent or for several agents in one step, all or nothing (BR-08); every rights
/// change checks I1 for every active personal agent after the change and rolls everything back
/// on a violation (10 5.4, BR-01); `resolve_i1` switches the listed agents to `organization`
/// first, in the same step, with one audit row per agent and the row of the outer action under
/// one `correlation_id` (10 11, BR-07). Agent tokens have the form of ADR 0061 and are shown
/// once (06 7).
library;

import 'dart:convert';
import 'dart:math';

import '../action_catalog.dart';
import '../audit_log.dart';
import '../fake_agent.dart';
import '../rest_fake.dart';
import '../world.dart';
import 'admin_handlers.dart' show requireCatalogStepUp;

/// The `hint` of an `i1_violation` (10 5.4: "switch to organization first").
const String i1Hint = 'Switch the agent to organization first';

/// What a rights change wrote: the answer, the resource of its audit row and the row's details.
typedef FakeChange = ({
  FakeResponse response,
  String? resourceId,
  JsonMap details,
});

/// Everything a rights change of the fake may write, so a violation of I1 can restore it all
/// (10 5.3: the check runs before the commit, a violation rolls the transaction back).
final class _Snapshot {
  _Snapshot(this._world, this._log)
    : _agents = {
        for (final a in _world.agents.values)
          a.id: (
            a.billingMode,
            a.credentialHolderId,
            a.version,
            a.billingModeChangedAt,
            a.mismatchAtHeartbeat,
          ),
      },
      _grants = {
        for (final MapEntry(:key, :value) in _world.grants.entries)
          key: Map.of(value),
      },
      _access = {
        for (final c in _world.channels.values) c.id: Map.of(c.access),
      },
      _members = {
        for (final MapEntry(:key, :value) in _world.groupMembers.entries)
          key: Map.of(value),
      },
      _owners = {
        for (final p in _world.people.values)
          p.id: (p.isOwner, p.ownerGrantedAt, p.ownerReason),
      },
      _permissionVersion = _world.permissionVersion,
      _rows = _log.rows.length;

  final FakeWorld _world;
  final FakeAuditLog _log;
  final Map<String, (String, String?, int, String?, bool)> _agents;
  final Map<String, JsonMap> _grants;
  final Map<String, Map<String, String>> _access;
  final Map<String, Map<String, JsonMap>> _members;
  final Map<String, (bool, String, String)> _owners;
  final int _permissionVersion;
  final int _rows;

  void restore() {
    for (final MapEntry(key: id, value: (mode, holder, version, at, seen))
        in _agents.entries) {
      _world.agents[id]
        ?..billingMode = mode
        ..credentialHolderId = holder
        ..version = version
        ..billingModeChangedAt = at
        ..mismatchAtHeartbeat = seen;
    }
    _world.grants
      ..clear()
      ..addAll(_grants);
    for (final c in _world.channels.values) {
      c.access
        ..clear()
        ..addAll(_access[c.id] ?? const {});
    }
    _world.groupMembers
      ..clear()
      ..addAll(_members);
    for (final MapEntry(key: id, value: (owner, at, reason))
        in _owners.entries) {
      _world.people[id]
        ?..isOwner = owner
        ..ownerGrantedAt = at
        ..ownerReason = reason;
    }
    _world.permissionVersion = _permissionVersion;
    _log.rows.removeRange(_rows, _log.rows.length);
  }
}

/// The I1 guard of the fake's rights changes (10 5.4, 11; BR-01, BR-07, BR-08).
///
/// [run] applies a change, checks I1 for every active personal agent of the world afterwards
/// and restores everything on a violation; the request's `resolve_i1` switches the listed
/// agents to `organization` first. [dryRun] does the same for a preview and always restores.
///
/// Example:
/// ```dart
/// final guard = FakeI1Guard(world, audit);
/// return guard.run(r, action: 'owner.add', resourceKind: 'user', apply: () {
///   person.isOwner = true;
///   return (response: FakeResponse(201, owner), resourceId: person.id, details: {});
/// });
/// ```
final class FakeI1Guard {
  /// Creates the guard over [world]; audit rows go to [audit].
  FakeI1Guard(this.world, this.audit);

  /// The world.
  final FakeWorld world;

  /// The audit log of the rows of every change.
  final FakeAuditLog audit;

  /// The agents of the `resolve_i1` of [body], in ascending ID (10 11: the server authorizes
  /// them in ascending id); none without `resolve_i1`.
  ///
  /// Throws `404 not_found` for an ID that is not an agent, before anything is written.
  List<FakeAgent> resolveAgents(JsonMap body) {
    final resolve = body['resolve_i1'] as JsonMap?;
    if (resolve == null) return const [];
    final ids = [
      for (final id in resolve['switch_to_organization']! as List) id as String,
    ]..sort();
    return [
      for (final id in ids)
        world.agentOf(id) ?? (throw const FakeProblem(404, 'not_found')),
    ];
  }

  /// Runs [apply] as one rights change of [r] with the audit row [action] on [resourceKind].
  ///
  /// Order of 10 11 point 4: the agents of `resolve_i1` switch to `organization` (one audit row
  /// each), then [apply] writes, then I1 is checked. On a violation everything is restored, a
  /// `deny` row with `deny_reason: i1_violation` is written (10 5.4) and the answer is
  /// `409 i1_violation` with `conflicts`. Every row of a successful request shares one
  /// `correlation_id`. A [FakeProblem] of [apply] restores the switches and passes through.
  /// Returns the response of [apply].
  FakeResponse run(
    FakeRequest r, {
    required String action,
    required String resourceKind,
    required FakeChange Function() apply,
  }) {
    final agents = resolveAgents(r.json);
    final saved = _Snapshot(world, audit);
    final correlation = audit.nextCorrelationId;
    final reason = (r.json['resolve_i1'] as JsonMap?)?['reason'] as String?;
    final FakeChange change;
    try {
      for (final a in agents) {
        switchToOrganization(
          a,
          actor: r.person.id,
          reason: reason!,
          correlationId: correlation,
        );
      }
      change = apply();
    } on Object {
      saved.restore();
      rethrow;
    }
    final conflicts = world.i1Conflicts();
    if (conflicts.isNotEmpty) {
      // Built before the rollback: the holder of a conflict is the one the change set.
      final list = conflictsJson(conflicts);
      saved.restore();
      audit.add(
        world,
        action: action,
        decision: 'deny',
        denyReason: 'i1_violation',
        actor: r.person.id,
        resourceKind: resourceKind,
        details: {'conflicts': list},
      );
      throw FakeProblem(
        409,
        'i1_violation',
        extra: {'hint': i1Hint, 'conflicts': list},
      );
    }
    audit.add(
      world,
      action: action,
      actor: r.person.id,
      resourceKind: resourceKind,
      resourceId: change.resourceId,
      details: change.details,
      correlationId: correlation,
    );
    return change.response;
  }

  /// The I1 conflicts the request [body] would cause: switches its `resolve_i1` agents, runs
  /// [apply] and checks, then restores everything (a preview writes nothing, 07 9.3).
  ///
  /// Throws `404 not_found` for a `resolve_i1` ID that is not an agent.
  Map<String, List<String>> dryRun(JsonMap body, void Function() apply) {
    final agents = resolveAgents(body);
    final saved = _Snapshot(world, audit);
    try {
      for (final a in agents) {
        setPolicy(a, 'organization', null);
      }
      apply();
      return world.i1Conflicts();
    } finally {
      saved.restore();
    }
  }

  /// Switches [a] to `organization` for `resolve_i1` with one audit row
  /// `agent.billing_mode.set` (10 11: an agent already there keeps its row and gets a row with
  /// `no_change`).
  void switchToOrganization(
    FakeAgent a, {
    required String actor,
    required String reason,
    required String correlationId,
  }) {
    final from = a.billingMode;
    final holder = holderOf(a);
    final details = policyDetails(from, holder, 'organization', null, reason);
    if (from != 'organization') setPolicy(a, 'organization', null);
    audit.add(
      world,
      action: 'agent.billing_mode.set',
      actor: actor,
      resourceKind: 'agent',
      resourceId: a.id,
      details: {...details, if (from == 'organization') 'no_change': true},
      correlationId: correlationId,
    );
  }

  /// The holder that counts for [a]: its `credential_holder_id` on `personal`, else null.
  static String? holderOf(FakeAgent a) =>
      a.billingMode == 'personal' ? a.credentialHolderId : null;

  /// Sets the policy of [a] to [mode] with [holder], raises its version and the permission
  /// version and stamps `billing_mode_changed_at`.
  void setPolicy(FakeAgent a, String mode, String? holder) {
    a
      ..billingMode = mode
      ..credentialHolderId = holder
      ..version += 1
      ..billingModeChangedAt = world.stamp()
      ..mismatchAtHeartbeat = false;
    world.permissionVersion++;
  }

  /// The audit details of a policy switch of one agent, as the server writes them
  /// (`policyAttrs` of services/server/internal/agents/app/policy.go, 10 14).
  static JsonMap policyDetails(
    String fromMode,
    String? fromHolder,
    String toMode,
    String? toHolder,
    String reason,
  ) => {
    'from_billing_mode': fromMode,
    'to_billing_mode': toMode,
    'reason': reason,
    'from_credential_holder_id': ?fromHolder,
    'to_credential_holder_id': ?toHolder,
  };

  /// The `conflicts` of a `409 i1_violation` (`InvariantViolation`, 10 5.4).
  List<JsonMap> conflictsJson(Map<String, List<String>> conflicts) => [
    for (final MapEntry(key: id, value: others) in conflicts.entries)
      {
        'code': 'i1_violation',
        'agent_id': id,
        'credential_holder_id': ?world.agents[id]?.credentialHolderId,
        'principal_ids': others,
      },
  ];

  /// The `invariant_violations` of a grant preview (10 11 point 1): the conflicts with
  /// `organization_ready` (the agent reports `organization` among `billing_modes_supported`)
  /// and the [i1Hint].
  List<JsonMap> previewViolations(Map<String, List<String>> conflicts) => [
    for (final c in conflictsJson(conflicts))
      {
        ...c,
        'organization_ready':
            world.agents[c['agent_id']]?.billingModesSupported?.contains(
              'organization',
            ) ??
            false,
        'hint': i1Hint,
      },
  ];

  /// One sentence per conflict for the preview text (10 11 point 2, English like the fake's
  /// other preview texts).
  List<String> previewSentences(Map<String, List<String>> conflicts) => [
    for (final id in conflicts.keys)
      '${world.people[id]?.displayName ?? id} runs on the personal subscription of '
          '${world.people[world.agents[id]?.credentialHolderId]?.displayName ?? 'nobody'}; '
          'switch it to organization first.',
  ];
}

/// The Base62 alphabet of ADR 0061 (digit values 0 to 61).
const String _base62 =
    '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';

final List<int> _crcTable = List.generate(256, (n) {
  var c = n;
  for (var k = 0; k < 8; k++) {
    c = (c & 1) == 1 ? 0xEDB88320 ^ (c >> 1) : c >> 1;
  }
  return c;
});

/// CRC-32 (IEEE) over the UTF-8 bytes of [text] (ADR 0061 point 1).
int _crc32(String text) {
  var crc = 0xFFFFFFFF;
  for (final b in utf8.encode(text)) {
    crc = _crcTable[(crc ^ b) & 0xFF] ^ (crc >> 8);
  }
  return crc ^ 0xFFFFFFFF;
}

/// [value] in Base62 as exactly [width] characters with leading zeros.
String _base62Of(int value, int width) {
  final digits = <String>[];
  for (var v = value; v > 0; v ~/= 62) {
    digits.add(_base62[v % 62]);
  }
  return digits.reversed.join().padLeft(width, '0');
}

/// Registers the agent and token handlers on [rest], with the step-up of [catalog]; the audit
/// rows go to [audit]. `listAgents` replaces the earlier handler of `access_handlers.dart`.
void registerAgentHandlers(
  RestFake rest,
  ActionCatalog catalog,
  FakeAuditLog audit,
) {
  final world = rest.world;
  final guard = FakeI1Guard(world, audit);
  final random = Random(28);
  // 06 7, ADR 0061 point 3: the token secret is shown exactly once.
  rest.oneTimeSecrets.add('createToken');

  void owner(FakeRequest r) {
    if (!r.person.isOwner) {
      throw const FakeProblem(403, 'forbidden', reason: 'owner_only');
    }
  }

  void stepUp(FakeRequest r) => requireCatalogStepUp(r, rest.contract, catalog);

  FakeAgent agentOf(String id) =>
      world.agentOf(id) ?? (throw const FakeProblem(404, 'not_found'));

  /// `If-Match` against the version of [a] (06: `428` without, `412` on a mismatch).
  void ifMatch(FakeRequest r, FakeAgent a) {
    final match = RegExp(r'^"?(\d+)"?$')
        .firstMatch(r.headers['if-match'] ?? '');
    if (match == null) throw const FakeProblem(428, 'precondition_required');
    if (int.parse(match[1]!) != a.version) {
      throw FakeProblem(412, 'version_conflict', current: world.agentJson(a));
    }
  }

  FakeResponse card(FakeAgent a) =>
      FakeResponse(200, world.agentJson(a), {'ETag': '"${a.version}"'});

  /// A holder must be an active person (10 4, I4); `required` without one.
  void validHolder(String? id) {
    if (id == null) {
      throw const FakeProblem(
        400,
        'validation_failed',
        errors: [
          {'pointer': '/credential_holder_id', 'code': 'required'},
        ],
      );
    }
    final p = world.people[id];
    if (p == null || p.kind != 'user' || p.status != 'active') {
      throw const FakeProblem(
        400,
        'validation_failed',
        errors: [
          {'pointer': '/credential_holder_id', 'code': 'invalid'},
        ],
      );
    }
  }

  FakeAgent tokenOwner(FakeRequest r) => agentOf(r.param('principal_id'));

  rest
    ..handle('listAgents', (r) {
      return FakeResponse(200, {
        'items': [
          for (final p in world.people.values)
            if (p.kind == 'agent') world.agentJson(world.agentOf(p.id)!),
        ],
      });
    })
    ..handle('getAgent', (r) => card(agentOf(r.param('agent_id'))))
    ..handle('setAgentBillingMode', (r) {
      owner(r);
      final a = agentOf(r.param('agent_id'));
      stepUp(r);
      ifMatch(r, a);
      final mode = r.json['billing_mode']! as String;
      final reason = r.json['reason']! as String;
      final holder = mode == 'personal'
          ? r.json['credential_holder_id'] as String?
          : null;
      if (mode == 'personal') validHolder(holder);
      return guard.run(
        r,
        action: 'agent.billing_mode.set',
        resourceKind: 'agent',
        apply: () {
          final from = FakeI1Guard.holderOf(a);
          final details = FakeI1Guard.policyDetails(
            a.billingMode,
            from,
            mode,
            holder,
            reason,
          );
          final same = a.billingMode == mode && from == holder;
          if (!same) guard.setPolicy(a, mode, holder);
          return (
            response: card(a),
            resourceId: a.id,
            details: {...details, if (same) 'no_change': true},
          );
        },
      );
    })
    ..handle('setAllAgentsBillingMode', (r) {
      owner(r);
      stepUp(r);
      final mode = r.json['billing_mode']! as String;
      final reason = r.json['reason']! as String;
      final ids = (r.json['agent_ids'] as List?)?.cast<String>();
      final holder = mode == 'personal'
          ? r.json['credential_holder_id'] as String?
          : null;
      if (mode == 'personal') {
        if (ids == null || ids.isEmpty) {
          throw const FakeProblem(
            400,
            'validation_failed',
            errors: [
              {'pointer': '/agent_ids', 'code': 'required'},
            ],
          );
        }
        validHolder(holder);
      }
      final listed = ids == null ? null : [for (final id in ids) agentOf(id)];
      return guard.run(
        r,
        action: 'agent.billing_mode.set_all',
        resourceKind: 'instance',
        apply: () {
          // Without a list: every agent not yet on organization (BillingModeSetAll), in
          // ascending ID as the server locks them (policy.go `lockForAll`).
          final candidates = [
            ...?listed,
            if (listed == null)
              for (final a in world.agents.values)
                if (a.billingMode == 'personal') a,
          ]..sort((x, y) => x.id.compareTo(y.id));
          final changed = <String>[];
          final fromModes = <String>[];
          final held = <String>[];
          final fromHolders = <String>[];
          for (final a in candidates) {
            final from = FakeI1Guard.holderOf(a);
            if (a.billingMode == mode && from == holder) continue;
            changed.add(a.id);
            fromModes.add(a.billingMode);
            if (from != null) {
              held.add(a.id);
              fromHolders.add(from);
            }
            guard.setPolicy(a, mode, holder);
          }
          // The details of policy.go `switched.attrs`.
          return (
            response: FakeResponse(200, {'changed_agent_ids': changed}),
            resourceId: null,
            details: {
              'to_billing_mode': mode,
              'reason': reason,
              if (changed.isEmpty) 'no_change': true,
              if (changed.isNotEmpty) ...{
                'agent_ids': changed,
                'from_billing_modes': fromModes,
              },
              if (held.isNotEmpty) ...{
                'held_agent_ids': held,
                'from_credential_holder_ids': fromHolders,
              },
              'to_credential_holder_id': ?holder,
            },
          );
        },
      );
    })
    ..handle('setCredentialHolder', (r) {
      owner(r);
      final a = agentOf(r.param('agent_id'));
      stepUp(r);
      ifMatch(r, a);
      if (a.billingMode != 'personal') {
        throw const FakeProblem(409, 'conflict');
      }
      final holder = r.json['credential_holder_id']! as String;
      validHolder(holder);
      final reason = r.json['reason']! as String;
      return guard.run(
        r,
        action: 'agent.credential_holder.set',
        resourceKind: 'agent',
        apply: () {
          final from = a.credentialHolderId;
          a
            ..credentialHolderId = holder
            ..version += 1;
          world.permissionVersion++;
          return (
            response: card(a),
            resourceId: a.id,
            details: {
              'from_credential_holder_id': ?from,
              'to_credential_holder_id': holder,
              'reason': reason,
            },
          );
        },
      );
    })
    ..handle('listTokens', (r) {
      owner(r);
      final a = tokenOwner(r);
      return FakeResponse(200, {
        'items': a.tokens.values.toList().reversed.toList(),
      });
    })
    ..handle('createToken', (r) {
      owner(r);
      final a = tokenOwner(r);
      stepUp(r);
      // 08 12: `auth.dpop_required_for` contains `agent` by default, so an agent token needs
      // the thumbprint of the adapter's key.
      if (r.json['dpop_jkt'] == null) {
        throw const FakeProblem(
          400,
          'validation_failed',
          errors: [
            {'pointer': '/dpop_jkt', 'code': 'required'},
          ],
        );
      }
      // ADR 0061 point 1: 43 Base62 characters of randomness, a CRC-32 over them in 6.
      final secretPart = [
        for (var i = 0; i < 43; i++) _base62[random.nextInt(62)],
      ].join();
      final secret =
          'bothy_agt_${secretPart}_${_base62Of(_crc32(secretPart), 6)}';
      final id = world.nextId();
      final token = <String, Object?>{
        'id': id,
        'principal_id': a.id,
        'name': r.json['name']! as String,
        'token_prefix': 'bothy_agt_${secretPart.substring(0, 6)}',
        'scopes': <Object?>[],
        'dpop_jkt': ?r.json['dpop_jkt'],
        'expires_at': r.json['expires_at']! as String,
        'created_at': world.stamp(),
      };
      a.tokens[id] = token;
      // The row names the token, never its secret (ADR 0061 point 3).
      audit.add(
        world,
        action: 'token.create',
        actor: r.person.id,
        resourceKind: 'token',
        resourceId: id,
        details: {
          'principal_id': a.id,
          'name': token['name'],
          'token_prefix': token['token_prefix'],
        },
      );
      return FakeResponse(201, {'token': token, 'secret': secret});
    })
    ..handle('revokeToken', (r) {
      owner(r);
      final id = r.param('token_id');
      final a = world.agents.values
          .where((a) => a.tokens.containsKey(id))
          .firstOrNull;
      if (a == null) throw const FakeProblem(404, 'not_found');
      a.tokens[id]!['revoked_at'] ??= world.stamp();
      audit.add(
        world,
        action: 'token.revoke',
        actor: r.person.id,
        resourceKind: 'token',
        resourceId: id,
        details: {'principal_id': a.id},
      );
      return const FakeResponse.noContent();
    });
}
