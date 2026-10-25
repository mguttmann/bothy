// SPDX-License-Identifier: Apache-2.0

/// The search in the open thread (docs/spec/01-product.md 7 "Search in thread", Cmd+F or
/// Ctrl+F; brief M4-24): a dialog over the history with a search field and the hits of the
/// in-thread search of M4-23 (`searchQueryControllerProvider` with the channel and the
/// thread, which sends both with every request and searches the local index offline).
/// Choosing a hit closes the dialog and jumps to the message (`/t/<thread>?m=<message>`).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import 'providers.dart';
import 'search_controller.dart';
import 'search_port.dart' show SearchResultItem;
import 'search_view.dart' show SearchHitTile;

/// Opens the search in [threadId] of [channelId] over [context].
///
/// Example: `showThreadSearch(context, channelId: c, threadId: t)`.
Future<void> showThreadSearch(
  BuildContext context, {
  required ChannelId channelId,
  required ThreadId threadId,
}) {
  final router = GoRouter.of(context);
  return showDialog<void>(
    context: context,
    builder: (dialog) => Dialog(
      key: const ValueKey('threadSearch.dialog'),
      child: ConstrainedBox(
        constraints: const BoxConstraints(
          maxWidth: DsLayout.columnMin * 3,
          maxHeight: DsLayout.columnMin * 3,
        ),
        child: ThreadSearch(
          channelId: channelId,
          threadId: threadId,
          onOpen: (path) {
            Navigator.of(dialog).pop();
            router.go(path);
          },
        ),
      ),
    ),
  );
}

/// The field and the hits of the in-thread search.
class ThreadSearch extends ConsumerStatefulWidget {
  /// Creates the search of [threadId] in [channelId]; [onOpen] gets the path of a chosen hit.
  const ThreadSearch({
    super.key,
    required this.channelId,
    required this.threadId,
    required this.onOpen,
  });

  /// The channel of the thread.
  final ChannelId channelId;

  /// The thread.
  final ThreadId threadId;

  /// Opens the path of a hit.
  final ValueChanged<String> onOpen;

  @override
  ConsumerState<ThreadSearch> createState() => _ThreadSearchState();
}

class _ThreadSearchState extends ConsumerState<ThreadSearch> {
  final TextEditingController _text = TextEditingController();

  SearchQueryControllerProvider get _provider => searchQueryControllerProvider(
    channelId: widget.channelId.value,
    threadId: widget.threadId.value,
  );

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  void _search() {
    final search = ref.read(_provider.notifier)..setInput(_text.text);
    unawaited(search.submit());
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final state = ref.watch(_provider);
    final error = state.error;
    return Padding(
      padding: const EdgeInsets.all(DsSpace.s16),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            l.threadSearchTitle,
            style: dsTextStyle(DsType.heading).copyWith(color: c.textPrimary),
          ),
          const SizedBox(height: DsSpace.s8),
          TextField(
            key: const ValueKey('threadSearch.field'),
            controller: _text,
            autofocus: true,
            textInputAction: TextInputAction.search,
            decoration: InputDecoration(labelText: l.searchFieldLabel),
            onSubmitted: (_) => _search(),
          ),
          if (error != null)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s8),
              child: Text(
                switch (error) {
                  SearchError.queryEmpty => l.searchQueryEmpty,
                  SearchError.queryTooLong => l.searchQueryTooLong,
                  SearchError.failed => l.searchFailed,
                },
                key: const ValueKey('threadSearch.error'),
                style: dsTextStyle(DsType.bodySm).copyWith(color: c.dangerFg),
              ),
            ),
          if (state.showLocalNote)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s8),
              child: DsBanner(tone: DsTone.info, text: l.searchLocalOnly),
            ),
          if (state.submitted && error == null && !state.pending)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s8),
              child: Text(
                state.items.isEmpty
                    ? l.searchNoResults
                    : l.searchResultCount(state.items.length),
                key: const ValueKey('threadSearch.count'),
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: c.textSecondary),
              ),
            ),
          Flexible(
            child: ListView(
              shrinkWrap: true,
              children: [
                for (final hit in state.items)
                  _Hit(item: hit, onOpen: widget.onOpen),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// One hit: the tile of the search view, opened through the dialog so it closes first.
class _Hit extends StatelessWidget {
  const _Hit({required this.item, required this.onOpen});

  final SearchResultItem item;
  final ValueChanged<String> onOpen;

  @override
  Widget build(BuildContext context) {
    final target = SearchHitTile.targetOf(item);
    return SearchHitTile(
      key: ValueKey('threadSearch.hit.${item.id}'),
      item: item,
      onOpen: target == null ? null : () => onOpen(target),
    );
  }
}
