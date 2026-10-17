// SPDX-License-Identifier: Apache-2.0

// 04 16.3 against SyncEngine with its local database: a reconnect catches up from the cursor in pages; hidden events are skipped (brief M4-17, Tests).

import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('16.3 reconnect with catch-up in pages with cursor frames', () async {
    await withScenario(scenarioCatchUp);
  });
}
