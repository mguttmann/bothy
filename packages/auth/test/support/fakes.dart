// SPDX-License-Identifier: Apache-2.0

// Fakes for the auth tests: a fake server behind a Dio adapter, a recording secret store, a
// manual clock and the platform ports. No test opens a socket (brief M4-09, Boundaries).

import 'dart:async';
import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';

import 'software_p256_signer.dart';

/// The server address of every test.
final Uri serverUrl = Uri.parse('https://chat.example.com');

/// One request as the fake server saw it.
class SeenRequest {
  /// Records a request.
  SeenRequest(this.method, this.path, this.headers, this.body, this.extra);

  /// HTTP method in upper case.
  final String method;

  /// Path below `/api/v1`, for example `/auth/token/refresh`.
  final String path;

  /// Headers with lower-case names.
  final Map<String, String> headers;

  /// The decoded JSON body, or `null`.
  final Object? body;

  /// The request options' extra map at send time.
  final Map<String, Object?> extra;

  /// The decoded DPoP proof: header and payload.
  ({Map<String, Object?> header, Map<String, Object?> payload})? get proof {
    final p = headers['dpop'];
    if (p == null) return null;
    final parts = p.split('.');
    return (
      header:
          jsonDecode(utf8.decode(b64Decode(parts[0]))) as Map<String, Object?>,
      payload:
          jsonDecode(utf8.decode(b64Decode(parts[1]))) as Map<String, Object?>,
    );
  }
}

/// A canned answer of the fake server.
class FakeReply {
  /// Creates an answer.
  const FakeReply(this.status, [this.body, this.headers = const {}]);

  /// An RFC 9457 problem with [code].
  factory FakeReply.problem(
    int status,
    String code, {
    String? reason,
    Map<String, String> headers = const {},
  }) => FakeReply(status, {
    'type': '/api/v1/problems/$code',
    'title': code,
    'status': status,
    'code': code,
    'reason': ?reason,
  }, headers);

  /// HTTP status.
  final int status;

  /// JSON body or `null`.
  final Object? body;

  /// Response headers.
  final Map<String, String> headers;
}

/// Handles one request.
typedef Route = FutureOr<FakeReply> Function(SeenRequest request);

/// Thrown by a route to simulate a network failure.
class NetworkDown implements Exception {}

/// A Dio adapter that dispatches to routes by `METHOD path` and records every request.
class FakeServer implements HttpClientAdapter {
  /// Every request seen, in order.
  final List<SeenRequest> requests = [];

  /// Routes by `METHOD /path`.
  final Map<String, Route> routes = {};

  /// Called before each request is recorded (for ordering checks).
  void Function(SeenRequest request)? onSend;

