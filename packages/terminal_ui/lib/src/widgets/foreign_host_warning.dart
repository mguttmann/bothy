// SPDX-License-Identifier: Apache-2.0

/// The warning before the step-up when an owner opens or attaches on a host assigned to
/// another person (docs/spec/24-terminal.md 8.4, TM-T26, threat model T-TM-12).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

/// Pops `true` only when the person chose "Connect anyway"; "Do not connect" (the main
/// action) and dismissing pop `false` or null, and then no step-up is requested.
///
/// ```dart
/// final go = await showDialog<bool>(context: context,
///     builder: (_) => ForeignHostWarningDialog(hostName: host.name));
/// ```
class ForeignHostWarningDialog extends StatelessWidget {
  /// Creates the warning for the host [hostName].
  const ForeignHostWarningDialog({super.key, required this.hostName});

  /// The name of the host, shown in the text.
  final String hostName;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    return AlertDialog(
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(DsRadius.lg),
        side: BorderSide(color: c.warningBorder, width: DsBorderWidth.focus),
      ),
      icon: DsIcon(
        DsIcons.stateWarning,
        color: c.warningFg,
        size: DsIconSizeRole.lg,
        decorative: true,
      ),
      title: Text(l.terminalForeignTitle),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.columnMin * 3),
        child: Text(l.terminalForeignBody(dsIsolate(hostName))),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(true),
          child: Text(l.terminalForeignConfirm),
        ),
        FilledButton(
          autofocus: true,
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(l.terminalKeyChangedReject),
        ),
      ],
    );
  }
}
