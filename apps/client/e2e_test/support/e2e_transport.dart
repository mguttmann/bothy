// SPDX-License-Identifier: Apache-2.0

// The gateway transport of the E2E apps: the `dart:io` WebSocket of a real build (as
// `IoGatewayTransport` of sync_engine), recording what happened to each connection, so a failed
// wait names the cause and the revocation flow can check the close code `4012` the server sent
// (04 6.6). Each connection gets its own `HttpClient`, created through the overrides of
// e2e_network.dart: the shared client of `WebSocket.connect` may exist before the overrides (the
// test runner of flutter-tester opens a WebSocket of its own first) and would then use neither
// the test CA nor the tunnel.

import 'dart:async';
import 'dart:io';

import 'package:sync_engine/sync_engine.dart';

/// A `dart:io` WebSocket transport with a record of its connections.
final class RecordingTransport implements GatewayTransport {
  final GatewayTransport _inner = _IoTransport();

  /// The close codes the server sent, oldest first.
  final List<int> closeCodes = [];

  /// The last failed connect, or null.
  Object? lastConnectError;

  /// How many connections opened.
  int opened = 0;

  @override
  Future<GatewayChannel> connect(Uri url) async {
    try {
      final channel = await _inner.connect(url);
      opened++;
      return _Recorded(channel, this);
    } on Object catch (e) {
      lastConnectError = e;
      rethrow;
    }
  }

  /// A short description for a failure message.
  String describe() =>
      'connections $opened, close codes $closeCodes, last connect error $lastConnectError';
}

final class _Recorded implements GatewayChannel {
  _Recorded(this._inner, RecordingTransport record)
    : frames = _inner.frames.transform(
        StreamTransformer.fromHandlers(
          handleDone: (sink) {
            final code = _inner.closeCode;
            if (code != null) record.closeCodes.add(code);
            sink.close();
          },
        ),
      );

  final GatewayChannel _inner;

  @override
  final Stream<String> frames;

  @override
  void send(String text) => _inner.send(text);

  @override
  Future<void> close(int code, [String? reason]) => _inner.close(code, reason);

  @override
  int? get closeCode => _inner.closeCode;

  @override
  String? get closeReason => _inner.closeReason;
}

/// The transport of `IoGatewayTransport` (text frames only, 04 2.3) with a fresh client.
final class _IoTransport implements GatewayTransport {
  @override
  Future<GatewayChannel> connect(Uri url) async {
    final socket = await WebSocket.connect(
      url.toString(),
      customClient: HttpClient(),
    );
    return _IoChannel(socket);
  }
}

final class _IoChannel implements GatewayChannel {
  _IoChannel(this._socket)
    : frames = _socket.where((m) => m is String).cast<String>();

  final WebSocket _socket;

  @override
  final Stream<String> frames;

  @override
  void send(String text) => _socket.add(text);

  @override
  Future<void> close(int code, [String? reason]) => _socket.close(code, reason);

  @override
  int? get closeCode => _socket.closeCode;

  @override
  String? get closeReason => _socket.closeReason;
}