  /// The requests to [path].
  List<SeenRequest> to(String path) =>
      requests.where((r) => r.path == path).toList();

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final bytes = <int>[];
    if (requestStream != null) {
      await for (final chunk in requestStream) {
        bytes.addAll(chunk);
      }
    }
    final text = utf8.decode(bytes);
    final path = options.uri.path.replaceFirst('/api/v1', '');
    final seen = SeenRequest(
      options.method.toUpperCase(),
      path,
      {
        for (final e in options.headers.entries)
          e.key.toLowerCase(): e.value.toString(),
      },
      text.isEmpty ? null : jsonDecode(text),
      Map.of(options.extra),
    );
    onSend?.call(seen);
    requests.add(seen);
    final route = routes['${seen.method} $path'];
    if (route == null) {
      return _reply(FakeReply.problem(404, 'not_found'));
    }
    try {
      return _reply(await route(seen));
    } on NetworkDown {
      throw DioException.connectionError(
        requestOptions: options,
        reason: 'fake network down',
      );
    }
  }

  ResponseBody _reply(FakeReply r) {
    final problem = r.body is Map && (r.body! as Map)['code'] is String;
    return ResponseBody.fromString(
      r.body == null ? '' : jsonEncode(r.body),
      r.status,
      headers: {
        if (r.body != null)
          Headers.contentTypeHeader: [
            problem ? 'application/problem+json' : 'application/json',
          ],
        for (final e in r.headers.entries) e.key.toLowerCase(): [e.value],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

/// A secret store that records every operation in a shared [log].
class FakeSecretStore implements SecretStore {
  /// Creates a store writing its operations into [log].
  FakeSecretStore(this.log);

  /// Stored values.
  final Map<String, String> values = {};

  /// Shared log of operations (`write <key>`, `delete <key>`, `send <path>`).
  final List<String> log;

  /// Delay of each write, to prove the caller waits for it.
  Duration writeDelay = Duration.zero;

  @override
  Future<String?> read(String key) async => values[key];

  @override
  Future<void> write(String key, String value) async {
    if (writeDelay > Duration.zero) await Future<void>.delayed(writeDelay);
    values[key] = value;
    log.add('write ${key.split('/').last}');
  }

  @override
  Future<void> delete(String key) async {
    values.remove(key);
    log.add('delete ${key.split('/').last}');
  }
}

/// A clock the test moves; [delay] advances it at once and records the wait.
class FakeClock implements Clock {
  /// Starts at [start].
  FakeClock(this._now);

  DateTime _now;

  /// Every wait asked for, in order.
  final List<Duration> waits = [];

  @override
  DateTime now() => _now;

  /// Moves the clock forward.
  void advance(Duration d) => _now = _now.add(d);

  @override
  Future<void> delay(Duration duration) async {
    waits.add(duration);
    _now = _now.add(duration);
    await Future<void>.delayed(Duration.zero);
  }
}

/// Records the opened URLs.
class FakeBrowser implements BrowserLauncher {
  /// URLs opened.
  final List<Uri> opened = [];

  /// Called after a URL was opened.
  void Function(Uri url)? onOpen;

  @override
  Future<void> open(Uri url) async {
    opened.add(url);
    onOpen?.call(url);
  }
}

/// Answers with a fixed redirect.
class FakeRedirects implements RedirectReceiver {
  /// The redirect URI the app registers.
  final Uri uri = Uri.parse('http://127.0.0.1:49152/callback');

  /// The redirect the browser will deliver.
  Uri? next;

  /// The prefix and timeout of the last wait.
  Uri? expectedPrefix;

  /// The timeout of the last wait.
  Duration? timeout;

  @override
  Future<Uri> redirectUri() async => uri;

  @override
  Future<Uri> waitForRedirect(Uri expectedPrefix, Duration timeout) async {
    this.expectedPrefix = expectedPrefix;
    this.timeout = timeout;
    final n = next;
    if (n == null) throw TimeoutException('no redirect', timeout);
    return n;
  }
}

/// A prompt that answers with [answer] and records the texts.
class FakePrompt implements StepUpPrompt {
  /// The answer of each prompt.
  bool answer = true;

  /// Texts shown.
  final List<String> asked = [];

  @override
  Future<bool> ask(String purposeText) async {
    asked.add(purposeText);
    return answer;
  }
}

/// A passkey authenticator that returns fixed credentials.
class FakePasskeys implements PasskeyAuthenticator {
  /// Options received.
  final List<Map<String, Object?>> options = [];

  @override
  Future<Map<String, Object?>> getAssertion(
    Map<String, Object?> requestOptions,
  ) async {
    options.add(requestOptions);
    return {'id': 'cred-1', 'type': 'public-key'};
  }

  @override
  Future<Map<String, Object?>> createCredential(
    Map<String, Object?> creationOptions,
  ) async {
    options.add(creationOptions);
    return {'id': 'cred-new', 'type': 'public-key'};
  }
}

/// IDs of the tests.
const String sessionId = '0192f4a0-7b1c-7000-8000-000000000001';

/// IDs of the tests.
const String userId = '0192f4a0-7b1c-7000-8000-000000000002';

/// A JWT-shaped access token whose payload carries [exp] (seconds).
String accessToken(String tag, DateTime exp) {
  String enc(Map<String, Object?> m) =>
      base64Url.encode(utf8.encode(jsonEncode(m))).replaceAll('=', '');
  return '${enc({'alg': 'EdDSA', 'typ': 'at+jwt'})}.'
      '${enc({'sub': userId, 'sid': sessionId, 'tag': tag, 'exp': exp.millisecondsSinceEpoch ~/ 1000})}.'
      'c2ln';
}

/// A `TokenResponse` body.
Map<String, Object?> tokenBody({
  required String access,
  required String refresh,
  required String deviceId,
  String state = 'active',
  String? verificationCode,
  int expiresIn = 300,
}) => {
  'access_token': access,
  'token_type': 'DPoP',
  'expires_in': expiresIn,
  'refresh_token': refresh,
  'refresh_expires_at': '2026-11-01T00:00:00.000Z',
  'session': {
    'id': sessionId,
    'user_id': userId,
    'device_id': deviceId,
    'auth_method': 'oidc',
    'created_at': '2026-10-15T10:00:00.000Z',
    'expires_at': '2026-11-01T00:00:00.000Z',
  },
  'device': {
    'id': deviceId,
    'user_id': userId,
    'name': 'Alex laptop',
    'platform': 'macos',
    'state': state,
    'verification_code': ?verificationCode,
    'created_at': '2026-10-15T10:00:00.000Z',
  },
};

/// A `Device` body.
Map<String, Object?> deviceBody(
  String deviceId,
  String state, {
  String? revokeReason,
}) => {
  'id': deviceId,
  'user_id': userId,
  'name': 'Alex laptop',
  'platform': 'macos',
  'state': state,
  'revoke_reason': ?revokeReason,
  'created_at': '2026-10-15T10:00:00.000Z',
};

/// A complete test setup: fake server, store, clock, keys and the session.
class Harness {
  /// Builds a session against a fresh fake server.
  Harness({DateTime? start})
    : clock = FakeClock(start ?? DateTime.utc(2026, 10, 15, 10)) {
    store = FakeSecretStore(log);
    server.onSend = (r) => log.add('send ${r.path}');
    var counter = 0;
    session = AuthSession(
      serverUrl: serverUrl,
      keys: DeviceKeys(dpop: dpopKey, stepUp: stepUpKey),
      secrets: store,
      clock: clock,
      ids: Uuid7Generator(
        nowMillis: () => clock.now().millisecondsSinceEpoch,
        random: (bits) => (counter++) & ((1 << bits) - 1),
      ),
      config: AppConfig.defaults(),
      device: const DeviceDescription(
        name: 'Alex laptop',
        platform: 'macos',
        appVersion: '1.0.0',
      ),
      buildClient: (interceptors) => createApiClient(
        baseUrl: serverUrl,
        traceContext: () => TraceContext(traceId: '1' * 32, sampled: false),
        readRetry: ReadRetryPolicy(
          base: Duration.zero,
          cap: Duration.zero,
          maxAttempts: 1,
        ),
        authInterceptors: interceptors,
        adapter: server,
      ),
      stepUpPrompt: prompt,
      browser: browser,
      redirects: redirects,
      passkeys: passkeys,
      random: Random(7),
    );
    session.events.listen(events.add);
  }

  /// Shared operation log of store and server.
  final List<String> log = [];

  /// The fake server.
  final FakeServer server = FakeServer();

  /// The secret store.
  late final FakeSecretStore store;

  /// The clock.
  final FakeClock clock;

  /// The DPoP key.
  final SoftwareP256Signer dpopKey = SoftwareP256Signer.seeded(1);

  /// The step-up key.
  final SoftwareP256Signer stepUpKey = SoftwareP256Signer.seeded(2);

  /// The step-up prompt.
  final FakePrompt prompt = FakePrompt();

  /// The browser.
  final FakeBrowser browser = FakeBrowser();

  /// The redirect receiver.
  final FakeRedirects redirects = FakeRedirects();

  /// The passkey port.
  final FakePasskeys passkeys = FakePasskeys();

  /// Events emitted so far.
  final List<AuthEvent> events = [];

  /// The session under test.
  late final AuthSession session;

  /// A device ID of the tests.
  final String deviceId = '0192f4a0-7b1c-7000-8000-0000000000d1';

  /// Signs in through a refresh-token restore: stores [refresh] and installs [access].
  Future<void> signIn({
    String access = 'A1',
    String refresh = 'R1',
    Duration validFor = const Duration(minutes: 5),
  }) async {
    server.routes['POST /auth/password/login'] = (_) => FakeReply(
      201,
      tokenBody(
        access: accessToken(access, clock.now().add(validFor)),
        refresh: refresh,
        deviceId: deviceId,
      ),
    );
    final outcome = await session.login.loginWithPassword(
      loginName: 'alex',
      password: 'pw-canary',
      totpCode: '123456',
    );
    if (outcome is! LoginSignedIn) throw StateError('sign-in failed: $outcome');
    server.requests.clear();
    log.clear();
    events.clear();
  }

  /// Lets pending microtasks and zero timers run.
  Future<void> settle() async {
    for (var i = 0; i < 20; i++) {
      await Future<void>.delayed(Duration.zero);
    }
  }
}
