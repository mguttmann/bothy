// SPDX-License-Identifier: Apache-2.0

/// Step-up with the step-up device key (08 6.2, 6.3, 14.3; 06 3.3; brief M4-09 Design note 6).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/src/errors.dart';
import 'package:auth/src/jose.dart';
import 'package:auth/src/session_core.dart';

/// Explains a step-up to the person before the operating system asks for biometrics or the
/// PIN; the confirmation itself happens inside the step-up signer (M4-10), whose key requires
/// user presence.
abstract interface class StepUpPrompt {
  /// Shows [purposeText] (already in the person's language) and returns whether the person
  /// wants to continue; `false` cancels the step-up.
  Future<bool> ask(String purposeText);
}

/// Confirms the current session with the step-up key.
///
/// One confirmation: `createStepUpChallenge`, [StepUpPrompt.ask], the step-up key signs a
/// compact JWS whose header is only `{"alg":"ES256"}` (no `jwk`, `jku`, `x5u`, `x5c`, `crit`)
/// over `{challenge, sid, iat}`, then `completeStepUpWithDeviceKey`. The terminal's fresh
/// step-up (08 6.3) is the same flow; the server consumes it.
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

  /// Runs one step-up for the current session.
  ///
  /// Throws [StepUpUnavailable] before any request when the device has no step-up key,
  /// [StepUpCancelled] when the person declines the prompt or the signer reports a
  /// cancelled confirmation, [NotSignedIn] without a session, and the `DioException` of a
  /// rejected challenge or confirmation (`401 step_up_required`, `403 forbidden` with
  /// `step_up_not_possible`).
  Future<void> confirm({required String purposeText}) async {
    final key = _core.keys.stepUp;
    if (key == null) throw const StepUpUnavailable();
    final challenge = (await _core.api.getAuthApi().createStepUpChallenge(
      extra: {internalRequestExtra: true},
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
      extra: {internalRequestExtra: true},
    );
    _core.log('step-up confirmed');
  }
}
