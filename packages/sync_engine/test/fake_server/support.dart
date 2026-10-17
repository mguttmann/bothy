// SPDX-License-Identifier: Apache-2.0

// The world of the fake server's own tests (brief M4-17, Tests): a fake on 127.0.0.1 with the
// standard world, an auth session of the real `auth` package over HTTP to it, and a raw
// client that builds DPoP proofs by hand for the negative cases.

import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:domain/domain.dart';
import 'package:sync_engine/testing.dart';

import '../support/fakes.dart' show MemorySecretStore;
import '../support/software_p256_signer.dart';

export '../support/software_p256_signer.dart' show SoftwareP256Signer;

/// A fake with the standard world.
class FakeWorldHarness {
  FakeWorldHarness._(this.server, this.seed);

  /// Starts the fake and fills the standard world.
  static Future<FakeWorldHarness> start({
    FakeServerOptions options = const FakeServerOptions(),
  }) async {
    final server = await FakeServer.start(options: options);
    return FakeWorldHarness._(server, WorldSeed.standard(server.world));
  }

  /// The fake.
  final FakeServer server;

  /// The IDs of the standard world.
  final WorldSeed seed;

  final List<TestDevice> _devices = [];

  /// A device of [login] with its own key ([keySeed]) and auth session.
  TestDevice device({String login = WorldSeed.alexLogin, int keySeed = 1}) {
    final d = TestDevice(
      server.baseUri,
      SoftwareP256Signer.seeded(keySeed),
      login,
    );
    _devices.add(d);
    return d;
  }

  /// Stops every device and the fake.
  Future<void> stop() async {
    for (final d in _devices) {
      await d.auth.dispose();
    }
    await server.stop();
  }
}

class _Prompt implements StepUpPrompt {
  @override
  Future<bool> ask(String purposeText) async => true;
}

/// One device: the auth session of `package:auth` against the fake.
class TestDevice {
  /// Builds the session for [login] with the DPoP key [key].
  TestDevice(this.baseUri, this.key, this.login) {
    final random = Random(key.thumbprint.hashCode);
    auth = AuthSession(
      serverUrl: baseUri,
      keys: DeviceKeys(dpop: key, stepUp: SoftwareP256Signer.seeded(99)),
      secrets: MemorySecretStore(),
      clock: const SystemClock(),
      ids: Uuid7Generator(
        nowMillis: () => DateTime.now().millisecondsSinceEpoch,
        random: (bits) => random.nextInt(1 << (bits > 31 ? 31 : bits)),
      ),
      config: AppConfig.defaults(),
      device: const DeviceDescription(
        name: 'Alex laptop',
        platform: 'macos',
        appVersion: '1.0.0',
      ),
      buildClient: (interceptors) => createApiClient(
        baseUrl: baseUri,
        traceContext: () => TraceContext(traceId: '1' * 32, sampled: false),
        readRetry: ReadRetryPolicy(
          base: Duration.zero,
          cap: Duration.zero,
          maxAttempts: 1,
        ),
        authInterceptors: interceptors,
      ),
      stepUpPrompt: _Prompt(),
    );
  }

  /// The address of the fake.
  final Uri baseUri;

  /// The DPoP key.
  final SoftwareP256Signer key;

  /// The login name.
  final String login;

  /// The auth session.
  late final AuthSession auth;

  /// The API client with the auth interceptors.
  ApiClient get api => auth.api;

  /// Signs in with password and TOTP; returns the outcome.
  Future<LoginOutcome> signIn() => auth.login.loginWithPassword(
    loginName: login,
    password: WorldSeed.password,
    totpCode: WorldSeed.totp,
  );
}

/// Builds DPoP proofs by hand and sends raw requests, for the negative cases.
class RawClient {
  /// Creates the client for [baseUri] with [key].
  RawClient(this.baseUri, this.key);

  /// The address of the fake.
  final Uri baseUri;

  /// The key.
  final SoftwareP256Signer key;

  final HttpClient _http = HttpClient();
  int _jti = 0;

  /// A proof for [method] [htu]; every member can be replaced.
  Future<String> proof({
    required String method,
    required String htu,
    String? accessToken,
    String? nonce,
    String? jti,
    SoftwareP256Signer? signWith,
  }) async {
    String enc(Object o) =>
        base64Url.encode(utf8.encode(jsonEncode(o))).replaceAll('=', '');
    final header = enc({
      'typ': 'dpop+jwt',
      'alg': 'ES256',
      'jwk': key.publicJwk,
    });
    final payload = enc({
      'jti': jti ?? 'jti-${++_jti}-${DateTime.now().microsecondsSinceEpoch}',
      'htm': method,
      'htu': htu,
      'iat': DateTime.now().millisecondsSinceEpoch ~/ 1000,
      'ath': ?(accessToken == null ? null : accessTokenHash(accessToken)),
      'nonce': ?nonce,
    });
    final signature = await (signWith ?? key).sign(
      Uint8List.fromList(ascii.encode('$header.$payload')),
    );
    return '$header.$payload.${base64Url.encode(signature).replaceAll('=', '')}';
  }

