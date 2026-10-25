// SPDX-License-Identifier: Apache-2.0

/// The WebSocket of one terminal stream in the desktop and mobile app
/// (docs/spec/15-client-architecture.md 8 and 23.1; asyncapi channel `terminalStream`; brief
/// M4-36 Design note 1): its own connection per session, never through `sync_engine`, and
/// `identify` with the access token and a DPoP proof of the device.
library;

import 'dart:async';
import 'dart:io' show HttpClient;
import 'dart:typed_data';

import 'package:auth/auth.dart' show AuthSession;
import 'package:terminal_ui/terminal_ui.dart'
    show TerminalChannel, TerminalTransport, terminalStreamProtocolVersion;
import 'package:web_socket_channel/io.dart';

/// [TerminalTransport] over `dart:io` WebSockets for one tab.
///
/// [connect] opens `wss://<host><stream_path>` of the instance [origin] (`ws` only with
/// [allowPlainHttpForTests], the fake server of the tests). [identify] answers `terminal.hello`
/// with the access token of [auth] and a DPoP proof built by `DpopProofFactory.forGateway`
/// for the stream URL: `htm = GET`, `htu` the HTTPS form of the URL, the nonce of `hello` and
/// `ath` of the token, the form asyncapi `TerminalStreamIdentify` names (D-M4-36-1). One
/// transport serves one tab, so the proof always names the stream the tab opened last.
///
/// Nothing of the stream is logged; errors carry no content (R-CL-17).
///
/// Example:
/// ```dart
/// final transport = IoTerminalTransport(origin: instance.address, auth: services.auth);
/// final channel = await transport.connect(grant.streamPath);
/// ```
final class IoTerminalTransport implements TerminalTransport {
  /// Creates the transport of one tab.
  IoTerminalTransport({
    required this.origin,
    required this._auth,
    this.allowPlainHttpForTests = false,
  });

  /// The instance origin (`https://<host>`).
  final Uri origin;

  final AuthSession _auth;

  /// Whether `http` origins (stream `ws`) are accepted: the fake server of the tests only.
  final bool allowPlainHttpForTests;

  Uri? _url;

  /// The URL of [streamPath] below [origin]: `wss` for `https`, `ws` for `http` with
  /// [allowPlainHttp] (the fake server of the tests). Throws an [ArgumentError] for any other
  /// origin.
  ///
  /// Example: `streamUrlOf(Uri.parse('https://chat.example.com'), '/api/v1/terminal/sessions/<id>/stream')`
  /// is `wss://chat.example.com/api/v1/terminal/sessions/<id>/stream`.
  static Uri streamUrlOf(
    Uri origin,
    String streamPath, {
    bool allowPlainHttp = false,
  }) {
    final scheme = origin.scheme.toLowerCase();
    final plain = scheme == 'http';
    if (!origin.isAbsolute ||
        origin.host.isEmpty ||
        !(scheme == 'https' || (plain && allowPlainHttp))) {
      throw ArgumentError.value(
        '${origin.scheme}://${origin.host}',
        'origin',
        'must be an absolute https origin',
      );
    }
    return origin.replace(
      scheme: plain ? 'ws' : 'wss',
      path: streamPath,
      query: null,
      fragment: null,
    );
  }

  @override
  Future<TerminalChannel> connect(String streamPath) async {
    final url = streamUrlOf(
      origin,
      streamPath,
      allowPlainHttp: allowPlainHttpForTests,
    );
    // A client of its own per stream: the shared client of `WebSocket.connect` is created
    // once per process and ignores the HttpOverrides of the zone it runs in; this one follows
    // them like the REST client does, and closes with the stream.
    final client = HttpClient();
    final channel = IOWebSocketChannel.connect(url, customClient: client);
    try {
      await channel.ready;
    } on Object {
      client.close(force: true);
      rethrow;
    }
    _url = url;
    return _IoTerminalChannel(channel, client);
  }

  @override
  Future<Map<String, Object?>> identify(Map<String, Object?> hello) async {
    final url = _url;
    final nonce = hello['dpop_nonce'];
    final device = _auth.session?.deviceId;
    if (url == null || nonce is! String || device == null) {
      throw StateError('terminal identify without stream, nonce or device');
    }
    final token = await _auth.refresh.ensureFresh();
    final proof = await _auth.proofs.forGateway(
      gatewayUrl: url,
      accessToken: token,
      nonce: nonce,
    );
    return {
      'protocol_version': terminalStreamProtocolVersion,
      'access_token': token,
      'dpop': proof,
      'device_id': device.value,
    };
  }
}

/// One stream: text messages stay strings, binary messages become [Uint8List].
final class _IoTerminalChannel implements TerminalChannel {
  _IoTerminalChannel(this._channel, HttpClient client)
    : messages = _channel.stream
          .map((m) => m is String ? m : Uint8List.fromList(m as List<int>))
          .transform(
            StreamTransformer.fromHandlers(
              handleDone: (sink) {
                client.close();
                sink.close();
              },
            ),
          );

  final IOWebSocketChannel _channel;

  @override
  final Stream<Object> messages;

  @override
  void sendText(String text) => _channel.sink.add(text);

  @override
  void sendBinary(Uint8List bytes) => _channel.sink.add(bytes);

  @override
  Future<void> close([int code = 1000]) => _channel.sink.close(code);

  @override
  int? get closeCode => _channel.closeCode;

  @override
  String? get closeReason => _channel.closeReason;
}
