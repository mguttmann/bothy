// SPDX-License-Identifier: Apache-2.0

/// Codes shown once and read character by character: the codes of onboarding (brief M4-18
/// Design note 6) and the enrollment codes of the administration (docs/spec/08-identity.md
/// 3.2; brief M4-26 Design note 4). It lives here, not in the app, so the console can show the
/// same view (packages never import from `apps/`, 15 3.1 rule 2).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

/// Splits [code] into groups of [groupSize] characters; spaces and hyphens of the input do not
/// count.
///
/// Example: `groupCode('HQ7P3MXR', 4)` is `['HQ7P', '3MXR']`.
List<String> groupCode(String code, int groupSize) {
  final compact = code.replaceAll(RegExp(r'[\s-]'), '');
  return [
    for (var i = 0; i < compact.length; i += groupSize)
      compact.substring(
        i,
        i + groupSize > compact.length ? compact.length : i + groupSize,
      ),
  ];
}

/// The characters of [code] separated by commas, so a screen reader reads each one on its own
/// (01 8): `'481207'` gives `'4, 8, 1, 2, 0, 7'`.
String spellCode(String code) =>
    code.replaceAll(RegExp(r'[\s-]'), '').split('').join(', ');

/// A code in groups, in the mono font, copyable, read character by character (01 5.6).
///
/// The semantics label spells the code ([spellCode]); the visible groups are excluded from
/// semantics so the screen reader never reads "481" as a number.
///
/// Example: `CodeText('HQ7P3MXR')` shows `HQ7P 3MXR`.
class CodeText extends StatefulWidget {
  /// Creates the code; [groupSize] is the length of a group; with [keepGroups] the groups
  /// are the parts of the code between its hyphens (an enrollment code `en_4KX9-2MQA`).
  const CodeText(
    this.code, {
    super.key,
    this.groupSize = 4,
    this.keepGroups = false,
  });

  /// The code.
  final String code;

  /// Characters per group.
  final int groupSize;

  /// Whether the code's own hyphens separate the groups.
  final bool keepGroups;

  @override
  State<CodeText> createState() => _CodeTextState();
}

class _CodeTextState extends State<CodeText> {
  bool _copied = false;

  Future<void> _copy() async {
    await Clipboard.setData(
      ClipboardData(text: widget.code.replaceAll(RegExp(r'[\s-]'), '')),
    );
    if (mounted) setState(() => _copied = true);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          label: l.onboardingCodeLabel(spellCode(widget.code)),
          excludeSemantics: true,
          child: Text(
            (widget.keepGroups
                    ? widget.code.split(RegExp(r'[\s-]+'))
                    : groupCode(widget.code, widget.groupSize))
                .join('  '),
            key: const ValueKey('code.groups'),
            textAlign: TextAlign.center,
            style: dsTextStyle(DsType.display).copyWith(
              fontFamily: DsType.code.fontFamily,
              fontFamilyFallback: DsType.code.fontFamilyFallback,
              color: colors.textPrimary,
            ),
          ),
        ),
        TextButton(
          key: const ValueKey('code.copy'),
          onPressed: _copy,
          child: Text(_copied ? l.onboardingCopied : l.onboardingCopy),
        ),
      ],
    );
  }
}
