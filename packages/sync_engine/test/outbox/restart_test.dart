// SPDX-License-Identifier: Apache-2.0

// Entries survive closing and reopening the database and are sent again with their stored IDs
// (04 13.4, 15 7.5).

import 'dart:async';
import 'dart:io';
import 'dart:typed_data';

import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

final class _Keys implements DatabaseKeySource {
  @override
  Future<Uint8List> obtainKey() async =>
      Uint8List.fromList(List.filled(32, 0x11));
}

void main() {
  test('pending, waiting and sending entries are sent again; confirmed ones are not', () async {
    final dir = await Directory.systemTemp.createTemp('sync_engine_outbox_');
    addTearDown(() => dir.delete(recursive: true));
    final file = File('${dir.path}/store.db');
    final time = FakeTime();

    var db = await openLocalStore(
      file: file,
      keys: _Keys(),
      inBackground: false,
      clock: time.clock,
    );
    var h = Harness.open(db: db, time: time);
    final exec = h.fake('createMessage');
    final held = Completer<OutboxResponse>();
    exec.then([
      const OutboxResponse(),
      problem(503, 'unavailable', retryAfter: const Duration(minutes: 1)),
      held,
    ]);
    final confirmed = await h.add('confirmed', thread: threadA);
    final waiting = await h.add('waiting', thread: threadB);
    final sending = await h.add('sending', thread: threadA);
    final pending = await h.add('pending', thread: threadA);
    await h.outbox.start();
    await settle();
    expect(exec.ids, [confirmed.id, waiting.id, sending.id]);
    expect((await h.entry(waiting.id))!.state, OutboxState.waiting);
    expect((await h.entry(sending.id))!.state, OutboxState.sending);
    expect((await h.entry(pending.id))!.state, OutboxState.pending);

    // The app stops while `sending` is in flight.
    await h.outbox.pause();
    await db.close();

    db = await openLocalStore(
      file: file,
      keys: _Keys(),
      inBackground: false,
      clock: time.clock,
    );
    h = Harness.open(db: db, time: time);
    final after = h.fake('createMessage');
    expect(await h.entry(confirmed.id), isNull);
    await h.outbox.start();
    await settle();
    expect(after.ids, [
      sending.id,
      pending.id,
    ], reason: 'FIFO in thread A, stored IDs');
    expect(after.calls.first.idempotencyKey, sending.id);
    await h.time.advance(const Duration(minutes: 1));
    expect(after.ids, [
      sending.id,
      pending.id,
      waiting.id,
    ], reason: 'waiting keeps its time');
    expect(await h.db.outboxDao.pendingThreads(), isEmpty);
    expect(after.ids, isNot(contains(confirmed.id)));
    // `held` stays open: the first outbox is gone with its database.
    await h.close();
  });

  test(
    'start resets a sending entry to pending once and keeps its attempts',
    () async {
      final h = Harness.open();
      addTearDown(h.close);
      final e = await h.add('hello', thread: threadA);
      await h.db.outboxDao.markState(e.id, OutboxState.sending);
      final exec = h.fake('createMessage')..then([Completer<OutboxResponse>()]);
      await h.outbox.start();
      await settle();
      expect(exec.ids, [e.id]);
      expect((await h.entry(e.id))!.attempts, 2);
    },
  );
}
