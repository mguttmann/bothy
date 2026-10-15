// SPDX-License-Identifier: Apache-2.0

/// The suggestion list of the input (components.md section 9).
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import '../theme/ds_theme.dart';
import 'ds_common.dart';
import 'view_models.dart';

/// The list of command and mention suggestions above the input.
///
/// An overlay on `color.surface.overlay` with `elevation.menu`; rows in
/// `density.row-suggestion` show the command in mono, the description, the origin agent and
/// the hint "Control command" for a control command. A listbox for screen readers: the row at
/// [activeIndex] is selected and the number of suggestions is a polite live region, so it is
/// announced when it changes. The list does not filter or throttle; the caller passes the
/// [suggestions] in their order. A tap on a row calls [onAccept].
///
/// ```dart
/// DsSuggestionList(suggestions, activeIndex: active, onAccept: insert)
/// ```
class DsSuggestionList extends StatelessWidget {
  /// Creates the list.
  const DsSuggestionList(
    this.suggestions, {
    super.key,
    required this.activeIndex,
    required this.onAccept,
  });

  /// The suggestions in display order.
  final List<DsSuggestion> suggestions;

  /// The index of the active row.
  final int activeIndex;

  /// Called with the accepted suggestion.
  final ValueChanged<DsSuggestion> onAccept;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final secondary = dsTextStyle(DsType.caption)
        .copyWith(color: c.textSecondary);
    return DecoratedBox(
      decoration: BoxDecoration(
        color: c.surfaceOverlay,
        boxShadow: context.dsElevation.menu,
        borderRadius: BorderRadius.circular(DsRadius.md),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Semantics(
            container: true,
            liveRegion: true,
            label: l.composerSuggestions(suggestions.length),
            child: const SizedBox.shrink(),
          ),
          for (var i = 0; i < suggestions.length; i++)
            DsTappable(
              key: ValueKey('ds-suggestion-${suggestions[i].id}'),
              onTap: () => onAccept(suggestions[i]),
              selected: i == activeIndex,
              button: false,
              label: [
                suggestions[i].command,
                if (suggestions[i].description != null)
                  suggestions[i].description!,
                if (suggestions[i].originName != null)
                  suggestions[i].originName!,
                if (suggestions[i].control) l.composerControlCommand,
              ].join(', '),
              excludeChildSemantics: true,
              child: Container(
                constraints: BoxConstraints(
                  minHeight: context.dsDensity.rowSuggestion,
                ),
                color: i == activeIndex ? c.surfaceSelected : null,
                padding: const EdgeInsets.symmetric(horizontal: DsSpace.s12),
                child: Row(
                  children: [
                    Text(
                      suggestions[i].command,
                      style: dsTextStyle(DsType.codeSm)
                          .copyWith(color: c.textPrimary),
                    ),
                    const SizedBox(width: DsSpace.s8),
                    Expanded(
                      child: Text(
                        suggestions[i].description ?? '',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: secondary,
                      ),
                    ),
                    if (suggestions[i].originName != null) ...[
                      const SizedBox(width: DsSpace.s8),
                      Text(
                        dsIsolate(suggestions[i].originName!),
                        style: secondary,
                      ),
                    ],
                    if (suggestions[i].control) ...[
                      const SizedBox(width: DsSpace.s8),
                      DsBadge(
                        text: l.composerControlCommand,
                        background: c.warningSubtle,
                        foreground: c.warningFg,
                      ),
                    ],
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}
