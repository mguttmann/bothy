// SPDX-License-Identifier: Apache-2.0

/// The backend policy badge and the cost text (components.md section 10).
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import '../theme/ds_theme.dart';
import 'ds_common.dart';
import 'view_models.dart';

/// Shows what an agent runs on (`agents.billing_mode`), in the thread header, the live box and
/// the agent card.
///
/// `personal` and `organization` each have their icon, badge colors and text; the label reads
/// "Backend: Personal, runs on the subscription of {holderName}". When [reportedMode]
/// (`reported_billing_mode`) is set and differs from [mode], a warning badge "Reported: {mode}"
/// follows (runs then fail with `billing_mode_mismatch`). An unknown mode shows [code].
///
/// ```dart
/// DsBillingBadge(mode: DsBillingMode.personal, holderName: 'Alex')
/// ```
class DsBillingBadge extends StatelessWidget {
  /// Creates the badge.
  const DsBillingBadge({
    super.key,
    required this.mode,
    this.holderName,
    this.reportedMode,
    this.code,
  });

  /// The configured mode.
  final DsBillingMode mode;

  /// For `personal`: the person whose subscription the agent uses.
  final String? holderName;

  /// The reported mode, or null when the adapter reports none.
  final DsBillingMode? reportedMode;

  /// The raw code of an unknown mode.
  final String? code;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final colors = context.dsColors;
    String word(DsBillingMode m, String? raw) => switch (m) {
      DsBillingMode.personal => l.billingPersonal,
      DsBillingMode.organization => l.billingOrganization,
      DsBillingMode.unknown => l.billingUnknown(raw ?? ''),
    };
    final badge = switch (mode) {
      DsBillingMode.personal => DsBadge(
        text: l.billingPersonal,
        glyph: DsIcons.billingPersonal,
        background: colors.badgePersonalBg,
        foreground: colors.badgePersonalFg,
        semanticsLabel: l.billingLabelPersonal(holderName ?? ''),
      ),
      DsBillingMode.organization => DsBadge(
        text: l.billingOrganization,
        glyph: DsIcons.billingOrganization,
        background: colors.badgeOrganizationBg,
        foreground: colors.badgeOrganizationFg,
        semanticsLabel: l.billingLabelOrganization,
      ),
      DsBillingMode.unknown => DsBadge(
        text: l.billingUnknown(code ?? ''),
        glyph: DsIcons.stateInfo,
        background: colors.surfaceSunken,
        foreground: colors.textSecondary,
      ),
    };
    final reported = reportedMode;
    if (reported == null || reported == mode) return badge;
    return Wrap(
      spacing: DsSpace.s4,
      runSpacing: DsSpace.s4,
      children: [
        badge,
        DsBadge(
          text: l.billingReported(word(reported, null)),
          glyph: DsIcons.stateWarning,
          background: colors.warningSubtle,
          foreground: colors.warningFg,
        ),
      ],
    );
  }
}

/// A cost in mono with its suffix: "Estimate" while an `organization` cost is not reconciled,
/// "for information only" for a `personal` cost (components.md section 10, 01 section 6.7).
///
/// The amount is formatted for the locale (`L10nFormat.money`).
///
/// ```dart
/// DsCostText(DsCost(DsMoney(1.2, 'USD'), mode: DsBillingMode.organization, isEstimate: true))
/// ```
class DsCostText extends StatelessWidget {
  /// Creates the text of [cost].
  const DsCostText(this.cost, {super.key, this.color});

  /// The cost.
  final DsCost cost;

  /// The text color, or null for `color.text.secondary`.
  final Color? color;

  /// The suffix of [cost] in [context]'s locale, or null.
  static String? suffix(BuildContext context, DsCost cost) {
    final l = dsL10n(context);
    return switch (cost.mode) {
      DsBillingMode.personal => l.costInformationOnly,
      DsBillingMode.organization when cost.isEstimate => l.costEstimate,
      _ => null,
    };
  }

  @override
  Widget build(BuildContext context) {
    final fg = color ?? context.dsColors.textSecondary;
    final amount = dsFormat(context)
        .money(cost.money.amount, cost.money.currency);
    final extra = suffix(context, cost);
    return Text.rich(
      TextSpan(
        children: [
          TextSpan(
            text: amount,
            style: dsTextStyle(DsType.codeSm).copyWith(color: fg),
          ),
          if (extra != null)
            TextSpan(
              text: ' $extra',
              style: dsTextStyle(DsType.caption).copyWith(color: fg),
            ),
        ],
      ),
    );
  }
}
