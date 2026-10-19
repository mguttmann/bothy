// SPDX-License-Identifier: Apache-2.0

/// A row of the post list: `DsThreadRow` of M4-07 over a [PostItem] (components.md 6).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'forum_model.dart';

/// The row of [item].
///
/// The state icon (active run, else mention), the title bold when unread, the time of the
/// last activity ([timeLabel] with [now]), "Waiting for you", the operator agent, the mention
/// counter, the tags and a failure reason; the label `<Title>, <State>, <Time>, <n> unread`.
/// A click or Enter calls [onOpen] (the router path `/t/<id>`).
class PostRow extends StatelessWidget {
  /// Creates the row.
  const PostRow({
    super.key,
    required this.item,
    required this.tags,
    required this.names,
    required this.now,
    required this.selected,
    required this.onOpen,
  });

  /// The post.
  final PostItem item;

  /// The tags of the forum by ID.
  final Map<ForumTagId, ForumTag> tags;

  /// The display names of agents (the operator).
  final Map<PrincipalId, String> names;

  /// Now, for the time label.
  final DateTime now;

  /// Whether the route shows this post.
  final bool selected;

  /// Opens the post.
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final format = L10nFormat(Localizations.localeOf(context));
    return DsThreadRow(
      item.rowData(
        l,
        tags: tags,
        names: names,
        timeLabel: timeLabel(
          item.thread.lastActivityAt,
          now: now,
          format: format,
        ),
      ),
      key: ValueKey('postList.post.${item.thread.id.value}'),
      selected: selected,
      onOpen: onOpen,
    );
  }
}
