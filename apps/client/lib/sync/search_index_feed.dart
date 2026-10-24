// SPDX-License-Identifier: Apache-2.0

/// The transcript feed of the offline search (docs/spec/14-search.md 6; brief M4-23): the
/// dispatcher of M4-14 indexes messages and thread titles on apply, but the transcript text is
/// not stored locally (15 6.1 `files`), so this feed fetches the done transcripts of attached
/// voice files with `getTranscript` and indexes them, and drops the rows of files no stored
/// message attaches any more.
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

/// Fetches the transcript text of a file (`getTranscript`); null when the transcript has no
/// text. Throws when the request fails.
typedef TranscriptFetch = Future<String?> Function(FileId fileId);

/// Keeps the transcript rows of `search_index` in line with the cache (14 6: "messages, thread
/// titles and transcripts that are in the cache").
///
/// It watches `SearchDao.watchTranscriptsToIndex`: each pending file is fetched once and indexed
/// with the thread, channel and space of the attaching message. A failed fetch writes nothing
/// and is retried on the next change of the watched tables, so there is no timer and no loop
/// while the server is unreachable. A transcript without text is fetched once per start. On every
/// change it also calls `SearchDao.pruneTranscripts`, so a deleted voice message or a dropped
/// attachment is not findable offline any more (04 2.2 item 5). Removed targets lose their rows
/// through `removeTargets` (04 8.3).
///
/// Example:
/// ```dart
/// final feed = SearchIndexFeed(
///   database: db,
///   fetchTranscript: FileTranscripts(api).text,
/// )..start();
/// // ...
/// await feed.stop();
/// ```
final class SearchIndexFeed {
  /// Creates the feed over [database]; [fetchTranscript] loads a text from the server.
  SearchIndexFeed({
    required LocalDatabase database,
    required TranscriptFetch fetchTranscript,
  }) : _db = database,
       _fetch = fetchTranscript;

  final LocalDatabase _db;
  final TranscriptFetch _fetch;

  StreamSubscription<List<PendingTranscript>>? _subscription;
  List<PendingTranscript>? _latest;
  Future<void>? _work;
  Future<void>? _write;
  bool _stopped = false;

  /// Files fetched in this run whose transcript had no text; they stay pending in the database
  /// but are not fetched again.
  final Set<FileId> _empty = {};

  /// Starts watching; a second call does nothing, a call after [stop] neither.
  void start() {
    if (_subscription != null || _stopped) return;
    _subscription = _db.searchDao.watchTranscriptsToIndex().listen(
      _onPending,
      // A failing query (a closed database) ends nothing else; the next start watches again.
      onError: (Object _) {},
    );
  }

  /// Stops watching and waits for a database write in progress, so the caller may close the
  /// database afterwards. A fetch still in flight is not awaited; its answer is dropped.
  Future<void> stop() async {
    _stopped = true;
    final subscription = _subscription;
    _subscription = null;
    await subscription?.cancel();
    await _write;
  }

  void _onPending(List<PendingTranscript> pending) {
    _latest = pending;
    _work ??= _drain().whenComplete(() => _work = null);
  }

  /// Works through the newest pending list until no newer one arrived meanwhile; one pass at a
  /// time, so a file is never fetched twice in parallel.
  Future<void> _drain() async {
    while (!_stopped) {
      final pending = _latest;
      if (pending == null) return;
      _latest = null;
      _empty.retainAll({for (final p in pending) p.fileId});
      if (!await _guard(_db.searchDao.pruneTranscripts)) return;
      for (final item in pending) {
        if (_stopped) return;
        if (_empty.contains(item.fileId)) continue;
        await _index(item);
      }
    }
  }

  Future<void> _index(PendingTranscript item) async {
    final String? text;
    try {
      text = await _fetch(item.fileId);
    } on Object {
      // Offline or a server error: the row stays pending and the next change retries.
      return;
    }
    if (_stopped) return;
    if (text == null || text.isEmpty) {
      _empty.add(item.fileId);
      return;
    }
    final body = text;
    await _guard(
      () => _db.searchDao.indexTranscript(
        fileId: item.fileId,
        threadId: item.threadId,
        channelId: item.channelId,
        spaceId: item.spaceId,
        text: body,
      ),
    );
  }

  /// Runs the database write [op] unless stopped, so [stop] can wait for it; returns whether it
  /// ran without an error. A failed write (a closed database) is left to the next change.
  Future<bool> _guard(Future<Object?> Function() op) async {
    if (_stopped) return false;
    final write = op();
    _write = write.then<void>((_) {}, onError: (Object _) {});
    try {
      await write;
      return true;
    } on Object {
      return false;
    }
  }
}
