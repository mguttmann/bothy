// SPDX-License-Identifier: Apache-2.0

/// Gaps in `thread_seq` (04 4, 15 7.3 item 4, brief M4-14 Design note 4): a new message that
/// does not continue the loaded range of its thread is stored but not added to the loaded
/// ranges, so `MessagesDao.watchContiguous` keeps it hidden; the missing range is loaded
/// through REST, and only then the range up to the new message counts as loaded.
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'appliers/message_appliers.dart';
import 'dispatcher.dart';

/// Records gaps per thread and fills them through the [Fetcher] of the [SyncContext].
///
/// A failed fill keeps the gap recorded; [fill] retries it, which the app calls when the
/// thread gets the focus again (the read retry of M4-02 already ran inside the fetch).
///
/// Example:
/// ```dart
/// if (await gaps.place(db, threadId, seq)) await gaps.fill(sync, threadId);
/// ```
final class GapFiller {
  final Map<ThreadId, SeqRange> _pending = {};

  /// The recorded gaps by thread: from the first missing position up to the message that
  /// revealed the gap.
  Map<ThreadId, SeqRange> get pending => Map.unmodifiable(_pending);

  /// Decides inside the caller's transaction where the new message at [seq] of [threadId]
  /// goes: when it continues a loaded range, or nothing of the thread is loaded yet (the
  /// thread shows from its newest message and loads older pages when scrolled), its position
  /// joins the loaded ranges; otherwise the gap up to it is recorded and true is returned, so
  /// the caller schedules [fill].
  Future<bool> place(LocalDatabase db, ThreadId threadId, ThreadSeq seq) async {
    final loaded = await db.messagesDao.loadedRanges(threadId);
    final below = [
      for (final r in loaded.ranges)
        if (r.to < seq) r,
    ];
    if (loaded.ranges.isEmpty || loaded.fits(seq) || below.isEmpty) {
      await db.messagesDao.addLoadedRange(threadId, seq, seq);
      return false;
    }
    final from = below.last.to.next;
    final known = _pending[threadId];
    _pending[threadId] = known == null
        ? SeqRange(from, seq)
        : SeqRange(
            known.from < from ? known.from : from,
            known.to > seq ? known.to : seq,
          );
    return true;
  }

  /// Loads the recorded gap of [threadId] through `Fetcher.range`, stores the messages and
  /// marks the range as loaded in one transaction. A fetch error leaves the gap recorded and
  /// propagates.
  Future<void> fill(SyncContext sync, ThreadId threadId) async {
    final gap = _pending[threadId];
    if (gap == null) return;
    final bundles = await sync.fetcher.range(threadId, gap.from, gap.to);
    await sync.db.transaction(() async {
      for (final b in bundles) {
        await storeMessage(sync.db, b, me: sync.me);
      }
      await sync.db.messagesDao.addLoadedRange(threadId, gap.from, gap.to);
    });
    // A gap that grew while the fetch ran stays for the next fill.
    if (_pending[threadId] == gap) _pending.remove(threadId);
  }

  /// Forgets every gap (a resync replaced the mirror).
  void clear() => _pending.clear();
}
