// SPDX-License-Identifier: Apache-2.0

part of 'world.dart';

/// One terminal session as the fake server keeps it (docs/spec/24-terminal.md 6, 10; brief
/// M4-36 Scope): metadata only, never content.
final class FakeTerminalSession {
  /// Creates a session in `opening`.
  FakeTerminalSession({
    required this.id,
    required this.hostId,
    required this.userId,
    required this.deviceId,
    required this.loginSessionId,
    required this.profile,
    required this.openedAt,
  });

  /// The session ID.
  final String id;

  /// The host.
  final String hostId;

  /// The person.
  final String userId;

  /// The device the session is bound to; an attach from another device moves it.
  String deviceId;

  /// The login session (`sid`) of the last opening or attach, for the quick reattach of 24 6.2.
  String loginSessionId;

  /// The profile name.
  final String profile;

  /// `opening`, `active`, `detached` or `closed`.
  String state = 'opening';

  /// When the session was opened (API timestamp).
  final String openedAt;

  /// When the stream started first, or null.
  String? startedAt;

  /// When the last stream of the device ended, or null while one is open.
  DateTime? detachedAt;

  /// [detachedAt] as an API timestamp.
  String? detachedAtText;

  /// When the session ended, or null.
  String? closedAt;

  /// The `close_reason` of a closed session.
  String? closeReason;

  /// The current one-time session token; null once a stream used it.
  String? token;

  /// The version.
  int version = 1;

  /// Whether the session is closed.
  bool get closed => state == 'closed';

  /// The API `TerminalSession`.
  JsonMap get json => {
    'id': id,
    'host_id': hostId,
    'user_id': userId,
    'device_id': deviceId,
    'profile': profile,
    'state': state,
    'opened_at': openedAt,
    'started_at': ?startedAt,
    'detached_at': ?detachedAtText,
    'closed_at': ?closedAt,
    'close_reason': ?closeReason,
    'bytes_in': 0,
    'bytes_out': 0,
    'version': version,
  };
}

/// The host end of one device stream in the fake (brief M4-36 Scope: a stream endpoint that
/// relays to an in-process host). The fake hands it to [FakeTerminal.hostLink] after the
/// device's `identify`; the link plays the host service: it reads [fromDevice] (each element one
/// Noise message of the device), answers with [toDevice] and ends the stream with [close].
final class FakeTerminalPipe {
  /// Creates the pipe of [sessionId] on [hostId] with the one-time [sessionToken].
  FakeTerminalPipe({
    required this.hostId,
    required this.sessionId,
    required this.sessionToken,
    required this._toDevice,
    required this._close,
  });

  /// The host.
  final String hostId;

  /// The session.
  final String sessionId;

  /// The session token the device carries in Noise message 1.
  final String sessionToken;

  final void Function(List<int> bytes) _toDevice;
  final void Function(int code, String? reason) _close;
  final StreamController<Uint8List> _fromDevice = StreamController<Uint8List>();
  final Completer<void> _done = Completer<void>();

  /// The binary messages of the device, in order; done when the device stream ends.
  Stream<Uint8List> get fromDevice => _fromDevice.stream;

  /// Completes when the device stream ended (either side closed it).
  Future<void> get done => _done.future;

  /// Sends one binary message of the host to the device.
  void toDevice(List<int> bytes) => _toDevice(bytes);

  /// Closes the device stream with [code] and [reason], as the relay does for the host.
  void close(int code, [String? reason]) => _close(code, reason);

  /// Hands one binary message of the device to the host; called by the stream endpoint.
  void deliverFromDevice(Uint8List bytes) {
    if (!_fromDevice.isClosed) _fromDevice.add(bytes);
  }

  /// Ends [fromDevice] and completes [done]; called by the stream endpoint when the device
  /// stream ended.
  void endFromDevice() {
    if (!_fromDevice.isClosed) unawaited(_fromDevice.close());
    if (!_done.isCompleted) _done.complete();
  }
}

