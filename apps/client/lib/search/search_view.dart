// SPDX-License-Identifier: Apache-2.0

/// The search view D7 (docs/spec/01-product.md 5.1 D7, route `/search?q=` of
/// docs/spec/15-client-architecture.md 11, brief M4-23 Design note 1): the filter column on the
/// left, the search field with its button, the count line of the loaded hits, the note for
/// local hits (14 6), and the hits with path, author, time and the highlighted excerpt (14 4.6).
/// Selecting a message opens it in its thread (`/t/<thread>?m=<message>`, brief M4-20).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../settings/settings_providers.dart' show personTimeZoneProvider;
import 'providers.dart';
import 'search_controller.dart';
import 'search_port.dart';
import 'segments_text.dart';

/// The width of the filter column of D7.
const double searchFilterColumnWidth = DsSpace.s64 * 4;

/// The search view.
///
/// Only what the server returns, or offline what the local index holds, is shown: the count
/// line counts the loaded hits and nothing else (01 D7, 14 4.3).
class SearchView extends ConsumerStatefulWidget {
  /// Creates the view; a non-empty [initialQuery] (the `q` of the route) runs on open.
  const SearchView({super.key, required this.initialQuery});

  /// The text the view searches when it opens.
  final String initialQuery;

  @override
  ConsumerState<SearchView> createState() => _SearchViewState();
}

class _SearchViewState extends ConsumerState<SearchView> {
  late final TextEditingController _text = TextEditingController(
    text: widget.initialQuery,
  );

  @override
  void initState() {
    super.initState();
    _searchInitial();
  }

