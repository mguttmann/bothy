// SPDX-License-Identifier: Apache-2.0

/// The plain-language preview of a grant (docs/spec/01-product.md 5.2 A2; 07 9.3; brief M4-27
/// Design note 1 and fixed requirements 9, 11 to 14).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/access_repository.dart';
import 'grant_labels.dart';

/// The server's preview of a grant: its `text` unchanged in `text_locale` (07 9.3: the text
/// comes from templates in `authz`, the client composes none), the effects grouped per
/// principal, the invariant violations (I1 explained with "switch to organization first",
/// 10 11), a delegation denial with its reason (07 4.1) and the hints as labels.
///
/// [names] turns the IDs of violations and hints into names. While [preview] is null the panel
/// says that the effect is being checked.
///
/// Example: `PreviewPanel(preview: answer, names: names)`.
class PreviewPanel extends StatelessWidget {
  /// Creates the panel.
  const PreviewPanel({super.key, required this.preview, required this.names});

  /// The server's answer, or null while it loads.
  final GrantPreviewResult? preview;

  /// The names of the IDs the answer names.
  final AccessNames names;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final p = preview;
    final heading = dsTextStyle(DsType.bodyStrong)
        .copyWith(color: colors.textPrimary);
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Container(
      key: const ValueKey('preview.panel'),
      padding: const EdgeInsets.all(DsSpace.s12),
      decoration: BoxDecoration(
        border: Border.all(color: colors.borderDefault),
        borderRadius: BorderRadius.circular(DsRadius.md),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Semantics(
            header: true,
            child: Text(l.adminPreviewTitle, style: heading),
          ),
          const SizedBox(height: DsSpace.s8),
          if (p == null)
            Text(
              l.adminPreviewLoading,
              key: const ValueKey('preview.loading'),
              style: secondary,
            )
          else ...[
            Semantics(
              liveRegion: true,
              child: Text(
                p.text,
                key: const ValueKey('preview.text'),
                locale: Locale(p.textLocale),
                style: body,
              ),
            ),
            if (!p.allowed && p.denialReason != null) ...[
              const SizedBox(height: DsSpace.s8),
              DsBanner(
                key: const ValueKey('preview.denied'),
                tone: DsTone.danger,
                text: l.adminPreviewDenied(
                  problemText(l, 'forbidden', reason: p.denialReason),
                ),
              ),
            ],
            if (p.effects.isNotEmpty) ...[
              const SizedBox(height: DsSpace.s12),
              Text(l.adminPreviewEffects, style: heading),
              for (final group in _byPrincipal(p.effects))
                Padding(
                  key: ValueKey('preview.effect.${group.first.principal.id}'),
                  padding: const EdgeInsets.only(top: DsSpace.s4),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(group.first.principal.displayName, style: body),
                      for (final e in group) ...[
                        Text(_effectLabel(l, e), style: secondary),
                        if (e.capabilitiesAdded.isNotEmpty)
                          Text(
                            l.adminPreviewCapabilities(
                              [
                                for (final c in e.capabilitiesAdded)
                                  capabilityLabel(l, c),
                              ].join(', '),
                            ),
                            style: secondary,
                          ),
                      ],
                    ],
                  ),
                ),
            ],
            for (final v in p.violations) ...[
              const SizedBox(height: DsSpace.s8),
              _Violation(finding: v, names: names),
            ],
            if (p.hints.isNotEmpty) ...[
              const SizedBox(height: DsSpace.s12),
              Text(l.adminPreviewHints, style: heading),
              for (final h in p.hints)
                Padding(
                  padding: const EdgeInsets.only(top: DsSpace.s4),
                  child: h.code == 'terminal_not_effective'
                      // A grant on a host that SR10 or SR12 keeps from taking effect: the
                      // badge marks it, the cause is the server's text above (07 9.3).
                      ? Row(
                          children: [
                            const NotEffectiveBadge(
                              key: ValueKey('preview.notEffective'),
                            ),
                            const SizedBox(width: DsSpace.s8),
                            Expanded(
                              child: DsBanner(
                                key: ValueKey('preview.hint.${h.code}'),
                                tone: DsTone.warning,
                                text: _hintLabel(l, h),
                              ),
                            ),
                          ],
                        )
                      : DsBanner(
                          key: ValueKey('preview.hint.${h.code}'),
                          tone: DsTone.warning,
                          text: _hintLabel(l, h),
                        ),
                ),
            ],
          ],
        ],
      ),
    );
  }

  /// The effects in the server's order, grouped by principal.
  static List<List<GrantEffect>> _byPrincipal(List<GrantEffect> effects) {
    final groups = <String, List<GrantEffect>>{};
    for (final e in effects) {
      groups.putIfAbsent(e.principal.id, () => []).add(e);
    }
    return groups.values.toList();
  }

  /// "Gets", "Loses" or "becomes": the direction of the server's before and after levels.
  static String _effectLabel(AppLocalizations l, GrantEffect e) {
    if (e.before == AccessLevel.none) {
      return l.adminPreviewAdded(levelLabel(l, e.after));
    }
    if (e.after == AccessLevel.none) {
      return l.adminPreviewRemoved(levelLabel(l, e.before));
    }
    return l.adminPreviewChanged(
      levelLabel(l, e.before),
      levelLabel(l, e.after),
    );
  }

  String _hintLabel(AppLocalizations l, GrantHint h) => switch (h.code) {
    'read_horizon' => l.adminHintReadHorizon(
      names.principal(h.principalId ?? ''),
      names.target(h.channelId ?? h.spaceId ?? ''),
      names.principal(h.agentId ?? ''),
    ),
    'trigger_blocked_tool_class' => l.adminHintTriggerBlocked(
      names.principal(h.agentId ?? ''),
    ),
    'group_reach' => l.adminHintGroupReach(h.count ?? 0),
    'expiry_missing' => l.adminHintExpiryMissing,
    'terminal_not_effective' => l.adminHintTerminalNotEffective,
    _ => l.adminHintUnknown(h.code),
  };
}

