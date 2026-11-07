// SPDX-License-Identifier: Apache-2.0

/// Handlers of `listThreadWakes`, `cancelRunWake` (09 5.8.2, 07 2.14 item 5) and
/// `setChannelAutonomousRuns` (07 2.14 item 1) of stage AR (brief AR-08).
library;

import '../rest_fake.dart';
import '../world.dart';

/// Registers the wake-up handlers on [rest].
///
/// `listThreadWakes` answers every wake-up of the thread, newest first, in one page, for
/// everyone who sees the thread (`404` otherwise). `cancelRunWake` is allowed for the person
/// of the wake-up and a person with `manage` on the channel (`403` otherwise; the fake has no
/// agent caller); a pending wake-up ends `cancelled` with `cancelled_by_person` and its note,
/// an ended one answers itself. `setChannelAutonomousRuns` needs `manage` (`403`), `If-Match`
/// (`428`, `412`) and refuses a DM (`409 conflict`, `dm_channel`).
void registerWakeHandlers(RestFake rest) {
  final world = rest.world;

  JsonMap visibleWake(FakeRequest r) {
    final wake = world.wakes[r.param('wake_id')];
    if (wake == null ||
        !world.seesThread(r.person.id, wake['thread_id']! as String)) {
      throw const FakeProblem(404, 'not_found');
    }
    return wake;
  }

  rest
    ..handle('listThreadWakes', (r) {
      final threadId = r.param('thread_id');
      world.visibleThread(r.person.id, threadId);
      return FakeResponse(200, {'items': world.wakesOf(threadId)});
    })
    ..handle('cancelRunWake', (r) {
      final wake = visibleWake(r);
      final channelId =
          world.threads[wake['thread_id']]!['channel_id']! as String;
      if (wake['triggered_by'] != r.person.id &&
          !world.manages(r.person.id, channelId)) {
        throw const FakeProblem(403, 'forbidden');
      }
      return FakeResponse(
        200,
        world.endWake(
          wake['id']! as String,
          'cancelled',
          reason: 'cancelled_by_person',
        ),
      );
    })
    ..handle('setChannelAutonomousRuns', (r) {
      final channelId = r.param('channel_id');
      final channel = world.channels[channelId];
      if (channel == null || !world.seesChannel(r.person.id, channelId)) {
        throw const FakeProblem(404, 'not_found');
      }
      if (!world.manages(r.person.id, channelId)) {
        throw const FakeProblem(403, 'forbidden');
      }
      if (channel.json['kind'] == 'dm') {
        throw const FakeProblem(409, 'conflict', reason: 'dm_channel');
      }
      final match = RegExp(r'^"?(\d+)"?$')
          .firstMatch(r.headers['if-match'] ?? '');
      if (match == null) throw const FakeProblem(428, 'precondition_required');
      if (int.parse(match[1]!) != channel.json['version']) {
        throw FakeProblem(412, 'version_conflict', current: channel.json);
      }
      final json = world.setAutonomousRuns(
        channelId,
        allowed: r.json['allow_autonomous_runs']! as bool,
      );
      return FakeResponse(200, json, {'ETag': '"${json['version']}"'});
    });
}
