// SPDX-License-Identifier: Apache-2.0

// Truncated and redacted payloads (04 2.2, brief M4-14 Tests): truncated: true loads the
// message through REST; a redacted message.updated stores the tombstone and removes the search
// entry.

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  test('truncated: true fetches the message', () async {
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
    final cut = messageCreated(31, 1, clientMsgId: id(0x9001), author: alexId);
    cut['truncated'] = true;
    ((cut['payload']! as Map)['message']! as Map)
      ..['content_md'] = 'a long'
      ..['content_truncated'] = true;
    w.rest.routes['GET /messages/${messageId(1, 1)}'] = (_) => FakeReply(
      200,
      messageJson(1, 1, content: 'a long release note in full', author: alexId),
    );
    await w.dispatch([cut]);
    expect(w.rest.count('/messages/${messageId(1, 1)}'), 1);
    final m = await w.db.messagesDao.byId(MessageId.parse(messageId(1, 1)));
    expect(m!.contentMd, 'a long release note in full');
    expect(m.contentTruncated, isFalse);
    expect(w.echoes, [
      id(0x9001),
    ], reason: 'the echo of a truncated message.created');
    expect(await w.cursor(), 31);
  });

  test('a redacted message.updated stores the tombstone and removes the search entry', () async {
    await w.connect(cursor: 30);
    await w.dispatch([
      threadCreated(31, 1),
      messageCreated(32, 1, content: 'rollback plan for friday'),
    ]);
    await w.push('cursor', {'seq': 32});
    expect(await w.db.searchDao.search('rollback', limit: 5), hasLength(1));
    await w.dispatch([messageUpdated(33, 1, version: 2, redacted: true)]);
    await w.push('cursor', {'seq': 33});
    final m = await w.db.messagesDao.byId(MessageId.parse(messageId(1, 1)));
    expect(m!.deletedAt, isNotNull);
    expect(m.contentMd, isEmpty);
    expect(m.threadSeq, const ThreadSeq(1));
    expect(await w.db.searchDao.search('rollback', limit: 5), isEmpty);
  });

  test('the echo of an own message.created calls the outbox hook', () async {
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
    await w.dispatch([
      messageCreated(31, 1, author: alexId, clientMsgId: id(0x9002)),
      messageCreated(32, 2, author: samId, clientMsgId: id(0x9003)),
    ]);
    expect(w.echoes, [id(0x9002)]);
  });
}