/// Plays the host service behind the stream endpoint of the fake: the app's tests connect
/// `FakeHost` of `package:terminal_ui/testing.dart` here.
abstract interface class FakeTerminalHostLink {
  /// Takes the host end of a new device stream.
  FutureOr<void> connect(FakeTerminalPipe pipe);
}

/// One open device stream: the fake closes it with a code on demand.
final class FakeTerminalStream {
  FakeTerminalStream._(this.sessionId, this.deviceId, this._close);

  /// The session.
  final String sessionId;

  /// The device of the stream.
  final String deviceId;

  final void Function(int code, String? reason) _close;

  /// The host end, once the stream passed `identify`.
  FakeTerminalPipe? pipe;

  /// Closes the stream with [code] and [reason] (24 13; asyncapi `x-close-codes`).
  void close(int code, [String? reason]) => _close(code, reason);
}

/// The terminal of the fake server: sessions, the host link, the open device streams and the
/// server values the handlers use (24 17 defaults; tests lower them).
final class FakeTerminal {
  /// The sessions by ID.
  final Map<String, FakeTerminalSession> sessions = {};

  /// The host service behind the stream endpoint; null closes every stream with `4102`.
  FakeTerminalHostLink? hostLink;

  /// `terminal.quick_reattach_window`.
  Duration quickReattachWindow = const Duration(seconds: 60);

  /// `terminal.session_token_ttl`.
  Duration sessionTokenTtl = const Duration(seconds: 30);

  /// `terminal.max_sessions_per_person`.
  int maxSessionsPerPerson = 5;

  /// The open device stream per session.
  final Map<String, FakeTerminalStream> streams = {};

  /// The computer sessions by ID (27 8; CU-07); the tests of the apps open them with
  /// [FakeWorldComputer.openComputerSession], as the runner would.
  final Map<String, FakeComputerSession> computerSessions = {};

  /// Every close code the fake sent to a device stream, in order, as `(session, code, reason)`.
  final List<(String, int, String?)> closes = [];

  /// Registers a new device stream of [sessionId] on [deviceId]; [close] closes its socket.
  FakeTerminalStream open(
    String sessionId,
    String deviceId,
    void Function(int code, String? reason) close,
  ) {
    final stream = FakeTerminalStream._(sessionId, deviceId, (code, reason) {
      closes.add((sessionId, code, reason));
      close(code, reason);
    });
    streams[sessionId] = stream;
    return stream;
  }

  /// Closes the open stream of [sessionId], if any, with [code] and [reason].
  void closeStream(String sessionId, int code, [String? reason]) {
    final stream = streams.remove(sessionId);
    stream?.close(code, reason);
  }
}

/// The terminal steps of the world: events and the state changes that end sessions.
extension FakeWorldTerminal on FakeWorld {
  /// Appends the personal `terminal_session.changed` of [s] (04 2.1, 24 10.2).
  void terminalSessionChanged(FakeTerminalSession s) {
    s.version += 1;
    append(
      'terminal_session.changed',
      resource: {'kind': 'terminal_session', 'id': s.id, 'version': s.version},
      payload: s.json,
      audience: s.userId,
    );
  }

  /// Raises the version of [h] and appends `host.updated` (04 2.1); [keyChanged] after a
  /// re-pairing (24 3.3).
  void hostUpdated(FakeHost h, {bool keyChanged = false}) {
    h.version += 1;
    append(
      'host.updated',
      resource: {'kind': 'host', 'id': h.id, 'version': h.version},
      payload: {'host': h.json, if (keyChanged) 'key_changed': true},
    );
  }

