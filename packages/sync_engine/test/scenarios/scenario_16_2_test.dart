// SPDX-License-Identifier: Apache-2.0

// 04 16.2 against SyncEngine with its local database: a message written offline is sent after
// the catch-up; the lost response is repeated with the same client_msg_id (brief M4-17, Tests).

import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test(
    '16.2 send offline, reconnect, lost response, exactly one message',
    () async {
      await withScenario(scenarioSendOffline);
    },
  );
}
