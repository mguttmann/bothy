// SPDX-License-Identifier: Apache-2.0

/// Step-up with the step-up device key, or on a device without one with a passkey or with
/// password and TOTP (08 6.2, 6.3, 14.3; 06 3.3; brief M4-09 Design note 6; V1-14).
library;

import 'dart:async';

import 'package:api_client/api_client.dart';
import 'package:auth/src/errors.dart';
import 'package:auth/src/jose.dart';
import 'package:auth/src/ports.dart';
import 'package:auth/src/session_core.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:dio/dio.dart';

/// Explains a step-up to the person before the operating system asks for biometrics or the
/// PIN; the confirmation itself happens inside the step-up signer (M4-10), whose key requires
/// user presence. On a device without a step-up key it asks for password and one-time code
/// instead ([askPassword]).
abstract interface class StepUpPrompt {
  /// Shows [purposeText] (already in the person's language) and returns whether the person
  /// wants to continue; `false` cancels the step-up.
  Future<bool> ask(String purposeText);

  /// Shows [purposeText] with a password field and a field for the six-digit one-time code of
  /// the person's authenticator app, for a device without a step-up key (08 6.2); [rejected]
  /// says that the previous attempt was not accepted. Returns what the person typed, or null
  /// when the person cancels.
  Future<StepUpCredentials?> askPassword(
    String purposeText, {
    bool rejected = false,
  });
}

/// The `reason` of `401 step_up_required` from `stepUpWithPassword` while the password path of
/// the account is locked after too many wrong logins (08 6.2 and 13); the step-up stops asking.
const String passwordLockedReason = 'password_locked';

/// The password and the one-time code a person typed for a step-up without a step-up key.
typedef StepUpCredentials = ({String password, String totpCode});

/// Confirms the current session with the step-up key, or with password and TOTP on a device
/// without one.
///
/// One confirmation with the key: [StepUpPrompt.ask], `createStepUpChallenge` (after the
/// prompt, because the challenge lives only `auth.step_up_challenge_ttl`), the step-up key
/// signs a compact JWS whose header is only `{"alg":"ES256"}` (no `jwk`, `jku`, `x5u`, `x5c`,
/// `crit`) over `{challenge, sid, iat}`, then `completeStepUpWithDeviceKey`. The terminal's
/// fresh step-up (08 6.3) is the same flow; the server consumes it.
///
/// [confirm] runs at most one step-up at a time: a caller that asks while one runs joins it
/// and gets its result, because one confirmation counts for every request of the session for
/// `auth.step_up_max_age`. [confirmFresh] never joins.
///
/// A device without a step-up key (Windows, D-M4-11-2) whose app has a [PasskeyAuthenticator]
/// confirms with a passkey when the person has one (V1-14, design note
/// docs/design/sign-in-methods.md item 6): `createStepUpChallenge` tells by its
/// `passkey_options`, then [StepUpPrompt.ask], a new challenge, the platform's assertion over its
/// options and `completeStepUpWithPasskey`. The server counts a passkey as fresh for the terminal
/// outside the console (08 6.3 item 2), so [confirmFresh] takes the same way.
///
/// Otherwise such a device confirms with password and TOTP (08
/// 6.2): `createStepUpChallenge`, [StepUpPrompt.askPassword], then `stepUpWithPassword`. A
/// wrong password or code (`401 step_up_required`, counted by the server) asks again with a new
/// challenge until the person cancels or the server ends the session at
/// `auth.step_up_fail_limit`; a password changed during the attempt (reason `password_changed`,
/// not counted) asks again too. A locked password path (reason [passwordLockedReason], not
/// counted) stops asking and throws the answer. Such a step-up never counts as fresh for the terminal (08 6.3).
///
/// Example:
/// ```dart
/// await session.stepUp.confirm(purposeText: l10n.stepUpOpenTerminal);
/// ```
final class StepUpService {
  /// Internal: created by the session over its [SessionCore]; the third argument is the passkey
  /// port of the app, null where the platform has none.
  StepUpService(this._core, this._prompt, [this._passkeys]);

  final SessionCore _core;
  final StepUpPrompt _prompt;
  final PasskeyAuthenticator? _passkeys;

  static const Map<String, Object?> _internal = {internalRequestExtra: true};

  Future<void>? _running;

