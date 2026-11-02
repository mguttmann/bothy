// SPDX-License-Identifier: Apache-2.0

/// Hosts of the terminal, their pairing and their session users over REST
/// (docs/spec/01-product.md 5.2 A8; 24 2.2, 2.3, 3, 4.3, 7, 8; brief M4-37 Design notes 1 to 3).
///
/// The types carry what the server answers, unchanged: the client never decides who may open
/// a terminal (15 1 principle 1). Whether a grant takes effect comes from the preview of the
/// grant calls ([AccessRepository]), never from these types.
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';

import 'access_repository.dart';
import 'self_repository.dart';

/// The reference default of `terminal.fingerprint_confirm_chars` (24 17). The server sends
/// neither this value nor a hint of it before the first attempt; a prefix of another length
/// answers `400 validation_failed` (`invalid_format`), which the screen shows. The constant
/// only names the number in the field's help text.
const int fingerprintConfirmCharsDefault = 8;

/// The life cycle of a host (`HostState`, 24 3.1).
enum HostLifecycle {
  /// Waiting for `bothy-host pair` with a pairing code.
  pairing,

  /// The host reported its keys; an owner confirms the fingerprint.
  confirming,

  /// Sessions are allowed.
  active,

  /// Disabled by an owner or after a second control connection (24 3.4).
  disabled,

  /// Retired for good.
  retired,

  /// A state this build does not know.
  unknown,
}

/// The declaration of the LLM login on a host (`LlmCredentials`, 24 7.2).
enum HostLlmLogin {
  /// No LLM login on the host.
  none('none'),

  /// The personal subscription of exactly one person (I6).
  personal('personal'),

  /// An organization API key of the host service.
  organization('organization');

  const HostLlmLogin(this.wire);

  /// The name on the wire, for example `personal`.
  final String wire;
}

/// The state an owner requests with `setHostState` (`HostStateSet.state`).
enum HostStateRequest {
  /// Enable again (not after `duplicate_connection`).
  active,

  /// Disable.
  disabled,

  /// Retire for good.
  retired,
}

/// One result of `bothy-host doctor` (`HostDoctorCheck`, 24 2.2): the check, passed or its
/// code, and the name of the account the finding is in where the rule for names gives one.
final class HostDoctorEntry {
  /// Creates the entry.
  const HostDoctorEntry({
    required this.check,
    required this.ok,
    this.code,
    this.sessionUser,
  });

  /// The check on the wire, for example `hd9_foreign_homes`.
  final String check;

  /// Whether the check passed.
  final bool ok;

  /// The code of a finding, for example `subscription_login_detected`, or null.
  final String? code;

  /// The session user or former session user the finding names (HD6, HD9, HD10, HD11), or
  /// null. A name, never a path.
  final String? sessionUser;

  /// The ID of the check as the specification writes it, for example `HD9`; the wire name for
  /// a check this build does not know.
  String get label {
    final match = RegExp(r'^hd(\d+)_').firstMatch(check);
    return match == null ? check : 'HD${match.group(1)}';
  }
}

/// A valid assignment of a person to a session user (`HostSessionUser`, ruling T12).
final class HostSessionUserInfo {
  /// Creates the assignment.
  const HostSessionUserInfo({
    required this.userId,
    required this.osUser,
    required this.createdAt,
    this.ownerAccount = false,
  });

  /// The person.
  final String userId;

  /// The name of the session user on the host.
  final String osUser;

  /// Since when.
  final DateTime createdAt;

  /// The assignment of the owner account of the host (ADR 0073): sessions run with
  /// administrator rights.
  final bool ownerAccount;
}

/// A session user the host service reported (`HostReportedSessionUser`).
final class HostReportedName {
  /// Creates the name.
  const HostReportedName(
    this.osUser, {
    this.workdirs = const [],
    this.ownerAccount = false,
  });

  /// The name.
  final String osUser;

  /// The owner account of the host (`host_service.owner_account`, ADR 0073): the account with
  /// administrator rights the host service runs as; it needs the explicit opt-in.
  final bool ownerAccount;

