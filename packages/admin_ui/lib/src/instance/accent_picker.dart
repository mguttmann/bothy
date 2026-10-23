// SPDX-License-Identifier: Apache-2.0

/// The accent of the instance: a choice of exactly the palette keys with a swatch each, and the
/// preview of the accent roles of M4-03 in light and dark (01 12; ADR 0044; R-PR-08; brief
/// M6-09 Design note 2; Ruling D-M6-09-2). No input takes a color value: the selection is a key.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'setting_kinds.dart';

/// The name of the accent [key] in the language of [l]; a key of a newer server shows the key.
String accentName(AppLocalizations l, String key) => switch (key) {
  'spruce' => l.instanceAccentSpruce,
  'loch' => l.instanceAccentLoch,
  'heather' => l.instanceAccentHeather,
  'rowan' => l.instanceAccentRowan,
  'amber' => l.instanceAccentAmber,
  'gorse' => l.instanceAccentGorse,
  'moss' => l.instanceAccentMoss,
  _ => key,
};

/// The seven accents of [accentPaletteKeys], each a radio with its swatch (the role
/// `color.accent.bg` of the current theme) and its name. [onChanged] receives the key; there is
/// no other way to set an accent.
///
/// Keys: `accent.picker`, `accent.<key>`.
class AccentPicker extends StatelessWidget {
  /// Creates the picker with [selected] marked.
  const AccentPicker({
    super.key,
    required this.selected,
    required this.onChanged,
  });

  /// The chosen key, or null when the stored accent is no palette key.
  final String? selected;

  /// Receives a palette key, or null when the picker is disabled.
  final ValueChanged<String>? onChanged;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final brightness = Theme.of(context).brightness;
    final onChanged = this.onChanged;
    return RadioGroup<String>(
      groupValue: selected,
      onChanged: (v) {
        if (v != null) onChanged?.call(v);
      },
      child: Semantics(
        label: l.instanceAccentLabel,
        child: Wrap(
          key: const ValueKey('accent.picker'),
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s8,
          children: [
            for (final key in accentPaletteKeys)
              InkWell(
                key: ValueKey('accent.$key'),
                borderRadius: BorderRadius.circular(DsRadius.md),
                onTap: onChanged == null ? null : () => onChanged(key),
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: DsSpace.s8,
                    vertical: DsSpace.s4,
                  ),
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(DsRadius.md),
                    border: Border.all(
                      color: key == selected
                          ? DsAccent.of(key, brightness).focusRing
                          : colors.borderDefault,
                      width: key == selected ? 2 : 1,
                    ),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Radio<String>(value: key, enabled: onChanged != null),
                      Container(
                        width: 20,
                        height: 20,
                        decoration: BoxDecoration(
                          color: DsAccent.of(key, brightness).accentBg,
                          shape: BoxShape.circle,
                        ),
                      ),
                      const SizedBox(width: DsSpace.s8),
                      Text(
                        accentName(l, key),
                        style: dsTextStyle(DsType.body)
                            .copyWith(color: colors.textPrimary),
                      ),
                    ],
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// The preview of [accent] in light and dark: each a card built with the theme builder of
/// `design_system` for that accent ([buildDsTheme]), showing the accent roles (a filled button
/// on `color.accent.bg`, a chip on `color.accent.subtle` with `color.accent.fg`, a link in
/// `color.text.link` and the focus ring `color.focus.ring`).
///
/// Keys: `accent.preview`, `accent.preview.light`, `accent.preview.dark`.
class AccentPreview extends StatelessWidget {
  /// Creates the preview of [accent] with the product name [productName].
  const AccentPreview({
    super.key,
    required this.accent,
    required this.productName,
  });

  /// The palette key.
  final String accent;

  /// The product name shown on the cards.
  final String productName;

  @override
  Widget build(BuildContext context) {
    return Wrap(
      key: const ValueKey('accent.preview'),
      spacing: DsSpace.s12,
      runSpacing: DsSpace.s12,
      children: [for (final mode in DsThemeMode.values) _card(context, mode)],
    );
  }

  Widget _card(BuildContext context, DsThemeMode mode) {
    final l = AppLocalizations.of(context);
    final theme = buildDsTheme(
      DsContext(
        theme: mode,
        density: DsDensityMode.comfortable,
        motion: DsMotionMode.reduced,
        accent: accent,
      ),
    );
    return Theme(
      key: ValueKey('accent.preview.${mode.name}'),
      data: theme,
      child: Builder(
        builder: (context) {
          final colors = context.dsColors;
          final roles = context.dsAccent;
          return Container(
            width: 260,
            padding: const EdgeInsets.all(DsSpace.s12),
            decoration: BoxDecoration(
              color: colors.surfaceCanvas,
              borderRadius: BorderRadius.circular(DsRadius.lg),
              border: Border.all(color: colors.borderDefault),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  mode == DsThemeMode.light
                      ? l.instancePreviewLight
                      : l.instancePreviewDark,
                  style: dsTextStyle(DsType.caption)
                      .copyWith(color: colors.textSecondary),
                ),
                const SizedBox(height: DsSpace.s4),
                Text(
                  productName,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: dsTextStyle(DsType.heading)
                      .copyWith(color: colors.textPrimary),
                ),
                const SizedBox(height: DsSpace.s8),
                Wrap(
                  spacing: DsSpace.s8,
                  runSpacing: DsSpace.s8,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    // A preview only: the button does nothing.
                    FilledButton(
                      onPressed: () {},
                      child: Text(l.instancePreviewButton),
                    ),
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: DsSpace.s8,
                        vertical: DsSpace.s4,
                      ),
                      decoration: BoxDecoration(
                        color: roles.accentSubtle,
                        borderRadius: BorderRadius.circular(DsRadius.md),
                        border: Border.all(color: roles.focusRing, width: 2),
                      ),
                      child: Text(
                        l.instancePreviewChip,
                        style: dsTextStyle(DsType.bodySm)
                            .copyWith(color: roles.accentFg),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: DsSpace.s8),
                Text(
                  l.instancePreviewLink,
                  style: dsTextStyle(DsType.body).copyWith(
                    color: roles.textLink,
                    decoration: TextDecoration.underline,
                    decorationColor: roles.textLink,
                  ),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
