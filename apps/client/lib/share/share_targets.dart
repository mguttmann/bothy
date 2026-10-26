// SPDX-License-Identifier: Apache-2.0

/// The targets of the share sheet from the local database and the search for more (brief
/// M5-06 Design notes 1 and 7; 01 14 `client.share.recent_targets`; 15 1 principle 1).
///
/// A target is offered only when the server reported its channel as writable: the stored
/// level of the caller (`ChannelView.level`) is `write` or `trigger` and the channel is not
/// read-only. The app adds no permission logic of its own; a send to a target that lost the
/// write level fails visibly in the outbox.
library;

import 'dart:async';

import 'package:api_client/api_client.dart' show ApiClient;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'share_target.dart';

/// The SQL condition of a writable text or forum channel `c` (the stored level and
/// `read_only` of the server; direct messages are no share target).
const String _writable =
    "c.level IN ('write', 'trigger') AND c.read_only = 0 "
    "AND c.kind IN ('text', 'forum')";

/// One hit of `searchThreadTitles` (14).
final class ThreadTitleHit {
  /// Creates the hit.
  const ThreadTitleHit({
    required this.threadId,
    required this.channelId,
    required this.title,
  });

  /// The thread.
  final ThreadId threadId;

  /// Its channel.
  final ChannelId channelId;

  /// Its title as the server found it.
  final String title;
}

/// The thread title search of the server (`searchThreadTitles`).
abstract interface class ThreadTitleSearch {
  /// The threads whose title matches [query]; throws what the API client throws.
  Future<List<ThreadTitleHit>> search(String query);
}

/// [ThreadTitleSearch] over the generated API client of a signed-in session.
final class ApiThreadTitleSearch implements ThreadTitleSearch {
  /// Creates the search over [api].
  ApiThreadTitleSearch(this._api);

  final ApiClient _api;

  @override
  Future<List<ThreadTitleHit>> search(String query) async {
    final response = await _api.getSearchApi().searchThreadTitles(q: query);
    final items = response.data?.items;
    if (items == null) return const [];
    final hits = <ThreadTitleHit>[];
    for (final item in items) {
      try {
        hits.add(
          ThreadTitleHit(
            threadId: ThreadId.parse(item.threadId),
            channelId: ChannelId.parse(item.channelId),
            title: item.title,
          ),
        );
      } on FormatException {
        // A hit with a malformed ID names nothing the app could send to.
      }
    }
    return hits;
  }
}

/// The targets of the local database.
///
/// Example:
/// ```dart
/// final source = ShareTargetSource(db);
/// final recent = await source.recent(me: me, max: config.shareRecentTargets);
/// ```
final class ShareTargetSource {
  /// Creates the source over [_db]; the queries use only the public API of the database
  /// (`customSelect` over the mirror tables), so `local_store` needs no change.
  ///
  /// [loadThread] stores a thread the database does not hold yet (`getThread`, stored by the
  /// version rule), so its title can name a recent target: the sync engine stores a message
  /// without its thread. Without it, or when the thread cannot be loaded, the target is left
  /// out; a thread that failed is not asked for again by this source.
  ShareTargetSource(
    this._db, {
    Future<void> Function(ThreadId thread)? loadThread,
  }) : _loadThread = loadThread; // ignore: prefer_initializing_formals

  final LocalDatabase _db;
  final Future<void> Function(ThreadId thread)? _loadThread;
  final Set<ThreadId> _unloadable = {};

  /// The last [max] targets [me] wrote a message into, newest first, one per thread: the
  /// main thread of a text channel gives its `channel` target, any other thread a `thread`
  /// target. Only writable channels count (the thread's channel, also after a move); [max]
  /// is `client.share.recent_targets`.
  Future<List<ShareTarget>> recent({
    required PrincipalId me,
    required int max,
  }) async {
    if (max <= 0) return const [];
    // The app has no drift dependency of its own, so the query takes no bound variables:
    // the person's ID is a canonical UUID (checked by `PrincipalId.parse`) and [max] an
    // integer, and nothing else enters the SQL text.
    final rows = await _db
        .customSelect(
          'SELECT m.thread_id AS thread_id, c.id AS channel_id, '
          'c.name AS channel_name, c.main_thread_id AS main_thread_id, '
          't.title AS title, MAX(m.created_at) AS sent_at, MAX(m.id) AS last_id '
          'FROM messages m '
          'LEFT JOIN threads t ON t.id = m.thread_id '
          'JOIN channels c ON c.id = COALESCE(t.channel_id, m.channel_id) '
          "WHERE m.author_principal_id = '${_uuid(me.value)}' AND $_writable "
          'GROUP BY m.thread_id '
          'ORDER BY sent_at DESC, last_id DESC '
          'LIMIT $max',
          readsFrom: {_db.messages, _db.threads, _db.channels},
        )
        .get();
    final targets = <ShareTarget>[];
    for (final r in rows) {
      final channel = ChannelId.parse(r.read<String>('channel_id'));
      final thread = ThreadId.parse(r.read<String>('thread_id'));
      final name = r.read<String>('channel_name');
      if (r.read<String?>('main_thread_id') == thread.value) {
        targets.add(
          ShareTarget(
            kind: ShareTargetKind.channel,
            channelId: channel,
            threadId: thread,
            path: name,
          ),
        );
        continue;
      }
      final title = r.read<String?>('title') ?? await _titleOf(thread);
      if (title == null) continue;
      targets.add(
        ShareTarget(
          kind: ShareTargetKind.thread,
          channelId: channel,
          threadId: thread,
          path: '$name > $title',
        ),
      );
    }
    return targets;
  }