  @override
  void didUpdateWidget(SearchView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.initialQuery != oldWidget.initialQuery) {
      _text.text = widget.initialQuery;
      _searchInitial();
    }
  }

  void _searchInitial() {
    if (widget.initialQuery.trim().isEmpty) return;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _search();
    });
  }

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  void _search() {
    final search = ref.read(searchQueryControllerProvider().notifier)
      ..setInput(_text.text);
    unawaited(search.submit());
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final state = ref.watch(searchQueryControllerProvider());
    return Material(
      key: const ValueKey('search.view'),
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
                l.searchTitle,
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
          Expanded(
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                SizedBox(
                  width: searchFilterColumnWidth,
                  child: SearchFilterColumn(filters: state.filters),
                ),
                VerticalDivider(
                  width: DsBorderWidth.hairline,
                  thickness: DsBorderWidth.hairline,
                  color: c.borderSubtle,
                ),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Padding(
                        padding: const EdgeInsets.all(DsSpace.s12),
                        child: Row(
                          children: [
                            Expanded(
                              child: TextField(
                                key: const ValueKey('search.field'),
                                controller: _text,
                                textInputAction: TextInputAction.search,
                                onSubmitted: (_) => _search(),
                                style: dsTextStyle(DsType.body)
                                    .copyWith(color: c.textPrimary),
                                decoration: InputDecoration(
                                  labelText: l.searchFieldLabel,
                                  hintText: l.searchFieldHint,
                                  prefixIcon: const Icon(Icons.search),
                                  isDense: true,
                                ),
                              ),
                            ),
                            const SizedBox(width: DsSpace.s8),
                            DsButton(
                              key: const ValueKey('search.submit'),
                              label: l.searchSubmit,
                              style: DsButtonStyle.primary,
                              onPressed: _search,
                            ),
                          ],
                        ),
                      ),
                      Expanded(child: SearchResults(state: state)),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// The filter column of D7: space, channel, from, type, time, status, "with files only" and
/// the sort. A filter outside the chosen type (14 4.4) is disabled, because the request leaves
/// it out.
class SearchFilterColumn extends ConsumerWidget {
  /// Creates the column showing [filters].
  const SearchFilterColumn({super.key, required this.filters});

  /// The current filters.
  final SearchFilters filters;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final spaces = ref.watch(searchSpacesProvider).value ?? const <Space>[];
    final channels = ref.watch(searchChannelsProvider);
    final principals =
        ref.watch(searchPrincipalsProvider).value ?? const <Principal>[];
    final search = ref.read(searchQueryControllerProvider().notifier);
    void set(SearchFilters next) => unawaited(search.setFilters(next));
    final f = filters;
    final type = f.type;
    final shownChannels = [
      for (final ch in channels)
        if (f.spaceId == null || ch.spaceId == f.spaceId) ch,
    ];
    return ListView(
      key: const ValueKey('search.filters'),
      padding: const EdgeInsets.all(DsSpace.s12),
      children: [
        Semantics(
          header: true,
          child: Text(
            l.searchFilters,
            style: dsTextStyle(DsType.section).copyWith(color: c.textSecondary),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        _Filter<SpaceId?>(
          name: 'space',
          label: l.searchFilterSpace,
          value: spaces.any((s) => s.id == f.spaceId) ? f.spaceId : null,
          options: [
            (null, l.searchFilterAll),
            for (final s in spaces) (s.id, s.name),
          ],
          onChanged: (v) => set(
            v == null
                ? f.copyWith(clearSpace: true)
                : f.copyWith(
                    spaceId: v,
                    clearChannel: !channels.any(
                      (ch) => ch.id == f.channelId && ch.spaceId == v,
                    ),
                  ),
          ),
        ),
        _Filter<ChannelId?>(
          name: 'channel',
          label: l.searchFilterChannel,
          value: shownChannels.any((ch) => ch.id == f.channelId)
              ? f.channelId
              : null,
          options: [
            (null, l.searchFilterAll),
            for (final ch in shownChannels) (ch.id, ch.name),
          ],
          onChanged: (v) => set(
            v == null
                ? f.copyWith(clearChannel: true)
                : f.copyWith(channelId: v),
          ),
        ),
        _Filter<PrincipalId?>(
          name: 'from',
          label: l.searchFilterFrom,
          value: principals.any((p) => p.id == f.authorId) ? f.authorId : null,
          options: [
            (null, l.searchFilterAll),
            for (final p in principals) (p.id, p.displayName),
          ],
          // author_id applies to messages and files (14 4.4).
          onChanged: type == SearchType.threads
              ? null
              : (v) => set(
                  v == null
                      ? f.copyWith(clearAuthor: true)
                      : f.copyWith(authorId: v),
                ),
        ),
        _Filter<SearchType>(
          name: 'type',
          label: l.searchFilterType,
          value: type,
          options: [
            (SearchType.messages, l.searchTypeMessages),
            (SearchType.threads, l.searchTypeThreads),
            (SearchType.files, l.searchTypeFiles),
          ],
          onChanged: (v) => set(f.copyWith(type: v)),
        ),
        _Filter<SearchTime>(
          name: 'time',
          label: l.searchFilterTime,
          value: f.time,
          options: [
            (SearchTime.any, l.searchTimeAny),
            (SearchTime.day, l.searchTimeDay),
            (SearchTime.week, l.searchTimeWeek),
            (SearchTime.month, l.searchTimeMonth),
          ],
          onChanged: (v) => set(f.copyWith(time: v)),
        ),
        _Filter<SearchStatus>(
          name: 'status',
          label: l.searchFilterStatus,
          value: f.status,
          options: [
            (SearchStatus.all, l.searchStatusAll),
            (SearchStatus.open, l.searchStatusOpen),
            (SearchStatus.done, l.searchStatusDone),
          ],
          // thread_state applies to threads and messages (14 4.4).
          onChanged: type == SearchType.files
              ? null
              : (v) => set(f.copyWith(status: v)),
        ),
        CheckboxListTile(
          key: const ValueKey('search.filter.hasFile'),
          value: f.hasFile,
          dense: true,
          contentPadding: EdgeInsets.zero,
          controlAffinity: ListTileControlAffinity.leading,
          title: Text(
            l.searchHasFile,
            style: dsTextStyle(DsType.bodySm).copyWith(color: c.textPrimary),
          ),
          // has_file applies to messages (14 4.4).
          onChanged: type == SearchType.messages
              ? (v) => set(f.copyWith(hasFile: v ?? false))
              : null,
        ),
        _Filter<SearchSort>(
          name: 'sort',
          label: l.searchSort,
          value: f.sort,
          options: [
            (SearchSort.relevance, l.searchSortRelevance),
            (SearchSort.newest, l.searchSortNewest),
          ],
          onChanged: (v) => set(f.copyWith(sort: v)),
        ),
      ],
    );
  }
}

/// One menu of the filter column with its label; [onChanged] null disables it.
class _Filter<T> extends StatelessWidget {
  const _Filter({
    super.key,
    required this.name,
    required this.label,
    required this.value,
    required this.options,
    required this.onChanged,
  });

  final String name;
  final String label;
  final T value;
  final List<(T, String)> options;
  final ValueChanged<T>? onChanged;

  @override
  Widget build(BuildContext context) {
    final c = context.dsColors;
    final changed = onChanged;
    return Padding(
      padding: const EdgeInsets.only(bottom: DsSpace.s8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            label,
            style: dsTextStyle(DsType.label).copyWith(color: c.textSecondary),
          ),
          DropdownButton<T>(
            key: ValueKey('search.filter.$name'),
            value: value,
            isExpanded: true,
            items: [
              for (final (v, text) in options)
                DropdownMenuItem<T>(
                  value: v,
                  child: Text(
                    text,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: c.textPrimary),
                  ),
                ),
            ],
            onChanged: changed == null
                ? null
                : (v) {
                    if (v is T) changed(v);
                  },
          ),
        ],
      ),
    );
  }
}

/// The result side of D7: the reason of a failed search, the notes, the count line, the hits
/// and "Load more".
class SearchResults extends ConsumerWidget {
  /// Creates the results of [state].
  const SearchResults({super.key, required this.state});

  /// The state of the search.
  final SearchState state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final error = state.error;
    final note = dsTextStyle(DsType.bodySm).copyWith(color: c.textSecondary);
    final hits = state.items;
    final search = ref.read(searchQueryControllerProvider().notifier);
    return ListView(
      key: const ValueKey('search.results'),
      padding: const EdgeInsets.only(bottom: DsSpace.s16),
      children: [
        if (error != null)
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: DsSpace.s12,
              vertical: DsSpace.s4,
            ),
            child: Text(
              switch (error) {
                SearchError.queryEmpty => l.searchQueryEmpty,
                SearchError.queryTooLong => l.searchQueryTooLong,
                SearchError.failed => l.searchFailed,
              },
              key: const ValueKey('search.error'),
              style: dsTextStyle(DsType.bodySm).copyWith(color: c.dangerFg),
            ),
          ),
        if (state.showLocalNote)
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: DsSpace.s12,
              vertical: DsSpace.s4,
            ),
            child: DsBanner(
              key: const ValueKey('search.localNote'),
              tone: DsTone.info,
              text: l.searchLocalOnly,
            ),
          ),
        if (state.candidatesCapped)
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: DsSpace.s12,
              vertical: DsSpace.s4,
            ),
            child: DsBanner(
              key: const ValueKey('search.capped'),
              tone: DsTone.warning,
              text: l.searchCandidatesCapped,
            ),
          ),
        if (state.submitted && error == null && !state.pending)
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: DsSpace.s12,
              vertical: DsSpace.s4,
            ),
            child: Text(
              hits.isEmpty
                  ? l.searchNoResults
                  : l.searchResultCount(hits.length),
              key: const ValueKey('search.count'),
              style: note,
            ),
          ),
        for (final hit in hits) SearchHitTile(item: hit),
        if (state.nextCursor != null && !state.localOnly)
          Padding(
            padding: const EdgeInsets.all(DsSpace.s12),
            child: Center(
              child: DsButton(
                key: const ValueKey('search.loadMore'),
                label: l.searchLoadMore,
                onPressed: state.loadingMore
                    ? null
                    : () => unawaited(search.loadMore()),
              ),
            ),
          ),
      ],
    );
  }
}