  /// The display names of its working directories of the profile `claude` (ruling T13).
  final List<String> workdirs;
}

/// A profile the host service reported (`HostProfile`).
final class HostProfileInfo {
  /// Creates the profile.
  const HostProfileInfo({required this.name, required this.displayName});

  /// The name, for example `shell`.
  final String name;

  /// The display name.
  final String displayName;
}

/// A host as the administration shows it (`Host`).
final class AdminHost {
  /// Creates the host.
  const AdminHost({
    required this.id,
    required this.name,
    required this.state,
    required this.llmLogin,
    required this.declaredServerHost,
    required this.keyGeneration,
    required this.connected,
    required this.createdAt,
    required this.version,
    this.ownerDisabled = false,
    this.platform,
    this.assignedUserId,
    this.credentialHolderId,
    this.reportedIsServerHost,
    this.keyFingerprint,
    this.profiles = const [],
    this.doctor = const [],
    this.sessionUsers = const [],
    this.reportedSessionUsers,
    this.hostVersion,
    this.osVersion,
    this.lastSeenAt,
  });

  /// The host ID.
  final String id;

  /// The name.
  final String name;

  /// The state.
  final HostLifecycle state;

  /// Whether an owner disabled the host (`disabled_reason = owner`); false for a host that a
  /// second control connection disabled, which only re-pairing brings back (24 3.4).
  final bool ownerDisabled;

  /// `macos`, `linux` or `windows`, or null before the first report.
  final String? platform;

  /// The assignee (SR10), or null for owners only.
  final String? assignedUserId;

  /// The declaration of the LLM login.
  final HostLlmLogin llmLogin;

  /// The holder of a personal LLM login, or null.
  final String? credentialHolderId;

  /// The owner's answer to "Does the server run on this host?" (24 2.3).
  final bool declaredServerHost;

  /// The report of the host service, or null before the first report.
  final bool? reportedIsServerHost;

  /// The fingerprint of the stored keys, or null while the host has no keys.
  final String? keyFingerprint;

  /// The key generation; it rises with every pairing.
  final int keyGeneration;

  /// The profiles.
  final List<HostProfileInfo> profiles;

  /// The check results, as the server returns them.
  final List<HostDoctorEntry> doctor;

  /// The valid assignments of session users (ended ones are not in `getHost`).
  final List<HostSessionUserInfo> sessionUsers;

  /// The names the host reported, or null before pairing.
  final List<HostReportedName>? reportedSessionUsers;

  /// Whether the control connection is open.
  final bool connected;

  /// The version of the host service, or null.
  final String? hostVersion;

  /// The operating system version, or null.
  final String? osVersion;

  /// When the host service was last seen, or null.
  final DateTime? lastSeenAt;

  /// When the host was created.
  final DateTime createdAt;

  /// The version for `If-Match`.
  final int version;

  /// The target of a grant on this host.
  AdminTarget get target =>
      AdminTarget(AdminTargetKind.host, id: id, name: name);
}

/// A pairing code, a one-time secret: shown once, never stored by the client (24 3.2).
final class HostPairingCodeInfo {
  /// Creates the code.
  const HostPairingCodeInfo({
    required this.hostId,
    required this.code,
    required this.expiresAt,
  });

  /// The host.
  final String hostId;

  /// The code, `bothy_hpc_...`.
  final String code;

  /// The end of its validity.
  final DateTime expiresAt;
}

/// One assignment of a person to a reported name (`HostSessionUserAssign`).
final class HostAssignment {
  /// Creates the assignment.
  const HostAssignment({
    required this.userId,
    required this.osUser,
    this.ownerAccount = false,
  });

  /// The person.
  final String userId;

  /// The reported name.
  final String osUser;

  /// The explicit opt-in for the owner account of the host (ADR 0073).
  final bool ownerAccount;
}

