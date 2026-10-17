// SPDX-License-Identifier: Apache-2.0

// 04 16.1 against SyncEngine with its local database: a message sent online; the echo confirms
// the outbox entry before the response arrives (brief M4-17, Tests).

import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('16.1 send online: one message, the echo confirms the entry', () async {
    await withScenario(scenarioSendOnline);
  });
}
