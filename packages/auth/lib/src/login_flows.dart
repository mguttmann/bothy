// SPDX-License-Identifier: Apache-2.0

/// The clients of the login operations the onboarding screens call (08 2, 3.2, 5, 14; 06 3.2;
/// brief M4-09 Design note 5).
library;

import 'dart:async';
import 'dart:math';

import 'package:api_client/api_client.dart';
import 'package:auth/src/auth_events.dart';
import 'package:auth/src/errors.dart';
import 'package:auth/src/pkce.dart';
import 'package:auth/src/ports.dart';
import 'package:auth/src/redact.dart';
import 'package:auth/src/refresh.dart';
import 'package:auth/src/session_core.dart';
import 'package:auth/src/tokens.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';

/// The result of one login flow.
sealed class LoginOutcome {
  const LoginOutcome();
}

/// The login succeeded and the device is active.
final class LoginSignedIn extends LoginOutcome {
  /// Creates the outcome.
  const LoginSignedIn(this.tokens);

  /// The new tokens; the refresh token is already stored.
  final TokenState tokens;

  @override
  String toString() => 'LoginSignedIn($tokens)';
}

/// The login succeeded but the device waits for approval (08 5.2); poll with
/// `LoginService.pollPendingDevice`.
final class LoginPending extends LoginOutcome {
  /// Creates the outcome.
  const LoginPending(this.device);

  /// The waiting device and its verification code.
  final PendingDevice device;

  @override
  String toString() => 'LoginPending($device)';
}

/// The password step needs the TOTP code before the request can be sent.
final class LoginNeedsSecondFactor extends LoginOutcome {
  /// Creates the outcome.
  const LoginNeedsSecondFactor();

  @override
  String toString() => 'LoginNeedsSecondFactor';
}

/// The login failed.
///
/// [reason] is the problem code of the server, the `error` of an OIDC redirect
/// (`access_denied`, `login_failed`, `flow_expired`), `network` without an answer or the
/// revoke reason of a rejected pending device.
final class LoginFailed extends LoginOutcome {
  /// Creates the outcome.
  const LoginFailed({required this.reason, this.problem});

  /// Machine-readable cause.
  final String reason;

  /// The server's problem, when there was an answer.
  final ApiProblem? problem;

  @override
  String toString() =>
      'LoginFailed($reason${problem == null ? '' : ', $problem'})';
}

/// The code and expiry the new device shows while it waits for pairing (08 5.3).
final class PairingTicket {
  /// Creates the ticket.
  const PairingTicket({
    required this.pairingId,
    required this.code,
    required this.expiresAt,
  });

  /// The pairing to poll with `LoginService.pollPairing`.
  final String pairingId;

  /// The pairing code (`auth.pairing_code_chars` Crockford Base32 characters); the UI groups
  /// it (M4-18).
  final String code;

  /// When the pairing expires (`auth.pairing_code_ttl` of the server).
  final DateTime expiresAt;

  @override
  String toString() =>
      'PairingTicket($pairingId, code: ${redacted('pairing code', code)}, '
      'expires: ${expiresAt.toIso8601String()})';
}

/// How this device describes itself in every registration (08 5.1).
final class DeviceDescription {
  /// Creates the description.
  const DeviceDescription({
    required this.name,
    required this.platform,
    this.appVersion,
  });

  /// Display name, for example `Alex laptop`.
  final String name;

  /// The `DevicePlatform` code: `ios`, `android`, `macos` or `windows`.
  final String platform;

  /// The app version for display.
  final String? appVersion;
}

/// Normalizes a typed code as 08 3.2 says (case, spaces and hyphens do not count): spaces and
/// hyphens are removed, a leading `en_` is written in lower case and everything else in upper
/// case; nothing else is dropped.
///
/// Example: `en_ 7k2m 9qxa`, `EN_7K2M-9QXA` and `en_7K2M9QXA` all give `en_7K2M9QXA`.
String normalizeCode(String typed) {
  final compact = typed.replaceAll(RegExp(r'[\s-]'), '');
  if (compact.length >= 3 && compact.substring(0, 3).toLowerCase() == 'en_') {
    return 'en_${compact.substring(3).toUpperCase()}';
  }
  return compact.toUpperCase();
}

