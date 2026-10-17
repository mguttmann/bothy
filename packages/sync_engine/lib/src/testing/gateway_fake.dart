// SPDX-License-Identifier: Apache-2.0

/// The gateway side of the fake server (brief M4-17 Design note 1; 04 6, 7, 8.3, 11.3):
/// `hello`, `identify` and `resume`, `ready` with the snapshot, catch-up in pages with
/// `cursor` frames, `resync_required`, live delivery, heartbeats, `reauth`, focus, typing,
/// coalesced `run.updated` and the close codes of 04 6.4.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'dpop_verifier.dart';
import 'faults.dart';
import 'rest/sync_handlers.dart' show resyncReason;
import 'schema_validator.dart';
import 'world.dart';

/// The values the gateway reports to clients (04 18), parameters so tests can shorten them.
final class GatewayTimings {
  /// Creates the values; the defaults are those of 04 18.
  const GatewayTimings({
    this.heartbeatInterval = const Duration(seconds: 30),
    this.resumeWindow = const Duration(minutes: 5),
    this.runPatchCoalesce = const Duration(milliseconds: 250),
    this.maxFocusedThreads = 20,
    this.typingTtl = const Duration(seconds: 8),
  });

  /// `sync.heartbeat_interval`, reported in `hello`.
  final Duration heartbeatInterval;

  /// `sync.resume_window`, reported in `hello`.
  final Duration resumeWindow;

  /// `sync.run_patch_coalesce_interval`.
  final Duration runPatchCoalesce;

  /// `sync.max_focused_threads`.
  final int maxFocusedThreads;

  /// `sync.typing_ttl`, the `expires_at` of `typing.started`.
  final Duration typingTtl;
}

/// One frame on a gateway connection, for assertions.
final class GatewayFrameRecord {
  /// Creates the record.
  const GatewayFrameRecord(this.connection, this.fromServer, this.frame);

  /// The connection number.
  final int connection;

  /// Whether the server sent it.
  final bool fromServer;

  /// The frame `{op, d}`.
  final JsonMap frame;

  /// The op.
  String get op => frame['op']! as String;

  /// The `d` object.
  JsonMap get data => (frame['d'] as JsonMap?) ?? const {};
}

/// One gateway connection as a test sees it.
abstract interface class FakeGatewayConnection {
  /// The number of the connection, from 1.
  int get number;

  /// The device after `identify` or `resume`.
  String? get deviceId;

  /// The principal after `identify` or `resume`.
  String? get principalId;

  /// The gateway session.
  String? get gatewaySessionId;

  /// Whether the socket is open.
  bool get open;

  /// The close code the server sent, or the one the client sent.
  int? get closeCode;

  /// The threads in focus.
  Set<String> get focus;

  /// Whether the connection is past `catchup.complete` (or `ready` without a cursor).
  bool get live;
}

class _GatewaySession {
  _GatewaySession(this.id, this.deviceId);

  final String id;
  final String deviceId;
  Set<String> focus = {};
  DateTime? disconnectedAt;
}

class _Pending {
  _Pending(this.base, this.version, this.ops);

  final int base;
  int version;
  final List<JsonMap> ops;
}

class _Conn implements FakeGatewayConnection {
  _Conn(this.number, this.socket, this.nonce);

  @override
  final int number;

  final WebSocket socket;

  /// The nonce of `hello`.
  final String nonce;

  @override
  String? deviceId;

  @override
  String? principalId;

  String? sessionId;

  _GatewaySession? session;

  @override
  String? get gatewaySessionId => session?.id;

  @override
  bool open = true;

  @override
  int? closeCode;

  @override
  bool live = false;

  bool identified = false;

  final List<JsonMap> held = [];

  final Map<String, _Pending> pendingPatches = {};

  Timer? patchTimer;

  @override
  Set<String> get focus => session?.focus ?? const {};
}

/// Serves `/api/v1/gateway` on the fake server.
///
/// One connection per device: a new `identify` or `resume` closes the older connection with
/// `4017`. Catch-up runs synchronously after `ready` or `resumed`, so no live event can slip
/// between its pages. Every frame the fake sends is checked against `asyncapi.yaml`; a frame
/// of the client that breaks its schema is recorded and the connection is closed with `4002`,
/// as the server does for an unknown member.
///
/// Simplifications (README): no heartbeat timeout (`4009`) and no `reauth_required`; tests
/// close with any code through the [FaultPlan].
final class GatewayFake implements GatewayFaultTarget {
  /// Creates the gateway.
  GatewayFake({
    required this.world,
    required this.contract,
    required this.verifier,
    required this.timings,
    required this.baseUri,
  }) {
    world.eventHooks.add(_onEvent);
    world.runPatchHooks.add(_onRunPatched);
  }