  /// Sends [method] to [path] with [headers] and a JSON [body]; returns status, headers and
  /// the decoded body.
  Future<RawResponse> send(
    String method,
    String path, {
    Map<String, String> headers = const {},
    Object? body,
  }) async {
    final request = await _http.openUrl(method, baseUri.resolve(path));
    headers.forEach(request.headers.set);
    if (body != null) {
      request.headers.contentType = ContentType.json;
      request.write(jsonEncode(body));
    }
    final response = await request.close();
    final text = await utf8.decoder.bind(response).join();
    return RawResponse(
      response.statusCode,
      response.headers,
      text.isEmpty ? null : jsonDecode(text),
    );
  }

  /// Closes the HTTP client.
  void close() => _http.close(force: true);
}

/// A raw response.
class RawResponse {
  /// Creates it.
  RawResponse(this.status, this.headers, this.body);

  /// The status.
  final int status;

  /// The headers.
  final HttpHeaders headers;

  /// The decoded body.
  final Object? body;

  /// The problem code of the body.
  String? get code => body is Map ? (body! as Map)['code'] as String? : null;
}

/// A gateway client that speaks the frames by hand.
class RawGateway {
  RawGateway._(this._socket, this._client) {
    _socket.listen(
      (data) => frames.add(jsonDecode(data as String) as Map<String, Object?>),
      onDone: () => _done = true,
    );
  }

  /// Connects to the gateway of [server] and waits for `hello`.
  static Future<RawGateway> connect(Uri gatewayUri, RawClient client) async {
    final g = RawGateway._(
      await WebSocket.connect(gatewayUri.toString()),
      client,
    );
    await g.waitFor('hello');
    return g;
  }

  final WebSocket _socket;
  final RawClient _client;
  bool _done = false;

  /// Every frame of the server, decoded.
  final List<Map<String, Object?>> frames = [];

  /// Whether the server closed.
  bool get done => _done;

  /// The close code.
  int? get closeCode => _socket.closeCode;

  /// The ops received.
  List<String> get ops => [for (final f in frames) f['op']! as String];

  /// The `d` of the frames with [op].
  List<Map<String, Object?>> data(String op) => [
    for (final f in frames)
      if (f['op'] == op) f['d']! as Map<String, Object?>,
  ];

  /// The nonce of `hello`.
  String get nonce => data('hello').first['dpop_nonce']! as String;

  /// Sends a raw frame.
  void send(String op, Map<String, Object?> d) =>
      _socket.add(jsonEncode({'op': op, 'd': d}));

  /// Sends `identify` with [token] and a proof over the `hello` nonce (or [nonce]).
  Future<void> identify(
    String token,
    String deviceId, {
    int? cursor,
    String? epoch,
    String? nonce,
    String? htu,
  }) async {
    final origin = _client.baseUri.origin;
    send('identify', {
      'protocol_version': 1,
      'access_token': token,
      'dpop': await _client.proof(
        method: 'GET',
        htu: htu ?? '$origin/api/v1/gateway',
        accessToken: token,
        nonce: nonce ?? this.nonce,
      ),
      'device_id': deviceId,
      'cursor': ?cursor,
      'sync_epoch': ?epoch,
      'client': {'platform': 'macos', 'version': '1.0.0'},
    });
  }

  /// Waits until a frame with [op] arrived (or [count] of them).
  Future<void> waitFor(String op, {int count = 1}) =>
      _until(() => ops.where((o) => o == op).length >= count, 'frame $op');

  /// Waits until the server closed the socket.
  Future<void> waitClosed() => _until(() => _done, 'close');

  Future<void> _until(bool Function() ok, String what) async {
    final end = DateTime.now().add(const Duration(seconds: 10));
    while (!ok()) {
      if (DateTime.now().isAfter(end)) throw StateError('timed out: $what');
      await Future<void>.delayed(const Duration(milliseconds: 5));
    }
  }

  /// Closes from the client side.
  Future<void> close() => _socket.close(1000);
}

/// Signs [device] in and returns the access token and device ID the fake issued.
Future<({String token, String deviceId})> signedIn(
  FakeWorldHarness h,
  TestDevice device,
) async {
  final outcome = await device.signIn();
  if (outcome is! LoginSignedIn) throw StateError('sign-in failed: $outcome');
  final session = h.server.world.sessions[outcome.tokens.session.sid]!;
  return (token: session.accessToken, deviceId: session.deviceId);
}
