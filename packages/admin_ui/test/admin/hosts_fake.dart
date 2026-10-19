// SPDX-License-Identifier: Apache-2.0

// The host calls of the fake repository (brief M4-37 Scope and Tests), behaving like the
// server of 24 3 and 7: a pairing code per creation or re-pairing, a fingerprint whose first
// characters are compared after normalization, `terminal.fingerprint_max_attempts` failures
// and the fallback to `pairing`, I6 against the grants on the host, names only from the
// reported list, a name stays taken on its host after the assignment ends, and an audit of the
// assignments that `assignmentHistory` reads.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';

/// The fingerprint the fake host reports (24 4.3: 13 groups of four Crockford characters).
const String fakeFingerprint =
    '7KQ2-9XTA-M4PB-1C8D-0000-0000-0000-0000-0000-0000-0000-0000-0000';

/// The changing host calls and whether 05 gives their action step-up (`host.manage`,
/// `audit.read`).
const Map<String, bool> hostsStepUpOf = {
  'createHost': true,
  'newPairingCode': true,
  'confirmHost': true,
  'updateHost': true,
  'setLlmLogin': true,
  'setHostState': true,
  'assignmentHistory': true,
};

/// A host with the given parts and the defaults of an active, connected host.
AdminHost fakeHost(
  String id, {
  String? name,
  HostLifecycle state = HostLifecycle.active,
  String? assignedUserId,
  HostLlmLogin llmLogin = HostLlmLogin.organization,
  String? credentialHolderId,
  bool declaredServerHost = false,
  bool? reportedIsServerHost = false,
  String? keyFingerprint = fakeFingerprint,
  int keyGeneration = 1,
  List<HostDoctorEntry> doctor = const [],
  List<HostSessionUserInfo> sessionUsers = const [],
  List<HostReportedName>? reported,
  bool connected = true,
  bool ownerDisabled = false,
  int version = 1,
}) => AdminHost(
  id: id,
  name: name ?? id,
  state: state,
  ownerDisabled: ownerDisabled,
  platform: 'macos',
  assignedUserId: assignedUserId,
  llmLogin: llmLogin,
  credentialHolderId: credentialHolderId,
  declaredServerHost: declaredServerHost,
  reportedIsServerHost: reportedIsServerHost,
  keyFingerprint: keyFingerprint,
  keyGeneration: keyGeneration,
  profiles: const [
    HostProfileInfo(name: 'shell', displayName: 'Shell'),
    HostProfileInfo(name: 'claude', displayName: 'Claude Code'),
  ],
  doctor: doctor,
  sessionUsers: sessionUsers,
  reportedSessionUsers: reported,
  connected: connected,
  hostVersion: '1.2.0',
  osVersion: 'macOS 26',
  lastSeenAt: DateTime.utc(2026, 10, 17, 7, 58),
  createdAt: DateTime.utc(2026, 10, 1),
  version: version,
);

/// [h] with the given parts changed and the version raised.
AdminHost changedHost(
  AdminHost h, {
  HostLifecycle? state,
  bool? ownerDisabled,
  String? assignedUserId,
  bool clearAssignee = false,
  HostLlmLogin? llmLogin,
  String? credentialHolderId,
  bool clearHolder = false,
  bool? declaredServerHost,
  String? keyFingerprint,
  bool clearKeys = false,
  int? keyGeneration,
  List<HostSessionUserInfo>? sessionUsers,
  List<HostReportedName>? reported,
  List<HostDoctorEntry>? doctor,
  bool? connected,
}) => AdminHost(
  id: h.id,
  name: h.name,
  state: state ?? h.state,
  ownerDisabled: ownerDisabled ?? h.ownerDisabled,
  platform: h.platform,
  assignedUserId: clearAssignee ? null : assignedUserId ?? h.assignedUserId,
  llmLogin: llmLogin ?? h.llmLogin,
  credentialHolderId: clearHolder
      ? null
      : credentialHolderId ?? h.credentialHolderId,
  declaredServerHost: declaredServerHost ?? h.declaredServerHost,
  reportedIsServerHost: h.reportedIsServerHost,
  keyFingerprint: clearKeys ? null : keyFingerprint ?? h.keyFingerprint,
  keyGeneration: keyGeneration ?? h.keyGeneration,
  profiles: h.profiles,
  doctor: doctor ?? h.doctor,
  sessionUsers: sessionUsers ?? h.sessionUsers,
  reportedSessionUsers: reported ?? h.reportedSessionUsers,
  connected: connected ?? h.connected,
  hostVersion: h.hostVersion,
  osVersion: h.osVersion,
  lastSeenAt: h.lastSeenAt,
  createdAt: h.createdAt,
  version: h.version + 1,
);

