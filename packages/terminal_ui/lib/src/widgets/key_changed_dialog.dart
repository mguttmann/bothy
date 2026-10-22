// SPDX-License-Identifier: Apache-2.0

/// The hard warning of a changed host key (screen D11 of docs/spec/01-product.md,
/// docs/spec/24-terminal.md 5.5, TM-T18; docs/design/components.md 12 "Key change warning").
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show fingerprintDisplay;
import 'package:flutter/material.dart';

/// Shows the stored and the current fingerprint in groups of four; "Do not connect" is the
/// main action and pops `false`, "Accept new key" pops `true`.
///
/// ```dart
/// final accept = await showDialog<bool>(context: context,
///     builder: (_) => KeyChangedDialog(stored: pin.fingerprint, current: host.keyFingerprint!));
/// ```
class KeyChangedDialog extends StatelessWidget {
  /// Creates the warning with the [stored] (pinned) and the [current] fingerprint.
  const KeyChangedDialog({
    super.key,
    required this.stored,
    required this.current,
  });

  /// The fingerprint of the pin on this device.
  final String stored;

  /// The fingerprint `getHost` delivers now.
  final String current;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final mono = dsTextStyle(DsType.code).copyWith(color: c.textPrimary);
    final label = dsTextStyle(DsType.label).copyWith(color: c.textSecondary);
    return AlertDialog(
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(DsRadius.lg),
        side: BorderSide(color: c.dangerBorder, width: DsBorderWidth.focus),
      ),
      icon: DsIcon(
        DsIcons.stateWarning,
        color: c.dangerFg,
        size: DsIconSizeRole.lg,
        decorative: true,
      ),
      title: Text(l.terminalKeyChangedTitle),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.columnMin * 3),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(l.terminalKeyChangedStored, style: label),
            SelectableText(fingerprintDisplay(stored), style: mono),
            const SizedBox(height: DsSpace.s8),
            Text(l.terminalKeyChangedNow, style: label),
            SelectableText(fingerprintDisplay(current), style: mono),
            const SizedBox(height: DsSpace.s12),
            Text(l.terminalKeyChangedBody),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(true),
          child: Text(l.terminalKeyChangedAccept),
        ),
        FilledButton(
          autofocus: true,
          style: FilledButton.styleFrom(
            backgroundColor: c.dangerBg,
            foregroundColor: c.dangerOnBg,
          ),
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(l.terminalKeyChangedReject),
        ),
      ],
    );
  }
}