  /// Runs one step-up for the current session.
  ///
  /// Throws [StepUpCancelled] when the person declines the prompt or the signer reports a
  /// cancelled confirmation, [StepUpUnavailable] when a device without a step-up key gets
  /// `403 step_up_not_possible` (no local account) or `404` (local passwords switched off) for
  /// the password, [NotSignedIn] without a session, and the `DioException` of a rejected
  /// challenge or confirmation otherwise (`401 step_up_required` of the device key or with
  /// reason [passwordLockedReason], `401 session_revoked`, `403 forbidden`). While a step-up
  /// runs, a further call joins it (its [purposeText] is not shown) and ends as that one ends.
  Future<void> confirm({required String purposeText}) {
    final running = _running;
    if (running != null) return running;
    final next = _confirm(purposeText);
    _running = next;
    // Free the slot when this step-up ends, whichever way; a later caller starts a new one.
    unawaited(
      next.then<void>((_) {}, onError: (Object _) {}).whenComplete(() {
        if (identical(_running, next)) _running = null;
      }),
    );
    return next;
  }

  Future<void> _confirm(String purposeText) async {
    final key = _core.keys.stepUp;
    if (key != null) return _confirmWithKey(key, purposeText);
    // A passkey the platform cannot answer (one stored on another device, Windows Hello refused)
    // leaves the password: only the person's own "No" to the prompt cancels (D-V1-14-4).
    if (await _confirmWithPasskey(purposeText) == _PasskeyStepUp.done) return;
    await _confirmWithPassword(purposeText);
  }

  /// Runs the fresh step-up of the terminal (08 6.3), which only the step-up key or a passkey
  /// can give.
  ///
  /// On a device without a step-up key it takes the passkey of the person ([confirm]); it
  /// throws [StepUpUnavailable] when the app has no passkey port (before any request) or the
  /// person has no passkey, because the server never counts a step-up with password and TOTP
  /// as fresh. Otherwise as [confirm].
  Future<void> confirmFresh({required String purposeText}) async {
    final key = _core.keys.stepUp;
    if (key != null) return _confirmWithKey(key, purposeText);
    switch (await _confirmWithPasskey(purposeText)) {
      case _PasskeyStepUp.done:
        return;
      case _PasskeyStepUp.none:
        throw const StepUpUnavailable();
      case _PasskeyStepUp.failed:
        throw const StepUpCancelled();
    }
  }

  /// The step-up with a passkey of a device without a step-up key: [_PasskeyStepUp.none], after
  /// at most one `createStepUpChallenge`, when the app has no passkey port or the challenge
  /// carries no `passkey_options` (the person has no passkey, or the instance has passkeys off);
  /// [_PasskeyStepUp.failed] when the platform gave no assertion (the passkey is on another
  /// device, the prompt was refused, or the options did not fit), logged with its error type.
  ///
  /// The first challenge only tells whether a passkey can answer; the one the passkey signs is
  /// created after the prompt, because a challenge lives only `auth.step_up_challenge_ttl`. A
  /// "No" to the prompt is [StepUpCancelled]; a refused assertion is the server's answer
  /// (`401 step_up_required`, counted).
  Future<_PasskeyStepUp> _confirmWithPasskey(String purposeText) async {
    final passkeys = _passkeys;
    if (passkeys == null) return _PasskeyStepUp.none;
    if (_core.tokens.state?.session == null) throw const NotSignedIn();
    final auth = _core.api.getAuthApi();
    final probe = (await auth.createStepUpChallenge(extra: _internal)).data!;
    if (probe.passkeyOptions == null) return _PasskeyStepUp.none;
    if (!await _prompt.ask(purposeText)) throw const StepUpCancelled();
    final challenge = (await auth.createStepUpChallenge(extra: _internal))
        .data!;
    final options = challenge.passkeyOptions;
    // The passkey was revoked between the two challenges: nothing can answer any more.
    if (options == null) return _PasskeyStepUp.failed;
    final Map<String, Object?> credential;
    try {
      credential = await passkeys.getAssertion({
        for (final e in options.entries) e.key: e.value?.value,
      });
    } on AuthException {
      rethrow;
    } on Object catch (e) {
      // The platform's prompt (Windows Hello) was refused or cancelled, no passkey on this device
      // answered, or the options did not fit the platform.
      _core.log('step-up passkey not given (${e.runtimeType})');
      return _PasskeyStepUp.failed;
    }
    await auth.completeStepUpWithPasskey(
      stepUpPasskeyRequest: StepUpPasskeyRequest(
        (b) => b
          ..challengeId = challenge.challengeId
          ..credential.replace(
            BuiltMap<String, JsonObject?>.of({
              for (final e in credential.entries)
                e.key: e.value == null ? null : JsonObject(e.value),
            }),
          ),
      ),
      extra: _internal,
    );
    _core.log('step-up confirmed with a passkey');
    return _PasskeyStepUp.done;
  }

