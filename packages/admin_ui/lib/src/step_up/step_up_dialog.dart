// SPDX-License-Identifier: Apache-2.0

/// The explanation before a step-up (docs/spec/08-identity.md 6.2 item 1: "The client shows
/// the text of the prompt in its language"; brief M4-25 Design note 3).
library;

import 'package:auth/auth.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
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

/// The [StepUpPrompt] of the apps: shows [StepUpDialog] over the navigator that [context]
/// returns; the step-up key then asks the operating system (08 6.1).
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
}
