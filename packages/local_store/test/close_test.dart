// SPDX-License-Identifier: Apache-2.0

// Closing the database releases it even while its watched queries come and go (R-SY-14: the
// wipe of 4012 and logout closes, then deletes the files; Windows refuses to delete an open
// file). drift's stream store iterates its active streams while it awaits each close; a stream
// that closes or opens meanwhile ended that loop with a ConcurrentModificationError before the
// executor closed, so the file stayed open (reproduced on the Windows runner after M4-40).

import 'dart:async';

import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

void main() {
  test('close releases the executor while a watched query goes away', () async {
    final db = openInMemoryForTests();
    final subs = [
      db.select(db.spaces).watch().listen((_) {}),
      db.select(db.channels).watch().listen((_) {}),
      db.select(db.threads).watch().listen((_) {}),
      db.select(db.runs).watch().listen((_) {}),
    ];
    // Let every stream run its first query, so all are active in the store.
    await pumpEventQueue();
    // A listener goes right before the close, as the widgets of a screen do when a 4012 ends
    // the account: drift removes its stream from the store on a timer, which fires while the
    // store awaits the close of another stream.
    // A paused listener (riverpod pauses the providers nobody watches) holds the close of its
    // stream until it resumes, so the timer fires inside the loop.
    subs[1].pause();
    await subs.first.cancel();
    final closing = db.close();
    await pumpEventQueue();
    subs[1].resume();
    await expectLater(closing, completes);
    // The executor is closed: a query fails instead of running.
    await expectLater(db.select(db.spaces).get(), throwsA(anything));
    for (final s in subs.skip(1)) {
      await s.cancel();
    }
  });

  // A widget test runs in fake time: a timer made there fires only when the test advances it.
  // drift removes a stream that lost its last listener on a timer and its close waits for that
  // timer, so a stream that ended in the fake time of a finished test held the close forever
  // (issue #238, run 40975481761: tearDownAll of new_chat_golden_test.dart timed out).
  test('with closeStreamsSynchronously a watch that ends starts no timer and close completes', () async {
    final db = openInMemoryForTests(closeStreamsSynchronously: true);
    final sub = db.select(db.spaces).watch().listen((_) {});
    await pumpEventQueue();
    final timers = <Duration>[];
    // A zone whose timers never fire, like the fake time of a test that has ended.
    await runZoned(
      sub.cancel,
      zoneSpecification: ZoneSpecification(
        createTimer: (self, parent, zone, duration, f) {
          timers.add(duration);
          return parent.createTimer(zone, const Duration(days: 1), () {});
        },
      ),
    );
    expect(timers, isEmpty);
    await expectLater(
      db.close().timeout(const Duration(seconds: 10)),
      completes,
    );
  });

  test(
    'without the option the close waits for the timer of a watch that ended',
    () async {
      final db = openInMemoryForTests();
      final sub = db.select(db.spaces).watch().listen((_) {});
      await pumpEventQueue();
      final pending = <void Function()>[];
      // The timers of this zone run only when the test calls them, like fake time.
      await runZoned(
        sub.cancel,
        zoneSpecification: ZoneSpecification(
          createTimer: (self, parent, zone, duration, f) {
            pending.add(() => zone.run(f));
            return parent.createTimer(zone, const Duration(days: 1), () {});
          },
        ),
      );
      expect(pending, hasLength(1));
      var closed = false;
      final closing = db.close().then((_) => closed = true);
      await pumpEventQueue();
      expect(
        closed,
        isFalse,
        reason: 'the close waits for the timer of the ended watch',
      );
      pending.single();
      await expectLater(
        closing.timeout(const Duration(seconds: 10)),
        completes,
      );
    },
  );
}
