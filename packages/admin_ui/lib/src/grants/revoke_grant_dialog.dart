// SPDX-License-Identifier: Apache-2.0

/// The confirmation of revoking a grant (docs/spec/07-permissions.md 9.5: revoking takes
/// effect immediately and says so; every revocation needs a reason; 05 `grant.revoke` with
/// step-up; brief M4-27 fixed requirement 15).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';

/// The one destructive confirmation before `revokeGrant` for the grant of [name]: it says that
/// the revocation takes effect immediately, and "Revoke" stays disabled until a reason is
/// typed (the `revokeGrant` body requires one). Pops the reason, or null for "Cancel".
///
/// Example: `final reason = await RevokeGrantDialog.show(context, name: 'Sam');`
class RevokeGrantDialog extends StatefulWidget {
  /// Creates the dialog.
  const RevokeGrantDialog({super.key, required this.name});

  /// Whose grant is revoked.
  final String name;

  /// Shows the dialog over [context]; returns the reason or null.
  static Future<String?> show(BuildContext context, {required String name}) =>
      showDialog<String>(
        context: context,
        builder: (_) => RevokeGrantDialog(name: name),
      );

  @override
  State<RevokeGrantDialog> createState() => _RevokeGrantDialogState();
}

class _RevokeGrantDialogState extends State<RevokeGrantDialog> {
  final TextEditingController _reason = TextEditingController();

  @override
  void initState() {
    super.initState();
    _reason.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final reason = _reason.text.trim();
    return ConfirmChange(
      title: l.adminRevokeGrantTitle(widget.name),
      body: l.adminRevokeGrantBody,
      confirmLabel: l.adminRevoke,
      destructive: true,
      canConfirm: reason.isNotEmpty,
      onConfirm: (_) => Navigator.of(context).pop(reason),
      fields: TextField(
        key: const ValueKey('revokeGrant.reason'),
        controller: _reason,
        decoration: InputDecoration(labelText: l.adminReasonRequired),
      ),
    );
  }
}
