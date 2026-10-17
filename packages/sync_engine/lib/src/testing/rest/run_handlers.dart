// SPDX-License-Identifier: Apache-2.0

/// Handlers of `getRun` and `cancelRun` (04 11, 05 `run.cancel`).
library;

import '../rest_fake.dart';
import '../world.dart';

/// Registers the run handlers on [rest].
///
/// `cancelRun` needs `trigger` in the channel and the triggering person (the fake has no
/// `manage`); an ended run answers `409 invalid_state_transition`. The fake ends the run at
/// once with `cancelled` instead of waiting for the agent (`agent.cancel_grace`).
void registerRunHandlers(RestFake rest) {
  final world = rest.world;

  JsonMap visibleRun(FakeRequest r) {
    final run = world.runs[r.param('run_id')];
    if (run == null ||
        !world.seesChannel(r.person.id, run['channel_id']! as String)) {
      throw const FakeProblem(404, 'not_found');
    }
    return run;
  }

  rest
    ..handle('getRun', (r) {
      final run = visibleRun(r);
      return FakeResponse(200, run, {'ETag': '"${run['version']}"'});
    })
    ..handle('cancelRun', (r) {
      final run = visibleRun(r);
      if (world.levelOf(r.person.id, run['channel_id']! as String) !=
              'trigger' ||
          run['triggered_by'] != r.person.id) {
        throw const FakeProblem(403, 'forbidden');
      }
      if (world.activeRunOf(run['thread_id']! as String)?['id'] != run['id']) {
        throw const FakeProblem(409, 'invalid_state_transition');
      }
      world.setRunState(
        run['id']! as String,
        'cancelled',
        triggerState: 'cancelled',
        reason: r.json['reason'] as String? ?? 'cancelled_by_user',
      );
      return FakeResponse(202, world.runSummary(run));
    });
}