  /// The world.
  final FakeWorld world;

  /// The API documents.
  final ContractSchemas contract;

  /// The DPoP check.
  final DpopVerifier verifier;

  /// The values reported to clients.
  final GatewayTimings timings;

  /// The address of the fake, for the `htu` of gateway proofs.
  final Uri Function() baseUri;

  final List<_Conn> _conns = [];
  final Map<String, _GatewaySession> _sessions = {};
  bool _duplicateNext = false;
  String? _forcedResync;
  bool _holding = false;

  /// Every frame in both directions, in order.
  final List<GatewayFrameRecord> frames = [];

  /// Contract violations of frames, as text with the pointer.
  final List<String> violations = [];

  /// Every connection, in order of opening.
  List<FakeGatewayConnection> get connections => List.unmodifiable(_conns);

  /// The frames the server sent on connection [number].
  List<GatewayFrameRecord> sentOn(int number) => [
    for (final f in frames)
      if (f.connection == number && f.fromServer) f,
  ];

  /// Takes over [socket] after the upgrade and sends `hello`.
  void accept(WebSocket socket) {
    // The nonces of the gateway are those of REST (08 7.3: stateless, current and previous).
    final conn = _Conn(_conns.length + 1, socket, verifier.currentNonce);
    _conns.add(conn);
    socket.listen(
      (data) => _onFrame(conn, data),
      onDone: () => _onDone(conn),
      onError: (Object _) => _onDone(conn),
    );
    _send(conn, 'hello', {
      'heartbeat_interval_ms': timings.heartbeatInterval.inMilliseconds,
      'resume_window_ms': timings.resumeWindow.inMilliseconds,
      'protocol_versions': [1],
      'dpop_nonce': conn.nonce,
      'server_time': world.stamp(),
      'sync_epoch': world.syncEpoch,
    });
  }

  /// Ends the connections of [deviceId] (every connection when null) without a close code,
  /// as a lost network does.
  void disconnect({String? deviceId}) {
    for (final c in _open.where(
      (c) => deviceId == null || c.deviceId == deviceId,
    )) {
      c.open = false;
      unawaited(c.socket.close());
    }
  }

  /// Closes every connection with `1001` and stops the timers.
  Future<void> close() async {
    for (final c in _open) {
      c.patchTimer?.cancel();
      c.open = false;
      await c.socket.close(1001);
    }
  }

  Iterable<_Conn> get _open => _conns.where((c) => c.open).toList();

  // -------------------------------------------------------------------------------------------
  // Faults

  @override
  void closeAll(int code, String? reason) {
    for (final c in _open) {
      _close(c, code, reason);
    }
  }

  @override
  void duplicateNextDispatch() => _duplicateNext = true;

  @override
  void forceResync(String reason) => _forcedResync = reason;

  @override
  void revoke(String? deviceId) {
    final sessions = world.sessions.values
        .where(
          (s) => !s.revoked && (deviceId == null || s.deviceId == deviceId),
        )
        .toList();
    for (final s in sessions) {
      // 04 8.5: the connections close at once with 4012; the event follows in the log.
      for (final c in _open.where((c) => c.sessionId == s.id)) {
        _close(c, 4012, 'revoked');
      }
      world.revokeSession(s.id);
    }
  }

  @override
  void holdDispatches() => _holding = true;

  @override
  void releaseDispatches() {
    _holding = false;
    for (final c in _open) {
      final held = List.of(c.held);
      c.held.clear();
      for (final d in held) {
        _dispatch(c, d);
      }
    }
  }

  // -------------------------------------------------------------------------------------------
  // Frames of the client

