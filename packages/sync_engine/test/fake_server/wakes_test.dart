// SPDX-License-Identifier: Apache-2.0

// The wake-ups of the fake (brief AR-08 Tests, 09 5.8, 07 2.14): `listThreadWakes` for every
// reader of the thread, `cancelRunWake` for the wake-up's person and a person with `manage` on
// the channel only, the note of an end other than `fired` (origin `system`, never a person's
// message) with `run.wake_changed` naming it, a wake run with its `triggering_wake_id`, and
// `setChannelAutonomousRuns` with `manage` and `If-Match`. Every response passes the contract
// check.

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  late WorldSeed w;
  late ApiClient alexApi;
  late ApiClient samApi;

  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
    w = h.seed;
    final alex = h.device();
    await signedIn(h, alex);
    final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
    await signedIn(h, sam);
    alexApi = alex.api;
    samApi = sam.api;
  });
  tearDown(() async {
    expect(h.server.violations, isEmpty);
    await h.stop();
  });

  Future<int> status(Future<Object?> call) => call.then(
    (_) => 200,
    onError: (Object e) => (e as DioException).response!.statusCode!,
  );

  /// A pending wake-up of Robin for Alex in Release notes.
  JsonMap pending({String label = 'CI of #123'}) {
    final trigger = w.post(w.alex, w.releaseNotes, '@robin watch the checks');
    final run = world.createRun(
      agentId: w.robin.id,
      triggeringMessageId: trigger['id']! as String,
      triggeredBy: w.alex.id,
    );
    return world.addWake(
      threadId: w.releaseNotes,
      agentId: w.robin.id,
      sourceRunId: run['id']! as String,
      triggeredBy: w.alex.id,
      kind: 'pr_checks',
      label: label,
      condition: {'repository': 'example/app', 'number': 123},
    );
  }

  List<JsonMap> eventsOf(String type) => [
    for (final e in world.log)
      if (e.envelope['type'] == type) e.envelope,
  ];

  test('listThreadWakes lists the wake-ups of a thread, newest first, for '
      'its readers', () async {
    final first = pending(label: 'first');
    final second = pending(label: 'second');
    final page = (await samApi.getRunsApi().listThreadWakes(
      threadId: w.releaseNotes,
    )).data!;
    expect([for (final x in page.items) x.id], [second['id'], first['id']]);
    expect(page.items.first.label, 'second');
    expect(page.items.first.state, RunWakeState.pending);
    // A person who cannot read the thread gets 404: Alex does not see the private channel.
    expect(
      await status(alexApi.getRunsApi().listThreadWakes(threadId: w.budget)),
      404,
    );
  });

  test('a new wake-up and its end come as run.wake_changed; the end posts a '
      'system note named in the event', () async {
    final wake = pending();
    final created = eventsOf('run.wake_changed').single;
    expect(created['payload'], containsPair('state', 'pending'));
    expect(created['payload'], containsPair('wake_id', wake['id']));

    world.endWake(wake['id']! as String, 'expired', reason: 'deadline_passed');
    final ended = eventsOf('run.wake_changed').last['payload']! as JsonMap;
    expect(ended['state'], 'expired');
    expect(ended['state_reason'], 'deadline_passed');
    final note = world.messages[ended['note_message_id']]!;
    expect(note['origin'], 'system');
    expect(note['kind'], 'text');
    expect(note['content_md'], startsWith("Wake-up 'CI of #123' expired"));
    expect(ended['note_thread_seq'], note['thread_seq']);
  });

  test('cancelRunWake: the person of the wake-up and a person with manage; '
      'anyone else gets 403', () async {
    final wake = pending();
    final id = wake['id']! as String;
    // Sam may read the thread but is neither the person nor holds manage.
    expect(
      await status(
        samApi.getRunsApi().cancelRunWake(
          wakeId: id,
          runWakeCancel: RunWakeCancel(),
        ),
      ),
      403,
    );
    final cancelled = (await alexApi.getRunsApi().cancelRunWake(
      wakeId: id,
      runWakeCancel: RunWakeCancel(),
    )).data!;
    expect(cancelled.state, RunWakeState.cancelled);
    expect(cancelled.stateReason, 'cancelled_by_person');
    // An ended wake-up answers itself without a change.
    final again = (await alexApi.getRunsApi().cancelRunWake(
      wakeId: id,
      runWakeCancel: RunWakeCancel(),
    )).data!;
    expect(again.version, cancelled.version);

    final other = pending()['id']! as String;
    world.channels[w.general]!.capabilities[w.sam.id] = {'manage'};
    final bySam = (await samApi.getRunsApi().cancelRunWake(
      wakeId: other,
      runWakeCancel: RunWakeCancel(),
    )).data!;
    expect(bySam.state, RunWakeState.cancelled);
  });

  test('a fired wake-up starts a wake run that names it', () async {
    final wake = pending();
    final run = world.fireWake(wake['id']! as String);
    expect(run['trigger_kind'], 'wake');
    expect(run['triggering_wake_id'], wake['id']);
    final listed = (await alexApi.getRunsApi().listThreadWakes(
      threadId: w.releaseNotes,
    )).data!.items.single;
    expect(listed.state, RunWakeState.fired);
    expect(listed.firedRunId, run['id']);
    final doc = (await alexApi.getRunsApi().getRun(runId: run['id']! as String))
        .data!;
    expect(doc.triggeringWakeId, wake['id']);
  });

  test('setChannelAutonomousRuns needs manage and If-Match and sends '
      'channel.updated', () async {
    final channel = world.channels[w.general]!;
    expect(channel.json['allow_autonomous_runs'], isFalse);
    Future<int> set(ApiClient api, int version) => status(
      api.getChannelsApi().setChannelAutonomousRuns(
        channelId: w.general,
        ifMatch: '"$version"',
        channelAutonomousRunsSet: ChannelAutonomousRunsSet(
          (b) => b..allowAutonomousRuns = true,
        ),
      ),
    );
    final version = channel.json['version']! as int;
    expect(await set(alexApi, version), 403);
    channel.capabilities[w.alex.id] = {'manage'};
    expect(await set(alexApi, version - 1), 412);
    expect(await set(alexApi, version), 200);
    expect(channel.json['allow_autonomous_runs'], isTrue);
    expect(channel.json['version'], version + 1);
    final updated = eventsOf('channel.updated').last['payload']! as JsonMap;
    expect((updated['channel']! as JsonMap)['allow_autonomous_runs'], isTrue);
  });

  test('a channel of an older server has no switch', () {
    world.withoutAutonomousRunsSwitch();
    for (final c in world.channels.values) {
      expect(c.json.containsKey('allow_autonomous_runs'), isFalse);
    }
  });
}