/// The body of `confirmHost` (`HostConfirm`).
final class HostConfirmRequest {
  /// Creates the request.
  const HostConfirmRequest({
    required this.fingerprintPrefix,
    required this.llmLogin,
    required this.declaredServerHost,
    required this.reason,
    this.credentialHolderId,
    this.sessionUsers = const [],
  });

  /// The typed first characters of the fingerprint, normalized ([normalizeFingerprint]).
  final String fingerprintPrefix;

  /// The declaration of the LLM login.
  final HostLlmLogin llmLogin;

  /// The holder of a personal login, or null.
  final String? credentialHolderId;

  /// The answer to the server host question.
  final bool declaredServerHost;

  /// The session users per person.
  final List<HostAssignment> sessionUsers;

  /// Why.
  final String reason;
}

/// [text] without separators (spaces, hyphens) in upper case: the form `confirmHost` reads
/// (24 4.3: groups of four joined with `-`, case ignored).
///
/// Example: `normalizeFingerprint('7kq2-9xta')` is `'7KQ29XTA'`.
String normalizeFingerprint(String text) =>
    text.replaceAll(RegExp(r'[\s-]'), '').toUpperCase();

/// The answer `409 fingerprint_mismatch` of `confirmHost` with the confirmations left; at
/// [attemptsRemaining] `0` the host fell back to `pairing` without keys (24 3.2, TM-T19).
final class FingerprintMismatch implements Exception {
  /// Creates the failure.
  const FingerprintMismatch(this.attemptsRemaining, this.problem);

  /// The confirmations left, from the server's answer.
  final int attemptsRemaining;

  /// The problem of the server.
  final ApiProblem problem;

  @override
  String toString() => 'FingerprintMismatch($attemptsRemaining)';
}

/// The calls of screen A8; every changing call carries the [purpose] text the step-up dialog
/// shows when the server answers `401 step_up_required` (05: `host.manage` and `audit.read`
/// have step-up, `host.read` and `terminal.read_own` none).
///
/// Errors: [SelfActionFailed] with the server's problem, [FingerprintMismatch] from
/// [confirmHost], [StepUpCancelled] when the person cancels the step-up.
abstract interface class HostsRepository {
  /// The origin of the instance for the command `bothy-host pair --server <origin>`, for
  /// example `https://chat.example.com`.
  String get serverOrigin;

  /// Whether the signed-in person is an owner (`getMe.is_owner`): the shell lists the hosts
  /// for owners only (brief M4-37 fixed requirement 11).
  Future<bool> callerIsOwner();

  /// Every host the caller sees (`listHosts`, all pages), ordered by name.
  Future<List<AdminHost>> hosts();

  /// One host (`getHost`).
  Future<AdminHost> host(String hostId);

  /// Creates a host (`createHost`) and returns its first pairing code.
  Future<HostPairingCodeInfo> createHost({
    required String name,
    String? assigneeId,
    required String reason,
    required String purpose,
  });

  /// Issues a new pairing code (`issueHostPairingCode`); from `active` this is the re-pairing
  /// of 24 3.3: the keys are dropped and the key generation rises at the next pairing.
  Future<HostPairingCodeInfo> newPairingCode(
    String hostId, {
    required String reason,
    required String purpose,
  });

  /// Confirms [host] in `confirming` (`confirmHost` with `If-Match` of its version).
  ///
  /// Throws [FingerprintMismatch] on `409 fingerprint_mismatch`.
  Future<AdminHost> confirmHost(
    AdminHost host,
    HostConfirmRequest request, {
    required String purpose,
  });

  /// Changes the assignee or the session users of [host] (`updateHost`): [assigneeId] sets
  /// the assignee, [clearAssignee] removes it, [add] assigns names, [revoke] ends the
  /// assignments of these people (their names stay taken on this host).
  Future<AdminHost> updateHost(
    AdminHost host, {
    String? assigneeId,
    bool clearAssignee = false,
    List<HostAssignment> add = const [],
    List<String> revoke = const [],
    required String reason,
    required String purpose,
  });

