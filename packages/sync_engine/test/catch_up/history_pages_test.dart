// SPDX-License-Identifier: Apache-2.0

// Paging up and jumping in a thread (brief M4-20 Design note 1; 15 7.3 item 4): older pages and
// ranges load through REST and join the loaded ranges, so the history shows them in order.

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  ThreadId t1() => ThreadId.parse(threadId(1));

  Future<void> liveWithNewest() async {
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
    await w.dispatch([
      threadCreated(31, 1),
      for (var s = 11; s <= 20; s++) messageCreated(21 + s, s),
    ]);
    expect(await w.visibleSeqs(1), [for (var s = 11; s <= 20; s++) s]);
  }

  test('an older page joins the loaded range below the window', () async {
    await liveWithNewest();
    Map<String, String>? asked;
    w.rest.routes['GET /threads/${threadId(1)}/messages'] = (r) {
      asked = r.query;
      return FakeReply(200, {
        'items': [for (var s = 1; s <= 10; s++) messageJson(1, s)],
        'has_more_before': false,
        'has_more_after': true,
      });
    };
    final more = await w.catchUp.loadOlderMessages(
      t1(),
      before: const ThreadSeq(11),
    );
    expect(asked!['before_seq'], '11');
    expect(more, isFalse);
    expect(await w.visibleSeqs(1), [for (var s = 1; s <= 20; s++) s]);
  });

  test('a range down to a message joins the window', () async {
    await liveWithNewest();
    w.rest.routes['GET /threads/${threadId(1)}/messages'] = (r) =>
        FakeReply(200, {
          'items': [
            for (var s = int.parse(r.query['after_seq']!) + 1; s <= 10; s++)
              messageJson(1, s),
          ],
          'has_more_before': true,
          'has_more_after': true,
        });
    await w.catchUp.loadMessageRange(
      t1(),
      const ThreadSeq(4),
      const ThreadSeq(10),
    );
    expect(await w.visibleSeqs(1), [for (var s = 4; s <= 20; s++) s]);
  });

  test('a page of a removed thread is not stored', () async {
    await liveWithNewest();
    w.rest.routes['GET /threads/${threadId(1)}/messages'] = (_) =>
        FakeReply(200, {
          'items': [for (var s = 1; s <= 10; s++) messageJson(1, s)],
          'has_more_before': false,
          'has_more_after': true,
        });
    await w.dispatch([accessRemoved(60, 1)]);
    await w.catchUp.loadOlderMessages(t1(), before: const ThreadSeq(11));
    expect(await w.storedSeqs(1), isEmpty);
  });
}