/// One method per login flow; each returns a [LoginOutcome] and never throws for an answer
/// of the server (brief M4-09 Design note 5).
///
/// Every completion sends the device fields of [registerDeviceFields] and a DPoP proof of the
/// DPoP key, which the new tokens are bound to (06 3.1 item 3). A successful answer is adopted
/// like a refresh: the refresh token is stored before the access token is published. Polling
/// waits with the backoff of `client.outbox_retry_base` doubling up to
/// `client.outbox_retry_cap` (fixed requirement 9).
///
/// Example:
/// ```dart
/// final outcome = await session.login.beginOidc(providerId);
/// switch (outcome) {
///   case LoginSignedIn(): showHome();
///   case LoginPending(:final device): showCode(device.verificationCode);
///   case LoginNeedsSecondFactor(): askForTotp();
///   case LoginFailed(:final reason): showError(reason);
/// }
/// ```
final class LoginService {
  /// Internal: created by the session over its [SessionCore].
  LoginService(
    this._core,
    this._device,
    this._random,
    this._browser,
    this._redirects,
    this._passkeys,
  );

  final SessionCore _core;
  final DeviceDescription _device;
  final Random _random;
  final BrowserLauncher? _browser;
  final RedirectReceiver? _redirects;
  final PasskeyAuthenticator? _passkeys;

  AuthApi get _auth => _core.api.getAuthApi();

  /// The registration fields of this device (08 5.1): `device_id` (a UUIDv7 created once and
  /// kept in the `SecretStore` for this server), name, platform and app version, the public
  /// JWK and `alg` of the DPoP key and the step-up key.
  ///
  /// The API names the keys `public_jwk` and `step_up_key.public_jwk` (the brief's `dpop_jwk`
  /// and `step_up_jwk`).
  Future<DeviceRegistration> registerDeviceFields() async {
    var id = await _core.secrets.read(_core.deviceIdKey);
    if (id == null) {
      id = _core.ids.next();
      await _core.secrets.write(_core.deviceIdKey, id);
    }
    final deviceId = id;
    final dpop = _core.keys.dpop;
    final stepUp = _core.keys.stepUp;
    return DeviceRegistration(
      (b) => b
        ..deviceId = deviceId
        ..name = _device.name
        ..platform = DevicePlatform.valueOf(_device.platform)
        ..appVersion = _device.appVersion
        ..publicJwk.replace(_json(dpop.publicJwk))
        ..alg = DeviceRegistrationAlgEnum.valueOf(dpop.alg)
        ..stepUpKey.replace(
          DeviceKeyRegistration(
            (k) => k
              ..publicJwk.replace(_json(stepUp.publicJwk))
              ..alg = DeviceKeyRegistrationAlgEnum.valueOf(stepUp.alg),
          ),
        ),
    );
  }

  /// OIDC in the system browser with PKCE on the app leg (08 2.2, 14.1).
  ///
  /// Creates `client_verifier` and `client_challenge` (`S256`), calls `beginOidcLogin` with the
  /// receiver's redirect URI, opens `authorization_url` through the browser port, waits for the
  /// redirect until the attempt's `expires_at`, and completes with `client_verifier`, the proof
  /// and the device fields. A redirect with `error`, of another attempt or none in time fails.
  /// [enrollmentCode] links an unknown identity (08 3.1).
  ///
  /// Throws a [StateError] when the session was built without browser or redirect port.
  Future<LoginOutcome> beginOidc(
    String providerId, {
    String? enrollmentCode,
  }) async {
    final browser = _browser;
    final redirects = _redirects;
    if (browser == null || redirects == null) {
      throw StateError('OIDC needs a BrowserLauncher and a RedirectReceiver');
    }
    final pkce = PkcePair.generate(_random);
    final redirectUri = await redirects.redirectUri();
    final OidcBeginResponse begun;
    try {
      begun = (await _auth.beginOidcLogin(
        oidcBeginRequest: OidcBeginRequest(
          (b) => b
            ..providerId = providerId
            ..appRedirectUri = redirectUri.toString()
            ..clientChallenge = pkce.challenge
            ..enrollmentCode = enrollmentCode == null
                ? null
                : normalizeCode(enrollmentCode),
        ),
      )).data!;
    } on DioException catch (e) {
      return _failed(e);
    }
    await browser.open(Uri.parse(begun.authorizationUrl));
    final Uri redirect;
    try {
      final left = begun.expiresAt.difference(_core.clock.now());
      redirect = await redirects.waitForRedirect(
        redirectUri,
        left.isNegative ? Duration.zero : left,
      );
    } on TimeoutException {
      return const LoginFailed(reason: 'flow_expired');
    }
    final query = redirect.queryParameters;
    if (query['login_attempt_id'] != begun.loginAttemptId) {
      return const LoginFailed(reason: 'login_failed');
    }
    final error = query['error'];
    final code = query['login_code'];
    if (error != null || code == null) {
      return LoginFailed(reason: error ?? 'login_failed');
    }
    final device = await registerDeviceFields();
    return _complete(
      () => _auth.completeOidcLogin(
        dPoP: proofFromInterceptor,
        oidcCompleteRequest: OidcCompleteRequest(
          (b) => b
            ..loginCode = code
            ..clientVerifier = pkce.verifier
            ..device.replace(device),
        ),
      ),
    );
  }