  /// Changes the declaration of the LLM login (`setHostLlmCredentials`); may fail with
  /// `409 i6_violation`.
  Future<AdminHost> setLlmLogin(
    AdminHost host,
    HostLlmLogin login, {
    String? holderId,
    required String reason,
    required String purpose,
  });

  /// Disables, enables or retires [host] (`setHostState`).
  Future<AdminHost> setHostState(
    AdminHost host,
    HostStateRequest state, {
    required String reason,
    required String purpose,
  });

  /// The person each session user name of [hostId] was last assigned to, read from the audit
  /// of the host (`listAuditEvents`, `host.manage`, operation `session_user`, 24 10.1): the
  /// only source of ended assignments, since `getHost` returns valid ones only.
  Future<Map<String, String>> assignmentHistory(
    String hostId, {
    required String purpose,
  });

  /// The number of terminal sessions on [hostId] that are not closed (`listTerminalSessions`).
  Future<int> openSessions(String hostId);
}

/// [HostsRepository] over the generated client of `api_client`.
///
/// Example:
/// ```dart
/// final hosts = ApiHostsRepository(session.api, newIdempotencyKey: ids.next);
/// final list = await hosts.hosts();
/// ```
final class ApiHostsRepository implements HostsRepository {
  /// Creates the repository over `api` (the client of the signed-in session, so DPoP, refresh
  /// and the step-up interceptor apply); [newIdempotencyKey] returns a fresh UUIDv7.
  ApiHostsRepository(this._api, {required String Function() newIdempotencyKey})
    : _newKey = newIdempotencyKey;

  final ApiClient _api;
  final String Function() _newKey;

  HostsApi get _hosts => _api.getHostsApi();

  Map<String, Object?> _purpose(String text) => {stepUpPurposeExtra: text};

  static String _ifMatch(AdminHost h) => '"${h.version}"';

  @override
  String get serverOrigin {
    final base = Uri.tryParse(_api.dio.options.baseUrl);
    return base == null || !base.hasScheme ? '' : base.origin;
  }

  @override
  Future<bool> callerIsOwner() =>
      _call(() async => (await _api.getMeApi().getMe()).data!.isOwner);

  @override
  Future<List<AdminHost>> hosts() => _call(() async {
    final out = <AdminHost>[];
    String? cursor;
    do {
      final page = (await _hosts.listHosts(cursor: cursor)).data!;
      out.addAll(page.items.map(_host));
      cursor = page.nextCursor;
    } while (cursor != null);
    return out;
  });

  @override
  Future<AdminHost> host(String hostId) =>
      _call(() async => _host((await _hosts.getHost(hostId: hostId)).data!));

