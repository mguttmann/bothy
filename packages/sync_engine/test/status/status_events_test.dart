// SPDX-License-Identifier: Apache-2.0

// The status page events (brief M7-21 Design note 1; 16 12): `status.updated` reaches the hub
// of the engine after the commit of its page, with the overall state only; a duplicate
// announces nothing, and nothing is stored.

import 'dart:async';

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import '../catch_up/support.dart';

Map<String, Object?> _statusUpdated(int seq, Map<String, Object?> payload) =>
    envelope(
      'status.updated',
      seq: seq,
      payload: payload,
      resourceKind: 'instance',
    );

void main() {
  late World w;
  late List<StatusUpdated> events;
  late StreamSubscription<StatusUpdated> sub;
  setUp(() async {
    w = World();
    events = [];
    sub = w.catchUp.statusUpdates.listen(events.add);
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
  });
  tearDown(() async {
    await sub.cancel();
    await w.dispose();
  });

  test('status.updated reaches the hub with the overall state', () async {
    await w.dispatch([
      _statusUpdated(31, {
        'overall_state': 'degraded',
        'changed_component_ids': [id(0x900)],
      }),
    ]);
    expect(events, [const StatusUpdated(overallState: 'degraded')]);
  });

  test('a duplicate below the cursor reaches nothing', () async {
    await w.dispatch([
      _statusUpdated(12, {'overall_state': 'major_outage'}),
    ]);
    expect(events, isEmpty);
  });

  test('status.updated is applied, not skipped', () {
    expect(appliedEventTypes, contains('status.updated'));
  });
}
