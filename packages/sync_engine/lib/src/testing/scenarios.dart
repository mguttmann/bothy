// SPDX-License-Identifier: Apache-2.0

/// The five flows of 04 section 16 as reusable drivers (brief M4-17 Design note 7): each takes
/// a [ScenarioClient] (the sync engine headless here, the app from M4-18 on) and a running
/// [FakeServer] with the standard world, drives the flow and checks the outcome.
library;

import 'dart:async';

import 'fake_server.dart';
import 'scripted_agent.dart';
import 'seed.dart';
import 'world.dart';

/// A message as the client under test holds it locally.
final class ScenarioMessage {
  /// Creates the view.
  const ScenarioMessage({
    required this.id,
    required this.clientMsgId,
    required this.threadSeq,
    required this.text,
    required this.authorId,
    this.triggerState,
  });

  /// The server ID.
  final String id;

  /// The `client_msg_id`.
  final String clientMsgId;

  /// The position in the thread.
  final int threadSeq;

  /// The Markdown text.
  final String text;

  /// The author.
  final String authorId;

  /// The `trigger_state`, if any.
  final String? triggerState;
}

/// An outbox entry of the client under test that is not confirmed yet.
final class ScenarioOutboxEntry {
  /// Creates the view.
  const ScenarioOutboxEntry({
    required this.id,
    required this.failed,
    this.code,
  });

  /// The entry ID (the `client_msg_id` of a message).
  final String id;

  /// Whether the entry failed finally.
  final bool failed;

  /// The failure code, if any.
  final String? code;
}

/// What a scenario does with the client under test (Design note 7).
///
/// The headless implementation runs over `SyncEngine` (`test/scenarios/`); M4-18 implements
/// it over the app. Notices use the forms `removed:<kind>:<id>`, `resynced:<reason>` and
/// `signed_out`.
abstract interface class ScenarioClient {
  /// Signs in as Alex of the standard world and connects; completes when the connection is
  /// live.
  Future<void> start();

  /// Puts a message with [text] for [threadId] into the outbox (mentioning [mentionIds]) and
  /// returns its `client_msg_id`.
  Future<String> send(String threadId, String text, {List<String> mentionIds});

  /// Loses the network: the gateway connection ends and nothing is sent until [goOnline].
  Future<void> goOffline();

  /// The network is back; the client reconnects.
  Future<void> goOnline();

  /// The messages of [threadId] in the local database, in `thread_seq` order.
  Future<List<ScenarioMessage>> messages(String threadId);

  /// The outbox entries that are not confirmed.
  Future<List<ScenarioOutboxEntry>> outbox();

  /// Whether the local database holds the channel [channelId].
  Future<bool> holdsChannel(String channelId);

  /// The notices of the client so far.
  List<String> get notices;

  /// Whether the connection is live.
  bool get live;

  /// The stored cursor, or null.
  Future<int?> cursor();

  /// Shows [threadId] on screen (gateway focus).
  Future<void> focusThread(String threadId);

  /// Focuses the live box of [runId] in [threadId].
  Future<void> focusRun(String runId, String threadId);

  /// The run document the live box shows now, or null.
  Map<String, Object?>? runDocument(String runId);

  /// The state of [runId] in the local run summaries, or null.
  Future<String?> runState(String runId);

  /// Disconnects and releases everything.
  Future<void> stop();
}

/// A scenario check that failed; [message] says what was expected.
final class ScenarioFailure implements Exception {
  /// Creates the failure.
  const ScenarioFailure(this.message);

  /// What was expected.
  final String message;

  @override
  String toString() => 'ScenarioFailure: $message';
}

/// Waits until [condition] holds, polling every few milliseconds; throws [ScenarioFailure]
/// naming [what] after [timeout].
Future<void> waitFor(
  String what,
  FutureOr<bool> Function() condition, {
  Duration timeout = const Duration(seconds: 15),
}) async {
  final end = DateTime.now().add(timeout);
  while (!await condition()) {
    if (DateTime.now().isAfter(end)) {
      throw ScenarioFailure('timed out waiting for $what');
    }
    await Future<void>.delayed(const Duration(milliseconds: 5));
  }
}