  /// Password with TOTP for local accounts (08 2.5, `loginWithPassword`).
  ///
  /// The API takes the TOTP code in the same request, so without [totpCode] nothing is sent
  /// and the outcome is [LoginNeedsSecondFactor]; the screen asks for the code and calls
  /// again. Spaces in the code are removed.
  Future<LoginOutcome> loginWithPassword({
    required String loginName,
    required String password,
    String? totpCode,
  }) async {
    final totp = totpCode?.replaceAll(RegExp(r'\s'), '');
    if (totp == null || totp.isEmpty) return const LoginNeedsSecondFactor();
    final device = await registerDeviceFields();
    return _complete(
      () => _auth.loginWithPassword(
        dPoP: proofFromInterceptor,
        passwordLoginRequest: PasswordLoginRequest(
          (b) => b
            ..loginName = loginName
            ..password = password
            ..totpCode = totp
            ..device.replace(device),
        ),
      ),
    );
  }

  /// Login behind a trusted proxy (08 2.4, `loginWithTrustedHeader`): no browser, the proxy
  /// supplies the identity; [enrollmentCode] links an unknown login.
  Future<LoginOutcome> loginWithTrustedHeader({String? enrollmentCode}) async {
    final device = await registerDeviceFields();
    return _complete(
      () => _auth.loginWithTrustedHeader(
        dPoP: proofFromInterceptor,
        trustedHeaderLoginRequest: TrustedHeaderLoginRequest(
          (b) => b
            ..device.replace(device)
            ..enrollmentCode = enrollmentCode == null
                ? null
                : normalizeCode(enrollmentCode),
        ),
      ),
    );
  }

  /// Passkey login (08 2.5, `beginPasskeyLogin` and `completePasskeyLogin`) through the
  /// [PasskeyAuthenticator] port; only where the platform offers passkeys (M4-18 decides).
  ///
  /// Throws a [StateError] when the session was built without a passkey port.
  Future<LoginOutcome> loginWithPasskey({String? loginName}) async {
    final passkeys = _passkeys;
    if (passkeys == null) throw StateError('no PasskeyAuthenticator');
    final WebAuthnChallenge challenge;
    try {
      challenge = (await _auth.beginPasskeyLogin(
        passkeyLoginBeginRequest: PasskeyLoginBeginRequest(
          (b) => b..loginName = loginName,
        ),
      )).data!;
    } on DioException catch (e) {
      return _failed(e);
    }
    final credential = await passkeys.getAssertion(_plain(challenge.publicKey));
    final device = await registerDeviceFields();
    return _complete(
      () => _auth.completePasskeyLogin(
        dPoP: proofFromInterceptor,
        passkeyLoginCompleteRequest: PasskeyLoginCompleteRequest(
          (b) => b
            ..challengeId = challenge.challengeId
            ..credential.replace(_json(credential))
            ..device.replace(device),
        ),
      ),
    );
  }