/// One hit: kind and path, author and time, the file name, and the excerpt as plain text with
/// marked matches (14 4.6).
class SearchHitTile extends ConsumerWidget {
  /// Creates the tile of [item]; [onOpen] replaces the jump to [targetOf] (the in-thread
  /// search closes its dialog first, M4-24).
  const SearchHitTile({super.key, required this.item, this.onOpen});

  /// The hit.
  final SearchResultItem item;

  /// Opens the hit instead of the router jump, or null for the jump.
  final VoidCallback? onOpen;

  /// Where selecting [hit] leads: a message to its place in the thread, a thread or a
  /// transcript to the thread, a server file hit to its channel, because the API names no
  /// message for a file (docs/decisions.d/M4-23.md); null when nothing is known.
  static String? targetOf(SearchResultItem hit) {
    final thread = hit.threadId;
    final message = hit.messageId;
    final channel = hit.channelId;
    if (thread != null && message != null) {
      return '/t/${thread.value}?m=${message.value}';
    }
    if (thread != null) return '/t/${thread.value}';
    if (channel != null) return '/c/${channel.value}';
    return null;
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final principals =
        ref.watch(searchPrincipalsProvider).value ?? const <Principal>[];
    final zone = ref.watch(personTimeZoneProvider);
    final now = ref.watch(appPortsProvider).clock.now();
    final format = dsFormat(context);
    final at = item.at;
    final time = at == null
        ? ''
        : '${format.relativeDay(at, now: now, timeZone: zone)} '
              '${format.time(at, timeZone: zone)}';
    String? stored(PrincipalId? id) {
      for (final p in principals) {
        if (p.id == id) return p.displayName;
      }
      return null;
    }

    final author = item.authorName ?? stored(item.authorId) ?? '';
    final kind = switch (item.type) {
      SearchHitType.message => null,
      SearchHitType.thread => l.searchHitThread,
      SearchHitType.file => l.searchHitFile,
      SearchHitType.transcript => l.searchHitTranscript,
    };
    final path = item.path.map(dsIsolate).join(' > ');
    final excerpt = item.segments.map((s) => s.text).join();
    final title = item.title;
    final target = targetOf(item);
    return DsTappable(
      key: ValueKey('search.hit.${item.id}'),
      label: l.searchHitLabel(
        [?kind, path, ?title].join(', '),
        dsIsolate(author),
        time,
        excerpt,
      ),
      excludeChildSemantics: true,
      onTap: target == null ? null : onOpen ?? () => context.go(target),
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
              [?kind, path].join('  '),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: dsTextStyle(DsType.caption)
                  .copyWith(color: c.textSecondary),
            ),
            const SizedBox(height: DsSpace.s2),
            Text(
              [
                if (author.isNotEmpty) dsIsolate(author),
                if (time.isNotEmpty) time,
              ].join('  '),
              style: dsTextStyle(DsType.label).copyWith(color: c.textPrimary),
            ),
            if (title != null) ...[
              const SizedBox(height: DsSpace.s2),
              Text(
                dsIsolate(title),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: dsTextStyle(DsType.bodyStrong)
                    .copyWith(color: c.textPrimary),
              ),
            ],
            const SizedBox(height: DsSpace.s2),
            SegmentsText(segments: item.segments, maxLines: 3),
          ],
        ),
      ),
    );
  }
}
