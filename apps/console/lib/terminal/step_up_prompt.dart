// SPDX-License-Identifier: Apache-2.0

/// The step-up of `terminal.open` and `terminal.attach` in the console: always the passkey with
/// user verification, never password with TOTP or a new OIDC login (ADR 0070 item 1;
/// docs/spec/08-identitaet.md 6.3 item 3; 15 8 and 23.1).
library;

import 'package:api_client/api_client.dart' show ApiClient;
import 'package:auth/auth.dart' show StepUpCancelled;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../admin/step_up.dart';
import '../platform/browser.dart';
import '../session/console_controller.dart' show StepUpMethod;

/// The `TerminalStepUp` of the console ([confirm] is its tear-off).
///
/// It runs the console's action step-up of M6-04 ([ConsoleActionStepUp]: the challenge, the
/// browser's WebAuthn and the completion, each without a second step-up) restricted to the
/// passkey: [methods] is narrowed to the passkey, the dialog offers only "Confirm with
/// passkey", and the options for the browser carry `userVerification: required` whatever the
/// server sent. Without a passkey (the session's `step_up_methods_available` lacks `passkey`,
/// or the challenge carries no passkey options) it shows the notice of [showSetupNotice] and
/// throws [StepUpCancelled], which the controller treats as "the person declined".
///
/// The client never decides whether a step-up is fresh (15 1 principle 1): the server answers
/// `401 step_up_required` with `reason: passkey_required`, and the controller asks again.
///
/// Example:
/// ```dart
/// final stepUp = PasskeyStepUpPrompt(api: () => c.api, passkeys: env.passkeys,
///     methods: () => c.sessionStepUpMethods, prompt: () => c.stepUpPrompt,
///     showSetupNotice: DialogPasskeySetupNotice(() => navigatorKey.currentContext).show);
/// TerminalController(..., stepUp: stepUp.confirm, ...);
/// ```
final class PasskeyStepUpPrompt {
  /// Creates the step-up. [api] is the console's API, [passkeys] the browser's WebAuthn,
  /// [methods] the session's `step_up_methods_available`, [prompt] the console's step-up
  /// dialog (null when no window is there, which cancels) and [showSetupNotice] the notice
  /// with the way to set up a passkey.
  PasskeyStepUpPrompt({
    required ApiClient Function() api,
    required PasskeyPort passkeys,
    required this.methods,
    required ConsoleStepUpPrompt? Function() prompt,
    required this.showSetupNotice,
  }) {
    _stepUp = ConsoleActionStepUp(
      api: api,
      passkeys: _UserVerificationRequired(passkeys),
      methods: () => [
        if (methods().contains(StepUpMethod.passkey)) StepUpMethod.passkey,
      ],
      oidcProviders: () => const [],
      prompt: () => _PasskeyOnly(prompt(), showSetupNotice),
      // Never reached: the dialog offers no OIDC way.
      onOidc: (_) {},
      onConfirmed: () async {},
    );
  }

  /// The session's `step_up_methods_available`.
  final List<StepUpMethod> Function() methods;

  /// Shows the notice with the way to set up a passkey.
  final Future<void> Function() showSetupNotice;

  late final ConsoleActionStepUp _stepUp;

  /// Runs one passkey step-up for [purposeText] (already in the person's language).
  ///
  /// Throws [StepUpCancelled] when the person has no passkey, cancels the dialog or the
  /// browser, and the `DioException` of a refused challenge or confirmation otherwise.
  Future<void> confirm({required String purposeText}) async {
    if (!methods().contains(StepUpMethod.passkey)) {
      // No challenge: without a passkey nothing could confirm it.
      await showSetupNotice();
      throw const StepUpCancelled();
    }
    await _stepUp.confirm(purposeText);
  }
}

/// The console's dialog with the passkey as the only way; without a passkey in the challenge
/// the setup notice instead of the dialog.
final class _PasskeyOnly implements ConsoleStepUpPrompt {
  _PasskeyOnly(this._inner, this._notice);

  final ConsoleStepUpPrompt? _inner;
  final Future<void> Function() _notice;

  @override
  Future<ConsoleStepUpAnswer?> ask(
    String purpose,
    ConsoleStepUpWays ways,
  ) async {
    if (!ways.passkey) {
      await _notice();
      return null;
    }
    final inner = _inner;
    if (inner == null) return null;
    final answer = await inner.ask(
      purpose,
      const ConsoleStepUpWays(passkey: true, password: false, oidc: []),
    );
    return answer is StepUpWithPasskey ? answer : null;
  }
}

/// Asks the browser with `userVerification: required` in the options, so the terminal's
/// requirement does not rest on the options of the server alone (08 6.3 item 3).
final class _UserVerificationRequired implements PasskeyPort {
  _UserVerificationRequired(this._port);

  final PasskeyPort _port;

  @override
  Future<Map<String, Object?>> get(Map<String, Object?> options) =>
      _port.get({...options, 'userVerification': 'required'});
}

/// The notice for a person without a passkey: the terminal in the console opens only with a
/// passkey, and where one is set up (ADR 0070 item 1: "without a passkey, the console shows
/// the way to set one up"). "Close" (also Escape) dismisses it.
class PasskeySetupNoticeDialog extends StatelessWidget {
  /// Creates the notice.
  const PasskeySetupNoticeDialog({super.key});

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return AlertDialog(
      key: const ValueKey('console.terminal.passkeySetup'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        l.consoleTerminalPasskeyTitle,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Text(
            l.consoleTerminalPasskeySetup,
            style: dsTextStyle(DsType.body).copyWith(color: colors.textPrimary),
          ),
        ),
      ),
      actions: [
        FilledButton(
          key: const ValueKey('console.terminal.passkeySetup.close'),
          autofocus: true,
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.consoleTerminalPasskeyClose),
        ),
      ],
    );
  }
}

/// Shows [PasskeySetupNoticeDialog] over the navigator that [context] returns; without a
/// mounted context it shows nothing (the step-up is cancelled either way).
///
/// Example:
/// ```dart
/// final notice = DialogPasskeySetupNotice(() => navigatorKey.currentContext);
/// await notice.show();
/// ```
final class DialogPasskeySetupNotice {
  /// Creates the notice over [context].
  DialogPasskeySetupNotice(this.context);

  /// The context of the navigator that shows the dialog, or null.
  final BuildContext? Function() context;

  /// Shows the notice and completes when the person closed it.
  Future<void> show() async {
    final where = context();
    if (where == null || !where.mounted) return;
    await showDialog<void>(
      context: where,
      builder: (_) => const PasskeySetupNoticeDialog(),
    );
  }
}
