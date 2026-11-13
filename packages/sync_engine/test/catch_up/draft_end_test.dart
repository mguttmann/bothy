// SPDX-License-Identifier: Apache-2.0

// The end of a stored draft (04 14; docs/decisions.d/FIX-COMPOSER-DRAFT-LOST.md). The server
// deletes a draft when it is emptied and when a message is sent with `clear_draft`; the next
// draft of that thread is a new row with a new ID. The device stored the empty final state of
// an empty write as a row, and it kept the row of a sent draft whenever the `draft.deleted` of
// its own send arrived while the person typed the next message (the typing rule). The row
// that was left then stood next to the next draft, or came back as the draft. The answer of a
// draft write that arrives after the person sent the text is no row either (FIX-CDL/9).

import 'dart:convert';

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  final t1 = ThreadId.parse(threadId(1));
  final ch1 = ChannelId.parse(channelId(1));

  /// The time of the messages of these tests (`at` of the support) and times around it.
  const before = '2026-10-15T09:59:00.000Z';
  const after = '2026-10-15T10:00:05.000Z';

  Future<Draft?> storedDraft() =>
      w.db.personalDao.watchDraft(channelId: ch1, threadId: t1).first;

  Map<String, Object?> draft(
    int n,
    String text, {
    int version = 1,
    String time = before,
    String? device,
  }) => {
    ...draftJson(n, text: text, version: version, device: device ?? deviceId),
    'updated_at': time,
  };

  Future<void> put(Map<String, Object?> draft) =>
      w.catchUp.applyDraftPutResult({'draft': draft, 'overwrote_newer': false});

  Map<String, Object?> draftDeleted(int seq, int n) => envelope(
    'draft.deleted',
    seq: seq,
    payload: {
      'draft_id': draftId(n),
      'channel_id': channelId(1),
      'thread_id': threadId(1),
    },
    resourceKind: 'draft',
    resourceId: draftId(n),
    personal: true,
  );

  group('the answer to an empty draft write', () {
    test('ends the stored draft and stores no empty row; the next draft of '
        'the thread, a new row on the server, is the draft', () async {
      await w.connect(cursor: 30);
      await put(draft(1, 'First try', version: 3));
      // The server deleted the row and answers the empty final state: the same ID, the last
      // version plus one.
      await put(draft(1, '', version: 4));
      expect(await w.db.personalDao.allDrafts(), isEmpty);
      // Typed again: a new row with a new ID that starts at version 1.
      await put(draft(2, 'Second try', time: after));
      expect((await storedDraft())!.contentMd, 'Second try');
      expect(await w.db.personalDao.allDrafts(), hasLength(1));
    });

    test('of a thread without a draft stores nothing', () async {
      await w.connect(cursor: 30);
      await put(draft(7, ''));
      expect(await w.db.personalDao.allDrafts(), isEmpty);
    });

    test('leaves a draft another device wrote after it', () async {
      await w.connect(cursor: 30);
      await w.dispatch([
        envelope(
          'draft.updated',
          seq: 31,
          payload: {
            'draft': draft(
              2,
              'From the phone',
              time: after,
              device: otherDevice,
            ),
          },
          resourceKind: 'draft',
          resourceId: draftId(2),
          version: 1,
          personal: true,
        ),
      ]);
      await w.push('cursor', {'seq': 31});
      // The answer of this device's earlier empty write arrives after that event.
      await put(draft(1, '', version: 4));
      expect((await storedDraft())!.contentMd, 'From the phone');
    });

    test('still raises the notice of an overwritten draft', () async {
      await w.connect(cursor: 30);
      await w.catchUp.applyDraftPutResult({
        'draft': draft(1, 'Mine wins', version: 5),
        'overwrote_newer': true,
      });
      expect(w.notices.whereType<DraftOverwritten>(), hasLength(1));
    });
  });

  group('the answer of a draft write that a send of this device has '
      'overtaken', () {
    /// Puts an entry into the outbox, as the app does for a send that waits.
    Future<String> queued(
      int n,
      String kind,
      Map<String, Object?> payload, {
      ThreadId? thread,
    }) async {
      final entry = await w.db.outboxDao.enqueue(
        id: id(0x700 + n),
        kind: kind,
        payload: jsonEncode(payload),
        threadId: thread,
      );
      return entry.id;
    }

    /// The draft of a new post in forum [channelN]: a draft without a thread.
    Map<String, Object?> newPostDraft(int n, String text, {int channelN = 1}) =>
        {
          ...draft(n, text),
          'thread_id': null,
          'channel_id': channelId(channelN),
        };

    test('stores no row: the draft ended with the send, and the server '
        'deletes it when the send arrives', () async {
      await w.connect(cursor: 30);
      await queued(1, MessageOps.createMessage, {
        'content_md': 'Message one',
        'clear_draft': true,
      }, thread: t1);
      await put(draft(1, 'Message one'));
      expect(await w.db.personalDao.allDrafts(), isEmpty);
      expect(await storedDraft(), isNull);
    });

    test('still raises the notice of an overwritten draft', () async {
      await w.connect(cursor: 30);
      await queued(1, MessageOps.createMessage, {
        'content_md': 'Message one',
        'clear_draft': true,
      }, thread: t1);
      await w.catchUp.applyDraftPutResult({
        'draft': draft(1, 'Message one', version: 5),
        'overwrote_newer': true,
      });
      expect(w.notices.whereType<DraftOverwritten>(), hasLength(1));
      expect(await w.db.personalDao.allDrafts(), isEmpty);
    });

    test('is the draft when the send keeps the draft, as a voice message '
        'does', () async {
      await w.connect(cursor: 30);
      await queued(1, MessageOps.createMessage, {
        'kind': 'voice',
        'content_md': '',
      }, thread: t1);
      await put(draft(1, 'Still typing'));
      expect((await storedDraft())!.contentMd, 'Still typing');
    });

    test('is the draft when the send was refused for good: the server kept '
        'it', () async {
      await w.connect(cursor: 30);
      final send = await queued(1, MessageOps.createMessage, {
        'content_md': 'Message one',
        'clear_draft': true,
      }, thread: t1);
      await w.db.outboxDao.recordFailure(send, code: 'forbidden');
      await put(draft(1, 'Message one'));
      expect((await storedDraft())!.contentMd, 'Message one');
    });

    test('is the draft when the send is a message of another thread', () async {
      await w.connect(cursor: 30);
      await queued(1, MessageOps.createMessage, {
        'content_md': 'Elsewhere',
        'clear_draft': true,
      }, thread: ThreadId.parse(threadId(2)));
      await put(draft(1, 'Still typing'));
      expect((await storedDraft())!.contentMd, 'Still typing');
    });

    test('of a new post stores no row while the post of its forum waits, and '
        'the draft of another forum stays', () async {
      await w.connect(cursor: 30);
      // The app's entry of a new post: without a thread, with its forum in the payload.
      await queued(1, 'createThread', {
        'channel_id': channelId(1),
        'title': '',
        'content_md': 'An idea',
        'clear_draft': true,
      });
      await put(newPostDraft(1, 'An idea'));
      expect(await w.db.personalDao.allDrafts(), isEmpty);
      await put(newPostDraft(2, 'Another forum', channelN: 2));
      expect(
        (await w.db.personalDao.allDrafts()).single.contentMd,
        'Another forum',
      );
    });

    test('of a thread is the draft while only a new post of its forum '
        'waits', () async {
      await w.connect(cursor: 30);
      await queued(1, 'createThread', {
        'channel_id': channelId(1),
        'title': '',
        'content_md': 'An idea',
        'clear_draft': true,
      });
      await put(draft(1, 'Still typing'));
      expect((await storedDraft())!.contentMd, 'Still typing');
    });
  });

  group('the draft.deleted of an own send that arrives while the next message '
      'is typed', () {
    Future<void> sendArrives({required int seq, required int deleted}) async {
      await w.dispatch([
        messageCreated(seq, seq - 25, author: alexId),
        draftDeleted(seq + 1, deleted),
      ]);
      await w.push('cursor', {'seq': seq + 1});
    }

    test('is not applied, and the next draft, a new row on the server, '
        'replaces the stored one', () async {
      await w.connect(cursor: 30);
      await put(draft(1, 'Message one', version: 2));
      w.editing.editingThread = threadId(1);
      await sendArrives(seq: 31, deleted: 1);
      // 04 14, unchanged: no change is taken over while the person types here.
      expect((await storedDraft())!.contentMd, 'Message one');
      await put(draft(2, 'Message two', time: after));
      expect(
        (await w.db.personalDao.allDrafts()).single.contentMd,
        'Message two',
      );
      expect((await storedDraft())!.contentMd, 'Message two');
    });

    test('and once the next message is sent too, no draft is left', () async {
      await w.connect(cursor: 30);
      await put(draft(1, 'Message one', version: 2));
      w.editing.editingThread = threadId(1);
      await sendArrives(seq: 31, deleted: 1);
      await put(draft(2, 'Message two', time: after));
      // Sent: the person no longer types, the delete of the second send applies.
      w.editing.editingThread = null;
      await sendArrives(seq: 33, deleted: 2);
      expect(await w.db.personalDao.allDrafts(), isEmpty);
      expect(await storedDraft(), isNull);
    });
  });
}