  void _onFrame(_Conn c, Object? data) {
    if (!c.open) return;
    if (data is! String) {
      _close(c, 4002, 'binary frame');
      return;
    }
    final Object? decoded;
    try {
      decoded = jsonDecode(data);
    } on FormatException {
      _close(c, 4002, 'invalid JSON');
      return;
    }
    if (decoded is! JsonMap) {
      _close(c, 4002, 'not an object');
      return;
    }
    frames.add(GatewayFrameRecord(c.number, false, decoded));
    final op = decoded['op'];
    const known = {
      'identify',
      'resume',
      'heartbeat',
      'reauth',
      'focus',
      'typing.start',
      'typing.stop',
      'ack',
      'run.update',
      'command.autocomplete_result',
      'stats.subscribe',
      'stats.unsubscribe',
    };
    if (op is! String || !known.contains(op)) {
      _close(c, 4001, 'unknown op');
      return;
    }
    final problems = contract.clientFrame(decoded);
    if (problems.isNotEmpty) {
      violations.add('client frame $op: ${problems.join('; ')}');
      _close(c, 4002, 'schema');
      return;
    }
    final d = decoded['d']! as JsonMap;
    if (op != 'identify' && op != 'resume' && !c.identified) {
      _close(c, 4003, 'not identified');
      return;
    }
    switch (op) {
      case 'identify':
      case 'resume':
        _start(c, op, d);
      case 'heartbeat':
        _send(c, 'heartbeat_ack', {'server_time': world.stamp()});
      case 'reauth':
        _reauth(c, d);
      case 'focus':
        _focus(c, d);
      case 'typing.start':
        _typing(c, d['thread_id']! as String);
      case 'typing.stop':
        break;
      default:
        // Agent and app ops are not for people (05 2).
        _send(c, 'error', {'op': op, 'code': 'forbidden'});
    }
  }

  /// Checks token and proof of `identify`, `resume` or `reauth`; closes with the code of
  /// 04 6.4 and returns null on failure.
  FakeSession? _checkLogin(_Conn c, JsonMap d) {
    final token = d['access_token'];
    if (token is! String) {
      _close(c, 4004, 'no token');
      return null;
    }
    final session = world.sessions.values
        .where(
          (s) => s.accessToken == token || s.olderAccessTokens.contains(token),
        )
        .firstOrNull;
    if (session == null) {
      _close(c, 4004, 'unknown token');
      return null;
    }
    if (session.revoked) {
      _close(c, 4012, 'revoked');
      return null;
    }
    if (_expired(token)) {
      _close(c, 4011, 'token expired');
      return null;
    }
    final base = baseUri();
    final result = verifier.verify(
      d['dpop'] as String?,
      method: 'GET',
      htu: '${base.origin}/api/v1/gateway',
      accessToken: token,
      expectedJkt: session.jkt,
    );
    if (result is DpopRejected) {
      _close(c, 4004, result.code);
      return null;
    }
    final device = world.devices[session.deviceId]!;
    if (device.state == 'revoked') {
      _close(c, 4012, 'revoked');
      return null;
    }
    if (device.state == 'pending') {
      _close(c, 4016, 'device pending');
      return null;
    }
    return session;
  }

  bool _expired(String token) {
    try {
      final payload = jsonDecode(
        utf8.decode(base64Url.decode(base64Url.normalize(token.split('.')[1]))),
      ) as JsonMap;
      return world.now().millisecondsSinceEpoch ~/ 1000 >=
          (payload['exp']! as int);
    } on Object {
      return true;
    }
  }

  void _start(_Conn c, String op, JsonMap d) {
    if (c.identified) {
      _close(c, 4005, 'already identified');
      return;
    }
    final session = _checkLogin(c, d);
    if (session == null) return;
    _GatewaySession? gs;
    if (op == 'identify') {
      if (d['protocol_version'] != 1) {
        _close(c, 4007, 'protocol version');
        return;
      }
      if (d['device_id'] != session.deviceId) {
        _close(c, 4004, 'device_id');
        return;
      }
      gs = _GatewaySession(world.nextId(), session.deviceId);
      _sessions[gs.id] = gs;
    } else {
      gs = _sessions[d['gateway_session_id']];
      final at = gs?.disconnectedAt;
      if (gs == null ||
          gs.deviceId != session.deviceId ||
          at == null ||
          world.now().difference(at) > timings.resumeWindow) {
        _close(c, 4006, 'session invalid');
        return;
      }
    }
    // One connection per device (04 6.1).
    for (final other in _open.where(
      (o) => o != c && o.deviceId == session.deviceId,
    )) {
      _close(other, 4017, 'device replaced');
    }
    c
      ..identified = true
      ..deviceId = session.deviceId
      ..principalId = session.userId
      ..sessionId = session.id
      ..session = gs;
    gs.disconnectedAt = null;
    final person = world.people[session.userId]!;
    if (op == 'identify') {
      _send(c, 'ready', {
        'gateway_session_id': gs.id,
        'principal': person.ref,
        'device_id': session.deviceId,
        'permission_version': world.permissionVersion,
        'head_seq': world.headSeq,
        'sync_epoch': world.syncEpoch,
        'snapshot': world.snapshot(person.id),
      });
    } else {
      _send(c, 'resumed', {
        'gateway_session_id': gs.id,
        'head_seq': world.headSeq,
        'sync_epoch': world.syncEpoch,
      });
    }
    final cursor = d['cursor'] as int?;
    if (cursor == null) {
      c.live = true;
      return;
    }
    _catchUp(c, cursor, d['sync_epoch'] as String?);
  }