void _check(bool ok, String message) {
  if (!ok) throw ScenarioFailure(message);
}

void _contract(FakeServer server) {
  final v = server.violations;
  _check(v.isEmpty, 'contract violations: ${v.join(' | ')}');
}

/// The messages of [client] in [threadId] with [clientMsgId].
Future<List<ScenarioMessage>> _withId(
  ScenarioClient client,
  String threadId,
  String clientMsgId,
) async => [
  for (final m in await client.messages(threadId))
    if (m.clientMsgId == clientMsgId) m,
];

/// 04 16.1: a message sent online; its echo confirms the outbox entry while the response is
/// still on its way (the fake holds it), and exactly one message exists on both sides.
Future<void> scenarioSendOnline(
  FakeServer server,
  WorldSeed w,
  ScenarioClient client,
) async {
  await client.start();
  server.faults.delay(const Duration(seconds: 2), operationId: 'createMessage');
  final id = await client.send(w.releaseNotes, 'Hello Sam');
  await waitFor(
    'the echo of $id',
    () async => (await _withId(client, w.releaseNotes, id)).isNotEmpty,
  );
  await waitFor(
    'the confirmed outbox entry',
    () async => (await client.outbox()).isEmpty,
  );
  _check(
    server.requestsOf('createMessage').isEmpty,
    'the echo, not the response, confirmed the entry: '
    '${[for (final r in server.requestsOf('createMessage')) r.status]}',
  );
  await waitFor(
    'the delayed response',
    () => server.requestsOf('createMessage').length == 1,
  );
  _check(
    server.requestsOf('createMessage').single.status == 201,
    'the response was 201',
  );
  _check(
    (await _withId(client, w.releaseNotes, id)).length == 1,
    'exactly one local message',
  );
  _check(
    server.world
            .messagesOf(w.releaseNotes)
            .where((m) => m['client_msg_id'] == id)
            .length ==
        1,
    'exactly one message on the server',
  );
  _contract(server);
}

/// 04 16.2: a message written offline is sent after the reconnect and its catch-up; the
/// first response is lost after the commit, the retry with the same `client_msg_id` is a
/// replay (`200`, `Idempotent-Replay: true`), and exactly one message exists.
Future<void> scenarioSendOffline(
  FakeServer server,
  WorldSeed w,
  ScenarioClient client,
) async {
  await client.start();
  await client.goOffline();
  for (var i = 1; i <= 3; i++) {
    w.post(w.sam, w.releaseNotes, 'While Alex is away $i');
  }
  final id = await client.send(w.releaseNotes, 'Written offline');
  _check(
    server.requestsOf('createMessage').isEmpty,
    'nothing is sent while offline',
  );
  _check((await client.outbox()).length == 1, 'the entry waits in the outbox');
  server.faults
    ..dropResponseAfterCommit('createMessage')
    ..holdDispatches();
  await client.goOnline();
  await waitFor(
    'the retry',
    () => server.requestsOf('createMessage').length >= 2,
  );
  final attempts = server.requestsOf('createMessage');
  final statuses = [for (final r in attempts) r.status];
  _check(attempts.first.status == 0, 'the first response was lost: $statuses');
  _check(
    attempts[1].status == 200,
    'the retry was a replay with 200: $statuses',
  );
  _check(
    attempts.every((r) => (r.body! as JsonMap)['client_msg_id'] == id),
    'every attempt carried the same client_msg_id',
  );
  server.faults.releaseDispatches();
  await waitFor(
    'the echo of $id',
    () async => (await _withId(client, w.releaseNotes, id)).isNotEmpty,
  );
  await waitFor(
    'the empty outbox',
    () async => (await client.outbox()).isEmpty,
  );
  _check(
    (await _withId(client, w.releaseNotes, id)).length == 1,
    'exactly one local message',
  );
  _check(
    server.world
            .messagesOf(w.releaseNotes)
            .where((m) => m['client_msg_id'] == id)
            .length ==
        1,
    'exactly one message on the server',
  );
  final texts = [for (final m in await client.messages(w.releaseNotes)) m.text];
  _check(
    texts.contains('While Alex is away 3'),
    'the catch-up delivered the missed messages',
  );
  _contract(server);
}