  /// Redeems an enrollment code with the device keys only (08 3.2, `redeemEnrollmentCode`);
  /// the device becomes `active`. The code is normalized first ([normalizeCode]).
  Future<LoginOutcome> redeemEnrollmentCode(String code) async {
    final device = await registerDeviceFields();
    return _complete(
      () => _auth.redeemEnrollmentCode(
        dPoP: proofFromInterceptor,
        enrollRedeemRequest: EnrollRedeemRequest(
          (b) => b
            ..enrollmentCode = normalizeCode(code)
            ..device.replace(device),
        ),
      ),
    );
  }

  /// Redeems an enrollment code together with a first passkey (`beginEnrollment`,
  /// `completeEnrollment`); [passkeyName] labels the new passkey.
  ///
  /// Throws a [StateError] when the session was built without a passkey port.
  Future<LoginOutcome> enrollWithPasskey(
    String code, {
    required String passkeyName,
  }) async {
    final passkeys = _passkeys;
    if (passkeys == null) throw StateError('no PasskeyAuthenticator');
    final WebAuthnChallenge challenge;
    try {
      challenge = (await _auth.beginEnrollment(
        enrollBeginRequest: EnrollBeginRequest(
          (b) => b..enrollmentCode = normalizeCode(code),
        ),
      )).data!;
    } on DioException catch (e) {
      return _failed(e);
    }
    final credential = await passkeys.createCredential(
      _plain(challenge.publicKey),
    );
    final device = await registerDeviceFields();
    return _complete(
      () => _auth.completeEnrollment(
        dPoP: proofFromInterceptor,
        enrollCompleteRequest: EnrollCompleteRequest(
          (b) => b
            ..challengeId = challenge.challengeId
            ..credential.replace(_json(credential))
            ..passkeyName = passkeyName
            ..device.replace(device),
        ),
      ),
    );
  }

  /// Starts a device pairing (08 5.3, `beginPairing`) and returns the code this device shows.
  ///
  /// Throws the `DioException` of a rejected request; the screen shows its problem.
  Future<PairingTicket> beginPairing() async {
    final device = await registerDeviceFields();
    final started = (await _auth.beginPairing(
      dPoP: proofFromInterceptor,
      pairingBeginRequest: PairingBeginRequest(
        (b) => b..device.replace(device),
      ),
    )).data!;
    return PairingTicket(
      pairingId: started.pairingId,
      code: started.pairingCode,
      expiresAt: started.expiresAt,
    );
  }

  /// Polls `completePairing` with backoff until the pairing is approved (signed in), expired
  /// or rejected (`410`, failed). A network error keeps polling; [isCancelled] stops the loop
  /// with `LoginFailed(reason: 'cancelled')`.
  Future<LoginOutcome> pollPairing(
    String pairingId, {
    bool Function()? isCancelled,
  }) async {
    for (var attempt = 0; ; attempt++) {
      if (isCancelled?.call() ?? false) {
        return const LoginFailed(reason: 'cancelled');
      }
      final outcome = await _completeOrWait(
        () => _auth.completePairing(
          dPoP: proofFromInterceptor,
          pairingCompleteRequest: PairingCompleteRequest(
            (b) => b..pairingId = pairingId,
          ),
        ),
        // `202` carries a PairingStatus, which the generated method cannot read as a
        // TokenResponse; that failure means "not yet approved". A network error, `429` and a
        // `5xx` are transient and keep polling, as in `pollPendingDevice`.
        stillWaiting: _transientOrPending,
      );
      if (outcome != null) return outcome;
      await _core.clock.delay(_backoff(attempt));
    }
  }

  /// Polls `getDevice` for the pending [deviceId] with backoff until it is `active` (emits
  /// `SignedIn`) or `revoked` (emits `SignedOut(deviceRevoked)` and returns `LoginFailed` with
  /// the revoke reason), 08 5.2. A network error keeps polling; `401 session_revoked` ends the
  /// session; [isCancelled] stops the loop with `LoginFailed(reason: 'cancelled')`.
  Future<LoginOutcome> pollPendingDevice(
    DeviceId deviceId, {
    bool Function()? isCancelled,
  }) async {
    for (var attempt = 0; ; attempt++) {
      if (isCancelled?.call() ?? false) {
        return const LoginFailed(reason: 'cancelled');
      }
      try {
        final device = (await _core.api.getDevicesApi().getDevice(
          deviceId: deviceId.value,
        )).data!;
        if (device.state == DeviceStateEnum.active) {
          final state = _core.tokens.state;
          if (state == null) {
            return const LoginFailed(reason: 'unauthenticated');
          }
          _core.emit(SignedIn(state.session));
          return LoginSignedIn(state);
        }
        if (device.state == DeviceStateEnum.revoked) {
          await _core.signOut(SignOutReason.deviceRevoked);
          return LoginFailed(
            reason: _snake(device.revokeReason?.name ?? 'revoked'),
          );
        }
      } on DioException catch (e) {
        final own = e.error;
        if (own is AuthException) return LoginFailed(reason: _reasonOf(own));
        final problem = problemOf(e);
        if (problem != null && problem.status != 429 && problem.status < 500) {
          return LoginFailed(reason: problem.code, problem: problem);
        }
      }
      await _core.clock.delay(_backoff(attempt));
    }
  }

