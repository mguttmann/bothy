// SPDX-License-Identifier: Apache-2.0

/// The explanation before a step-up (docs/spec/08-identity.md 6.2 item 1: "The client shows
/// the text of the prompt in its language"; brief M4-25 Design note 3).
library;

import 'package:auth/auth.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

/// The dialog that explains a step-up before the operating system asks for biometrics or the
/// device password: the purpose of the action, what comes next, "Cancel" and "Continue".
///
/// Pops `true` for "Continue" and `false` for "Cancel" (Escape and a tap outside count as
/// "Cancel").
class StepUpDialog extends StatelessWidget {
  /// Creates the dialog for [purposeText] (already in the person's language; empty shows a
  /// general text).
  const StepUpDialog({super.key, required this.purposeText});

  /// What the confirmation is for.
  final String purposeText;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return AlertDialog(
      key: const ValueKey('stepUp.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        l.shellStepUpTitle,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              purposeText.isEmpty ? l.stepUpGenericPurpose : purposeText,
              key: const ValueKey('stepUp.purpose'),
              style: dsTextStyle(DsType.bodyStrong)
                  .copyWith(color: colors.textPrimary),
            ),
            const SizedBox(height: DsSpace.s8),
            Text(
              l.stepUpNext,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('stepUp.cancel'),
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: const ValueKey('stepUp.continue'),
          autofocus: true,
          onPressed: () => Navigator.of(context).pop(true),
          child: Text(l.stepUpContinue),
        ),
      ],
    );
  }
}

/// The password variant of the step-up dialog, for a device without a step-up key (Windows,
/// docs/spec/08-identity.md 6.2): the purpose of the action, a password field, a field for the
/// six-digit one-time code of the authenticator app, "Cancel" and "Confirm". "Confirm" stays
/// disabled until a password and six digits are typed; [rejected] adds the line that the
/// previous attempt was not accepted.
///
/// Pops the typed [StepUpCredentials], or null for "Cancel" (Escape and a tap outside count as
/// "Cancel").
class StepUpPasswordDialog extends StatefulWidget {
  /// Creates the dialog for [purposeText] (already in the person's language; empty shows a
  /// general text).
  const StepUpPasswordDialog({
    super.key,
    required this.purposeText,
    this.rejected = false,
  });

  /// What the confirmation is for.
  final String purposeText;

  /// Whether the previous attempt was not accepted.
  final bool rejected;

  @override
  State<StepUpPasswordDialog> createState() => _StepUpPasswordDialogState();
}

class _StepUpPasswordDialogState extends State<StepUpPasswordDialog> {
  static const int _codeLength = 6;

  final _password = TextEditingController();
  final _code = TextEditingController();

  @override
  void initState() {
    super.initState();
    _password.addListener(_changed);
    _code.addListener(_changed);
  }

  @override
  void dispose() {
    _password.dispose();
    _code.dispose();
    super.dispose();
  }

  void _changed() => setState(() {});

  bool get _complete =>
      _password.text.isNotEmpty && _code.text.length == _codeLength;

  void _confirm() {
    if (!_complete) return;
    Navigator.of(
      context,
    ).pop<StepUpCredentials>((password: _password.text, totpCode: _code.text));
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textSecondary);
    return AlertDialog(
      key: const ValueKey('stepUp.passwordDialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        l.shellStepUpTitle,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: AutofillGroup(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  widget.purposeText.isEmpty
                      ? l.stepUpGenericPurpose
                      : widget.purposeText,
                  key: const ValueKey('stepUp.purpose'),
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
                const SizedBox(height: DsSpace.s8),
                Text(l.stepUpPasswordNext, style: body),
                if (widget.rejected) ...[
                  const SizedBox(height: DsSpace.s8),
                  DsBanner(
                    key: const ValueKey('stepUp.rejected'),
                    tone: DsTone.danger,
                    text: l.stepUpPasswordRejected,
                  ),
                ],
                const SizedBox(height: DsSpace.s8),
                TextField(
                  key: const ValueKey('stepUp.password'),
                  controller: _password,
                  obscureText: true,
                  autofocus: true,
                  autofillHints: const [AutofillHints.password],
                  textInputAction: TextInputAction.next,
                  decoration: InputDecoration(labelText: l.stepUpPasswordLabel),
                ),
                const SizedBox(height: DsSpace.s8),
                TextField(
                  key: const ValueKey('stepUp.code'),
                  controller: _code,
                  autofillHints: const [AutofillHints.oneTimeCode],
                  keyboardType: TextInputType.number,
                  inputFormatters: [
                    FilteringTextInputFormatter.digitsOnly,
                    LengthLimitingTextInputFormatter(_codeLength),
                  ],
                  decoration: InputDecoration(labelText: l.stepUpCodeLabel),
                  onSubmitted: (_) => _confirm(),
                ),
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('stepUp.cancel'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: const ValueKey('stepUp.confirm'),
          onPressed: _complete ? _confirm : null,
          child: Text(l.stepUpConfirm),
        ),
      ],
    );
  }
}

/// The [StepUpPrompt] of the apps: shows [StepUpDialog] over the navigator that [context]
/// returns; the step-up key then asks the operating system (08 6.1). On a device without a
/// step-up key it shows [StepUpPasswordDialog] instead (08 6.2).
///
/// Without a context (no window yet) the step-up is cancelled, so no confirmation happens
/// unexplained.
///
/// Example:
/// ```dart
/// final prompt = StepUpDialogPrompt(() => navigatorKey.currentContext);
/// ```
final class StepUpDialogPrompt implements StepUpPrompt {
  /// Creates the prompt over [context].
  StepUpDialogPrompt(this.context);

  /// The context of the navigator that shows the dialog, or null.
  final BuildContext? Function() context;

  @override
  Future<bool> ask(String purposeText) async {
    final where = context();
    if (where == null || !where.mounted) return false;
    final answer = await showDialog<bool>(
      context: where,
      builder: (_) => StepUpDialog(purposeText: purposeText),
    );
    return answer ?? false;
  }

  @override
  Future<StepUpCredentials?> askPassword(
    String purposeText, {
    bool rejected = false,
  }) async {
    final where = context();
    if (where == null || !where.mounted) return null;
    return showDialog<StepUpCredentials>(
      context: where,
      builder: (_) =>
          StepUpPasswordDialog(purposeText: purposeText, rejected: rejected),
    );
  }
}