/// 04 16.3: a reconnect catches up from the cursor in pages with `cursor` frames (the private
/// channel's events are skipped), then `catchup.complete`; only visible events arrive.
Future<void> scenarioCatchUp(
  FakeServer server,
  WorldSeed w,
  ScenarioClient client,
) async {
  server.world
    ..catchupPageSize = 3
    ..cursorAdvanceEvery = 2;
  await client.start();
  await client.goOffline();
  for (var i = 1; i <= 4; i++) {
    w
      ..post(w.sam, w.releaseNotes, 'Visible $i')
      ..post(w.sam, w.budget, 'Hidden $i');
  }
  final before = server.gateway.connections.length;
  await client.goOnline();
  await waitFor(
    'live again',
    () => client.live && server.gateway.connections.length > before,
  );
  await waitFor(
    'the cursor at the head',
    () async => await client.cursor() == server.world.headSeq,
  );
  final number = server.gateway.connections.last.number;
  final sent = server.gateway.sentOn(number);
  final ops = [for (final f in sent) f.op];
  _check(
    ops.where((o) => o == 'cursor').length >= 3,
    'several pages with cursor frames: $ops',
  );
  _check(
    ops.contains('catchup.complete'),
    'catchup.complete ends the catch-up',
  );
  _check(
    sent
        .where((f) => f.op == 'dispatch')
        .every(
          (f) =>
              f.data['scope'] is! Map ||
              (f.data['scope']! as Map)['channel_id'] != w.private,
        ),
    'no event of the hidden channel arrives',
  );
  final texts = [for (final m in await client.messages(w.releaseNotes)) m.text];
  for (var i = 1; i <= 4; i++) {
    _check(texts.contains('Visible $i'), 'message "Visible $i" was caught up');
  }
  _check(
    !await client.holdsChannel(w.private),
    'the hidden channel is not stored',
  );
  _contract(server);
}

/// 04 16.3, the `resync_required` branch: the log no longer holds the client's cursor
/// (`retention`); the client keeps its outbox, loads the snapshot, reloads the focused thread
/// and goes on live from the head without reconnecting.
Future<void> scenarioResync(
  FakeServer server,
  WorldSeed w,
  ScenarioClient client,
) async {
  await client.start();
  await client.focusThread(w.releaseNotes);
  await client.goOffline();
  w.post(w.sam, w.releaseNotes, 'Posted during the gap');
  server.world.dropEventsUpTo(server.world.headSeq);
  final id = await client.send(w.releaseNotes, 'Kept in the outbox');
  final before = server.gateway.connections.length;
  await client.goOnline();
  await waitFor(
    'resync notice',
    () => client.notices.contains('resynced:retention'),
  );
  final number = server.gateway.connections.last.number;
  _check(
    server.gateway.connections.length == before + 1,
    'one new connection, no reconnect',
  );
  _check(
    server.gateway.sentOn(number).any((f) => f.op == 'resync_required'),
    'the server sent resync_required',
  );
  await waitFor(
    'the outbox entry sent',
    () async => (await client.outbox()).isEmpty,
  );
  await waitFor(
    'own message',
    () async => (await _withId(client, w.releaseNotes, id)).isNotEmpty,
  );
  await waitFor(
    'the reloaded thread',
    () async =>
        [for (final m in await client.messages(w.releaseNotes)) m.text]
            .contains('Posted during the gap'),
  );
  _check(server.gateway.connections.last.open, 'the connection stayed open');
  _contract(server);
}

