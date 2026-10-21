// SPDX-License-Identifier: Apache-2.0

/// The stream of the terminal in the console: the browser WebSocket on the same origin and an
/// `identify` without a token (docs/spec/15-client-architektur.md 23.1, 08 7.5; brief M6-18
/// Design note 2).
library;

import 'dart:async';
import 'dart:typed_data';

import 'package:terminal_ui/terminal_ui.dart'
    show TerminalChannel, TerminalTransport, terminalStreamProtocolVersion;

import 'ports.dart';

/// [TerminalTransport] of the console.
///
/// [connect] opens the stream path of the grant on [origin] (`wss` for an `https` page, `ws`
/// for `http`), so the browser sends the console cookie with the upgrade and the page's
/// `Origin`, which the server checks before the upgrade (SEC-CORS-3). The `connect-src 'self'`
/// of the console's CSP allows exactly this origin (ADR 0070 item 2).
///
/// [identify] returns only the protocol version and the device of the session token, never an
/// access token or a DPoP proof: the console holds neither, and the server accepts an
/// `identify` without a token only from the console session that opened the session (asyncapi
/// `TerminalStreamIdentify`). The device comes from [deviceIdFor] for the stream path of the
/// last [connect], so one transport serves one tab (one controller) at a time.
///
/// The channel keeps the order of the protocol: one text message (`identify`) before any
/// binary message, after it only binary messages; anything else throws a [StateError] instead
/// of reaching the server.
///
/// Example:
/// ```dart
/// final transport = ConsoleTerminalTransport(origin: env.origin,
///     openSocket: ports.connect, deviceIdFor: api.deviceIdFor);
/// ```
final class ConsoleTerminalTransport implements TerminalTransport {
  /// Creates the transport of one tab.
  ConsoleTerminalTransport({
    required this.origin,
    required this.openSocket,
    required this.deviceIdFor,
  });

  /// The origin of the page, which is also the origin of the API and the stream.
  final Uri origin;

  /// Opens the browser WebSocket.
  final TerminalSocketConnector openSocket;

  /// The device of the grant of a stream path (`ConsoleTerminalApi.deviceIdFor`).
  final String? Function(String streamPath) deviceIdFor;

  String? _streamPath;

  /// The WebSocket URL of [streamPath] on [origin].
  static Uri streamUrl(Uri origin, String streamPath) => Uri(
    scheme: origin.scheme == 'https' ? 'wss' : 'ws',
    host: origin.host,
    // Only an explicit port carries over: the default of https is the default of wss.
    port: origin.hasPort ? origin.port : null,
    path: streamPath,
  );

  @override
  Future<TerminalChannel> connect(String streamPath) async {
    final socket = await openSocket(streamUrl(origin, streamPath));
    _streamPath = streamPath;
    return _ConsoleChannel(socket);
  }

  /// Throws a [StateError] before a [connect] or when the grant of the stream path is unknown,
  /// so no `identify` leaves without the device the server expects.
  @override
  Future<Map<String, Object?>> identify(Map<String, Object?> hello) async {
    final path = _streamPath;
    final device = path == null ? null : deviceIdFor(path);
    if (device == null) throw StateError('no grant for this stream');
    return {
      'protocol_version': terminalStreamProtocolVersion,
      'device_id': device,
    };
  }
}

/// One stream of the console: text until `identify`, then only binary.
final class _ConsoleChannel implements TerminalChannel {
  _ConsoleChannel(this._socket);

  final TerminalSocket _socket;
  bool _identified = false;

  @override
  Stream<Object> get messages => _socket.messages;

  @override
  void sendText(String text) {
    if (_identified) throw StateError('only binary after identify');
    _identified = true;
    _socket.sendText(text);
  }

  @override
  void sendBinary(Uint8List bytes) {
    if (!_identified) throw StateError('identify comes first');
    _socket.sendBinary(bytes);
  }

  @override
  Future<void> close([int code = 1000]) => _socket.close(code);

  @override
  int? get closeCode => _socket.closeCode;

  @override
  String? get closeReason => _socket.closeReason;
}
