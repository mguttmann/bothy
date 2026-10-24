// SPDX-License-Identifier: Apache-2.0

/// The state and the logic of the search view D7 (docs/spec/01-product.md 5.1 D7, brief M4-23
/// Design note 1, docs/spec/14-search.md 4 and 6): the typed text and the filter column, the
/// merge of server and local hits, paging with the opaque cursor and the reasons of a failed
/// search. The riverpod controller of `providers.dart` holds one [SearchSession].
library;

import 'package:domain/domain.dart';

import 'query_notation.dart';
import 'search_port.dart';

/// The time filter of the filter column; the window ends now.
enum SearchTime {
  /// No time filter.
  any,

  /// The last 24 hours.
  day,

  /// The last 7 days.
  week,

  /// The last 30 days.
  month,
}

/// The thread state filter of the filter column (`thread_state`, 14 4.4).
enum SearchStatus {
  /// Open and done threads.
  all,

  /// Open threads only.
  open,

  /// Done (closed) threads only.
  done,
}

/// The filter column of D7.
final class SearchFilters {
  /// Creates the filters; every filter is off by default.
  const SearchFilters({
    this.type = SearchType.messages,
    this.spaceId,
    this.channelId,
    this.authorId,
    this.time = SearchTime.any,
    this.status = SearchStatus.all,
    this.hasFile = false,
    this.sort = SearchSort.relevance,
  });

  /// The result type.
  final SearchType type;

  /// The space, or null for all.
  final SpaceId? spaceId;

  /// The channel, or null for all.
  final ChannelId? channelId;

  /// The author ("From"), or null for all.
  final PrincipalId? authorId;

  /// The time window.
  final SearchTime time;

  /// The thread state.
  final SearchStatus status;

  /// Only messages with attachments.
  final bool hasFile;

  /// The order.
  final SearchSort sort;

  /// A copy with the given fields replaced; the `clear*` flags reset a filter to "all".
  SearchFilters copyWith({
    SearchType? type,
    SpaceId? spaceId,
    bool clearSpace = false,
    ChannelId? channelId,
    bool clearChannel = false,
    PrincipalId? authorId,
    bool clearAuthor = false,
    SearchTime? time,
    SearchStatus? status,
    bool? hasFile,
    SearchSort? sort,
  }) => SearchFilters(
    type: type ?? this.type,
    spaceId: clearSpace ? null : spaceId ?? this.spaceId,
    channelId: clearChannel ? null : channelId ?? this.channelId,
    authorId: clearAuthor ? null : authorId ?? this.authorId,
    time: time ?? this.time,
    status: status ?? this.status,
    hasFile: hasFile ?? this.hasFile,
    sort: sort ?? this.sort,
  );
}

/// The reason a search shows instead of hits.
enum SearchError {
  /// `400 validation_failed` on `q` with `empty`: `searchQueryEmpty`.
  queryEmpty,

  /// `400 validation_failed` on `q` with `too_long`: `searchQueryTooLong`.
  queryTooLong,

  /// Any other failure: `searchFailed`.
  failed,
}

/// What the search view shows.
final class SearchState {
  /// Creates the state.
  const SearchState({
    this.input = '',
    this.filters = const SearchFilters(),
    this.submitted = false,
    this.items = const [],
    this.localOnly = false,
    this.offline = false,
    this.pending = false,
    this.loadingMore = false,
    this.nextCursor,
    this.candidatesCapped = false,
    this.error,
  });

  /// The typed text, notation included.
  final String input;

  /// The filter column.
  final SearchFilters filters;

  /// Whether a search was started; before that the view shows no result line.
  final bool submitted;

  /// The loaded hits in their order.
  final List<SearchResultItem> items;

  /// Whether [items] come from the local index (14 6) instead of the server.
  final bool localOnly;

  /// Whether the server cannot be reached: the connection is not live or the request failed
  /// with a connection error.
  final bool offline;

  /// Whether the first page of the server is pending.
  final bool pending;

  /// Whether a further page is loading.
  final bool loadingMore;

  /// The opaque cursor of the next server page, or null on the last page (14 4.7).
  final String? nextCursor;

  /// Whether the server reported `candidates_capped` (14 4.5).
  final bool candidatesCapped;

  /// Why no hits are shown, or null.
  final SearchError? error;

  /// Whether the view shows the note "Messages stored only on this device" (14 6): local hits
  /// are shown, while the server is pending or because it cannot be reached; offline also
  /// without a local hit, so the empty list is not read as the server's answer.
  bool get showLocalNote =>
      submitted && localOnly && (items.isNotEmpty || offline);

