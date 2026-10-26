// SPDX-License-Identifier: Apache-2.0

// An AdminRepository that behaves like the server behind the step-up interceptor (brief M4-26
// Design note 3): the changing calls whose action has step-up in 05 ask the prompt while no
// confirmation is fresh, `revokeSession` never asks; enrollment codes are one-time secrets,
// the last owner stays (SR8), disabling ends sessions and devices (08 11), enabling may meet
// I1. The step-up enumeration against the real table of actions.yaml runs in apps/client.
// The audit log (brief M4-29) holds entries newest first per page, a chain that can be broken
// at a given entry and the NDJSON bytes of the export, which can break off after a line.

import 'dart:async';
import 'dart:convert';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';

import 'access_fake.dart';
import 'agents_fake.dart';
import 'apps_fake.dart';
import 'hosts_fake.dart';
import 'import_fake.dart';
import 'instance_fake.dart';
import 'stats_fake.dart';
import 'status_fake.dart';

export '../self/support.dart' show pumpInApp, testClock, testNow;
export 'access_fake.dart';
export 'agents_fake.dart';
export 'apps_fake.dart';
export 'hosts_fake.dart';
export 'import_fake.dart';
export 'instance_fake.dart';
export 'stats_fake.dart';
export 'status_fake.dart';

/// The changing calls of this fake and whether 05 gives their action step-up.
const Map<String, bool> stepUpOf = {
  'createPerson': true,
  'disablePerson': true,
  'enablePerson': true,
  'newEnrollmentCode': true,
  'unlinkIdentity': true,
  'revokePasskey': true,
  'approveDevice': true,
  'revokeDevice': true,
  'addOwner': true,
  'revokeOwner': true,
  'createGroup': true,
  'renameGroup': true,
  'deleteGroup': true,
  'addMember': true,
  'removeMember': true,
  'endSession': false,
  ...accessStepUpOf,
  // 05: audit.read (list and export) and audit.verify have step-up.
  'auditEvents': true,
  'exportAudit': true,
  'verifyAuditChain': true,
  ...hostsStepUpOf,
  ...agentsStepUpOf,
  ...appsStepUpOf,
  ...instanceStepUpOf,
  ...importStepUpOf,
};

/// The audit entries of the fake: Alex created a grant, Sam was denied an agent run, Alex
/// changed an owner with a reason that starts like a formula; seq 1 to 3, oldest first.
List<AuditEntry> auditFixture() => [
  AuditEntry(
    seq: 1,
    id: 'e1',
    occurredAt: DateTime.utc(2026, 10, 16, 7, 58),
    actorId: 'alex',
    action: 'grant.create',
    result: AuditResult.allow,
    resourceKind: 'space',
    resourceId: '0192f4a0-7b1c-7000-8000-000000000101',
    details: const {'reason': 'Deputy, Q4'},
    correlationId: '4bf92f35-77b3-4da6-a3ce-000000000001',
    prevHash: '0' * 64,
    hash: 'a1' * 32,
  ),
  AuditEntry(
    seq: 2,
    id: 'e2',
    occurredAt: DateTime.utc(2026, 10, 17, 6, 12),
    actorId: 'sam',
    action: 'agent.invoke',
    result: AuditResult.deny,
    denyReason: 'missing_grant',
    resourceKind: 'run',
    resourceId: '0192f4a0-7b1c-7000-8000-000000000202',
    details: const {'note': 'said "no"'},
    correlationId: '4bf92f35-77b3-4da6-a3ce-000000000002',
    prevHash: 'a1' * 32,
    hash: 'b2' * 32,
  ),
  AuditEntry(
    seq: 3,
    id: 'e3',
    occurredAt: DateTime.utc(2026, 10, 17, 8),
    actorId: 'alex',
    action: 'owner.add',
    result: AuditResult.allow,
    resourceKind: 'user',
    resourceId: '0192f4a0-7b1c-7000-8000-000000000303',
    details: const {'reason': '=HYPERLINK("x")'},
    correlationId: '4bf92f35-77b3-4da6-a3ce-000000000003',
    prevHash: 'b2' * 32,
    hash: 'c3' * 32,
  ),
];

