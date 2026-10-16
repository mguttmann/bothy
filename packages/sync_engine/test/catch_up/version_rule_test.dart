// SPDX-License-Identifier: Apache-2.0

// The version rule and the seq rule (04 9 item 3, 15 5, brief M4-14 Tests): an older
// `message.updated` after a newer REST page is ignored, reactions and pins apply in seq order,
// and a type without a local table advances the cursor.

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  test('an older message.updated after a newer REST page is ignored', () async {
    w.rest.routes['GET /threads/${threadId(1)}/messages'] = (_) =>
        FakeReply(200, {
          'items': [messageJson(1, 1, version: 5, content: 'from REST v5')],
          'has_more_before': false,
          'has_more_after': false,
        });
    w.openThreads.add(ThreadId.parse(threadId(1)));
    await w.connect(cursor: 30);
    // A resync loads the open thread through REST (version 5).
    await w.push('resync_required', {
      'reason': 'retention',
      'head_seq': 40,
      'sync_epoch': 'epoch-1',
    });
    final id = MessageId.parse(messageId(1, 1));
    expect((await w.db.messagesDao.byId(id))!.contentMd, 'from REST v5');
    await w.dispatch([messageUpdated(41, 1, version: 4, content: 'stale v4')]);
    expect((await w.db.messagesDao.byId(id))!.contentMd, 'from REST v5');
    await w.dispatch([messageUpdated(42, 1, version: 6, content: 'event v6')]);
    expect((await w.db.messagesDao.byId(id))!.contentMd, 'event v6');
    expect(await w.cursor(), 42);
  });

  test('reactions and pins without version apply in seq order', () async {
    await w.connect(cursor: 30);
    await w.dispatch([threadCreated(31, 1), messageCreated(32, 1)]);
    final mid = messageId(1, 1);
    Map<String, Object?> reaction(
      int seq,
      String type,
      int count,
      String who,
    ) => envelope(
      type,
      seq: seq,
      payload: {
        'message_id': mid,
        'thread_id': threadId(1),
        'emoji': ':+1:',
        'principal_id': who,
        'count': count,
      },
      resourceKind: 'message',
      resourceId: mid,
    );
    Map<String, Object?> pin(int seq, String type) => envelope(
      type,
      seq: seq,
      payload: {
        'pin': {
          'message_id': mid,
          'thread_id': threadId(1),
          'pinned_by': samId,
          'pinned_at': at,
        },
      },
      resourceKind: 'message',
      resourceId: mid,
    );
    await w.dispatch([
      reaction(33, 'reaction.added', 1, samId),
      reaction(34, 'reaction.added', 2, alexId),
      pin(35, 'pin.added'),
      reaction(36, 'reaction.removed', 1, samId),
    ]);
    await w.push('cursor', {'seq': 36});
    final id = MessageId.parse(mid);
    final reactions = await w.db.messagesDao.reactionsOf(id);
    expect(reactions.single.count, 1);
    expect(reactions.single.me, isTrue, reason: 'Alex reacted, Sam took back');
    final m = await w.db.messagesDao.byId(id);
    expect(m!.pinned, isTrue);
    expect(m.version, const ResourceVersion(1), reason: 'no version change');
    await w.dispatch([pin(37, 'pin.removed')]);
    await w.push('cursor', {'seq': 37});
    expect((await w.db.messagesDao.byId(id))!.pinned, isFalse);
  });

  test('a type without a local table advances the cursor', () async {
    await w.connect(cursor: 30);
    await w.dispatch([
      envelope(
        'grant.changed',
        seq: 31,
        payload: {'grant_id': id(0x601), 'change': 'created'},
        resourceKind: 'grant',
        resourceId: id(0x601),
      ),
      envelope(
        'stats.unknown_future_type',
        seq: 32,
        payload: const {},
        resourceKind: 'instance',
      ),
    ]);
    await w.push('cursor', {'seq': 32});
    expect(await w.cursor(), 32);
    expect(w.h.phase.name, 'catchingUp');
  });
}