  /// The people who see [h] (07 2.12): owners, the assignee and the holders of a valid grant
  /// `terminal` on it.
  Set<String> hostViewers(FakeHost h) => {
    for (final p in people.values)
      if (p.kind == 'user' && p.isOwner) p.id,
    ?h.assignedUserId,
    for (final g in grants.values)
      if (g['revoked_at'] == null &&
          g['target_kind'] == 'host' &&
          g['target_id'] == h.id &&
          g['subject_kind'] == 'user')
        g['subject_id']! as String,
  };

  /// Sets the connection of host [hostId] and publishes the ephemeral `host.presence_changed`
  /// to the devices of everyone who sees the host.
  void setHostConnected(String hostId, {required bool connected}) {
    final h = hosts[hostId]!..connected = connected;
    for (final viewer in hostViewers(h)) {
      publishEphemeral(
        'host.presence_changed',
        resource: {'kind': 'host', 'id': h.id},
        payload: {'host_id': h.id, 'connected': connected},
        audience: viewer,
        devices: {
          for (final d in devices.values)
            if (d.userId == viewer) d.id,
        },
      );
    }
  }

  /// Ends [s] with [reason]: state `closed`, the event, and the open stream closed with
  /// `4100` and the reason (24 8.3, 10.3).
  void endTerminalSession(FakeTerminalSession s, String reason) {
    if (s.closed) return;
    s
      ..state = 'closed'
      ..closeReason = reason
      ..closedAt = stamp()
      ..token = null;
    terminal.closeStream(s.id, 4100, reason);
    terminalSessionChanged(s);
  }

  /// [userId] lost the capability `terminal` on host [hostId] (04 8.2, 24 8.3): the personal
  /// `access.changed` with the host in `removed`, and every open session of the person there
  /// ends with `access_revoked`.
  void revokeHostAccess(String userId, String hostId) {
    permissionVersion++;
    append(
      'access.changed',
      resource: {'kind': 'principal', 'id': userId},
      payload: {
        'permission_version': permissionVersion,
        'added': <Object?>[],
        'removed': [
          {'kind': 'host', 'id': hostId},
        ],
        'changed': <Object?>[],
      },
      audience: userId,
    );
    for (final s in terminal.sessions.values.toList()) {
      if (s.userId == userId && s.hostId == hostId) {
        endTerminalSession(s, 'access_revoked');
      }
    }
  }

  /// [userId] got the capability `terminal` on host [hostId]: the personal `access.changed`
  /// with the host in `added` (04 8.2).
  void grantHostAccess(String userId, String hostId) {
    append(
      'access.changed',
      resource: {'kind': 'principal', 'id': userId},
      payload: {
        'permission_version': permissionVersion,
        'added': [
          {
            'kind': 'host',
            'id': hostId,
            'capabilities': ['terminal'],
          },
        ],
        'removed': <Object?>[],
        'changed': <Object?>[],
      },
      audience: userId,
    );
  }
}

