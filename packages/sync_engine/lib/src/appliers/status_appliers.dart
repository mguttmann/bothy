// SPDX-License-Identifier: Apache-2.0

/// Applier of the durable event `status.updated` (docs/spec/16-operations.md 12; brief M7-21):
/// it writes no table, because the status view reads `getStatus` again, and announces the
/// change on the [StatusEventsHub] after the commit of the page, so a page that rolls back
/// announces nothing.
library;

import '../dispatcher.dart';
import '../status_events.dart';
import '../wire.dart';

/// The appliers of this group by event type.
final Map<String, Applier> statusAppliers = {
  'status.updated': (c) async {
    final event = StatusUpdated(overallState: str(c.payload, 'overall_state'));
    c.after.steps.add(() => c.sync.status.add(event));
  },
};
