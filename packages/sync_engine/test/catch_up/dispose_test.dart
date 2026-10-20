// SPDX-License-Identifier: Apache-2.0

// Disposing never waits for a paused listener (docs/DECISIONS.md D-M4-19-14): a provider that
// nobody watches pauses its stream, and the close of a broadcast stream would wait for it, so
// the engine, and with it the database, would stay open.

import 'dart:async';

import 'package:test/test.dart';

import 'support.dart';

void main() {
  test(
    'catch-up disposes while its counter and notice listeners are paused',
    () async {
      final w = World();
      final counts = w.catchUp.countChanges.listen((_) {})..pause();
      final notices = w.catchUp.notices.listen((_) {})..pause();
      await w.dispose().timeout(const Duration(seconds: 5));
      await counts.cancel();
      await notices.cancel();
    },
  );

  test(
    'catch-up disposes while a background read still waits for its answer',
    () async {
      final w = World();
      // The read of the instance after the snapshot never gets an answer.
      w.rest.gates['GET /instance'] = Completer<void>().future;
      await w.h.open();
      w.ch.push('hello', helloData());
      await settle();
      final ready = readyData(headSeq: 40);
      ready['snapshot'] = snapshotJson(headSeq: 40);
      w.ch.push('ready', ready);
      await settle();
      await w.dispose().timeout(const Duration(seconds: 5));
    },
  );
}
