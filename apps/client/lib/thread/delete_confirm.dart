// SPDX-License-Identifier: Apache-2.0

/// The one confirmation before a message is deleted (01 6.7: destructive actions ask once).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// Asks once whether the message should be deleted; returns true when the person confirms.
///
/// Example: `if (await confirmDelete(context)) await actions.delete(message);`
Future<bool> confirmDelete(BuildContext context) async =>
    await showDialog<bool>(
      context: context,
      builder: (context) {
        final l = AppLocalizations.of(context);
        return AlertDialog(
          key: const ValueKey('delete.dialog'),
          title: Text(l.messageDeleteTitle),
          content: Text(l.messageDeleteBody),
          actions: [
            DsButton(
              key: const ValueKey('delete.cancel'),
              label: l.threadCancel,
              style: DsButtonStyle.ghost,
              onPressed: () => Navigator.of(context).pop(false),
            ),
            DsButton(
              key: const ValueKey('delete.confirm'),
              label: l.messageActionDelete,
              style: DsButtonStyle.danger,
              onPressed: () => Navigator.of(context).pop(true),
            ),
          ],
        );
      },
    ) ??
    false;
