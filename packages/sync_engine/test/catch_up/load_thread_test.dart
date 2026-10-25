// SPDX-License-Identifier: Apache-2.0

// A thread the device never received an event for is loaded through REST (15 11: a target is
// loaded locally or through REST): a device that signs in after a channel was created never sees
// the `thread.created` of its main thread, so the view asks for it (found by the desktop E2E
// suite of M4-34 against the real server).

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  test('loadThread stores the thread by the version rule and indexes its '
      'title; never for a channel this device does not hold', () async {
    await w.connect();
    w.rest.routes['GET /me/read-states'] = (_) =>
        const FakeReply(200, {'items': <Object?>[]});
    await w.idle();
    w.rest.routes['GET /threads/${threadId(7)}'] = (_) =>
        FakeReply(200, threadJson(7, title: 'Printer jam', version: 2));
    await w.catchUp.loadThread(ThreadId.parse(threadId(7)));
    expect(
      (await w.db.threadsDao.byId(ThreadId.parse(threadId(7))))?.title,
      'Printer jam',
    );
    expect(await w.db.searchDao.search('printer', limit: 10), isNotEmpty);

    // An older version changes nothing.
    w.rest.routes['GET /threads/${threadId(7)}'] = (_) =>
        FakeReply(200, threadJson(7, title: 'Old', version: 1));
    await w.catchUp.loadThread(ThreadId.parse(threadId(7)));
    expect(
      (await w.db.threadsDao.byId(ThreadId.parse(threadId(7))))?.title,
      'Printer jam',
    );

    // A thread of a channel this device does not hold is not stored.
    w.rest.routes['GET /threads/${threadId(9)}'] = (_) =>
        FakeReply(200, threadJson(9, channelN: 9));
    await w.catchUp.loadThread(ThreadId.parse(threadId(9)));
    expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(9))), isNull);

    // A refused read throws, so the caller can try again.
    w.rest.routes['GET /threads/${threadId(8)}'] = (_) =>
        const FakeReply(404, {'code': 'not_found'});
    await expectLater(
      w.catchUp.loadThread(ThreadId.parse(threadId(8))),
      throwsA(anything),
    );
  });
}