/// The badge "Not effective" of a grant whose preview has the hint `terminal_not_effective`
/// (07 2.12, 9.3). It names no cause: the cause is the server's preview text.
class NotEffectiveBadge extends StatelessWidget {
  /// Creates the badge.
  const NotEffectiveBadge({super.key});

  @override
  Widget build(BuildContext context) {
    final t = dsToneColors(context, DsTone.warning);
    return DsBadge(
      text: AppLocalizations.of(context).hostNotEffective,
      background: t.subtle,
      foreground: t.fg,
    );
  }
}

/// One invariant violation: the server's problem text, the agent and the affected principals,
/// and for I1 the guidance "switch to organization first" (the guided switch is M4-28).
class _Violation extends StatelessWidget {
  const _Violation({required this.finding, required this.names});

  final InvariantFinding finding;
  final AccessNames names;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final t = dsToneColors(context, DsTone.danger);
    final style = dsTextStyle(DsType.bodySm).copyWith(color: t.fg);
    final agent = finding.agentId;
    return Container(
      key: ValueKey('preview.violation.${finding.code}'),
      padding: const EdgeInsets.all(DsSpace.s8),
      decoration: BoxDecoration(
        color: t.subtle,
        border: Border.all(color: t.border, width: DsBorderWidth.hairline),
        borderRadius: BorderRadius.circular(DsRadius.md),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '${l.adminPreviewRejected}: ${problemText(l, finding.code)}',
            style: style.copyWith(fontWeight: FontWeight.w600),
          ),
          if (agent != null)
            Text(l.adminPreviewAgent(names.principal(agent)), style: style),
          if (finding.principalIds.isNotEmpty)
            Text(
              l.adminPreviewAffected(
                finding.principalIds.map(names.principal).join(', '),
              ),
              style: style,
            ),
          if (finding.code == 'i1_violation')
            Text(
              l.adminPreviewI1Hint,
              key: const ValueKey('preview.i1Hint'),
              style: style,
            ),
          if (finding.code == 'i6_violation')
            Text(
              l.hostLlmI6Explained,
              key: const ValueKey('preview.i6Hint'),
              style: style,
            ),
        ],
      ),
    );
  }
}
