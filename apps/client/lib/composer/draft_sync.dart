// SPDX-License-Identifier: Apache-2.0

/// Drafts across devices (04 14; brief M4-20 Design note 3; docs/DECISIONS.md D-M4-20-2): the
/// `DraftEditing` port of M4-14, which keeps a foreign draft from overwriting what the person
/// types, and the coalesced draft writes through the outbox.
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

/// The drafts being edited on this device, for `draft.updated` of another device (04 14).
///
/// The bootstrap passes one registry to `SyncEngine.create`; each composer marks its draft
/// while its field has the focus and the person changed the text since the stored draft.
///
/// Example: `registry.setEditing(channel, thread, editing: true);`
final class DraftEditingRegistry implements DraftEditing {
  final Set<(ChannelId, ThreadId?)> _editing = {};

  /// Marks the draft of [threadId] (or of a new post in [channelId]) as edited or not.
  void setEditing(
    ChannelId channelId,
    ThreadId? threadId, {
    required bool editing,
  }) {
    if (editing) {
      _editing.add((channelId, threadId));
    } else {
      _editing.remove((channelId, threadId));
    }
  }

  @override
  bool isEditing({required ChannelId channelId, ThreadId? threadId}) =>
      _editing.contains((channelId, threadId));
}

/// Writes the draft of one thread through the outbox, coalesced: every change updates the one
/// `putThreadDraft` entry that has not been sent yet (04 13.3: a never attempted entry changes
/// in place) or enqueues one when none waits, so at most one draft write is in flight and one
/// waits, with the newest text, without a timer (D-M4-20-2).
///
/// Example:
/// ```dart
/// final drafts = DraftWriter(outbox: engine.outbox, dao: db.outboxDao, thread: id);
/// await drafts.write('Hello Sam', baseVersion: stored?.version);
/// ```
final class DraftWriter {
  /// Creates the writer of [thread] on [outbox] and [dao].
  DraftWriter({
    required this._outbox,
    required this._dao,
    required this.thread,
  });

  final Outbox _outbox;
  final OutboxDao _dao;

  /// The thread of the draft.
  final ThreadId thread;

  String? _waiting;
  Future<void> _last = Future<void>.value();

  /// Saves [text] (with [replyTo] and [baseVersion], the version of the draft this device
  /// last saw) as the draft; writes run one after another in the order of the calls.
  Future<void> write(
    String text, {
    MessageId? replyTo,
    ResourceVersion? baseVersion,
  }) => _serial(() async {
    final payload = <String, Object?>{
      'content_md': text,
      'reply_to_message_id': ?replyTo?.value,
      'base_version': ?baseVersion?.value,
    };
    final waiting = await _waitingEntry();
    if (waiting != null) {
      try {
        // The full payload replaces what waits, so a dropped reply reference goes too.
        await _outbox.edit(waiting.id, {
          'reply_to_message_id': null,
          'base_version': null,
          ...payload,
        });
        return;
      } on StateError {
        // Sent meanwhile: the next write waits behind it in the thread's lane.
      }
    }
    final entry = await _outbox.enqueue(
      OutboxRequest(
        kind: PersonalOps.putThreadDraft,
        threadId: thread,
        payload: payload,
      ),
    );
    _waiting = entry.id;
  });

  /// Drops the draft write that waits, because the message was sent with `clear_draft`
  /// (04 14); a write that is in flight stays and the send clears it afterwards.
  Future<void> dropWaiting() => _serial(() async {
    final waiting = await _waitingEntry();
    _waiting = null;
    if (waiting != null) await _outbox.discard(waiting.id);
  });

  /// The draft writes of the thread that the server has not confirmed yet, oldest first.
  Future<List<OutboxEntry>> _unsent() async => [
    for (final e in await _dao.entriesFor(thread))
      if (e.kind == PersonalOps.putThreadDraft && !e.state.isFinal) e,
  ];

  /// Whether a draft write of this device waits or is in flight; a stored draft is then older
  /// than what the person typed here (the next local write wins, 04 14).
  Future<bool> hasUnsent() async => (await _unsent()).isNotEmpty;

  /// The text of the newest draft write that waits or is in flight, or null; a write that was
  /// not attempted becomes the one that later changes update (a composer opened again, or the
  /// app started again while offline).
  Future<String?> restore() async {
    final unsent = await _unsent();
    if (unsent.isEmpty) return null;
    final last = unsent.last;
    if (last.attempts == 0 && last.state == OutboxState.pending) {
      _waiting = last.id;
    }
    return decodePayload(last.payload)['content_md'] as String?;
  }

  Future<OutboxEntry?> _waitingEntry() async {
    final id = _waiting;
    if (id == null) return null;
    final entry = await _dao.byId(id);
    if (entry == null ||
        entry.attempts > 0 ||
        entry.state != OutboxState.pending) {
      _waiting = null;
      return null;
    }
    return entry;
  }

  Future<void> _serial(Future<void> Function() step) {
    final next = _last.then((_) => step());
    _last = next.catchError((Object _) {});
    return next;
  }
}