  /// 04 7.2 and 7.3: pages of `catchupPageSize` checked events, a `cursor` frame after
  /// `cursorAdvanceEvery` skipped ones and at the end of every page, then `catchup.complete`;
  /// or `resync_required` and live from the head.
  void _catchUp(_Conn c, int cursor, String? epoch) {
    final forced = _forcedResync;
    _forcedResync = null;
    final reason = forced ?? resyncReason(world, cursor, epoch);
    if (reason != null) {
      _send(c, 'resync_required', {
        'reason': reason,
        'head_seq': world.headSeq,
        'sync_epoch': world.syncEpoch,
      });
      c.live = true;
      return;
    }
    final head = world.headSeq;
    final events = world.eventsAfter(cursor, upTo: head).toList();
    var inPage = 0;
    var skipped = 0;
    for (var i = 0; i < events.length; i++) {
      final e = events[i];
      if (world.mayReceive(c.principalId!, e)) {
        _dispatch(c, e.envelope, catchUp: true);
      } else if (++skipped >= world.cursorAdvanceEvery) {
        skipped = 0;
        _send(c, 'cursor', {'seq': e.seq});
      }
      if (++inPage >= world.catchupPageSize || i == events.length - 1) {
        inPage = 0;
        skipped = 0;
        _send(c, 'cursor', {'seq': e.seq});
      }
    }
    _send(c, 'catchup.complete', {'head_seq': head});
    c.live = true;
  }

  void _reauth(_Conn c, JsonMap d) {
    final session = _checkLogin(c, d);
    if (session == null) return;
    if (session.id != c.sessionId) _close(c, 4004, 'other session');
  }

  void _focus(_Conn c, JsonMap d) {
    final ids = (d['thread_ids']! as List<Object?>).cast<String>();
    if (ids.length > timings.maxFocusedThreads) {
      _send(c, 'error', {'op': 'focus', 'code': 'validation_failed'});
      return;
    }
    // Unreadable threads are ignored silently (asyncapi `focus`).
    final next = {
      for (final id in ids)
        if (world.seesThread(c.principalId!, id)) id,
    };
    final before = c.session!.focus;
    c.session!.focus = next;
    for (final thread in next.difference(before)) {
      _sendRunDocument(c, thread);
    }
  }

  /// 04 11.3 item 5: a newly focused thread with an active run gets the whole document once.
  void _sendRunDocument(_Conn c, String threadId) {
    final run = world.activeRunOf(threadId);
    if (run == null) return;
    c.pendingPatches.remove(run['id']);
    _ephemeral(c, 'run.updated', run, {
      'run_id': run['id'],
      'version': run['version'],
      'snapshot': run,
    });
  }

  void _typing(_Conn c, String threadId) {
    if (!world.seesThread(c.principalId!, threadId)) return;
    final thread = world.threads[threadId]!;
    final expires = world.stamp(world.now().add(timings.typingTtl));
    for (final other in _open) {
      if (other == c || !other.live || other.principalId == c.principalId)
        continue;
      if (!other.focus.contains(threadId)) continue;
      _send(other, 'dispatch', {
        'id': world.nextId(),
        'type': 'typing.started',
        'schema_version': 1,
        'occurred_at': world.stamp(),
        'scope': {'channel_id': thread['channel_id'], 'thread_id': threadId},
        'resource': {'kind': 'thread', 'id': threadId},
        'payload': {
          'thread_id': threadId,
          'principal_id': c.principalId,
          'expires_at': expires,
        },
      });
    }
  }

