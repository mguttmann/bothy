// SPDX-License-Identifier: Apache-2.0

/// The shared state of one `AuthSession`: token holder, secrets, events and the rule how a
/// token response is adopted and how a session ends. Not exported; the public classes of the
/// session use it.
library;

import 'dart:async';

import 'package:api_client/api_client.dart';
import 'package:auth/src/auth_events.dart';
import 'package:auth/src/nonce_store.dart';
import 'package:auth/src/ports.dart';
import 'package:auth/src/tokens.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';

/// Internal: the state every part of one session shares.
final class SessionCore {
  /// Creates the core for the server at [serverUrl].
  SessionCore({
    required this.serverUrl,
    required this.keys,
    required this.secrets,
    required this.clock,
    required this.ids,
    required this.config,
  }) : _prefix = 'auth/${serverUrl.origin}/';

  /// The server address (origin) this session belongs to.
  final Uri serverUrl;

  /// The device keys.
  final DeviceKeys keys;

  /// The persistent secret store.
  final SecretStore secrets;

  /// Time.
  final Clock clock;

  /// UUIDv7 source for `jti` and `device_id`.
  final Uuid7Generator ids;

  /// The app configuration (polling backoff keys).
  final AppConfig config;

  /// The current tokens.
  final TokenHolder tokens = TokenHolder();

  /// The newest DPoP nonce of this server.
  final NonceStore nonces = NonceStore();

  /// Receives one line per notable step; never a secret (Design note 7).
  void Function(String line)? logHook;

  final StreamController<AuthEvent> _events = StreamController.broadcast();

  final String _prefix;

  /// Counts sign-outs, so a refresh that completes after a sign-out is not adopted.
  int generation = 0;

  /// The API client of this session; set once by the session after construction.
  late final ApiClient api;

  /// The event stream.
  Stream<AuthEvent> get events => _events.stream;

  /// Store key of the refresh token of this server.
  String get refreshTokenKey => '${_prefix}refresh_token';

  /// Store key of the device ID of this server.
  String get deviceIdKey => '${_prefix}device_id';

  /// Emits [event] and logs it.
  void emit(AuthEvent event) {
    log('event $event');
    _events.add(event);
  }

  /// Writes a log line through [logHook].
  void log(String line) => logHook?.call('auth: $line');

  /// Adopts a `TokenResponse`: writes the refresh token and waits for the write, then
  /// publishes the access token (docs/DECISIONS.md D-M4-09-3, 08 7.4).
  ///
  /// Returns the new state, or `null` when the session was signed out while [generationAtStart]
  /// was current (a late answer is then dropped, never adopted).
  Future<TokenState?> adopt(
    TokenResponse response, {
    required int generationAtStart,
  }) async {
    final session = SessionInfo(
      sid: response.session.id,
      deviceId: DeviceId.parse(response.device.id),
      accessExpiresAt:
          accessTokenExpiry(response.accessToken) ??
          clock.now().add(Duration(seconds: response.expiresIn)),
    );
    final state = TokenState(
      accessToken: response.accessToken,
      refreshToken: response.refreshToken,
      session: session,
    );
    if (generation != generationAtStart) return null;
    await secrets.write(refreshTokenKey, response.refreshToken);
    if (generation != generationAtStart) {
      await secrets.delete(refreshTokenKey);
      return null;
    }
    tokens.state = state;
    log('tokens adopted: $state');
    return state;
  }

  /// Ends the session for [reason]: clears the access token, deletes the refresh token (and
  /// the device ID when the device or session was revoked, R-SY-14) and emits `SignedOut`
  /// once.
  Future<void> signOut(SignOutReason reason) async {
    final hadSession =
        tokens.state != null || await secrets.read(refreshTokenKey) != null;
    generation++;
    tokens.state = null;
    await secrets.delete(refreshTokenKey);
    if (reason == SignOutReason.sessionRevoked ||
        reason == SignOutReason.deviceRevoked) {
      await secrets.delete(deviceIdKey);
    }
    if (hadSession) emit(SignedOut(reason));
  }

  /// Signs out for [reason] only while [generationAtStart] is still current, so a late answer
  /// of an earlier session never ends the session that replaced it; returns whether it did.
  /// A `null` [generationAtStart] counts as current.
  Future<bool> signOutIfCurrent(
    int? generationAtStart,
    SignOutReason reason,
  ) async {
    if (generationAtStart != null && generationAtStart != generation) {
      log('late answer of an earlier session ignored');
      return false;
    }
    await signOut(reason);
    return true;
  }

  /// Closes the event stream.
  Future<void> close() => _events.close();
}

/// Key of `RequestOptions.extra` that marks the requests of `auth` itself, which the
/// interceptors never repeat for refresh or step-up.
const String internalRequestExtra = 'auth.internal';

/// Whether [options] belongs to an operation that needs a person's token: the generated client
/// lists the security schemes in `extra['secure']`, empty for `security: []` (06 3).
bool needsToken(RequestOptions options) {
  final secure = options.extra['secure'];
  return secure is List &&
      secure.any((s) => s is Map && s['name'] == 'dpopAuth');
}

/// Whether [options] carries the header parameter `DPoP` of a login operation.
bool carriesProofHeader(RequestOptions options) =>
    options.headers.keys.any((k) => k.toLowerCase() == 'dpop');

/// The problem of a response error, read from the response when the problem interceptor has
/// not mapped it yet (it runs after the auth interceptors, M4-02).
ApiProblem? problemOf(DioException error) {
  final mapped = ApiProblem.of(error);
  if (mapped != null) return mapped;
  final response = error.response;
  if (response == null || error.type != DioExceptionType.badResponse) {
    return null;
  }
  return ApiProblem.fromResponse(response);
}

/// A [DioException] for [options] that carries [error] (an `AuthException`), with the error's
/// own text as the message so no request detail reaches logs.
DioException authError(RequestOptions options, Object error) => DioException(
  requestOptions: options,
  error: error,
  message: error.toString(),
);
