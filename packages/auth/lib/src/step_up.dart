// SPDX-License-Identifier: Apache-2.0

/// Step-up with the step-up device key, or with password and TOTP on a device without one (08
/// 6.2, 6.3, 14.3; 06 3.3; brief M4-09 Design note 6).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/src/errors.dart';
import 'package:auth/src/jose.dart';
import 'package:auth/src/ports.dart';
import 'package:auth/src/session_core.dart';
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

/// The password and the one-time code a person typed for a step-up without a step-up key.
typedef StepUpCredentials = ({String password, String totpCode});

/// Confirms the current session with the step-up key, or with password and TOTP on a device
/// without one.
///
/// One confirmation with the key: `createStepUpChallenge`, [StepUpPrompt.ask], the step-up key
/// signs a compact JWS whose header is only `{"alg":"ES256"}` (no `jwk`, `jku`, `x5u`, `x5c`,
/// `crit`) over `{challenge, sid, iat}`, then `completeStepUpWithDeviceKey`. The terminal's
/// fresh step-up (08 6.3) is the same flow; the server consumes it.
///
/// A device without a step-up key (Windows, D-M4-11-2) confirms with password and TOTP (08
/// 6.2): `createStepUpChallenge`, [StepUpPrompt.askPassword], then `stepUpWithPassword`. A
/// wrong password or code (`401 step_up_required`, counted by the server) asks again with a new
/// challenge until the person cancels or the server ends the session at
/// `auth.step_up_fail_limit`. Such a step-up never counts as fresh for the terminal (08 6.3).
///
/// Example:
/// ```dart
/// await session.stepUp.confirm(purposeText: l10n.stepUpOpenTerminal);
/// ```
final class StepUpService {
  /// Internal: created by the session over its [SessionCore].
  StepUpService(this._core, this._prompt);

  final SessionCore _core;
  final StepUpPrompt _prompt;

  static const Map<String, Object?> _internal = {internalRequestExtra: true};

  /// Runs one step-up for the current session.
  ///
  /// Throws [StepUpCancelled] when the person declines the prompt or the signer reports a
  /// cancelled confirmation, [StepUpUnavailable] when a device without a step-up key gets
  /// `403 step_up_not_possible` (no local account) or `404` (local passwords switched off) for
  /// the password, [NotSignedIn] without a session, and the `DioException` of a rejected
  /// challenge or confirmation otherwise (`401 step_up_required` of the device key,
  /// `401 session_revoked`, `403 forbidden`).
  Future<void> confirm({required String purposeText}) async {
    final key = _core.keys.stepUp;
    if (key == null) return _confirmWithPassword(purposeText);
    await _confirmWithKey(key, purposeText);
  }

  /// Runs the fresh step-up of the terminal (08 6.3), which only the step-up key can give.
  ///
  /// Throws [StepUpUnavailable] before any request on a device without a step-up key, because
  /// the server never counts a step-up with password and TOTP as fresh; otherwise as
  /// [confirm].
  Future<void> confirmFresh({required String purposeText}) async {
    final key = _core.keys.stepUp;
    if (key == null) throw const StepUpUnavailable();
    await _confirmWithKey(key, purposeText);
  }

  Future<void> _confirmWithKey(DeviceKeySigner key, String purposeText) async {
    final challenge = (await _core.api.getAuthApi().createStepUpChallenge(
      extra: _internal,
    )).data!;
    final session = _core.tokens.state?.session;
    if (session == null) throw const NotSignedIn();
    if (!await _prompt.ask(purposeText)) throw const StepUpCancelled();
    final jws = await signCompactJws(
      signer: key,
      header: {'alg': key.alg},
      payload: {
        'challenge': challenge.challenge,
        'sid': session.sid,
        'iat': _core.clock.now().millisecondsSinceEpoch ~/ 1000,
      },
    );
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
        if (problem?.status != 401 || problem?.code != 'step_up_required') {
          rethrow;
        }
        _core.log('step-up with password rejected');
      }
    }
  }
}
