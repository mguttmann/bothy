// SPDX-License-Identifier: Apache-2.0

/// Test support of the terminal view (brief M4-35 Design note 4): a [FakeHost] that answers
/// the handshake with `NkResponder` of `package:bothy_noise/testing.dart`, and a
/// [FakeTerminalTransport] that pairs the controller with it in memory. Import it only from
/// tests; M4-36 and M4-38 use the real host service instead.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:bothy_noise/testing.dart';
import 'package:cryptography/cryptography.dart';
import 'package:terminal_ui/src/ports.dart';

/// What [FakeHost] does with message 1.
enum FakeHostHandshake {
  /// Checks the token and answers message 2.
  normal,

  /// Answers message 2 with a broken ciphertext, so the device's check fails.
  corruptMessage2,

  /// Never answers message 2.
  silent,
}

/// A host service in memory with a static X25519 key: reads message 1, checks its payload
/// against the expected session token, answers message 2, records every frame of the device
/// and sends `data`, `exit`, `replay_begin` and `replay_end` or closes the stream with any code.
///
/// ```dart
/// final host = await FakeHost.create(hostId: hostId);
/// final transport = FakeTerminalTransport(host);
/// host.expectToken(sessionId, 'token');
/// ```
final class FakeHost {
  FakeHost._(this.hostId, this._keyPair, this.noisePublicKey);

  /// Creates a host with a fixed static key from [seed] (32 bytes; a default seed when null).
  static Future<FakeHost> create({
    required String hostId,
    List<int>? seed,
  }) async {
    final pair = await X25519().newKeyPairFromSeed(
      seed ?? List<int>.generate(32, (i) => i + 1),
    );
    final public = await pair.extractPublicKey();
    return FakeHost._(
      hostId,
      pair,
      base64Url.encode(public.bytes).replaceAll('=', ''),
    );
  }

  /// The ID of the host (the prologue's host ID).
  final String hostId;

  final SimpleKeyPair _keyPair;

  /// The static key as `Host.noise_public_key` (base64url without padding).
  final String noisePublicKey;

  final Map<String, String> _tokens = {};

  /// How the host answers message 1.
  FakeHostHandshake handshake = FakeHostHandshake.normal;

  /// Whether the host sends `terminal.hello` when a stream connects.
  bool sendHello = true;

  /// The protocol versions announced in `hello`.
  List<int> protocolVersions = const [1];

  /// The `d` members of every `terminal.identify` the device sent.
  final List<Map<String, Object?>> identifies = [];

  /// Every frame the device sent after the handshake, in order, over all streams.
  final List<TerminalFrame> frames = [];

  /// For every entry of [frames], the value of [deviceMessages] when the device sent it.
  final List<int> frameSentAt = [];

  /// The number of messages the device has put on any stream so far (counted when sent, not
  /// when the host has read them).
  int deviceMessages = 0;

  /// The payloads of message 1 that did not match the expected token.
  final List<String> rejectedTokens = [];

  _FakeStream? _current;
  final List<_FakeStream> _streams = [];

  /// The number of streams that connected so far.
  int connections = 0;

  /// The number of streams that neither side has closed yet.
  int get openStreams => _streams.where((s) => !s.closed).length;

  /// The close codes the device closed its streams with, in order (`1000` for a detach).
  final List<int> deviceCloseCodes = [];

  /// Expects [token] as the payload of message 1 on the stream of [sessionId].
  void expectToken(String sessionId, String token) =>
      _tokens[sessionId] = token;

  /// Whether the current stream completed the handshake.
  bool get handshakeDone => _current?.transport != null;

  /// The `data` frames of the device as bytes, concatenated.
  Uint8List get input => Uint8List.fromList([
    for (final f in frames)
      if (f is DataFrame) ...f.bytes,
  ]);

  /// Sends [bytes] as one `data` frame to the device.
  Future<void> sendData(List<int> bytes) =>
      _send(DataFrame(Uint8List.fromList(bytes)));

  /// Sends `exit` with [code].
  Future<void> sendExit(int code) => _send(ExitFrame(code));

  /// Sends [scrollback] between `replay_begin` and `replay_end`.
  Future<void> sendReplay(List<int> scrollback) async {
    await _send(const ReplayBeginFrame());
    if (scrollback.isNotEmpty) await sendData(scrollback);
    await _send(const ReplayEndFrame());
  }

  /// Sends one raw message (no frame encoding) through the transport, for tampering tests.
  Future<void> sendRawEncrypted(List<int> plaintext) async {
    final s = _current;
    if (s == null || s.transport == null) throw StateError('no handshake');
    s.toDevice(await s.transport!.encrypt(plaintext));
  }

  /// Closes the current stream with [code] and [reason].
  Future<void> close(int code, [String? reason]) async {
    final s = _current;
    if (s == null) return;
    s.closeFromServer(code, reason);
  }

  Future<void> _send(TerminalFrame frame) async {
    final s = _current;
    if (s == null || s.transport == null) throw StateError('no handshake');
    s.toDevice(await s.transport!.encrypt(encodeFrame(frame)));
  }