  /// A copy with the given fields replaced; [clearCursor] and [clearError] reset those.
  SearchState copyWith({
    String? input,
    SearchFilters? filters,
    bool? submitted,
    List<SearchResultItem>? items,
    bool? localOnly,
    bool? offline,
    bool? pending,
    bool? loadingMore,
    String? nextCursor,
    bool clearCursor = false,
    bool? candidatesCapped,
    SearchError? error,
    bool clearError = false,
  }) => SearchState(
    input: input ?? this.input,
    filters: filters ?? this.filters,
    submitted: submitted ?? this.submitted,
    items: items ?? this.items,
    localOnly: localOnly ?? this.localOnly,
    offline: offline ?? this.offline,
    pending: pending ?? this.pending,
    loadingMore: loadingMore ?? this.loadingMore,
    nextCursor: clearCursor ? null : nextCursor ?? this.nextCursor,
    candidatesCapped: candidatesCapped ?? this.candidatesCapped,
    error: clearError ? null : error ?? this.error,
  );
}

/// One search of the view: runs the server and the local query of a submit and decides what
/// is shown (brief M4-23 Design note 1, docs/decisions.d/M4-23.md).
///
/// Merge rule: on submit the server query starts, and at the same time the local query; the
/// local hits are shown, marked as local, while the server is pending or when it cannot be
/// reached; the server's first page replaces them (14 6). Offline means the connection is not
/// live ([live] is false, no server request is sent) or the request fails with a connection
/// error; going live again reruns the server query ([reconnected]). A newer submit makes every
/// answer of an older one void. `400 cursor_invalid` restarts at page one; the reasons of
/// `400 validation_failed` on `q` and every other failure replace the hits.
///
/// [scopeChannel] and [scopeThread] make it the in-thread search of M4-24: every request then
/// carries `channel_id` and `thread_id`, whatever the notation or the filter column says.
///
/// Example:
/// ```dart
/// final session = SearchSession(server: port, local: local, ..., emit: (s) => state = s);
/// session.setInput('from:sam deploy');
/// await session.submit();
/// ```
final class SearchSession {
  /// Creates the session; [emit] receives every new state.
  ///
  /// [principals] and [channels] resolve the notation; [now] is the end of the time window;
  /// [localLimit] is the most local hits (`ui.quick_switch_max_results` of `AppConfig`); a
  /// null [server] or [local] port (signed out) is treated as offline or as no local hits.
  SearchSession({
    required this.server,
    required this.local,
    required this.principals,
    required this.channels,
    required this.now,
    required this.live,
    required this.localLimit,
    required this.emit,
    this.scopeChannel,
    this.scopeThread,
  });

  /// The server search, or null.
  final SearchPort? server;

  /// The offline search, or null.
  final LocalSearchPort? local;

  /// The stored principals for `from:`.
  final List<Principal> Function() principals;

  /// The stored channels for `in:`.
  final List<Channel> Function() channels;

  /// The current time.
  final DateTime Function() now;

  /// Whether the connection is live.
  final bool Function() live;

  /// The most local hits.
  final int localLimit;

  /// Receives each new state.
  final void Function(SearchState state) emit;

  /// The channel of the in-thread search, or null.
  final ChannelId? scopeChannel;

  /// The thread of the in-thread search, or null.
  final ThreadId? scopeThread;

  SearchState _state = const SearchState();
  SearchRequest? _request;
  int _generation = 0;
  bool _disposed = false;

  /// The current state.
  SearchState get state => _state;

  /// The request of the last submit, or null before the first one.
  SearchRequest? get request => _request;

  void _set(SearchState next) {
    if (_disposed) return;
    _state = next;
    emit(next);
  }

  /// Stores the typed text; the search runs on [submit].
  void setInput(String input) => _set(_state.copyWith(input: input));

  /// Stores the filter column and, after a first submit, searches again with it.
  Future<void> setFilters(SearchFilters filters) async {
    _set(_state.copyWith(filters: filters));
    if (_state.submitted) await submit();
  }

  /// Starts a search of the typed text with the filters: the first server page and the local
  /// query at the same time. Returns when both have answered or were replaced by a newer
  /// submit.
  Future<void> submit() async {
    final generation = ++_generation;
    final request = buildRequest();
    _request = request;
    final offline = server == null || !live();
    _set(
      SearchState(
        input: _state.input,
        filters: _state.filters,
        submitted: true,
        localOnly: true,
        offline: offline,
        pending: !offline,
      ),
    );
    final localDone = _runLocal(generation, request);
    if (!offline) await _firstPage(generation, request);
    await localDone;
  }

