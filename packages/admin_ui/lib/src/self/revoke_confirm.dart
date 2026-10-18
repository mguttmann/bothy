// SPDX-License-Identifier: Apache-2.0

/// The one confirmation before a destructive action (docs/spec/01-product.md 6.7: "Destructive
/// actions ... ask for confirmation once").
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// Asks once before revoking a device or ending a session.
///
/// With [withReason] a text field takes the reason the server stores in the audit
/// (`revokeDevice` requires one; for an empty field the caller sends its default). Pops the
/// typed reason (empty without a field or text) for [confirmLabel], null for "Cancel".
class RevokeConfirmDialog extends StatefulWidget {
  /// Creates the dialog.
  const RevokeConfirmDialog({
    super.key,
    required this.title,
    required this.body,
    required this.confirmLabel,
    this.withReason = false,
  });

  /// The question, for example "Revoke Alex phone?".
  final String title;

  /// What happens.
  final String body;

  /// The label of the destructive button.
  final String confirmLabel;

  /// Whether a reason field is shown.
  final bool withReason;

  /// Shows the dialog over [context]; returns the reason (empty when none was typed) or null.
  static Future<String?> show(
    BuildContext context, {
    required String title,
    required String body,
    required String confirmLabel,
    bool withReason = false,
  }) => showDialog<String>(
    context: context,
    builder: (_) => RevokeConfirmDialog(
      title: title,
      body: body,
      confirmLabel: confirmLabel,
      withReason: withReason,
    ),
  );

  @override
  State<RevokeConfirmDialog> createState() => _RevokeConfirmDialogState();
}

class _RevokeConfirmDialogState extends State<RevokeConfirmDialog> {
  final TextEditingController _reason = TextEditingController();

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return AlertDialog(
      key: const ValueKey('revoke.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        widget.title,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              widget.body,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
            if (widget.withReason) ...[
              const SizedBox(height: DsSpace.s16),
              TextField(
                key: const ValueKey('revoke.reason'),
                controller: _reason,
                decoration: InputDecoration(labelText: l.devicesRevokeReason),
              ),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('revoke.cancel'),
          autofocus: true,
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: const ValueKey('revoke.confirm'),
          style: FilledButton.styleFrom(
            backgroundColor: colors.dangerBg,
            foregroundColor: colors.dangerOnBg,
          ),
          onPressed: () => Navigator.of(context).pop(_reason.text.trim()),
          child: Text(widget.confirmLabel),
        ),
      ],
    );
  }
}