/// Building terminal hosts for the tests of the apps (brief M4-36).
extension FakeWorldTerminalHosts on FakeWorld {
  /// Adds an active, connected host named [name] with the static key [noisePublicKey], the
  /// session users [sessionUsers] (person to name; every name is reported), the working
  /// directories [workdirs] of the profile `claude` per name, [profiles] (default `shell` and,
  /// with any working directory, `claude`), the assignee [assignedUserId], and a valid grant
  /// `terminal` for each of [grantees]; returns the host. [id] is the host ID (a new one when
  /// null), so a test can give the in-process host the same ID for the Noise prologue.
  FakeHost addTerminalHost({
    String? id,
    required String name,
    required String noisePublicKey,
    Map<String, String> sessionUsers = const {},
    Map<String, List<String>> workdirs = const {},
    List<HostJson>? profiles,
    String? assignedUserId,
    List<String> grantees = const [],
    String llmCredentials = 'organization',
    String fingerprint =
        '7KQ2-9XTA-M4PB-1C8D-0000-0000-0000-0000-0000-0000-0000-0000-0000',
  }) {
    final h = FakeHost(
      id: id ?? nextId(),
      name: name,
      createdAt: stamp(),
      assignedUserId: assignedUserId,
    );
    hosts[h.id] = h;
    pairHost(
      h.id,
      {...sessionUsers.values, ...workdirs.keys}.toList(),
      fingerprint: fingerprint,
    );
    h
      ..state = 'active'
      ..llmCredentials = llmCredentials
      ..noisePublicKey = noisePublicKey
      ..profiles =
          profiles ??
          [
            {'name': 'shell', 'display_name': 'Shell', 'kind': 'shell'},
            if (workdirs.isNotEmpty)
              {
                'name': 'claude',
                'display_name': 'Claude Code',
                'kind': 'claude',
              },
          ];
    h.workdirs.addAll(workdirs);
    for (final MapEntry(key: user, value: os) in sessionUsers.entries) {
      h.sessionUsers[user] = (os, stamp());
      h.taken[os] = user;
    }
    for (final user in grantees) {
      final id = nextId();
      grants[id] = {
        'id': id,
        'subject_kind': 'user',
        'subject_id': user,
        'target_kind': 'host',
        'target_id': h.id,
        'capabilities': ['terminal'],
        'reason': 'Terminal for the tests',
        'granted_by': user,
        'granted_at': stamp(),
      };
    }
    return h;
  }

  /// The ID of the valid grant `terminal` of [userId] on [hostId], or null.
  String? terminalGrantOf(String userId, String hostId) => grants.values
      .where(
        (g) =>
            g['revoked_at'] == null &&
            g['target_kind'] == 'host' &&
            g['target_id'] == hostId &&
            g['subject_id'] == userId,
      )
      .map((g) => g['id']! as String)
      .firstOrNull;
}

/// One computer session as the fake server keeps it (docs/spec/27-computer-use.md 8, 12.2;
/// brief CU-07 Scope): the metadata of `ComputerSession`, never an image.
final class FakeComputerSession {
  /// Creates a session in `opening`.
  FakeComputerSession({
    required this.id,
    required this.hostId,
    required this.desktop,
    required this.userId,
    required this.agentId,
    required this.runId,
    required this.generation,
    required this.keyGeneration,
    required this.openedAt,
  });

  /// The session ID.
  final String id;

  /// The host.
  final String hostId;

  /// The host's name.
  final String desktop;

  /// The triggering owner.
  final String userId;

  /// The agent of the run.
  final String agentId;

  /// The run.
  final String runId;

  /// The enablement generation the session was opened with.
  final int generation;

  /// The key generation of the host.
  final int keyGeneration;

  /// When the session was opened (API timestamp).
  final String openedAt;

  /// `opening`, `active`, `paused` or `closed`.
  String state = 'opening';

  /// `locked` while paused, else null.
  String? pauseReason;

  /// The count of actions.
  int actionCount = 0;

  /// The identity of the frontmost app, or null.
  String? frontApp;

  /// When the session ended, or null.
  String? closedAt;

  /// The person who stopped it, or null.
  String? stoppedBy;

  /// The `close_reason` of a closed session.
  String? closeReason;

  /// The `close_detail` of a closed session, or null.
  String? closeDetail;

  /// The version.
  int version = 1;

  /// Whether the session is closed.
  bool get closed => state == 'closed';

  /// The API `ComputerSession`.
  JsonMap get json => {
    'id': id,
    'host_id': hostId,
    'desktop': desktop,
    'user_id': userId,
    'agent_id': agentId,
    'run_id': runId,
    'state': state,
    'pause_reason': ?pauseReason,
    'generation': generation,
    'key_generation': keyGeneration,
    'opened_at': openedAt,
    'paused_ms': 0,
    'closed_at': ?closedAt,
    'stopped_by': ?stoppedBy,
    'close_reason': ?closeReason,
    'close_detail': ?closeDetail,
    'action_count': actionCount,
    'front_app': ?frontApp,
    'version': version,
  };
}

