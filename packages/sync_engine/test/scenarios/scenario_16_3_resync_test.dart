// SPDX-License-Identifier: Apache-2.0

// 04 16.3, the resync_required branch, against SyncEngine with its local database: the cursor
// fell out of the retained log; the outbox stays, the snapshot replaces the mirror and the
// connection stays live (brief M4-17, Tests).

import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test(
    '16.3 resync_required: snapshot, outbox kept, live without reconnect',
    () async {
      await withScenario(scenarioResync);
    },
  );
}
