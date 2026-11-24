// SPDX-License-Identifier: Apache-2.0

/// The dart:io WebSocket transport of the gateway on `web_socket_channel` (15 7.1, brief
/// M4-13 Scope).
library;

import 'dart:async';
import 'dart:io';

import 'package:web_socket_channel/io.dart';

import 'ports.dart';

/// [GatewayTransport] over `dart:io` WebSockets.
///
/// No upgrade headers are set: the DPoP proof travels in `identify`, `resume` and `reauth`
/// (04 6.3 item 3, R-SY-01). The server enforces `sync.ws_max_frame_bytes` (`4002`), so the
/// client has no size check of its own.
///
/// Example:
/// ```dart
/// final transport = IoGatewayTransport();
/// final channel = await transport.connect(
///   Uri.parse('wss://chat.example.com/api/v1/gateway'),
/// );
/// ```
final class IoGatewayTransport implements GatewayTransport {
  /// Creates the transport; [pingInterval] lets `dart:io` send WebSocket pings, `null` (the
  /// default) leaves liveness to the gateway heartbeat of 04 6.3 item 5.
  IoGatewayTransport({this.pingInterval});

  /// The interval of WebSocket pings of `dart:io`, or `null` for none.
  final Duration? pingInterval;

  /// Each attempt has its own [HttpClient], so a handshake that passes [timeout] is aborted
  /// with it instead of running on beside the next attempt (04 6.1); a socket that still
  /// opens later is closed at once.
  @override
  Future<GatewayChannel> connect(Uri url, {Duration? timeout}) async {
    final client = HttpClient();
    final opening = WebSocket.connect(url.toString(), customClient: client);
    final WebSocket socket;
    try {
      socket = timeout == null ? await opening : await opening.timeout(timeout);
    } on TimeoutException {
      client.close(force: true);
      unawaited(
        opening.then<void>((late) => late.close(1000), onError: (Object _) {}),
      );
      rethrow;
    } finally {
      // The socket of an upgrade is detached from the client, so closing it keeps the socket.
      client.close();
    }
    socket.pingInterval = pingInterval;
    final channel = IOWebSocketChannel(socket);
    await channel.ready;
    return _IoChannel(channel);
  }
}

final class _IoChannel implements GatewayChannel {
  _IoChannel(this._channel)
    : frames = _channel.stream
          // Binary frames are not part of the gateway (04 2.3); the server never sends them.
          .where((m) => m is String)
          .cast<String>();

  final IOWebSocketChannel _channel;

  @override
  final Stream<String> frames;

  @override
  void send(String text) => _channel.sink.add(text);

  @override
  Future<void> close(int code, [String? reason]) async {
    await _channel.sink.close(code, reason);
  }

  @override
  int? get closeCode => _channel.closeCode;

  @override
  String? get closeReason => _channel.closeReason;
}