  /// Logs out (`logout`), then clears the refresh token and emits `SignedOut(loggedOut)`, also
  /// when the call fails offline or the session already ended.
  Future<void> logout() async {
    try {
      if (_core.tokens.state != null ||
          await _core.secrets.read(_core.refreshTokenKey) != null) {
        await _core.api.getAuthApi().logout();
      }
    } on DioException catch (e) {
      _core.log('logout call failed: ${problemOf(e)?.code ?? 'network error'}');
    }
    await _core.signOut(SignOutReason.loggedOut);
  }

  static bool _transientOrPending(DioException e) {
    if (e.error is AuthException) return false;
    final status = e.response?.statusCode;
    return status == null || status == 202 || status == 429 || status >= 500;
  }

  Duration _backoff(int attempt) {
    final base = _core.config.outboxRetryBase;
    final cap = _core.config.outboxRetryCap;
    final wait = base * pow(2, min(attempt, 30)).toInt();
    return wait > cap ? cap : wait;
  }

  /// Runs a completion; `null` only when [stillWaiting] matched (a pairing not approved yet).
  Future<LoginOutcome?> _completeOrWait(
    Future<Response<TokenResponse>> Function() call, {
    bool Function(DioException e)? stillWaiting,
  }) async {
    final generation = _core.generation;
    final TokenResponse response;
    try {
      response = (await call()).data!;
    } on DioException catch (e) {
      if (stillWaiting?.call(e) ?? false) return null;
      return _failed(e);
    }
    final state = await _core.adopt(response, generationAtStart: generation);
    if (state == null) return const LoginFailed(reason: 'cancelled');
    if (response.device.state == DeviceStateEnum.pending) {
      final pending = PendingDevice(
        deviceId: state.session.deviceId,
        verificationCode: response.device.verificationCode,
      );
      _core.emit(DevicePending(pending));
      return LoginPending(pending);
    }
    _core.emit(SignedIn(state.session));
    return LoginSignedIn(state);
  }

  Future<LoginOutcome> _complete(
    Future<Response<TokenResponse>> Function() call,
  ) async => (await _completeOrWait(call))!;

  LoginFailed _failed(DioException e) {
    final own = e.error;
    if (own is AuthException) return LoginFailed(reason: _reasonOf(own));
    final problem = problemOf(e);
    final outcome = problem == null
        ? const LoginFailed(reason: 'network')
        : LoginFailed(reason: problem.code, problem: problem);
    _core.log(
      'login failed: ${outcome.reason} (${e.type.name}, ${e.error.runtimeType})',
    );
    return outcome;
  }

  static String _reasonOf(AuthException e) => switch (e) {
    NotSignedIn() => 'unauthenticated',
    SessionEnded(:final reason) => reason.name,
    RefreshUnavailable() => 'network',
    StepUpCancelled() => 'cancelled',
  };

  /// The wire code of a generated enum value: its Dart name is the camelCase of the code.
  static String _snake(String name) =>
      name.replaceAllMapped(RegExp('[A-Z]'), (m) => '_${m[0]!.toLowerCase()}');

  static BuiltMap<String, JsonObject?> _json(Map<String, Object?> map) =>
      BuiltMap.of({
        for (final e in map.entries)
          e.key: e.value == null ? null : JsonObject(e.value),
      });

  static Map<String, Object?> _plain(BuiltMap<String, JsonObject?> map) => {
    for (final e in map.entries) e.key: e.value?.value,
  };
}
