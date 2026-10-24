// SPDX-License-Identifier: Apache-2.0

/// What the history column of the shell shows for a location (01 5.1 D1 and D2; brief M4-20
/// Design note 1): a thread, the main history of a text channel, or both side by side when a
/// thread of a text channel is open; "Not found or no access" for an invisible target
/// (01 6.7); the cross-cutting views Search, Mentions and Running now for `/search`,
/// `/mentions` and `/running` (15 11; brief M4-23).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/target.dart';
import '../app/router.dart';
import '../app/target_resolver.dart';
import '../mentions/mentions_view.dart';
import '../running/running_now_view.dart';
import '../search/search_view.dart';
import '../sidebar/providers.dart';
import 'history_column.dart';

/// The history area of [location] (path and query of the router).
///
/// It keeps the main history it showed for the location when the channel's rows go (a
/// removal, 04 8.3), so the columns stay where they are and close with "No access anymore"
/// themselves (01 6.6).
class HistoryArea extends ConsumerStatefulWidget {
  /// Creates the area.
  const HistoryArea({super.key, required this.location});

  /// The location the router shows.
  final Uri location;

  @override
  ConsumerState<HistoryArea> createState() => _HistoryAreaState();
}

class _HistoryAreaState extends ConsumerState<HistoryArea> {
  /// The main history shown for this location, kept while the rows are gone.
  ThreadId? _main;

  @override
  void didUpdateWidget(HistoryArea old) {
    super.didUpdateWidget(old);
    if (old.location != widget.location) _main = null;
  }

  @override
  Widget build(BuildContext context) {
    final location = widget.location;
    final status = ref.watch(targetStatusProvider(location.toString())).value;
    if (status == TargetStatus.notFound) {
      return Center(
        child: Text(
          AppLocalizations.of(context).accessNotFoundOrNoAccess,
          key: const ValueKey('target.notFound'),
          style: dsTextStyle(DsType.body)
              .copyWith(color: context.dsColors.textSecondary),
        ),
      );
    }
    Widget history(
      ThreadId thread, {
      MessageId? jumpTo,
      bool besideMain = false,
    }) => HistoryColumn(
      key: ValueKey('historyColumn.${thread.value}.${jumpTo?.value}'),
      threadId: thread,
      jumpTo: jumpTo,
      besideMain: besideMain,
    );
    switch (parseLocation(location)) {
      case ThreadLocation(:final threadId, :final messageId):
        final thread = ref.watch(storedThreadProvider(threadId.value)).value;
        // Without a stored thread the column stays: it waits for the thread, or shows "No
        // access anymore" after a removal deleted it (01 6.6).
        final channel = thread == null
            ? null
            : ref
                  .watch(storedChannelOfRouteProvider(thread.channelId.value))
                  .value;
        if (thread != null &&
            channel?.kind == ChannelKind.text &&
            thread.kind != ThreadKind.main) {
          _main = channel?.mainThreadId ?? _main;
        }
        final main = _main;
        // D2: a thread of a text channel opens to the right of the main history.
        if (main != null && main != threadId) {
          return Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Expanded(child: history(main, besideMain: true)),
              VerticalDivider(
                width: DsBorderWidth.hairline,
                thickness: DsBorderWidth.hairline,
                color: context.dsColors.borderSubtle,
              ),
              Expanded(
                child: history(threadId, jumpTo: messageId, besideMain: true),
              ),
            ],
          );
        }
        return history(threadId, jumpTo: messageId);
      case ChannelLocation(:final channelId):
        final channel = ref
            .watch(storedChannelOfRouteProvider(channelId.value))
            .value;
        if (channel != null && channel.kind != ChannelKind.forum) {
          _main = channel.mainThreadId ?? _main;
        }
        final main = _main;
        return main == null ? const SizedBox.expand() : history(main);
      case ViewLocation(view: 'search', :final query):
        // A new query is a new view: the route `/search?q=` runs it on open.
        return SearchView(
          key: ValueKey('searchView.$query'),
          initialQuery: query ?? '',
        );
      case ViewLocation(view: 'mentions'):
        return const MentionsView();
      case ViewLocation(view: 'running'):
        return const RunningNowView();
      default:
        return const SizedBox.expand();
    }
  }
}
