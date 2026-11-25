// SPDX-License-Identifier: Apache-2.0

// A failed read state or draft write leaves no entry behind: no view shows it, so a kept row
// would only grow the outbox (and keep the draft text) for good. A failed message stays for
// the person to retry or discard.

import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late Harness h;
  setUp(() => h = Harness.open());
  tearDown(() => h.close());

  for (final kind in [
    PersonalOps.putReadState,
    PersonalOps.putThreadDraft,
    PersonalOps.deleteThreadDraft,
  ]) {
    test('a failed $kind is removed', () async {
      h.fake(kind).then([problem(422, 'validation_failed')]);
      final e = await h.outbox.enqueue(
        OutboxRequest(kind: kind, threadId: threadA, payload: const {}),
      );
      await h.outbox.start();
      await settle();
      expect(await h.entry(e.id), isNull);
    });
  }

  test('a failed message stays for the person', () async {
    h.fake('createMessage').then([problem(422, 'validation_failed')]);
    final e = await h.add('hello', thread: threadA);
    await h.outbox.start();
    await settle();
    expect((await h.entry(e.id))!.state, OutboxState.failed);
  });
}
