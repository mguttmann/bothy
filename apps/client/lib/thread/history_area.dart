// SPDX-License-Identifier: Apache-2.0

/// What the history column of the shell shows for a location (01 5.1 D1 and D2; brief M4-20
/// Design note 1): a thread, the main history of a text channel, or both side by side when a
/// thread of a text channel is open, with a draggable border and the main history hidden while
/// the person chose so (brief M4-42); "Not found or no access" for an invisible target
/// (01 6.7); the cross-cutting views Search, Mentions and Running now for `/search`,
/// `/mentions` and `/running` (15 11; brief M4-23).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/layout/column_layout.dart';
import '../app/layout/column_widths.dart';
import '../app/providers/settings.dart';
import '../app/providers/target.dart';
import '../app/router.dart';
import '../app/target_resolver.dart';
import '../forum/providers.dart' show channelColumnHiddenProvider;
import '../mentions/mentions_view.dart';
import '../running/run_graph_screen.dart';
import '../running/running_now_view.dart';
import '../search/search_view.dart';
import '../settings/settings_providers.dart';
import '../sidebar/providers.dart';
import '../viz/live_page.dart';
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
          return BesideMainHistory(
            main: history(main, besideMain: true),
            thread: history(threadId, jumpTo: messageId, besideMain: true),
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
      case ViewLocation(view: 'live'):
        // D12: the views of `bothy_viz` with TV mode (brief M6-16).
        return const LivePage();
      case final RunGraphLocation location:
        return RunGraphScreen(
          key: ValueKey('runGraph.${location.runId.value}'),
          location: location,
        );
      default:
        return const SizedBox.expand();
    }
  }
}

/// The main history of a text channel and an open thread of it side by side (01 D2): the
/// border between them can be dragged, and the width of the main history is stored as
/// `main_history` in `layout.column_widths` (without one the two share the room); each keeps at
/// least `DsLayout.columnMin`. While the person hid the channel column (M4-42) the thread takes
/// the whole width; the thread keeps its place in the tree, so its scroll position and input
/// stay when the main history goes or comes.
class BesideMainHistory extends ConsumerStatefulWidget {
  /// Creates the pair.
  const BesideMainHistory({
    super.key,
    required this.main,
    required this.thread,
  });

  /// The main history of the channel.
  final Widget main;

  /// The open thread.
  final Widget thread;

  @override
  ConsumerState<BesideMainHistory> createState() => _BesideMainHistoryState();
}

class _BesideMainHistoryState extends ConsumerState<BesideMainHistory> {
  /// The width while a drag runs and until the stored setting comes back.
  double? _dragging;

  /// The width the last build gave the main history and its bounds.
  double _width = 0;
  double _min = 0;
  double _max = 0;

  void _drag(double dx) {
    final next = (_dragging ?? _width) + dx;
    setState(() => _dragging = next.clamp(_min, _max));
  }

  void _end() {
    final width = _dragging;
    final settings = ref.read(settingsRepositoryProvider);
    if (width == null || settings == null) return;
    unawaited(
      ColumnWidthsWriter(settings).save(
        ref.read(columnWidthsProvider).withWidth(AppColumn.mainHistory, width),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final hidden = ref.watch(channelColumnHiddenProvider).value ?? false;
    final widths = ref.watch(columnWidthsProvider);
    // The dragged width stays until the stored setting comes back, so the border does not
    // jump back while the write travels.
    ref.listen(
      columnWidthsProvider,
      (_, _) => setState(() => _dragging = null),
    );
    return LayoutBuilder(
      builder: (context, constraints) {
        final room = constraints.maxWidth - DsSpace.s4;
        _min = DsLayout.columnMin;
        _max = room - DsLayout.columnMin;
        if (_max < _min) _min = _max = room / 2;
        final wanted = _dragging ?? widths.stored(AppColumn.mainHistory);
        _width = (wanted ?? room / 2).clamp(_min, _max);
        return Row(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (!hidden) ...[
              SizedBox(width: _width, child: widget.main),
              ColumnDragHandle(
                key: const ValueKey('column.handle.${AppColumn.mainHistory}'),
                onDrag: _drag,
                onEnd: _end,
              ),
            ],
            Expanded(
              key: const ValueKey('besideMain.thread'),
              child: widget.thread,
            ),
          ],
        );
      },
    );
  }
}
