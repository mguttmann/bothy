// SPDX-License-Identifier: Apache-2.0

/// The one confirmation before a changing administration action (docs/spec/01-product.md 5.2:
/// "every changing administration action with step-up shows the confirmation"; 6.7:
/// destructive actions ask once; brief M4-26 Design note 2).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// One dialog that says in plain words what the change does, with optional form [fields], an
/// optional reason field and "Cancel" and [confirmLabel].
///
/// After "Confirm" the caller sends the change; the step-up interceptor of `auth` then opens
/// the step-up dialog when, and only when, the server answers `401 step_up_required`. A
/// [destructive] change (revoke, delete, disable, reject) shows the danger banner of
/// `DsBanner` and a danger button. Pops the typed reason (empty without a field or text) for
/// "Confirm", null for "Cancel".
///
/// Example:
/// ```dart
/// final reason = await ConfirmChange.show(
///   context,
///   title: l.adminDisableTitle('Sam'),
///   body: l.adminDisableBody('Sam'),
///   confirmLabel: l.adminPersonDisable,
///   destructive: true,
///   withReason: true,
/// );
/// ```
class ConfirmChange extends StatefulWidget {
  /// Creates the dialog.
  const ConfirmChange({
    super.key,
    required this.title,
    required this.body,
    required this.confirmLabel,
    this.destructive = false,
    this.withReason = false,
    this.fields,
    this.canConfirm = true,
    this.onConfirm,
  });

  /// The question, for example "Disable Sam?".
  final String title;

  /// What the change does, for example "Sam is disabled. All sessions of Sam end now.".
  final String body;

  /// The label of the confirming button.
  final String confirmLabel;

  /// Whether the change removes or ends something (danger style, 01 6.7).
  final bool destructive;

  /// Whether a reason field is shown (the server stores the reason in the audit log).
  final bool withReason;

  /// Form fields between the text and the reason, or null.
  final Widget? fields;

  /// Whether "Confirm" is enabled (a form dialog disables it while a field is invalid).
  final bool canConfirm;

  /// Runs instead of popping the reason, for a form dialog that pops its own result.
  final ValueChanged<String>? onConfirm;

  /// Shows the dialog over [context]; returns the reason (empty when none was typed) or null.
  static Future<String?> show(
    BuildContext context, {
    required String title,
    required String body,
    required String confirmLabel,
    bool destructive = false,
    bool withReason = false,
  }) => showDialog<String>(
    context: context,
    builder: (_) => ConfirmChange(
      title: title,
      body: body,
      confirmLabel: confirmLabel,
      destructive: destructive,
      withReason: withReason,
    ),
  );

  @override
  State<ConfirmChange> createState() => _ConfirmChangeState();
}

class _ConfirmChangeState extends State<ConfirmChange> {
  final TextEditingController _reason = TextEditingController();

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  void _confirm() {
    final reason = _reason.text.trim();
    final onConfirm = widget.onConfirm;
    if (onConfirm != null) {
      onConfirm(reason);
    } else {
      Navigator.of(context).pop(reason);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return AlertDialog(
      key: const ValueKey('confirm.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        widget.title,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (widget.destructive)
                DsBanner(
                  key: const ValueKey('confirm.body'),
                  tone: DsTone.danger,
                  text: widget.body,
                )
              else
                Text(
                  widget.body,
                  key: const ValueKey('confirm.body'),
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: colors.textSecondary),
                ),
              if (widget.fields case final fields?) ...[
                const SizedBox(height: DsSpace.s16),
                fields,
              ],
              if (widget.withReason) ...[
                const SizedBox(height: DsSpace.s16),
                TextField(
                  key: const ValueKey('confirm.reason'),
                  controller: _reason,
                  decoration: InputDecoration(labelText: l.adminReason),
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('confirm.cancel'),
          autofocus: widget.destructive,
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: const ValueKey('confirm.submit'),
          style: widget.destructive
              ? FilledButton.styleFrom(
                  backgroundColor: colors.dangerBg,
                  foregroundColor: colors.dangerOnBg,
                )
              : null,
          onPressed: widget.canConfirm ? _confirm : null,
          child: Text(widget.confirmLabel),
        ),
      ],
    );
  }
}