  @override
  Future<HostPairingCodeInfo> createHost({
    required String name,
    String? assigneeId,
    required String reason,
    required String purpose,
  }) => _call(
    () async => _code(
      (await _hosts.createHost(
        hostCreate: HostCreate(
          (b) => b
            ..name = name
            ..assignedUserId = assigneeId
            ..reason = reason,
        ),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<HostPairingCodeInfo> newPairingCode(
    String hostId, {
    required String reason,
    required String purpose,
  }) => _call(
    () async => _code(
      (await _hosts.issueHostPairingCode(
        hostId: hostId,
        reasonBody: ReasonBody((b) => b..reason = reason),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<AdminHost> confirmHost(
    AdminHost host,
    HostConfirmRequest request, {
    required String purpose,
  }) => _call(
    () async => _host(
      (await _hosts.confirmHost(
        hostId: host.id,
        ifMatch: _ifMatch(host),
        hostConfirm: HostConfirm((b) {
          b
            ..fingerprintPrefix = normalizeFingerprint(
              request.fingerprintPrefix,
            )
            ..llmCredentials = _wireLogin(request.llmLogin)
            ..credentialHolderId = request.credentialHolderId
            ..declaredServerHost = request.declaredServerHost
            ..reason = request.reason;
          if (request.sessionUsers.isNotEmpty) {
            b.sessionUsers.replace(request.sessionUsers.map(_assign));
          }
        }),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<AdminHost> updateHost(
    AdminHost host, {
    String? assigneeId,
    bool clearAssignee = false,
    List<HostAssignment> add = const [],
    List<String> revoke = const [],
    required String reason,
    required String purpose,
  }) => _call(
    () async => _host(
      (await _hosts.updateHost(
        hostId: host.id,
        ifMatch: _ifMatch(host),
        hostUpdate: HostUpdate((b) {
          b
            ..assignedUserId = assigneeId
            ..clearAssignedUser = clearAssignee ? true : null
            ..reason = reason;
          if (add.isNotEmpty) b.addSessionUsers.replace(add.map(_assign));
          if (revoke.isNotEmpty) b.revokeSessionUsers.replace(revoke);
        }),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<AdminHost> setLlmLogin(
    AdminHost host,
    HostLlmLogin login, {
    String? holderId,
    required String reason,
    required String purpose,
  }) => _call(
    () async => _host(
      (await _hosts.setHostLlmCredentials(
        hostId: host.id,
        ifMatch: _ifMatch(host),
        hostLlmCredentialsSet: HostLlmCredentialsSet(
          (b) => b
            ..llmCredentials = _wireLogin(login)
            ..credentialHolderId = login == HostLlmLogin.personal
                ? holderId
                : null
            ..reason = reason,
        ),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<AdminHost> setHostState(
    AdminHost host,
    HostStateRequest state, {
    required String reason,
    required String purpose,
  }) => _call(
    () async => _host(
      (await _hosts.setHostState(
        hostId: host.id,
        ifMatch: _ifMatch(host),
        hostStateSet: HostStateSet(
          (b) => b
            ..state = switch (state) {
              HostStateRequest.active => HostStateSetStateEnum.active,
              HostStateRequest.disabled => HostStateSetStateEnum.disabled,
              HostStateRequest.retired => HostStateSetStateEnum.retired,
            }
            ..reason = reason,
        ),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<Map<String, String>> assignmentHistory(
    String hostId, {
    required String purpose,
  }) => _call(() async {
    final events = <AuditEvent>[];
    String? cursor;
    do {
      final page = (await _api.getAuditApi().listAuditEvents(
        cursor: cursor,
        action: 'host.manage',
        resourceId: hostId,
        extra: _purpose(purpose),
      )).data!;
      events.addAll(page.items);
      cursor = page.nextCursor;
    } while (cursor != null);
    events.sort((a, b) => a.seq.compareTo(b.seq));
    final out = <String, String>{};
    for (final e in events) {
      final d = e.details;
      if (d['operation']?.value != 'session_user') continue;
      final name = d['os_user']?.value;
      final user = d['user_id']?.value;
      if (name is String && user is String) out[name] = user;
    }
    return out;
  });

  @override
  Future<int> openSessions(String hostId) => _call(() async {
    var count = 0;
    String? cursor;
    do {
      final page = (await _api.getTerminalApi().listTerminalSessions(
        cursor: cursor,
        hostId: hostId,
      )).data!;
      count += page.items
          .where((s) => s.state != TerminalSessionState.closed)
          .length;
      cursor = page.nextCursor;
    } while (cursor != null);
    return count;
  });

  static HostSessionUserAssign _assign(HostAssignment a) =>
      HostSessionUserAssign(
        (b) => b
          ..userId = a.userId
          ..osUser = a.osUser
          ..ownerAccount = a.ownerAccount ? true : null,
      );

  static LlmCredentials _wireLogin(HostLlmLogin login) => switch (login) {
    HostLlmLogin.none => LlmCredentials.none,
    HostLlmLogin.personal => LlmCredentials.personal,
    HostLlmLogin.organization => LlmCredentials.organization,
  };

  static HostPairingCodeInfo _code(HostPairingCode c) => HostPairingCodeInfo(
    hostId: c.hostId,
    code: c.pairingCode,
    expiresAt: c.expiresAt,
  );

  static const Map<String, String> _checks = {
    'hd1User': 'hd1_user',
    'hd2Groups': 'hd2_groups',
    'hd3Sudo': 'hd3_sudo',
    'hd4StateDir': 'hd4_state_dir',
    'hd5ServerHost': 'hd5_server_host',
    'hd6LlmLogin': 'hd6_llm_login',
    'hd7Profiles': 'hd7_profiles',
    'hd8Separation': 'hd8_separation',
    'hd9ForeignHomes': 'hd9_foreign_homes',
    'hd10HelperIntegrity': 'hd10_helper_integrity',
    'hd11SharedLocations': 'hd11_shared_locations',
  };

  static AdminHost _host(Host h) => AdminHost(
    id: h.id,
    name: h.name,
    state: switch (h.state) {
      HostState.pairing => HostLifecycle.pairing,
      HostState.confirming => HostLifecycle.confirming,
      HostState.active => HostLifecycle.active,
      HostState.disabled => HostLifecycle.disabled,
      HostState.retired => HostLifecycle.retired,
      _ => HostLifecycle.unknown,
    },
    ownerDisabled: h.disabledReason == HostDisabledReasonEnum.owner,
    platform: switch (h.platform) {
      HostPlatform.macos => 'macos',
      HostPlatform.linux => 'linux',
      HostPlatform.windows => 'windows',
      _ => null,
    },
    assignedUserId: h.assignedUserId,
    llmLogin: switch (h.llmCredentials) {
      LlmCredentials.personal => HostLlmLogin.personal,
      LlmCredentials.organization => HostLlmLogin.organization,
      _ => HostLlmLogin.none,
    },
    credentialHolderId: h.credentialHolderId,
    declaredServerHost: h.declaredServerHost,
    reportedIsServerHost: h.reportedIsServerHost,
    keyFingerprint: h.keyFingerprint,
    keyGeneration: h.keyGeneration,
    profiles: [
      for (final p in h.profiles)
        HostProfileInfo(name: p.name, displayName: p.displayName),
    ],
    doctor: [
      for (final d in h.doctor)
        HostDoctorEntry(
          check: _checks[d.check.name] ?? d.check.name,
          ok: d.ok,
          code: d.code,
          sessionUser: d.sessionUser,
        ),
    ],
    sessionUsers: [
      for (final s in [...?h.sessionUsers])
        HostSessionUserInfo(
          userId: s.userId,
          osUser: s.osUser,
          createdAt: s.createdAt,
          ownerAccount: s.ownerAccount ?? false,
        ),
    ],
    reportedSessionUsers: h.reportedSessionUsers == null
        ? null
        : [
            for (final r in h.reportedSessionUsers!)
              HostReportedName(
                r.osUser,
                workdirs: r.workdirs.toList(),
                ownerAccount: r.ownerAccount ?? false,
              ),
          ],
    connected: h.connected,
    hostVersion: h.hostVersion,
    osVersion: h.osVersion,
    lastSeenAt: h.lastSeenAt,
    createdAt: h.createdAt,
    version: h.version,
  );

  /// Runs [call] and turns its errors into [SelfActionFailed] or [FingerprintMismatch]; an
  /// error of the login layer ([StepUpCancelled], a lost session) passes as it is.
  static Future<T> _call<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on DioException catch (e) {
      final cause = e.error;
      if (cause is AuthException) throw cause;
      final response = e.response;
      final problem =
          ApiProblem.of(e) ??
          (response == null ? null : ApiProblem.fromResponse(response));
      final body = response?.data;
      // `attempts_remaining` is a member of the problem body that ApiProblem does not carry
      // (06 5; 24 3.2), so it is read from the body here.
      if (problem?.code == 'fingerprint_mismatch' && body is Map) {
        final left = body['attempts_remaining'];
        throw FingerprintMismatch(left is int ? left : 0, problem!);
      }
      throw SelfActionFailed(problem);
    }
  }
}
