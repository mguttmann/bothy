// SPDX-License-Identifier: Apache-2.0

// 04 16.4 against SyncEngine with its local database: access to a channel is removed while connected (brief M4-17, Tests).

import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('16.4 access removal during an open connection', () async {
    await withScenario(scenarioAccessRemoval);
  });
}
