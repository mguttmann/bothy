// SPDX-License-Identifier: Apache-2.0

/// The confirmation before a hyperlink of the terminal opens (docs/spec/24-terminal.md 11.2,
/// SEC-TERM-6, TM-T14): the full target is visible, and only `https` and `http` open.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:terminal_ui/src/render/terminal_screen.dart';

/// Shows [target] in full; pops `true` on "Open link" (only offered for [isOpenableLink]),
/// `false` or null otherwise.
///
/// ```dart
/// final open = await showDialog<bool>(context: context,
///     builder: (_) => const LinkConfirmDialog(target: 'https://example.com/docs'));
/// ```
class LinkConfirmDialog extends StatelessWidget {
  /// Creates the confirmation for [target].
  const LinkConfirmDialog({super.key, required this.target});

  /// The full target of the link as the program sent it.
  final String target;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final openable = isOpenableLink(target);
    final host = openable ? Uri.parse(target).host : null;
    return AlertDialog(
      title: Text(l.terminalLinkTitle),
      // The host comes first and the target scrolls, so a long target cannot push the host
      // out of view (red team M4-35).
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.columnMin * 3),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(l.terminalLinkBody),
              if (host != null) ...[
                const SizedBox(height: DsSpace.s8),
                Text(
                  l.terminalLinkHost(host),
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: c.textPrimary),
                ),
              ],
              const SizedBox(height: DsSpace.s8),
              SelectableText(
                target,
                style: dsTextStyle(DsType.code).copyWith(color: c.textPrimary),
              ),
              if (!openable) ...[
                const SizedBox(height: DsSpace.s8),
                Text(
                  l.terminalLinkUnsupported,
                  style: dsTextStyle(DsType.bodySm).copyWith(color: c.dangerFg),
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          autofocus: true,
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(l.terminalCancel),
        ),
        if (openable)
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(l.terminalLinkOpen),
          ),
      ],
    );
  }
}
