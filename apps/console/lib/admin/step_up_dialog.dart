// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'step_up.dart';

/// The step-up of an action in the console (08 6.2; brief M6-04 Design note 2): the purpose of
/// the action and the one way the person confirms. With a passkey the browser asks next
/// (WebAuthn with `userVerification = required`); without one the dialog takes password and
/// one-time code, or offers the new OIDC login, which leaves the tab.
///
/// Pops the [ConsoleStepUpAnswer], or null for "Cancel" (Escape and a tap outside count as
/// "Cancel"). The texts come from `packages/l10n`.
class ConsoleStepUpDialog extends StatefulWidget {
  /// Creates the dialog for [purpose] with [ways].
  const ConsoleStepUpDialog({
    super.key,
    required this.purpose,
    required this.ways,
  });

  /// What the confirmation is for, already in the person's language; empty shows a general
  /// text.
  final String purpose;

  /// The ways the person can confirm.
  final ConsoleStepUpWays ways;

  @override
  State<ConsoleStepUpDialog> createState() => _ConsoleStepUpDialogState();
}

class _ConsoleStepUpDialogState extends State<ConsoleStepUpDialog> {
  final _password = TextEditingController();
  final _code = TextEditingController();

  @override
  void dispose() {
    _password.dispose();
    _code.dispose();
    super.dispose();
  }

  void _submitPassword() =>
      Navigator.of(context).pop(StepUpWithPassword(_password.text, _code.text));

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final ways = widget.ways;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textSecondary);
    final none = !ways.passkey && !ways.password && ways.oidc.isEmpty;
    return AlertDialog(
      key: const ValueKey('consoleStepUp.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        l.shellStepUpTitle,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                widget.purpose.isEmpty
                    ? l.stepUpGenericPurpose
                    : widget.purpose,
                key: const ValueKey('consoleStepUp.purpose'),
                style: dsTextStyle(DsType.bodyStrong)
                    .copyWith(color: colors.textPrimary),
              ),
              const SizedBox(height: DsSpace.s8),
              if (ways.passkey)
                Text(l.consoleStepUpPasskeyNext, style: body)
              else if (ways.password) ...[
                TextField(
                  key: const ValueKey('consoleStepUp.passwordField'),
                  controller: _password,
                  obscureText: true,
                  autofocus: true,
                  autofillHints: const [AutofillHints.password],
                  decoration: InputDecoration(
                    labelText: l.consoleLoginPassword,
                  ),
                ),
                const SizedBox(height: DsSpace.s8),
                TextField(
                  key: const ValueKey('consoleStepUp.codeField'),
                  controller: _code,
                  autofillHints: const [AutofillHints.oneTimeCode],
                  keyboardType: TextInputType.number,
                  decoration: InputDecoration(
                    labelText: l.consoleLoginOneTimeCode,
                  ),
                  onSubmitted: (_) => _submitPassword(),
                ),
              ] else if (ways.oidc.isNotEmpty) ...[
                Text(l.consoleStepUpOidcAction, style: body),
                const SizedBox(height: DsSpace.s8),
                for (final p in ways.oidc)
                  Padding(
                    padding: const EdgeInsets.only(bottom: DsSpace.s8),
                    child: OutlinedButton(
                      key: ValueKey('consoleStepUp.oidc.${p.id}'),
                      onPressed: () =>
                          Navigator.of(context).pop(StepUpWithOidc(p)),
                      child: Text(l.consoleStepUpOidc(p.name)),
                    ),
                  ),
              ] else if (none)
                DsBanner(tone: DsTone.warning, text: l.consoleStepUpNoMethod),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('consoleStepUp.cancel'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        if (ways.passkey)
          FilledButton(
            key: const ValueKey('consoleStepUp.passkey'),
            autofocus: true,
            onPressed: () =>
                Navigator.of(context).pop(const StepUpWithPasskey()),
            child: Text(l.consoleStepUpPasskey),
          )
        else if (ways.password)
          FilledButton(
            key: const ValueKey('consoleStepUp.password'),
            onPressed: _submitPassword,
            child: Text(l.consoleStepUpConfirm),
          ),
      ],
    );
  }
}

/// The [ConsoleStepUpPrompt] of the console: [ConsoleStepUpDialog] over the navigator that
/// [context] returns. Without a mounted context (no window yet) the step-up is cancelled, so
/// no confirmation happens unexplained.
///
/// Example:
/// ```dart
/// controller.stepUpPrompt = DialogStepUpPrompt(() => navigatorKey.currentContext);
/// ```
final class DialogStepUpPrompt implements ConsoleStepUpPrompt {
  /// Creates the prompt over [context].
  DialogStepUpPrompt(this.context);

  /// The context of the navigator that shows the dialog, or null.
  final BuildContext? Function() context;

  @override
  Future<ConsoleStepUpAnswer?> ask(String purpose, ConsoleStepUpWays ways) {
    final where = context();
    if (where == null || !where.mounted) return Future.value();
    return showDialog<ConsoleStepUpAnswer>(
      context: where,
      builder: (_) => ConsoleStepUpDialog(purpose: purpose, ways: ways),
    );
  }
}
