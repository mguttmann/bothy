// SPDX-License-Identifier: Apache-2.0

/// The fingerprint field of the confirmation (docs/spec/01-product.md 5.2 A8; 24 3.2, 4.3;
/// brief M4-37 Design note 1 and fixed requirements 9, 13).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/hosts_repository.dart';

/// The field for the first characters of the fingerprint that `bothy-host pair` shows on the
/// host, and the answer of a wrong entry.
///
/// The typed text is sent as it is; the repository removes separators and ignores the case
/// ([normalizeFingerprint]). The number in the label is the reference default of
/// `terminal.fingerprint_confirm_chars` ([fingerprintConfirmCharsDefault]), because the server
/// sends the value with no answer; a prefix of another length comes back as
/// `400 validation_failed`. [attemptsRemaining] is the count of the last
/// `409 fingerprint_mismatch`, shown as the server named it, or null without a mismatch;
/// [mismatchText] is the translated problem.
///
/// Example: `FingerprintConfirm(controller: prefix, attemptsRemaining: 2, mismatchText: t)`.
class FingerprintConfirm extends StatelessWidget {
  /// Creates the field.
  const FingerprintConfirm({
    super.key,
    required this.controller,
    this.attemptsRemaining,
    this.mismatchText,
  });

  /// The typed prefix.
  final TextEditingController controller;

  /// The confirmations left after the last mismatch, or null.
  final int? attemptsRemaining;

  /// The text of the last mismatch, or null.
  final String? mismatchText;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final left = attemptsRemaining;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        TextField(
          key: const ValueKey('fingerprint.prefix'),
          controller: controller,
          autocorrect: false,
          enableSuggestions: false,
          textCapitalization: TextCapitalization.characters,
          style: dsTextStyle(DsType.code)
              .copyWith(color: context.dsColors.textPrimary),
          decoration: InputDecoration(
            labelText: l.hostFingerprintLabel(fingerprintConfirmCharsDefault),
          ),
        ),
        if (left != null && left > 0) ...[
          const SizedBox(height: DsSpace.s8),
          DsBanner(
            key: const ValueKey('fingerprint.mismatch'),
            tone: DsTone.danger,
            liveRegion: true,
            text:
                '${mismatchText ?? problemText(l, 'fingerprint_mismatch')}. '
                '${l.hostFingerprintCompare} ${l.hostAttemptsLeft(left)}',
          ),
        ],
      ],
    );
  }
}
