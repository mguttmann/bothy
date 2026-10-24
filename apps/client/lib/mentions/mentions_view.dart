// SPDX-License-Identifier: Apache-2.0

/// The view of the person's own mentions (docs/spec/01-product.md 3.5, route `/mentions` of
/// 15 11): the newest mention first, each with author, time, path and the start of the message;
/// selecting one jumps to the message in its thread (`/t/<thread>?m=<message>`, brief M4-20).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../settings/settings_providers.dart' show personTimeZoneProvider;
import 'providers.dart';

/// The mentions view.
///
/// Only what `listMyMentions` returns is shown: the server lists the mentions of channels the
/// person still sees (15 1 principle 1).
class MentionsView extends ConsumerWidget {
  /// Creates the view.
  const MentionsView({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final state = ref.watch(mentionsControllerProvider);
    final page = state.value;
    final Widget body;
    if (page == null && state.hasError) {
      body = Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              l.mentionsFailed,
              key: const ValueKey('mentions.failed'),
              style: dsTextStyle(DsType.body).copyWith(color: c.textSecondary),
            ),
            const SizedBox(height: DsSpace.s8),
            DsButton(
              key: const ValueKey('mentions.retry'),
              label: l.mentionsRetry,
              onPressed: () => ref.invalidate(mentionsControllerProvider),
            ),
          ],
        ),
      );
    } else if (page == null) {
      body = const SizedBox.expand();
    } else if (page.entries.isEmpty) {
      body = Center(
        child: Text(
          l.mentionsEmpty,
          key: const ValueKey('mentions.empty'),
          style: dsTextStyle(DsType.body).copyWith(color: c.textSecondary),
        ),
      );
    } else {
      body = ListView(
        children: [
          for (final entry in page.entries) MentionTile(entry: entry),
          if (page.nextCursor != null)
            Padding(
              padding: const EdgeInsets.all(DsSpace.s12),
              child: Center(
                child: DsButton(
                  key: const ValueKey('mentions.loadMore'),
                  label: l.searchLoadMore,
                  onPressed: state.isLoading
                      ? null
                      : () => ref
                            .read(mentionsControllerProvider.notifier)
                            .loadMore(),
                ),
              ),
            ),
        ],
      );
    }
    return Material(
      key: const ValueKey('mentions.view'),
      color: c.surfaceCanvas,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(
              DsSpace.s16,
              DsSpace.s12,
              DsSpace.s16,
              DsSpace.s8,
            ),
            child: Semantics(
              header: true,
              child: Text(
                l.mentionsTitle,
                style: dsTextStyle(DsType.heading)
                    .copyWith(color: c.textPrimary),
              ),
            ),
          ),
          Divider(
            height: DsBorderWidth.hairline,
            thickness: DsBorderWidth.hairline,
            color: c.borderSubtle,
          ),
          Expanded(child: body),
        ],
      ),
    );
  }
}

/// One mention: path, author and time, and the first lines of the message as plain text.
class MentionTile extends ConsumerWidget {
  /// Creates the tile of [entry].
  const MentionTile({super.key, required this.entry});

  /// The mention.
  final MentionEntry entry;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.dsColors;
    final m = entry.message;
    final zone = ref.watch(personTimeZoneProvider);
    final now = ref.watch(appPortsProvider).clock.now();
    final format = dsFormat(context);
    final time =
        '${format.relativeDay(m.createdAt, now: now, timeZone: zone)} '
        '${format.time(m.createdAt, timeZone: zone)}';
    final path = entry.path.map(dsIsolate).join(' > ');
    return DsTappable(
      key: ValueKey('mentions.entry.${m.id.value}'),
      label: '$path, ${dsIsolate(entry.authorName)}, $time: ${m.contentMd}',
      excludeChildSemantics: true,
      onTap: () => context.go('/t/${m.threadId.value}?m=${m.id.value}'),
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s16,
          vertical: DsSpace.s8,
        ),
        decoration: BoxDecoration(
          border: Border(
            bottom: BorderSide(
              color: c.borderSubtle,
              width: DsBorderWidth.hairline,
            ),
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              path,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: dsTextStyle(DsType.caption)
                  .copyWith(color: c.textSecondary),
            ),
            const SizedBox(height: DsSpace.s2),
            Text(
              '${dsIsolate(entry.authorName)}  $time',
              style: dsTextStyle(DsType.label).copyWith(color: c.textPrimary),
            ),
            const SizedBox(height: DsSpace.s2),
            // Agent and person content stays text: no Markdown or HTML is interpreted here.
            Text(
              m.contentMd,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: dsTextStyle(DsType.body).copyWith(color: c.textPrimary),
            ),
          ],
        ),
      ),
    );
  }
}