  _FakeStream _connect(String streamPath) {
    connections++;
    final sessionId = _sessionOf(streamPath);
    final stream = _FakeStream(this, sessionId);
    _current = stream;
    _streams.add(stream);
    if (sendHello) {
      scheduleMicrotask(
        () => stream.toDevice(
          jsonEncode({
            'op': 'hello',
            'd': {
              'protocol_versions': protocolVersions,
              'dpop_nonce': 'nonce-1',
              'server_time': '2026-10-20T10:00:00Z',
            },
          }),
        ),
      );
    }
    return stream;
  }

  Future<void> _fromDevice(_FakeStream s, Object message) async {
    // Messages the device sent before it closed still arrive, as on a WebSocket.
    if (s.closedByServer) return;
    if (message is String) {
      final json = jsonDecode(message) as Map<String, Object?>;
      if (json['op'] == 'identify') {
        identifies.add((json['d']! as Map).cast<String, Object?>());
      }
      return;
    }
    final (sentAt, bytes) = message as (int, Uint8List);
    if (s.transport == null) {
      await _handshake(s, bytes);
      return;
    }
    try {
      frames.add(
        decodeFrame(await s.transport!.decrypt(bytes), FrameSender.device),
      );
      frameSentAt.add(sentAt);
    } on Object {
      s.closeFromServer(4104, null);
    }
  }

  Future<void> _handshake(_FakeStream s, Uint8List message1) async {
    try {
      final responder = await NkResponder.start(
        staticKeyPair: _keyPair,
        sessionId: uuidToBytes(s.sessionId),
        hostId: uuidToBytes(hostId),
      );
      final token = utf8.decode(await responder.readMessage1(message1));
      if (_tokens[s.sessionId] != token) {
        rejectedTokens.add(token);
        s.closeFromServer(4104, null);
        return;
      }
      if (handshake == FakeHostHandshake.silent) return;
      final (message2, transport) = await responder.writeMessage2(
        utf8.encode(
          jsonEncode({'v': 1, 'host_version': '1.0.0', 'profile': 'shell'}),
        ),
      );
      if (handshake == FakeHostHandshake.corruptMessage2) {
        message2[message2.length - 1] ^= 0xff;
      }
      s.transport = transport;
      s.toDevice(message2);
    } on Object {
      s.closeFromServer(4104, null);
    }
  }

  static String _sessionOf(String streamPath) {
    final parts = streamPath.split('/');
    final i = parts.indexOf('sessions');
    return parts[i + 1];
  }
}

final class _FakeStream implements TerminalChannel {
  _FakeStream(this.host, this.sessionId);

  final FakeHost host;
  final String sessionId;
  final StreamController<Object> _toDevice = StreamController<Object>();
  NoiseTransport? transport;
  bool closed = false;
  bool closedByServer = false;
  Future<void> _inbox = Future<void>.value();

  @override
  int? closeCode;

  @override
  String? closeReason;

  @override
  Stream<Object> get messages => _toDevice.stream;

  void toDevice(Object message) {
    if (!closed) _toDevice.add(message);
  }

  void closeFromServer(int code, String? reason) {
    if (closed) return;
    closed = true;
    closedByServer = true;
    closeCode = code;
    closeReason = reason;
    unawaited(_toDevice.close());
  }

  @override
  void sendText(String text) => _enqueue(text);

  @override
  void sendBinary(Uint8List bytes) {
    // Like the sink of a closed WebSocket, a send after close throws.
    if (closed) throw StateError('stream closed');
    _enqueue((++host.deviceMessages, Uint8List.fromList(bytes)));
  }

  void _enqueue(Object message) {
    // The host handles messages strictly in order, like one WebSocket.
    _inbox = _inbox.then((_) => host._fromDevice(this, message));
  }

  @override
  Future<void> close([int code = 1000]) async {
    if (closed) return;
    closed = true;
    host.deviceCloseCodes.add(code);
    unawaited(_toDevice.close());
  }
}

/// A [TerminalTransport] that connects the controller to a [FakeHost] in memory.
final class FakeTerminalTransport implements TerminalTransport {
  /// Creates the transport for [host].
  FakeTerminalTransport(this.host);

  /// The host every stream goes to.
  final FakeHost host;

  /// The stream paths the controller connected to, in order.
  final List<String> connected = [];

  /// When set, [connect] throws it (a network error).
  Object? connectError;

  @override
  Future<TerminalChannel> connect(String streamPath) async {
    final error = connectError;
    if (error != null) throw error;
    connected.add(streamPath);
    return host._connect(streamPath);
  }

  /// When set, [identify] throws it (a failed token refresh or a network error).
  Object? identifyError;

  @override
  Future<Map<String, Object?>> identify(Map<String, Object?> hello) async {
    final error = identifyError;
    if (error != null) throw error;
    return {
      'protocol_version': 1,
      'device_id': '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e01',
    };
  }
}
