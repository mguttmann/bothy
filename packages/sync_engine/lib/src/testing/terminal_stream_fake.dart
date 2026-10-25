// SPDX-License-Identifier: Apache-2.0

/// The device stream of the terminal in the fake server (asyncapi channel `terminalStream`;
/// docs/spec/24-terminal.md 5; brief M4-36 Scope): `terminal.hello` with the DPoP nonce, the
/// check of `terminal.identify` (access token, DPoP proof with `htm = GET` and the HTTPS form
/// of the stream URL, the device of the session), then only binary frames, relayed to the
/// in-process host of [FakeTerminal.hostLink]. Nothing of the frames is read or kept: the fake
/// is a relay like the server (24 5.1).
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'dpop_verifier.dart';
import 'world.dart';

/// The pattern of the stream path.
final RegExp terminalStreamPath = RegExp(
  r'^/api/v1/terminal/sessions/([0-9a-f-]{36})/stream$',
);

/// Serves the device streams of the terminal for one fake server.
final class TerminalStreamFake {
  /// Creates the endpoint over [world], checking proofs with [verifier]; [baseUri] is the
  /// origin of the fake.
  TerminalStreamFake({
    required this.world,
    required this.verifier,
    required this.baseUri,
  });

  /// The world.
  final FakeWorld world;

  /// The DPoP check.
  final DpopVerifier verifier;

  /// The origin of the fake.
  final Uri Function() baseUri;

  final Set<WebSocket> _open = {};

  /// Takes the upgraded [socket] of the stream of session [sessionId] at [path].
  void accept(WebSocket socket, String sessionId, String path) {
    _open.add(socket);
    unawaited(_run(socket, sessionId, path));
  }

  Future<void> _run(WebSocket socket, String sessionId, String path) async {
    final terminal = world.terminal;
    var closed = false;
    void close(int code, String? reason) {
      if (closed) return;
      closed = true;
      unawaited(socket.close(code, reason));
    }

    final session = terminal.sessions[sessionId];
    if (session == null || session.closed) {
      close(4100, session?.closeReason);
      _open.remove(socket);
      return;
    }
    socket.add(
      jsonEncode({
        'op': 'hello',
        'd': {
          'protocol_versions': [1],
          'dpop_nonce': verifier.currentNonce,
          'server_time': world.stamp(),
        },
      }),
    );
    FakeTerminalStream? stream;
    FakeTerminalPipe? pipe;
    try {
      await for (final message in socket) {
        if (stream == null) {
          if (message is! String) {
            close(4002, null);
            break;
          }
          final failure = _identify(message, session, path);
          if (failure != null) {
            close(4004, failure);
            break;
          }
          // 24 6.2: one device stream per session; an older one of the same device gives way.
          terminal.closeStream(sessionId, 4103);
          stream = terminal.open(sessionId, session.deviceId, close);
          final token = session.token;
          session.token = null;
          if (session.state != 'active') {
            session
              ..state = 'active'
              ..startedAt ??= world.stamp()
              ..detachedAt = null
              ..detachedAtText = null;
            world.terminalSessionChanged(session);
          }
          final link = terminal.hostLink;
          if (link == null || token == null) {
            terminal.closeStream(sessionId, link == null ? 4102 : 4101);
            break;
          }
          final p = FakeTerminalPipe(
            hostId: session.hostId,
            sessionId: sessionId,
            sessionToken: token,
            toDevice: (bytes) {
              if (!closed) socket.add(Uint8List.fromList(bytes));
            },
            close: (code, reason) {
              if (identical(terminal.streams[sessionId], stream)) {
                terminal.streams.remove(sessionId);
              }
              if (code == 4100 && !session.closed) {
                world.endTerminalSession(session, reason ?? 'process_exited');
              }
              close(code, reason);
            },
          );
          pipe = p;
          stream.pipe = p;
          await link.connect(p);
          continue;
        }
        if (message is! List<int>) {
          // asyncapi `x-close-codes`: a text frame after `identify` is a decode error.
          terminal.closeStream(sessionId, 4002);
          break;
        }
        pipe?.deliverFromDevice(Uint8List.fromList(message));
      }
    } on Object {
      // The device went away without a close frame.
    } finally {
      _open.remove(socket);
      pipe?.endFromDevice();
      if (stream != null && identical(terminal.streams[sessionId], stream)) {
        terminal.streams.remove(sessionId);
        if (!session.closed) {
          final now = world.now();
          session
            ..state = 'detached'
            ..detachedAt = now
            ..detachedAtText = world.stamp(now);
          world.terminalSessionChanged(session);
        }
      }
      close(1000, null);
    }
  }

  /// Checks `terminal.identify` in [text]; returns the failure, or null when it passes.
  String? _identify(String text, FakeTerminalSession session, String path) {
    final Object? json;
    try {
      json = jsonDecode(text);
    } on FormatException {
      return 'decode';
    }
    if (json is! Map || json['op'] != 'identify' || json['d'] is! Map) {
      return 'no identify';
    }
    final d = (json['d']! as Map).cast<String, Object?>();
    if (d['protocol_version'] != 1) return 'protocol_version';
    final token = d['access_token'];
    if (token is! String) return 'no token';
    final login = world.sessions.values
        .where(
          (s) => s.accessToken == token || s.olderAccessTokens.contains(token),
        )
        .firstOrNull;
    if (login == null || login.revoked) return 'unknown token';
    final result = verifier.verify(
      d['dpop'] as String?,
      method: 'GET',
      htu: '${baseUri().origin}$path',
      accessToken: token,
      expectedJkt: login.jkt,
    );
    if (result is DpopRejected) return result.code;
    // `terminal.stream`: only the device of the current session token passes.
    if (login.userId != session.userId ||
        login.deviceId != session.deviceId ||
        d['device_id'] != session.deviceId) {
      return 'device';
    }
    return null;
  }

  /// Closes every open stream.
  Future<void> close() async {
    for (final s in List.of(_open)) {
      await s.close(1001);
    }
    _open.clear();
  }
}
