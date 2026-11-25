// SPDX-License-Identifier: Apache-2.0

// The steps of the outbox that nobody awaits (a send, the next pump) log a failure through the
// log hook instead of throwing it into the zone as an unhandled error, for example when the
// local data is deleted and the database closes while a request runs.

import 'dart:async';

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('a send that ends on a closed database is logged, not thrown', () async {
    final errors = <Object>[];
    final logs = <String>[];
    await runZonedGuarded(() async {
      final h = Harness.open(log: logs.add);
      final gate = Completer<OutboxResponse>();
      h.fake('createMessage').then([gate]);
      await h.add('hello', thread: threadA);
      await h.outbox.start();
      await settle();
      await h.db.close();
      gate.complete(const OutboxResponse());
      await settle();
    }, (error, _) => errors.add(error));
    expect(errors, isEmpty);
    expect(logs, isNotEmpty);
  });
}