  Future<void> _confirmWithKey(DeviceKeySigner key, String purposeText) async {
    if (_core.tokens.state?.session == null) throw const NotSignedIn();
    if (!await _prompt.ask(purposeText)) throw const StepUpCancelled();
    // The challenge comes after the prompt, as in the password path: it lives
    // auth.step_up_challenge_ttl, and a prompt left open longer must not sign an expired one.
    final challenge = (await _core.api.getAuthApi().createStepUpChallenge(
      extra: _internal,
    )).data!;
    final session = _core.tokens.state?.session;
    if (session == null) throw const NotSignedIn();
    final String jws;
    try {
      jws = await signCompactJws(
        signer: key,
        header: {'alg': key.alg},
        payload: {
          'challenge': challenge.challenge,
          'sid': session.sid,
          'iat': _core.clock.now().millisecondsSinceEpoch ~/ 1000,
        },
      );
    } on AuthException {
      // The signer may say more itself.
      rethrow;
    } on Object {
      // The operating system's confirmation was refused or cancelled (the port promises this
      // mapping, ports.dart): the person cancelled, the request had no network error.
      throw const StepUpCancelled();
    }
    await _core.api.getAuthApi().completeStepUpWithDeviceKey(
      stepUpDeviceKeyRequest: StepUpDeviceKeyRequest(
        (b) => b
          ..challengeId = challenge.challengeId
          ..signature = jws,
      ),
      extra: _internal,
    );
    _core.log('step-up confirmed');
  }

  /// The step-up with password and TOTP of a device without a step-up key; each attempt has
  /// its own challenge, created after the prompt, because the server ends a challenge with
  /// every answer.
  Future<void> _confirmWithPassword(String purposeText) async {
    final auth = _core.api.getAuthApi();
    for (var rejected = false; ; rejected = true) {
      if (_core.tokens.state?.session == null) throw const NotSignedIn();
      final typed = await _prompt.askPassword(purposeText, rejected: rejected);
      if (typed == null) throw const StepUpCancelled();
      // The challenge comes after the prompt: it lives auth.step_up_challenge_ttl, and a
      // prompt left open longer must not send an expired one with correct credentials.
      final challenge = (await auth.createStepUpChallenge(extra: _internal))
          .data!;
      try {
        await auth.stepUpWithPassword(
          stepUpPasswordRequest: StepUpPasswordRequest(
            (b) => b
              ..challengeId = challenge.challengeId
              ..password = typed.password
              ..totpCode = typed.totpCode.trim(),
          ),
          extra: _internal,
        );
        _core.log('step-up confirmed with password');
        return;
      } on DioException catch (e) {
        final problem = problemOf(e);
        // 403 step_up_not_possible: no local account; 404: the instance has local passwords
        // switched off (auth.local_password_enabled), so the operation does not exist.
        if ((problem?.status == 403 &&
                problem?.reason == 'step_up_not_possible') ||
            problem?.status == 404) {
          throw const StepUpUnavailable();
        }
        // A locked password path (reason password_locked) is no wrong confirmation: the server
        // counts nothing, and asking again would only repeat the refusal, so the reason goes to
        // the caller, whose screen says to try later or use another device (08 13, R-ID-10).
        if (problem?.status != 401 ||
            problem?.code != 'step_up_required' ||
            problem?.reason == passwordLockedReason) {
          rethrow;
        }
        _core.log('step-up with password rejected');
      }
    }
  }
}

/// How the passkey step-up of a device without a step-up key ended.
enum _PasskeyStepUp {
  /// Confirmed.
  done,

  /// No passkey port, or the person has no passkey.
  none,

  /// The platform gave no assertion.
  failed,
}
