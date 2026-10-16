// SPDX-License-Identifier: Apache-2.0

// Gaps in thread_seq (04 4, 15 7.3 item 4, brief M4-14 Tests): thread_seq 10 then 13 loads 11
// to 12 through REST before 13 is visible; a fetch error keeps the gap recorded.

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  ThreadId t1() => ThreadId.parse(threadId(1));

  Future<void> liveWithTen() async {
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
    await w.dispatch([
      threadCreated(31, 1),
      for (var s = 1; s <= 10; s++) messageCreated(31 + s, s),
    ]);
    expect(await w.visibleSeqs(1), [for (var s = 1; s <= 10; s++) s]);
  }

  test('10 then 13 loads 11 to 12 before 13 is visible', () async {
    await liveWithTen();
    w.rest.routes['GET /threads/${threadId(1)}/messages'] = (_) =>
        FakeReply(200, {
          'items': [for (var s = 11; s <= 13; s++) messageJson(1, s)],
          'has_more_before': true,
          'has_more_after': false,
        });
    await w.dispatch([messageCreated(50, 13)]);
    expect(w.rest.requests.map((r) => r.path), [
      '/threads/${threadId(1)}/messages',
    ]);
    expect(await w.storedSeqs(1), [for (var s = 1; s <= 13; s++) s]);
    expect(await w.visibleSeqs(1), [for (var s = 1; s <= 13; s++) s]);
    expect(w.catchUp.gaps.pending, isEmpty);
  });

  test('the range request asks after the last loaded position', () async {
    await liveWithTen();
    Map<String, String>? asked;
    w.rest.routes['GET /threads/${threadId(1)}/messages'] = (r) {
      asked = r.query;
      return FakeReply(200, {
        'items': [for (var s = 11; s <= 13; s++) messageJson(1, s)],
        'has_more_before': true,
        'has_more_after': false,
      });
    };
    await w.dispatch([messageCreated(50, 13)]);
    expect(asked!['after_seq'], '10');
  });

  test('a fetch error keeps the gap recorded and the message hidden', () async {
    await liveWithTen();
    w.rest.routes['GET /threads/${threadId(1)}/messages'] = (_) =>
        FakeReply.problem(503, 'unavailable');
    await w.dispatch([messageCreated(50, 13)]);
    expect(await w.storedSeqs(1), contains(13));
    expect(await w.visibleSeqs(1), [for (var s = 1; s <= 10; s++) s]);
    expect(
      w.catchUp.gaps.pending[t1()],
      SeqRange(const ThreadSeq(11), const ThreadSeq(13)),
    );
    // The next focus retries.
    w.rest.routes['GET /threads/${threadId(1)}/messages'] = (_) =>
        FakeReply(200, {
          'items': [for (var s = 11; s <= 13; s++) messageJson(1, s)],
          'has_more_before': true,
          'has_more_after': false,
        });
    await w.catchUp.onFocus(t1());
    await w.idle();
    expect(w.catchUp.gaps.pending, isEmpty);
    expect(await w.visibleSeqs(1), [for (var s = 1; s <= 13; s++) s]);
  });
}