/// 04 16.4: access to a channel is removed while the client is connected: its content goes,
/// the view closes with a notice, and later events of the channel never arrive.
Future<void> scenarioAccessRemoval(
  FakeServer server,
  WorldSeed w,
  ScenarioClient client,
) async {
  await client.start();
  await client.focusThread(w.printer);
  w.post(w.sam, w.printer, 'The printer is out of paper');
  await waitFor(
    'the printer message',
    () async => (await client.messages(w.printer)).isNotEmpty,
  );
  final number = server.gateway.connections.last.number;
  server.world.setAccess(w.alex.id, w.support, null);
  final removedAt = server.gateway.sentOn(number).length;
  await waitFor(
    'the removal notice',
    () => client.notices.contains('removed:channel:${w.support}'),
  );
  await waitFor(
    'the channel is gone',
    () async => !await client.holdsChannel(w.support),
  );
  _check(
    (await client.messages(w.printer)).isEmpty,
    'the channel content was deleted',
  );
  _check(
    !server.gateway.connections.last.focus.contains(w.printer),
    'the server ended the focus of the removed channel',
  );
  w.post(w.sam, w.printer, 'Fixed the printer');
  final marker = w.post(w.sam, w.releaseNotes, 'Marker after the removal');
  await waitFor(
    'the marker',
    () async =>
        [for (final m in await client.messages(w.releaseNotes)) m.id]
            .contains(marker['id']),
  );
  final later = server.gateway.sentOn(number).skip(removedAt);
  _check(
    later.every(
      (f) =>
          f.op != 'dispatch' ||
          (f.data['scope'] as Map?)?['channel_id'] != w.support,
    ),
    'no later event of the removed channel arrived',
  );
  _check(
    (await client.messages(w.printer)).isEmpty,
    'nothing of the channel came back',
  );
  _contract(server);
}

/// 04 16.5: a mention of the agent Robin starts a run; the live box follows the coalesced
/// patches, a follow-up is delivered and absorbed, the agent replies and the run ends; the
/// live box document is discarded at the end.
///
/// The script pauses longer than the coalescing window of the fake
/// (`GatewayTimings.runPatchCoalesce`), so each patch reaches the client before the next
/// step.
Future<void> scenarioAgentRun(
  FakeServer server,
  WorldSeed w,
  ScenarioClient client,
) async {
  final stepId = server.world.nextId();
  final base = AgentScript.followUpRun(
    stepId,
    server.world.stamp(),
    reply: 'Here is the summary.',
  );
  final agent = server.addAgent(
    w.robin.id,
    AgentScript(base.steps, pause: server.gateway.timings.runPatchCoalesce * 3),
  );
  await client.start();
  await client.focusThread(w.releaseNotes);
  final trigger = await client.send(
    w.releaseNotes,
    'Robin, summarize the notes',
    mentionIds: [w.robin.id],
  );
  await waitFor('the run', () => agent.runIds.isNotEmpty);
  final runId = agent.runIds.single;
  await client.focusRun(runId, w.releaseNotes);
  await waitFor('the live step', () {
    final steps = client.runDocument(runId)?['steps'];
    return steps is Map && steps.containsKey(stepId);
  });
  final followup = await client.send(w.releaseNotes, 'Also mention the budget');
  String? followupState() {
    final f = client.runDocument(runId)?['followups'];
    if (f is! Map) return null;
    for (final v in f.values) {
      if (v is Map) return v['state'] as String?;
    }
    return null;
  }

  await waitFor('the absorbed follow-up', () => followupState() == 'absorbed');
  await waitFor(
    'the reply',
    () async =>
        (await client.messages(w.releaseNotes))
            .any((m) => m.authorId == w.robin.id),
  );
  await waitFor(
    'the run end',
    () async => await client.runState(runId) == 'completed',
  );
  await waitFor(
    'the discarded document',
    () => client.runDocument(runId) == null,
  );
  await waitFor(
    'trigger state done',
    () async =>
        (await _withId(client, w.releaseNotes, trigger)).single.triggerState ==
        'done',
  );
  _check(
    (await _withId(client, w.releaseNotes, followup)).single.triggerState ==
        'followup_delivered',
    'the follow-up reached the agent',
  );
  _contract(server);
}
