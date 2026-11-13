// SPDX-License-Identifier: Apache-2.0

// One stored draft per thread and per forum (04 14; docs/decisions.d/FIX-COMPOSER-DRAFT-LOST.md).
// The server keeps one draft per person and target and deletes it when it is sent or emptied;
// the next draft of that target is a new row with a new ID whose version starts at 1 again. The
// store kept rows by ID alone, so the row of a deleted draft stayed next to the new one, the
// read of the composer failed on two rows and the composer showed nothing.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

final ChannelId _channel = ChannelId.parse(uuid(0x201));

/// Draft [n] with [text], written at minute [minute]; of thread 1, or of a new post in
/// channel 1 with [thread] null.
Draft _draft(
  int n,
  String text, {
  int version = 1,
  int? minute,
  int? thread = 1,
}) => Draft(
  id: DraftId.parse(uuid(0x400 + n)),
  threadId: thread == null ? null : threadId(thread),
  channelId: _channel,
  contentMd: text,
  fileIds: const [],
  version: ResourceVersion(version),
  updatedAt: minute == null ? null : at(minute),
);

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  Future<Draft?> shown({int? thread = 1}) => db.personalDao
      .watchDraft(
        channelId: _channel,
        threadId: thread == null ? null : threadId(thread),
      )
      .first;

  /// Stores [draft] as an earlier version of the app did: by its ID alone.
  Future<void> leftover(Draft draft) => db.customStatement(
    'INSERT INTO drafts (id, thread_id, channel_id, content_md, file_ids, version, updated_at) '
    'VALUES (?, ?, ?, ?, ?, ?, ?)',
    [
      draft.id.value,
      draft.threadId?.value,
      draft.channelId.value,
      draft.contentMd,
      '[]',
      draft.version.value,
      draft.updatedAt?.toIso8601String(),
    ],
  );

  test('the next draft of a thread is a new row on the server and replaces '
      'the stored one, although its version starts again', () async {
    final dao = db.personalDao;
    expect(
      await dao.upsertDraftIfNewer(
        _draft(1, 'Message one', version: 7, minute: 1),
      ),
      isTrue,
    );
    final next = _draft(2, 'Message two', minute: 2);
    expect(await dao.upsertDraftIfNewer(next), isTrue);
    expect(await dao.allDrafts(), [next]);
    expect(await shown(), next);
  });

  test('the next draft of a new post replaces the stored one of its forum '
      'and leaves the draft of a thread of that forum alone', () async {
    final dao = db.personalDao;
    final reply = _draft(1, 'A reply', minute: 1);
    await dao.upsertDraftIfNewer(reply);
    await dao.upsertDraftIfNewer(
      _draft(2, 'An idea', version: 3, minute: 2, thread: null),
    );
    final next = _draft(3, 'A better idea', minute: 3, thread: null);
    expect(await dao.upsertDraftIfNewer(next), isTrue);
    expect(await shown(thread: null), next);
    expect(await shown(), reply);
    expect(await dao.allDrafts(), hasLength(2));
  });

  test(
    'a late change of a draft that was replaced already is not stored',
    () async {
      final dao = db.personalDao;
      final current = _draft(2, 'Typed now', minute: 5);
      await dao.upsertDraftIfNewer(current);
      expect(
        await dao.upsertDraftIfNewer(
          _draft(1, 'From the phone, earlier', version: 9, minute: 4),
        ),
        isFalse,
      );
      expect(await dao.allDrafts(), [current]);
    },
  );

  test('with the same ID the version rule stays', () async {
    final dao = db.personalDao;
    await dao.upsertDraftIfNewer(_draft(1, 'v2', version: 2, minute: 2));
    expect(await dao.upsertDraftIfNewer(_draft(1, 'v1', minute: 3)), isFalse);
    expect(
      await dao.upsertDraftIfNewer(_draft(1, 'again', version: 2, minute: 3)),
      isFalse,
    );
    expect((await shown())!.contentMd, 'v2');
    expect(
      await dao.upsertDraftIfNewer(_draft(1, 'v3', version: 3, minute: 3)),
      isTrue,
    );
    expect((await shown())!.contentMd, 'v3');
  });

  test(
    'a draft without a time replaces a stored one, and is replaced',
    () async {
      final dao = db.personalDao;
      await dao.upsertDraftIfNewer(_draft(1, 'First', minute: 1));
      expect(await dao.upsertDraftIfNewer(_draft(2, 'No time')), isTrue);
      expect((await shown())!.contentMd, 'No time');
      expect(
        await dao.upsertDraftIfNewer(_draft(3, 'Third', minute: 0)),
        isTrue,
      );
      expect(await dao.allDrafts(), hasLength(1));
      expect((await shown())!.contentMd, 'Third');
    },
  );

  group('rows an earlier version of the app left', () {
    test('the read gives the newest row instead of failing on two', () async {
      await leftover(_draft(1, '', version: 4, minute: 1));
      await leftover(_draft(2, 'Still here', minute: 2));
      expect((await shown())!.contentMd, 'Still here');
      // The order of the rows does not decide.
      await leftover(_draft(3, 'Older', minute: 0));
      expect((await shown())!.contentMd, 'Still here');
    });

    test('the next write of the thread removes them', () async {
      await leftover(_draft(1, 'Sent long ago', version: 4, minute: 1));
      await leftover(_draft(2, 'Typed last week', minute: 2));
      final next = _draft(2, 'Typed today', version: 2, minute: 3);
      expect(await db.personalDao.upsertDraftIfNewer(next), isTrue);
      expect(await db.personalDao.allDrafts(), [next]);
    });

    test('deleting the draft removes them, so none comes back', () async {
      await leftover(_draft(1, 'Sent long ago', version: 4, minute: 1));
      await leftover(_draft(2, 'Sent now', minute: 2));
      await db.personalDao.deleteDraft(_draft(2, '').id);
      expect(await db.personalDao.allDrafts(), isEmpty);
      expect(await shown(), isNull);
    });
  });

  test('deleting a draft that was replaced leaves the newer one', () async {
    final dao = db.personalDao;
    await leftover(_draft(1, 'Deleted on the server', minute: 1));
    await leftover(_draft(2, 'Its successor', minute: 2));
    await dao.deleteDraft(_draft(1, '').id);
    expect((await dao.allDrafts()).single.contentMd, 'Its successor');
    // An unknown ID deletes nothing.
    await dao.deleteDraft(_draft(9, '').id);
    expect(await dao.allDrafts(), hasLength(1));
  });

  group('endDraft', () {
    test(
      'removes the stored draft of the thread up to the given time',
      () async {
        final dao = db.personalDao;
        await dao.upsertDraftIfNewer(_draft(1, 'Sent', minute: 1));
        await dao.upsertDraftIfNewer(
          _draft(5, 'Other thread', minute: 1, thread: 2),
        );
        await dao.upsertDraftIfNewer(
          _draft(6, 'New post', minute: 1, thread: null),
        );
        await dao.endDraft(
          channelId: _channel,
          threadId: threadId(1),
          until: at(1),
        );
        expect([
          for (final d in await dao.allDrafts()) d.contentMd,
        ], unorderedEquals(['Other thread', 'New post']));
      },
    );

    test('keeps a draft written after that time', () async {
      final dao = db.personalDao;
      final later = _draft(1, 'From the phone, after the send', minute: 3);
      await dao.upsertDraftIfNewer(later);
      await dao.endDraft(
        channelId: _channel,
        threadId: threadId(1),
        until: at(2),
      );
      expect(await dao.allDrafts(), [later]);
    });

    test('of a forum removes the draft of a new post only', () async {
      final dao = db.personalDao;
      final reply = _draft(1, 'A reply', minute: 1);
      await dao.upsertDraftIfNewer(reply);
      await dao.upsertDraftIfNewer(
        _draft(2, 'An idea', minute: 1, thread: null),
      );
      await dao.endDraft(channelId: _channel, until: at(5));
      expect(await dao.allDrafts(), [reply]);
    });

    test('without a time removes every stored row of the target', () async {
      final dao = db.personalDao;
      await leftover(_draft(1, 'One', minute: 1));
      await leftover(_draft(2, 'Two'));
      await dao.endDraft(channelId: _channel, threadId: threadId(1));
      expect(await dao.allDrafts(), isEmpty);
    });
  });
}
