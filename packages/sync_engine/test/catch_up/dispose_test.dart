// SPDX-License-Identifier: Apache-2.0

// Disposing never waits for a paused listener (docs/DECISIONS.md D-M4-19-14): a provider that
// nobody watches pauses its stream, and the close of a broadcast stream would wait for it, so
// the engine, and with it the database, would stay open.

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
}