  // -------------------------------------------------------------------------------------------
  // Delivery

  void _onEvent(FakeEvent e) {
    if (e.seq == null) return;
    for (final c in _open) {
      if (!c.live || !world.mayReceive(c.principalId!, e)) continue;
      _dispatch(c, e.envelope);
      if (e.type == 'access.changed') {
        // 04 8.3 item 2: the focus of removed targets ends.
        c.session!.focus = {
          for (final t in c.session!.focus)
            if (world.seesThread(c.principalId!, t)) t,
        };
      }
      if (e.type == 'session.revoked' &&
          (e.envelope['payload']! as JsonMap)['session_id'] == c.sessionId) {
        _close(c, 4012, 'revoked');
      }
    }
  }

  void _dispatch(_Conn c, JsonMap envelope, {bool catchUp = false}) {
    if (_holding && !catchUp) {
      c.held.add(envelope);
      return;
    }
    _send(c, 'dispatch', envelope);
    if (_duplicateNext) {
      _duplicateNext = false;
      _send(c, 'dispatch', envelope);
    }
  }

  void _onRunPatched(String runId, int base, int version, List<JsonMap> ops) {
    final run = world.runs[runId]!;
    final thread = run['thread_id']! as String;
    for (final c in _open) {
      if (!c.live || !c.focus.contains(thread)) continue;
      final pending = c.pendingPatches[runId];
      if (pending == null) {
        c.pendingPatches[runId] = _Pending(base, version, List.of(ops));
      } else {
        // Later `replace` operations on the same path replace earlier ones (04 11.3 item 3).
        for (final op in ops) {
          if (op['op'] == 'replace') {
            pending.ops.removeWhere(
              (o) => o['op'] == 'replace' && o['path'] == op['path'],
            );
          }
          pending.ops.add(op);
        }
        pending.version = version;
      }
      c.patchTimer ??= Timer(timings.runPatchCoalesce, () => _flushPatches(c));
    }
  }

  void _flushPatches(_Conn c) {
    c.patchTimer = null;
    if (!c.open) return;
    final pending = Map.of(c.pendingPatches);
    c.pendingPatches.clear();
    for (final MapEntry(key: runId, value: p) in pending.entries) {
      final run = world.runs[runId];
      if (run == null || !c.focus.contains(run['thread_id'])) continue;
      _ephemeral(c, 'run.updated', run, {
        'run_id': runId,
        'base_version': p.base,
        'version': p.version,
        'ops': p.ops,
      });
    }
  }

  void _ephemeral(_Conn c, String type, JsonMap run, JsonMap payload) => _send(
    c,
    'dispatch',
    {
      'id': world.nextId(),
      'type': type,
      'schema_version': 1,
      'occurred_at': world.stamp(),
      'scope': {'channel_id': run['channel_id'], 'thread_id': run['thread_id']},
      'resource': {'kind': 'run', 'id': run['id'], 'version': run['version']},
      'payload': jsonDecode(jsonEncode(payload)),
    },
  );

  // -------------------------------------------------------------------------------------------
  // Sending and closing

  void _send(_Conn c, String op, JsonMap d) {
    if (!c.open) return;
    final frame = <String, Object?>{'op': op, 'd': d};
    final problems = contract.serverFrame(frame);
    if (problems.isNotEmpty) {
      violations.add(
        'server frame $op ${d['type'] ?? ''}: ${problems.join('; ')}',
      );
      throw ContractError('frame $op', problems);
    }
    frames.add(GatewayFrameRecord(c.number, true, frame));
    c.socket.add(jsonEncode(frame));
  }

  void _close(_Conn c, int code, String? reason) {
    if (!c.open) return;
    c
      ..open = false
      ..closeCode = code;
    c.patchTimer?.cancel();
    unawaited(c.socket.close(code, reason));
  }

  void _onDone(_Conn c) {
    c.patchTimer?.cancel();
    c.closeCode ??= c.socket.closeCode;
    c.open = false;
    c.live = false;
    final gs = c.session;
    // A resumed session lives on in the newer connection; only its last end starts the window.
    if (gs != null && c.identified && !_open.any((o) => o.session == gs)) {
      gs.disconnectedAt = world.now();
    }
  }
}
