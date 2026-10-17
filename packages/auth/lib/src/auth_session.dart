// SPDX-License-Identifier: Apache-2.0

/// One login session per server address (brief M4-09 Design note 1).
library;

import 'dart:math';

import 'package:api_client/api_client.dart';
import 'package:auth/src/auth_events.dart';
import 'package:auth/src/dpop.dart';
import 'package:auth/src/interceptors.dart';
import 'package:auth/src/login_flows.dart';
import 'package:auth/src/nonce_store.dart';
import 'package:auth/src/ports.dart';
import 'package:auth/src/refresh.dart';
import 'package:auth/src/session_core.dart';
import 'package:auth/src/step_up.dart';
import 'package:auth/src/tokens.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';

/// Owns the token state, the nonce store, the refresh coordinator, the step-up and the login
/// flows of one server, and the API client they run on.
///
/// The package has no global state: an app with two servers has two sessions. The session
/// builds its client itself through [buildClient], passing the auth interceptors for the slot
/// of `createApiClient` (M4-02), in the order [RefreshInterceptor], [DpopInterceptor],
/// [StepUpInterceptor].
///
/// After a restart the session knows only the stored refresh token; the first request that
/// needs a token refreshes and so restores the session.
///
/// Example:
/// ```dart
/// final session = AuthSession(
///   serverUrl: Uri.parse('https://chat.example.com'),
///   keys: DeviceKeys(dpop: dpopKey, stepUp: stepUpKey),
///   secrets: secureStore,
///   clock: const SystemClock(),
///   ids: uuid7,
///   config: appConfig,
///   device: const DeviceDescription(name: 'Alex laptop', platform: 'macos'),
///   stepUpPrompt: prompt,
///   buildClient: (interceptors) => createApiClient(
///     baseUrl: Uri.parse('https://chat.example.com'),
///     traceContext: tracer.next,
///     readRetry: retry,
///     authInterceptors: interceptors,
///   ),
///   browser: browser,
///   redirects: loopback,
/// );
/// session.events.listen(onAuthEvent);
/// final outcome = await session.login.beginOidc(providerId);
/// ```
final class AuthSession {
  /// Creates the session for [serverUrl].
  ///
  /// [random] draws the PKCE verifier; it defaults to `Random.secure()`. [browser],
  /// [redirects] and [passkeys] are needed only by the flows that use them.
  AuthSession({
    required Uri serverUrl,
    required DeviceKeys keys,
    required SecretStore secrets,
    required Clock clock,
    required Uuid7Generator ids,
    required AppConfig config,
    required DeviceDescription device,
    required StepUpPrompt stepUpPrompt,
    required ApiClient Function(List<Interceptor> authInterceptors) buildClient,
    BrowserLauncher? browser,
    RedirectReceiver? redirects,
    PasskeyAuthenticator? passkeys,
    Random? random,
  }) : _core = SessionCore(
         serverUrl: serverUrl,
         keys: keys,
         secrets: secrets,
         clock: clock,
         ids: ids,
         config: config,
       ) {
    proofs = DpopProofFactory(keys.dpop, _core.nonces, clock, ids);
    refresh = RefreshCoordinator(_core);
    stepUp = StepUpService(_core, stepUpPrompt);
    login = LoginService(
      _core,
      device,
      random ?? Random.secure(),
      browser,
      redirects,
      passkeys,
    );
    interceptors = List.unmodifiable(<Interceptor>[
      RefreshInterceptor(_core, refresh),
      DpopInterceptor(_core, proofs),
      StepUpInterceptor(_core, stepUp),
    ]);
    _core.api = buildClient(interceptors);
  }

  final SessionCore _core;

  /// The DPoP proofs of this device key; M4-13 calls `forGateway` for `identify`, `resume`
  /// and `reauth`.
  late final DpopProofFactory proofs;

  /// The single-flight refresh; M4-13 calls `refreshNow` on `reauth_required`.
  late final RefreshCoordinator refresh;

  /// The step-up service.
  late final StepUpService stepUp;

  /// The login flows.
  late final LoginService login;

  /// The interceptors in their order, as handed to [buildClient].
  late final List<Interceptor> interceptors;

  /// The API client of this server, with the auth interceptors installed.
  ApiClient get api => _core.api;

  /// The newest DPoP nonce; M4-13 feeds `hello.dpop_nonce` in with `update`.
  NonceStore get nonces => _core.nonces;

  /// The events: signed in, device pending, signed out, refresh failed (broadcast).
  Stream<AuthEvent> get events => _core.events;

  /// The current session, or `null` while signed out or before the first refresh after a
  /// restart.
  SessionInfo? get session => _core.tokens.state?.session;

  /// Receives one line per notable step (refresh, retries, sign-out); lines never contain a
  /// token, proof, code, verifier, password or TOTP value (15 16).
  set logHook(void Function(String line)? hook) => _core.logHook = hook;

  /// Ends the session as revoked, for the gateway's close code `4012` (04 6.4, R-SY-14):
  /// clears the store and emits `SignedOut(sessionRevoked)`.
  Future<void> sessionRevoked() => _core.signOut(SignOutReason.sessionRevoked);

  /// Closes the event stream; the session is unusable afterwards.
  Future<void> dispose() => _core.close();
}