/// The computer steps of the world (27 4, 8; brief CU-07): what the host service and the
/// runner would report, and the ends the server writes.
extension FakeWorldComputer on FakeWorld {
  /// Sets the `computer` object host [hostId] reports (27 12.3): a connected helper, [enabled]
  /// with [generation], the account [osAccount], and both permissions [permission]; raises the
  /// host's version and appends `host.updated` (without the object, D-CU-04-6).
  void reportComputer(
    String hostId, {
    required bool enabled,
    required int generation,
    String osAccount = 'alex',
    String permission = 'granted',
    String helper = 'connected',
    bool locked = false,
  }) {
    final h = hosts[hostId]!
      ..computer = {
        'helper': helper,
        'enabled': enabled,
        'generation': generation,
        'os_account': osAccount,
        'permissions': {'capture': permission, 'input': permission},
        'helper_version': '1.0.0',
        'signature': 'verified',
        'locked': locked,
      };
    hostUpdated(h);
  }

  /// Opens a computer session of [userId] on [hostId] for the run [runId] of [agentId] (fresh
  /// IDs when null), as `openComputerSession` of the runner does, and appends its event.
  FakeComputerSession openComputerSession({
    required String hostId,
    required String userId,
    String? agentId,
    String? runId,
  }) {
    final h = hosts[hostId]!;
    final s = FakeComputerSession(
      id: nextId(),
      hostId: h.id,
      desktop: h.name,
      userId: userId,
      agentId: agentId ?? nextId(),
      runId: runId ?? nextId(),
      generation: (h.computer?['generation'] as int?) ?? 0,
      keyGeneration: h.keyGeneration,
      openedAt: stamp(),
    );
    terminal.computerSessions[s.id] = s;
    computerSessionChanged(s);
    return s;
  }

  /// Appends the personal `computer_session.changed` of [s] (27 12.4).
  void computerSessionChanged(FakeComputerSession s) {
    s.version += 1;
    append(
      'computer_session.changed',
      resource: {'kind': 'computer_session', 'id': s.id, 'version': s.version},
      payload: s.json,
      audience: s.userId,
    );
  }

  /// A `computer.state` of the host (27 12.3): [state] (`active` or `paused`), the action count
  /// and the frontmost app; `paused` carries `locked`.
  void setComputerState(
    FakeComputerSession s, {
    required String state,
    int? actionCount,
    String? frontApp,
  }) {
    s
      ..state = state
      ..pauseReason = state == 'paused' ? 'locked' : null
      ..actionCount = actionCount ?? s.actionCount
      ..frontApp = frontApp ?? s.frontApp;
    computerSessionChanged(s);
  }

  /// Ends [s] with [reason] and [detail] (27 8.3); [stoppedBy] for a stop from a client.
  void endComputerSession(
    FakeComputerSession s,
    String reason, {
    String? detail,
    String? stoppedBy,
  }) {
    if (s.closed) return;
    s
      ..state = 'closed'
      ..pauseReason = null
      ..closeReason = reason
      ..closeDetail = detail
      ..stoppedBy = stoppedBy
      ..closedAt = stamp();
    computerSessionChanged(s);
  }

  /// `computer.device.disable` on [h] (27 4 item 4): the reported generation becomes the
  /// disabled one, the host's open session ends with `device_disabled`, and the host's version
  /// rises with `host.updated`; a host without a newer generation changes nothing.
  void disableComputer(FakeHost h) {
    final generation = h.computer?['generation'] as int?;
    final disabled = h.computerDisabledGeneration;
    if (generation == null || (disabled != null && disabled >= generation)) {
      return;
    }
    h.computerDisabledGeneration = generation;
    for (final s in terminal.computerSessions.values.toList()) {
      if (s.hostId == h.id) endComputerSession(s, 'device_disabled');
    }
    hostUpdated(h);
  }
}