/// The host calls of the fake; [change] is the step-up of the admin fake and [grantList] its
/// grants (for I6).
mixin FakeHosts {
  /// Records the change [op] and asks for the step-up as the server would.
  Future<void> change(String op, String args, String purpose);

  /// Throws the refusal of [op], if any.
  void refused(String op);

  /// The valid grants (of the access fake).
  List<AdminGrant> get grantList;

  /// The origin of the instance.
  String get serverOrigin => 'https://chat.example.com';

  /// What `getMe.is_owner` answers.
  bool callerOwner = true;

  /// The hosts by ID.
  final Map<String, AdminHost> hostsById = {};

  /// `terminal.fingerprint_max_attempts` of the fake (the reference default of 24 17).
  int maxAttempts = 3;

  /// Failed confirmations per host.
  final Map<String, int> failures = {};

  /// Every name ever assigned on a host, with its person (T12: a name stays taken).
  final Map<String, Map<String, String>> taken = {};

  /// The audit of the assignments per host: name to the person of its last assignment or end.
  final Map<String, Map<String, String>> audit = {};

  /// Open terminal sessions per host.
  final Map<String, int> sessionCounts = {};

  /// The pairing codes issued, in order.
  final List<String> pairingCodes = [];

  int _codes = 0;

  AdminHost _hostOf(String id) =>
      hostsById[id] ??
      (throw const SelfActionFailed(
        ApiProblem(status: 404, code: 'not_found'),
      ));

  HostPairingCodeInfo _code(String hostId) {
    final code =
        'bothy_hpc_${'A' * 40}${(_codes++).toString().padLeft(3, '0')}_ABCDEF';
    pairingCodes.add(code);
    return HostPairingCodeInfo(
      hostId: hostId,
      code: code,
      expiresAt: DateTime.utc(2026, 10, 17, 9, 14, 12),
    );
  }

  /// Simulates `bothy-host pair` on [hostId]: the keys with [fingerprint] and the reported
  /// [names]; the host becomes `confirming` and its key generation rises.
  void simulatePair(
    String hostId,
    List<String> names, {
    String fingerprint = fakeFingerprint,
  }) {
    final h = _hostOf(hostId);
    hostsById[hostId] = changedHost(
      h,
      state: HostLifecycle.confirming,
      keyFingerprint: fingerprint,
      keyGeneration: h.keyGeneration + 1,
      reported: [for (final n in names) HostReportedName(n)],
      connected: true,
    );
  }

  /// I6 (24 7.3): with [login] personal, every valid grant on [hostId] must name [holder].
  void _i6(String hostId, HostLlmLogin login, String? holder) {
    if (login != HostLlmLogin.personal) return;
    final others = [
      for (final g in grantList)
        if (g.targetKind == AdminTargetKind.host &&
            g.targetId == hostId &&
            (g.subjectKind != AdminSubjectKind.user || g.subjectId != holder))
          g,
    ];
    if (others.isNotEmpty) {
      throw const SelfActionFailed(
        ApiProblem(
          status: 409,
          code: 'i6_violation',
          hint: 'Revoke the other grants or declare organization first.',
        ),
      );
    }
  }

  /// The checks of new assignments (24 2.2, T12): reported names only, a name taken by
  /// another person or a second valid assignment of one person is `session_user_taken`.
  void _checkAssignments(AdminHost h, List<HostAssignment> add) {
    final reported = {
      for (final r in h.reportedSessionUsers ?? const <HostReportedName>[])
        r.osUser,
    };
    for (final a in add) {
      if (!reported.contains(a.osUser)) {
        throw const SelfActionFailed(
          ApiProblem(
            status: 400,
            code: 'validation_failed',
            errors: [
              ProblemFieldError(
                pointer: '/session_users/0/os_user',
                code: 'session_user_not_on_host',
              ),
            ],
          ),
        );
      }
      final owner = taken[h.id]?[a.osUser];
      final second = h.sessionUsers.any((s) => s.userId == a.userId);
      if ((owner != null && owner != a.userId) || second) {
        throw const SelfActionFailed(
          ApiProblem(
            status: 409,
            code: 'conflict',
            reason: 'session_user_taken',
          ),
        );
      }
    }
  }

  List<HostSessionUserInfo> _assigned(AdminHost h, List<HostAssignment> add) {
    for (final a in add) {
      (taken[h.id] ??= {})[a.osUser] = a.userId;
      (audit[h.id] ??= {})[a.osUser] = a.userId;
    }
    return [
      ...h.sessionUsers,
      for (final a in add)
        HostSessionUserInfo(
          userId: a.userId,
          osUser: a.osUser,
          createdAt: DateTime.utc(2026, 10, 17, 9),
        ),
    ];
  }

  /// Whether the caller is an owner.
  Future<bool> callerIsOwner() async {
    refused('callerIsOwner');
    return callerOwner;
  }

  /// Every host by name.
  Future<List<AdminHost>> hosts() async {
    refused('hosts');
    return hostsById.values.toList()..sort((a, b) => a.name.compareTo(b.name));
  }

  /// One host.
  Future<AdminHost> host(String hostId) async {
    refused('host');
    return _hostOf(hostId);
  }

  /// Creates a host in `pairing` and returns its code.
  Future<HostPairingCodeInfo> createHost({
    required String name,
    String? assigneeId,
    required String reason,
    required String purpose,
  }) async {
    await change('createHost', '$name ${assigneeId ?? '-'} $reason', purpose);
    final id = name.toLowerCase().replaceAll(' ', '-');
    hostsById[id] = AdminHost(
      id: id,
      name: name,
      state: HostLifecycle.pairing,
      assignedUserId: assigneeId,
      llmLogin: HostLlmLogin.none,
      declaredServerHost: false,
      keyGeneration: 0,
      connected: false,
      createdAt: DateTime.utc(2026, 10, 17, 9),
      version: 1,
    );
    return _code(id);
  }

  /// A new code; from `confirming`, `active` and `disabled` the keys are dropped (24 3.3).
  Future<HostPairingCodeInfo> newPairingCode(
    String hostId, {
    required String reason,
    required String purpose,
  }) async {
    await change('newPairingCode', '$hostId $reason', purpose);
    final h = _hostOf(hostId);
    if (h.state == HostLifecycle.retired) {
      throw const SelfActionFailed(
        ApiProblem(status: 409, code: 'invalid_state_transition'),
      );
    }
    hostsById[hostId] = changedHost(
      h,
      state: HostLifecycle.pairing,
      clearKeys: true,
      connected: false,
    );
    failures.remove(hostId);
    return _code(hostId);
  }

  /// Confirms the host as the server does (24 3.2, 7.3; TM-T19, TM-T28).
  Future<AdminHost> confirmHost(
    AdminHost host,
    HostConfirmRequest request, {
    required String purpose,
  }) async {
    await change(
      'confirmHost',
      '${host.id} ${request.fingerprintPrefix} ${request.llmLogin.wire} '
          '${request.declaredServerHost} '
          '${[for (final a in request.sessionUsers) '${a.userId}=${a.osUser}'].join(',')} '
          '${request.reason}',
      purpose,
    );
    final h = _hostOf(host.id);
    if (h.state != HostLifecycle.confirming) {
      throw const SelfActionFailed(
        ApiProblem(status: 409, code: 'invalid_state_transition'),
      );
    }
    final prefix = normalizeFingerprint(request.fingerprintPrefix);
    if (prefix.length != fingerprintConfirmCharsDefault) {
      throw const SelfActionFailed(
        ApiProblem(
          status: 400,
          code: 'validation_failed',
          errors: [
            ProblemFieldError(
              pointer: '/fingerprint_prefix',
              code: 'invalid_format',
            ),
          ],
        ),
      );
    }
    if (!normalizeFingerprint(h.keyFingerprint ?? '').startsWith(prefix)) {
      final failed = (failures[h.id] ?? 0) + 1;
      final left = maxAttempts - failed;
      if (left <= 0) {
        failures.remove(h.id);
        hostsById[h.id] = changedHost(
          h,
          state: HostLifecycle.pairing,
          clearKeys: true,
          connected: false,
        );
      } else {
        failures[h.id] = failed;
      }
      throw FingerprintMismatch(
        left < 0 ? 0 : left,
        const ApiProblem(status: 409, code: 'fingerprint_mismatch'),
      );
    }
    _i6(h.id, request.llmLogin, request.credentialHolderId);
    _checkAssignments(h, request.sessionUsers);
    failures.remove(h.id);
    return hostsById[h.id] = changedHost(
      h,
      state: HostLifecycle.active,
      llmLogin: request.llmLogin,
      credentialHolderId: request.credentialHolderId,
      clearHolder: request.credentialHolderId == null,
      declaredServerHost: request.declaredServerHost,
      sessionUsers: _assigned(h, request.sessionUsers),
    );
  }

  /// Changes the assignee or the session users.
  Future<AdminHost> updateHost(
    AdminHost host, {
    String? assigneeId,
    bool clearAssignee = false,
    List<HostAssignment> add = const [],
    List<String> revoke = const [],
    required String reason,
    required String purpose,
  }) async {
    await change(
      'updateHost',
      '${host.id} ${assigneeId ?? '-'} $clearAssignee '
          '${[for (final a in add) '${a.userId}=${a.osUser}'].join(',')} '
          '${revoke.join(',')} $reason',
      purpose,
    );
    final h = _hostOf(host.id);
    _checkAssignments(h, add);
    final kept = [
      for (final s in h.sessionUsers)
        if (!revoke.contains(s.userId)) s,
    ];
    for (final s in h.sessionUsers) {
      if (revoke.contains(s.userId)) (audit[h.id] ??= {})[s.osUser] = s.userId;
    }
    final base = changedHost(h, sessionUsers: kept);
    return hostsById[h.id] = changedHost(
      base,
      assignedUserId: assigneeId,
      clearAssignee: clearAssignee,
      sessionUsers: _assigned(base, add),
    );
  }

  /// Changes the declaration of the LLM login; I6 is checked.
  Future<AdminHost> setLlmLogin(
    AdminHost host,
    HostLlmLogin login, {
    String? holderId,
    required String reason,
    required String purpose,
  }) async {
    await change(
      'setLlmLogin',
      '${host.id} ${login.wire} ${holderId ?? '-'} $reason',
      purpose,
    );
    _i6(host.id, login, holderId);
    final h = _hostOf(host.id);
    return hostsById[h.id] = changedHost(
      h,
      llmLogin: login,
      credentialHolderId: holderId,
      clearHolder: login != HostLlmLogin.personal,
    );
  }

  /// Disables, enables or retires.
  Future<AdminHost> setHostState(
    AdminHost host,
    HostStateRequest state, {
    required String reason,
    required String purpose,
  }) async {
    await change('setHostState', '${host.id} ${state.name} $reason', purpose);
    final h = _hostOf(host.id);
    return hostsById[h.id] = changedHost(
      h,
      state: switch (state) {
        HostStateRequest.active => HostLifecycle.active,
        HostStateRequest.disabled => HostLifecycle.disabled,
        HostStateRequest.retired => HostLifecycle.retired,
      },
      ownerDisabled: state == HostStateRequest.disabled,
    );
  }

  /// The audit of the assignments (step-up: `audit.read`).
  Future<Map<String, String>> assignmentHistory(
    String hostId, {
    required String purpose,
  }) async {
    await change('assignmentHistory', hostId, purpose);
    return Map.of(audit[hostId] ?? const {});
  }

  /// Open sessions.
  Future<int> openSessions(String hostId) async {
    refused('openSessions');
    return sessionCounts[hostId] ?? 0;
  }
}