  /// Loads the next server page; does nothing without a cursor, while a page loads or for a
  /// list of local hits.
  Future<void> loadMore() async {
    final cursor = _state.nextCursor;
    final request = _request;
    final port = server;
    if (cursor == null ||
        request == null ||
        port == null ||
        _state.loadingMore ||
        _state.pending ||
        _state.localOnly) {
      return;
    }
    final generation = _generation;
    _set(_state.copyWith(loadingMore: true));
    try {
      final page = await port.search(request, cursor: cursor);
      if (generation != _generation) return;
      _set(
        _state.copyWith(
          items: [..._state.items, ...page.items],
          loadingMore: false,
          nextCursor: page.nextCursor,
          clearCursor: page.nextCursor == null,
          candidatesCapped: _state.candidatesCapped || page.candidatesCapped,
        ),
      );
    } on SearchFailure catch (f) {
      if (generation != _generation) return;
      if (f.kind == SearchFailureKind.cursorInvalid) {
        // 14 4.7: the cursor no longer fits the request; the search starts at page one.
        _set(
          _state.copyWith(
            items: const [],
            loadingMore: false,
            pending: true,
            clearCursor: true,
          ),
        );
        await _firstPage(generation, request);
        return;
      }
      _set(
        _state.copyWith(
          loadingMore: false,
          offline: f.kind == SearchFailureKind.offline,
          error: f.kind == SearchFailureKind.offline
              ? null
              : SearchError.failed,
        ),
      );
    }
  }

  /// Called when the connection becomes live: an offline search asks the server again.
  Future<void> reconnected() async {
    final request = _request;
    if (!_state.submitted ||
        !_state.offline ||
        request == null ||
        server == null) {
      return;
    }
    final generation = _generation;
    _set(_state.copyWith(offline: false, pending: true));
    await _firstPage(generation, request);
  }

  /// Stops emitting states; answers that arrive later are dropped.
  void dispose() => _disposed = true;

  /// The request of the typed text and the filter column (docs/decisions.d/M4-23.md): the
  /// notation wins over the filter column for the same parameter; a filter outside its types
  /// (14 4.4: `author_id` for messages and files, `has_file` for messages, `thread_state` for
  /// threads and messages) is left out, because the server answers it with `not_applicable`;
  /// the in-thread scope replaces every place filter.
  SearchRequest buildRequest() {
    final f = _state.filters;
    final n = parseQueryNotation(
      _state.input,
      principals: principals(),
      channels: channels(),
    );
    final type = f.type;
    final scoped = scopeChannel != null || scopeThread != null;
    final window = switch (f.time) {
      SearchTime.any => null,
      SearchTime.day => const Duration(days: 1),
      SearchTime.week => const Duration(days: 7),
      SearchTime.month => const Duration(days: 30),
    };
    final channel = f.channelId;
    final space = f.spaceId;
    return SearchRequest(
      q: n.text,
      type: type,
      sort: f.sort,
      spaceIds: scoped || space == null ? const [] : [space],
      channelIds: scoped
          ? [?scopeChannel]
          : n.channelIds.isNotEmpty
          ? n.channelIds
          : [?channel],
      threadId: scopeThread,
      authorId: type == SearchType.threads ? null : n.authorId ?? f.authorId,
      after: n.after ?? (window == null ? null : now().subtract(window)),
      before: n.before,
      hasFile: type == SearchType.messages
          ? n.hasFile ?? (f.hasFile ? true : null)
          : null,
      threadState: type == SearchType.files
          ? null
          : switch (f.status) {
              SearchStatus.all => null,
              SearchStatus.open => ThreadState.open,
              SearchStatus.done => ThreadState.closed,
            },
    );
  }

  Future<void> _runLocal(int generation, SearchRequest request) async {
    final port = local;
    if (port == null) return;
    final List<SearchResultItem> hits;
    try {
      hits = await port.search(request, limit: localLimit);
    } on Object {
      // The local index is a supplement (14 6); a failure there must not hide the server.
      return;
    }
    // Only while the server has not answered this search: its hits always win.
    if (generation != _generation || !_state.localOnly) return;
    _set(_state.copyWith(items: hits));
  }

  Future<void> _firstPage(int generation, SearchRequest request) async {
    final port = server;
    if (port == null) return;
    try {
      final page = await port.search(request);
      if (generation != _generation) return;
      _set(
        _state.copyWith(
          items: page.items,
          localOnly: false,
          offline: false,
          pending: false,
          nextCursor: page.nextCursor,
          clearCursor: page.nextCursor == null,
          candidatesCapped: page.candidatesCapped,
          clearError: true,
        ),
      );
    } on SearchFailure catch (f) {
      if (generation != _generation) return;
      _set(switch (f.kind) {
        // 14 6: without the server the local hits stay, with the note.
        SearchFailureKind.offline => _state.copyWith(
          offline: true,
          pending: false,
        ),
        SearchFailureKind.queryEmpty => _failed(SearchError.queryEmpty),
        SearchFailureKind.queryTooLong => _failed(SearchError.queryTooLong),
        SearchFailureKind.cursorInvalid ||
        SearchFailureKind.failed => _failed(SearchError.failed),
      });
    }
  }

  SearchState _failed(SearchError error) => _state.copyWith(
    items: const [],
    localOnly: false,
    pending: false,
    clearCursor: true,
    error: error,
  );
}
