// SPDX-License-Identifier: Apache-2.0

/// Handlers of `getRun`, `cancelRun` (04 11, 05 `run.cancel`) and `retryRun` (09 5.7).
library;

import '../rest_fake.dart';
import '../world.dart';

/// Registers the run handlers on [rest].
///
/// `cancelRun` needs `trigger` in the channel and the triggering person (the fake has no
/// `manage`); an ended run answers `409 invalid_state_transition`. The fake ends the run at
/// once with `cancelled` instead of waiting for the agent (`agent.cancel_grace`).
///
/// `retryRun` needs `trigger` in the channel (`agent.invoke`, else `403 agent_invocation_denied`
/// with `reason` `no_trigger_right`); only the newest run of its thread, triggered by a message,
/// in `failed` or `cancelled` qualifies (`409 invalid_state_transition` otherwise). It opens a
/// queued run of the same agent for the same message with the caller as `triggered_by`; when the
/// newest run is already such a retry it answers that run (idempotent). The fake knows no thread
/// operator rule (09 3.4 step 4a): a test that needs `operator_not_in_channel` scripts it with
/// `faults.onRequest`.
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
    })
    ..handle('retryRun', (r) {
      final run = visibleRun(r);
      if (world.levelOf(r.person.id, run['channel_id']! as String) !=
          'trigger') {
        throw const FakeProblem(
          403,
          'agent_invocation_denied',
          reason: 'no_trigger_right',
        );
      }
      JsonMap? newest;
      for (final x in world.runs.values) {
        if (x['thread_id'] == run['thread_id']) newest = x;
      }
      if (newest != null && newest['id'] != run['id']) {
        if (newest['triggering_message_id'] == run['triggering_message_id'] &&
            newest['agent_id'] == run['agent_id']) {
          return FakeResponse(202, world.runSummary(newest));
        }
        throw const FakeProblem(409, 'invalid_state_transition');
      }
      final message = run['triggering_message_id'] as String?;
      if (run['trigger_kind'] != 'message' ||
          message == null ||
          (run['state'] != 'failed' && run['state'] != 'cancelled')) {
        throw const FakeProblem(409, 'invalid_state_transition');
      }
      final next = world.createRun(
        agentId: run['agent_id']! as String,
        triggeringMessageId: message,
        triggeredBy: r.person.id,
        threadId: run['thread_id']! as String,
      );
      return FakeResponse(202, world.runSummary(next));
    });
}