  /// The title of [thread] after [_loadThread] stored it, or null.
  Future<String?> _titleOf(ThreadId thread) async {
    final load = _loadThread;
    if (load == null || _unloadable.contains(thread)) return null;
    try {
      await load(thread);
    } on Object {
      // Not found, no access or offline: the target stays out of the list.
      _unloadable.add(thread);
      return null;
    }
    final stored = await _db.threadsDao.byId(thread);
    if (stored == null) _unloadable.add(thread);
    return stored?.title;
  }

  /// The writable channels whose name contains [query] (case-insensitive), by name: a text
  /// channel as its main timeline (`channel`), a forum as a new post (`new_post`). A text
  /// channel without a known main thread is left out.
  Future<List<ShareTarget>> channelsMatching(String query) async {
    final needle = query.trim().toLowerCase();
    if (needle.isEmpty) return const [];
    // The text the person typed is compared here, never put into the SQL text.
    return [
      for (final c in await _writableChannels())
        if (c.name.toLowerCase().contains(needle)) ?c.target,
    ];
  }

  /// The target of the title hit [hit] when its channel is stored here and writable, else
  /// null; the path takes the stored channel name.
  Future<ShareTarget?> threadHit(ThreadTitleHit hit) async =>
      _hitTarget(hit, {for (final c in await _writableChannels()) c.id: c});

  ShareTarget? _hitTarget(
    ThreadTitleHit hit,
    Map<ChannelId, _Channel> writable,
  ) {
    final channel = writable[hit.channelId];
    if (channel == null) return null;
    if (channel.mainThreadId == hit.threadId) {
      return ShareTarget(
        kind: ShareTargetKind.channel,
        channelId: hit.channelId,
        threadId: hit.threadId,
        path: channel.name,
      );
    }
    return ShareTarget(
      kind: ShareTargetKind.thread,
      channelId: hit.channelId,
      threadId: hit.threadId,
      path: '${channel.name} > ${hit.title}',
    );
  }

  Future<List<_Channel>> _writableChannels() async {
    final rows = await _db
        .customSelect(
          'SELECT c.id AS id, c.name AS name, c.kind AS kind, '
          'c.main_thread_id AS main_thread_id '
          'FROM channels c WHERE $_writable ORDER BY lower(c.name), c.id',
          readsFrom: {_db.channels},
        )
        .get();
    return [
      for (final r in rows)
        _Channel(
          id: ChannelId.parse(r.read<String>('id')),
          name: r.read<String>('name'),
          forum: r.read<String>('kind') == ChannelKind.forum.code,
          mainThreadId: switch (r.read<String?>('main_thread_id')) {
            final String id => ThreadId.parse(id),
            null => null,
          },
        ),
    ];
  }

  /// One value on listen and after every change of the stored channels: their names, levels
  /// and `read_only` decide the targets, and `access.changed` changes them (04 8.3).
  Stream<void> changes() => _db
      .customSelect(
        'SELECT c.id, c.name, c.level, c.read_only, c.kind, c.main_thread_id '
        'FROM channels c',
        readsFrom: {_db.channels},
      )
      .watch()
      .map((_) {});

  /// [text], a canonical UUID, unchanged; anything else is a programming error.
  static String _uuid(String text) {
    if (!RegExp(
      r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$',
    ).hasMatch(text)) {
      throw ArgumentError.value(text.length, 'uuid', 'not a canonical UUID');
    }
    return text;
  }
}

/// A writable channel of the local database.
final class _Channel {
  const _Channel({
    required this.id,
    required this.name,
    required this.forum,
    required this.mainThreadId,
  });

  final ChannelId id;
  final String name;
  final bool forum;
  final ThreadId? mainThreadId;

  /// The channel as a target: a forum as a new post, a text channel as its main timeline, or
  /// null for a text channel without a known main thread.
  ShareTarget? get target {
    if (forum) {
      return ShareTarget(
        kind: ShareTargetKind.newPost,
        channelId: id,
        path: name,
      );
    }
    final main = mainThreadId;
    if (main == null) return null;
    return ShareTarget(
      kind: ShareTargetKind.channel,
      channelId: id,
      threadId: main,
      path: name,
    );
  }
}

/// The search of the share sheet: the writable channels by name from the local database,
/// then the thread titles of the server whose channels are writable here (brief M5-06 Design
/// note 1). A failing title search still gives the channels.
final class ShareTargetSearch {
  /// Creates the search over [source] and [titles].
  ShareTargetSearch({required this.source, required this.titles});

  /// The local targets.
  final ShareTargetSource source;

  /// The thread titles of the server.
  final ThreadTitleSearch titles;

  /// The targets for [query]; empty for a blank query, without asking the server.
  Future<List<ShareTarget>> search(String query) async {
    final q = query.trim();
    if (q.isEmpty) return const [];
    final found = <ShareTarget>[...await source.channelsMatching(q)];
    List<ThreadTitleHit> hits;
    try {
      hits = await titles.search(q);
    } on Object {
      // Offline or refused: the channels of the local database stay.
      hits = const [];
    }
    if (hits.isEmpty) return found;
    final writable = {
      for (final c in await source._writableChannels()) c.id: c,
    };
    for (final hit in hits) {
      final target = source._hitTarget(hit, writable);
      if (target != null && !found.contains(target)) found.add(target);
    }
    return found;
  }
}