/// [entry] as one line of the server's NDJSON export (`AuditEvent`).
Map<String, Object?> auditJson(AuditEntry entry) => {
  'seq': entry.seq,
  'id': entry.id,
  'occurred_at': entry.occurredAt.toIso8601String(),
  'actor_principal_id': ?entry.actorId,
  'action': entry.action,
  'decision': entry.result.name,
  'deny_reason': ?entry.denyReason,
  'resource_kind': ?entry.resourceKind,
  'resource_id': ?entry.resourceId,
  'details': entry.details,
  'correlation_id': ?entry.correlationId,
  'prev_hash': entry.prevHash,
  'hash': entry.hash,
};

AdminPerson _p(
  String id,
  String name, {
  bool owner = false,
  AdminPersonStatus status = AdminPersonStatus.active,
}) => AdminPerson(
  id: id,
  handle: name.toLowerCase(),
  displayName: name,
  status: status,
  isOwner: owner,
  createdAt: DateTime.utc(2026, 10),
);

/// The fake repository with Alex (owner), Sam and Robin; Sam has a waiting tablet that logged
/// in with OIDC, an active laptop, a passkey and a linked identity; group "Support" holds
/// Robin until the end of the year. The grant calls come from [FakeAccess], the host calls
/// from [FakeHosts].
class FakeAdminRepository
    with
        FakeAccess,
        FakeHosts,
        FakeAgents,
        FakeApps,
        FakeInstance,
        FakeImport,
        FakeStats,
        FakeStatus
    implements AdminRepository {
  /// Creates the fake; [prompt] is asked as the step-up interceptor would.
  FakeAdminRepository({required this.prompt});

  /// The step-up prompt.
  final StepUpPrompt prompt;

  /// The people by ID.
  final Map<String, AdminPerson> peopleById = {
    'alex': _p('alex', 'Alex', owner: true),
    'sam': _p('sam', 'Sam'),
    'robin': _p('robin', 'Robin'),
  };

  /// The devices per person.
  final Map<String, List<SelfDevice>> devices = {
    'sam': [
      SelfDevice(
        id: 'sam-laptop',
        name: 'Sam laptop',
        platform: 'windows',
        state: SelfDeviceState.active,
        createdAt: DateTime.utc(2026, 10, 2),
        lastSeenAt: DateTime.utc(2026, 10, 16, 18),
      ),
      SelfDevice(
        id: 'sam-tablet',
        name: 'Pixel',
        platform: 'android',
        state: SelfDeviceState.pending,
        createdAt: DateTime.utc(2026, 10, 17, 8, 12),
      ),
    ],
  };

  /// The live sessions per person.
  final Map<String, List<SelfSession>> sessions = {
    'sam': [
      SelfSession(
        id: 's-laptop',
        deviceId: 'sam-laptop',
        deviceName: 'Sam laptop',
        authMethod: SelfAuthMethod.passwordTotp,
        lastUsedAt: DateTime.utc(2026, 10, 16, 18),
      ),
      SelfSession(
        id: 's-tablet',
        deviceId: 'sam-tablet',
        deviceName: 'Pixel',
        authMethod: SelfAuthMethod.oidc,
        lastUsedAt: DateTime.utc(2026, 10, 17, 8, 12),
      ),
    ],
  };

  /// The verification code per waiting device.
  final Map<String, String> codes = {'sam-tablet': '481207'};

  /// The credentials per person.
  final Map<String, AdminCredentials> credentialsById = {
    'sam': AdminCredentials(
      identities: [
        AdminIdentity(
          id: 'id-sam',
          providerKind: 'oidc',
          subject: 'sam',
          issuer: 'https://login.example.com',
          linkedAt: DateTime.utc(2026, 10, 2),
        ),
      ],
      passkeys: [
        AdminPasskey(
          id: 'pk-sam',
          name: 'Sam laptop',
          createdAt: DateTime.utc(2026, 10, 3),
        ),
      ],
      localPasswordSet: true,
      totpEnrolled: true,
    ),
  };

  /// The owners.
  final List<String> ownerIds = ['alex'];

  /// The groups by ID.
  final Map<String, AdminGroup> groupsById = {
    'support': const AdminGroup(
      id: 'support',
      name: 'Support',
      description: 'Support team',
      memberCount: 1,
    ),
  };

  /// The members per group.
  final Map<String, List<AdminGroupMember>> membersById = {
    'support': [
      AdminGroupMember(
        userId: 'robin',
        displayName: 'Robin',
        addedAt: DateTime.utc(2026, 10, 5),
        expiresAt: DateTime.utc(2026, 12, 31),
      ),
    ],
  };

  /// People per page of `people` (an offset cursor).
  int pageSize = 50;

  /// People whose enabling answers `409 i1_violation`.
  final Set<String> i1 = {};

  /// Calls the server refuses, by name, with their problem (lists included).
  final Map<String, ApiProblem> refuse = {};

  /// The audit entries, oldest first.
  List<AuditEntry> audit = auditFixture();

  /// Audit entries per page.
  int auditPageSize = 50;

  /// The `listAuditEvents` requests: the filter and the cursor, as text.
  final List<String> auditRequests = [];

  /// The `verifyAuditChain` requests: `from..to` (empty for no range).
  final List<String> verifyRequests = [];

  /// The `exportAudit` requests: `from..to`.
  final List<String> exportRequests = [];

  /// The entry at which the chain is broken, or null for an intact chain.
  int? brokenAt;

  /// The warnings of every chain check.
  List<String> chainWarnings = ['checkpoint_sink_not_configured'];

  /// When set, the export stream breaks off after this many lines.
  int? exportBreaksAfter;

  /// The bytes of the last export, as the server sent them.
  List<int> exportedBytes = const [];

  /// Whether a step-up of the session is fresh.
  bool stepUpFresh = false;

  /// The purpose texts of the step-ups asked so far.
  final List<String> stepUps = [];

  /// The changing calls in order, with their arguments.
  final List<String> calls = [];

  /// The `listUsers` requests: query and status.
  final List<String> listRequests = [];

  int _codes = 0;

  void _refused(String op) {
    final problem = refuse[op];
    if (problem != null) throw SelfActionFailed(problem);
  }

  @override
  void refused(String op) => _refused(op);

  @override
  Future<void> change(String op, String args, String purpose) =>
      _change(op, args, purpose);

  @override
  Future<void> read(String op, String purpose) => _read(op, purpose);

  @override
  String nameOf(String userId) => peopleById[userId]?.displayName ?? userId;

  Future<void> _change(String op, String args, String purpose) async {
    calls.add('$op $args'.trim());
    _refused(op);
    if (stepUpOf[op]! && !stepUpFresh) {
      stepUps.add(purpose);
      if (!await prompt.ask(purpose)) throw const StepUpCancelled();
      stepUpFresh = true;
    }
  }

  Future<void> _read(String op, String purpose) async {
    _refused(op);
    if (stepUpOf[op]! && !stepUpFresh) {
      stepUps.add(purpose);
      if (!await prompt.ask(purpose)) throw const StepUpCancelled();
      stepUpFresh = true;
    }
  }

  static String _day(DateTime? t) => t?.toUtc().toIso8601String() ?? '';

  @override
  Future<AuditPage> auditEvents(
    AuditFilter filter, {
    String? cursor,
    required String purpose,
  }) async {
    auditRequests.add(
      [
        'action=${filter.action ?? ''}',
        'actor=${filter.actorId ?? ''}',
        'decision=${filter.result?.name ?? ''}',
        'kind=${filter.resourceKind ?? ''}',
        'id=${filter.resourceId ?? ''}',
        'from=${_day(filter.from)}',
        'to=${_day(filter.to)}',
        'cursor=${cursor ?? ''}',
      ].join(' '),
    );
    await _read('auditEvents', purpose);
    final matching = [
      for (final e in audit.reversed)
        if ((filter.action == null || e.action == filter.action) &&
            (filter.actorId == null || e.actorId == filter.actorId) &&
            (filter.result == null || e.result == filter.result) &&
            (filter.resourceKind == null ||
                e.resourceKind == filter.resourceKind) &&
            (filter.resourceId == null || e.resourceId == filter.resourceId) &&
            (filter.from == null || !e.occurredAt.isBefore(filter.from!)) &&
            (filter.to == null || e.occurredAt.isBefore(filter.to!)))
          e,
    ];
    final start = int.tryParse(cursor ?? '') ?? 0;
    final end = (start + auditPageSize).clamp(0, matching.length);
    return AuditPage(
      matching.sublist(start, end),
      nextCursor: end < matching.length ? '$end' : null,
    );
  }

  @override
  Future<AuditChainCheck> verifyAuditChain({
    int? fromSeq,
    int? toSeq,
    required String purpose,
  }) async {
    verifyRequests.add(
      fromSeq == null && toSeq == null ? '' : '$fromSeq..$toSeq',
    );
    await _read('verifyAuditChain', purpose);
    final from = fromSeq ?? 1;
    final to = toSeq ?? audit.length;
    final broken = brokenAt;
    return AuditChainCheck(
      ok: broken == null,
      checkedFromSeq: from,
      checkedToSeq: broken ?? to,
      firstBrokenSeq: broken,
      failureReason: broken == null ? null : 'hash_mismatch',
      checkpointsCompared: 2,
      lastCheckpointSeq: 1,
      warnings: List.of(chainWarnings),
    );
  }

  @override
  Stream<List<int>> exportAudit({
    required DateTime from,
    required DateTime to,
    required String purpose,
  }) async* {
    exportRequests.add('${_day(from)}..${_day(to)}');
    await _read('exportAudit', purpose);
    final lines = [
      for (final e in audit)
        if (!e.occurredAt.isBefore(from) && e.occurredAt.isBefore(to))
          utf8.encode('${jsonEncode(auditJson(e))}\n'),
    ];
    exportedBytes = [for (final l in lines) ...l];
    for (final (i, line) in lines.indexed) {
      if (exportBreaksAfter case final n? when i >= n) {
        throw const SelfActionFailed(null);
      }
      // Two chunks per line, so a consumer must join lines across chunks.
      yield line.sublist(0, line.length ~/ 2);
      yield line.sublist(line.length ~/ 2);
    }
  }

  AdminPerson _with(AdminPerson p, {AdminPersonStatus? status, bool? owner}) =>
      AdminPerson(
        id: p.id,
        handle: p.handle,
        displayName: p.displayName,
        status: status ?? p.status,
        isOwner: owner ?? p.isOwner,
        createdAt: p.createdAt,
      );

  EnrollmentCodeInfo _code(EnrollmentPurpose purpose) => EnrollmentCodeInfo(
    code: 'en_4KX9-2MQA-${(_codes++).toString().padLeft(4, '0')}',
    purpose: purpose,
    expiresAt: DateTime.utc(2026, 10, 24, 9),
  );

  @override
  Future<AdminPage<AdminPerson>> people({
    String? query,
    AdminPersonStatus? status,
    String? cursor,
  }) async {
    listRequests.add('${query ?? ''}|${status?.name ?? ''}');
    _refused('people');
    final q = query?.toLowerCase() ?? '';
    final all = [
      for (final p in peopleById.values)
        if ((status == null || p.status == status) &&
            (q.isEmpty || p.displayName.toLowerCase().contains(q)))
          p,
    ];
    final start = int.tryParse(cursor ?? '') ?? 0;
    final end = (start + pageSize).clamp(0, all.length);
    return AdminPage(
      all.sublist(start, end),
      nextCursor: end < all.length ? '$end' : null,
    );
  }

  @override
  Future<AdminPerson> person(String userId) async {
    _refused('person');
    return peopleById[userId] ??
        (throw const SelfActionFailed(
          ApiProblem(status: 404, code: 'not_found'),
        ));
  }

  @override
  Future<PersonCreated> createPerson({
    required String handle,
    required String displayName,
    required bool localAccount,
    required String purpose,
  }) async {
    await _change('createPerson', handle, purpose);
    final p = AdminPerson(
      id: handle,
      handle: handle,
      displayName: displayName,
      status: AdminPersonStatus.pending,
      isOwner: false,
      createdAt: testNowValue,
    );
    peopleById[p.id] = p;
    return PersonCreated(
      p,
      enrollment: localAccount ? _code(EnrollmentPurpose.firstLogin) : null,
    );
  }

  @override
  Future<AdminPerson> disablePerson(
    String userId, {
    required String reason,
    required String purpose,
  }) async {
    await _change('disablePerson', '$userId $reason', purpose);
    final p = peopleById[userId]!;
    if (p.isOwner && ownerIds.length == 1) {
      throw const SelfActionFailed(ApiProblem(status: 409, code: 'last_owner'));
    }
    sessions.remove(userId);
    devices[userId] = [
      for (final d in devices[userId] ?? const <SelfDevice>[])
        SelfDevice(
          id: d.id,
          name: d.name,
          platform: d.platform,
          state: SelfDeviceState.revoked,
          createdAt: d.createdAt,
        ),
    ];
    return peopleById[userId] = _with(p, status: AdminPersonStatus.disabled);
  }

  @override
  Future<AdminPerson> enablePerson(
    String userId, {
    required String reason,
    required String purpose,
  }) async {
    await _change('enablePerson', '$userId $reason', purpose);
    if (i1.contains(userId)) {
      throw const SelfActionFailed(
        ApiProblem(status: 409, code: 'i1_violation'),
      );
    }
    return peopleById[userId] = _with(
      peopleById[userId]!,
      status: AdminPersonStatus.active,
    );
  }

  @override
  Future<EnrollmentCodeInfo> newEnrollmentCode(
    String userId, {
    required String purpose,
  }) async {
    await _change('newEnrollmentCode', userId, purpose);
    return _code(EnrollmentPurpose.recovery);
  }

  @override
  Future<AdminCredentials> credentials(String userId) async {
    _refused('credentials');
    return credentialsById[userId] ??
        const AdminCredentials(
          identities: [],
          passkeys: [],
          localPasswordSet: false,
          totpEnrolled: false,
        );
  }

  @override
  Future<void> unlinkIdentity(
    String userId,
    String identityId, {
    required String purpose,
  }) async {
    await _change('unlinkIdentity', '$userId $identityId', purpose);
    final c = credentialsById[userId]!;
    credentialsById[userId] = AdminCredentials(
      identities: [
        for (final i in c.identities)
          if (i.id != identityId) i,
      ],
      passkeys: c.passkeys,
      localPasswordSet: c.localPasswordSet,
      totpEnrolled: c.totpEnrolled,
    );
  }

  @override
  Future<void> revokePasskey(
    String userId,
    String passkeyId, {
    required String purpose,
  }) async {
    await _change('revokePasskey', '$userId $passkeyId', purpose);
    final c = credentialsById[userId]!;
    credentialsById[userId] = AdminCredentials(
      identities: c.identities,
      passkeys: [
        for (final p in c.passkeys)
          if (p.id != passkeyId) p,
      ],
      localPasswordSet: c.localPasswordSet,
      totpEnrolled: c.totpEnrolled,
    );
  }

  @override
  Future<List<SelfDevice>> devicesOf(String userId) async {
    _refused('devicesOf');
    return List.of(devices[userId] ?? const []);
  }

  @override
  Future<List<SelfSession>> sessionsOf(String userId) async {
    _refused('sessionsOf');
    return List.of(sessions[userId] ?? const []);
  }

  @override
  Future<void> endSession(String sessionId) async {
    await _change('endSession', sessionId, '');
    for (final entry in sessions.entries) {
      entry.value.removeWhere((s) => s.id == sessionId);
    }
  }

  void _setDevice(String deviceId, SelfDeviceState state) {
    for (final entry in devices.entries) {
      entry.value.replaceRange(0, entry.value.length, [
        for (final d in entry.value)
          d.id == deviceId
              ? SelfDevice(
                  id: d.id,
                  name: d.name,
                  platform: d.platform,
                  state: state,
                  createdAt: d.createdAt,
                )
              : d,
      ]);
    }
  }

  SelfDevice _device(String id) =>
      devices.values.expand((d) => d).firstWhere((d) => d.id == id);

  @override
  Future<SelfDevice> approveDevice(
    String deviceId,
    String code, {
    required String purpose,
  }) async {
    await _change('approveDevice', '$deviceId $code', purpose);
    _setDevice(
      deviceId,
      codes[deviceId] == code
          ? SelfDeviceState.active
          : SelfDeviceState.revoked,
    );
    return _device(deviceId);
  }

  @override
  Future<SelfDevice> revokeDevice(
    String deviceId, {
    required String reason,
    required String purpose,
  }) async {
    await _change('revokeDevice', '$deviceId $reason', purpose);
    _setDevice(deviceId, SelfDeviceState.revoked);
    return _device(deviceId);
  }

  @override
  Future<List<WaitingDevice>> waitingDevices() async {
    final out = <WaitingDevice>[];
    for (final p in (await people()).items) {
      final pending = [
        for (final d in await devicesOf(p.id))
          if (d.state == SelfDeviceState.pending) d,
      ];
      final s = await sessionsOf(p.id);
      for (final d in pending) {
        out.add(
          WaitingDevice(
            person: p,
            device: d,
            origin: s.where((x) => x.deviceId == d.id).firstOrNull?.authMethod,
          ),
        );
      }
    }
    return out;
  }

  @override
  Future<List<AdminOwner>> owners() async {
    _refused('owners');
    return [
      for (final id in ownerIds)
        AdminOwner(
          userId: id,
          displayName: peopleById[id]!.displayName,
          grantedAt: DateTime.utc(2026, 10),
          reason: 'bootstrap',
        ),
    ];
  }

  @override
  Future<void> addOwner(
    String userId, {
    required String reason,
    required String purpose,
    I1Resolution? resolveI1,
  }) async {
    await _change('addOwner', '$userId $reason', purpose);
    i1Guard('addOwner', resolveI1);
    ownerIds.add(userId);
    peopleById[userId] = _with(peopleById[userId]!, owner: true);
  }

  @override
  Future<void> revokeOwner(
    String userId, {
    required String reason,
    required String purpose,
  }) async {
    await _change('revokeOwner', '$userId $reason', purpose);
    if (ownerIds.length == 1) {
      throw const SelfActionFailed(ApiProblem(status: 409, code: 'last_owner'));
    }
    ownerIds.remove(userId);
    peopleById[userId] = _with(peopleById[userId]!, owner: false);
  }

  AdminGroup _counted(AdminGroup g) => AdminGroup(
    id: g.id,
    name: g.name,
    description: g.description,
    memberCount: membersById[g.id]?.length ?? 0,
  );

  @override
  Future<AdminPage<AdminGroup>> groups({String? cursor}) async {
    _refused('groups');
    return AdminPage([for (final g in groupsById.values) _counted(g)]);
  }

  @override
  Future<AdminGroup> group(String groupId) async {
    _refused('group');
    return _counted(
      groupsById[groupId] ??
          (throw const SelfActionFailed(
            ApiProblem(status: 404, code: 'not_found'),
          )),
    );
  }

  @override
  Future<AdminGroup> createGroup(
    String name, {
    required String description,
    required String purpose,
  }) async {
    await _change('createGroup', name, purpose);
    final id = name.toLowerCase();
    membersById[id] = [];
    return groupsById[id] = AdminGroup(
      id: id,
      name: name,
      description: description,
      memberCount: 0,
    );
  }

  @override
  Future<AdminGroup> renameGroup(
    String groupId,
    String name, {
    required String purpose,
  }) async {
    await _change('renameGroup', '$groupId $name', purpose);
    final g = groupsById[groupId]!;
    return groupsById[groupId] = AdminGroup(
      id: g.id,
      name: name,
      description: g.description,
      memberCount: g.memberCount,
    );
  }

  @override
  Future<void> deleteGroup(String groupId, {required String purpose}) async {
    await _change('deleteGroup', groupId, purpose);
    groupsById.remove(groupId);
    membersById.remove(groupId);
  }

  @override
  Future<List<AdminGroupMember>> members(String groupId) async {
    _refused('members');
    return List.of(membersById[groupId] ?? const []);
  }

  @override
  Future<void> addMember(
    String groupId,
    String userId, {
    DateTime? expiresAt,
    required String purpose,
    I1Resolution? resolveI1,
  }) async {
    await _change(
      'addMember',
      '$groupId $userId ${expiresAt?.toIso8601String() ?? ''}',
      purpose,
    );
    i1Guard('addMember', resolveI1);
    membersById[groupId]!.add(
      AdminGroupMember(
        userId: userId,
        displayName: peopleById[userId]!.displayName,
        addedAt: testNowValue,
        expiresAt: expiresAt,
      ),
    );
  }

  @override
  Future<void> removeMember(
    String groupId,
    String userId, {
    required String purpose,
  }) async {
    await _change('removeMember', '$groupId $userId', purpose);
    membersById[groupId]!.removeWhere((m) => m.userId == userId);
  }
}

/// The fixed time of these tests (the same as `testNow` of the self tests).
final DateTime testNowValue = DateTime.utc(2026, 10, 17, 9);

/// A prompt that answers [answer] and records what it was asked.
class RecordingPrompt implements StepUpPrompt {
  /// Creates the prompt.
  RecordingPrompt({this.answer = true});

  /// What [ask] answers.
  bool answer;

  /// The purposes asked.
  final List<String> asked = [];

  @override
  Future<bool> ask(String purposeText) async {
    asked.add(purposeText);
    return answer;
  }
}
