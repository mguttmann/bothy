// SPDX-License-Identifier: Apache-2.0

// The fake server of M4-17 for the mobile performance level, on its own isolate (brief M5-11
// Design note 5: "patches from the fake server of M4-17 inside the app, so the measurement
// isolates the client"). The fake checks every request, response and frame against the API
// contract and keeps its own logs; on the app's isolate that work would land in the frames the
// level measures. On a second isolate of the same process it runs on another core, the app reaches
// it on 127.0.0.1 as it reaches a server, and the test drives the world through the commands of
// [PerfFake]. The isolates share one heap, so a garbage collection of the fake can still pause the
// app, and its memory stays in the resident size of the process: both can only make the values
// worse, never better.

import 'dart:async';
import 'dart:isolate';

import 'package:sync_engine/testing.dart';

/// The threads the measurement opens: the five open threads of the memory target of 15 19.
const int openThreads = 5;

/// The messages of each thread before the run, so a thread opened from the cache shows rows.
const int messagesPerThread = 3;

/// The status line patches of one stream, the samples of the patch-to-frame target (the
/// stream-agent of the desktop level sends as many, tools/ci/desktop_e2e_agent.py).
const int streamPatches = 30;

/// The pause between two patches of a stream: longer than the coalescing window of the gateway
/// ([_coalesce]), so every patch arrives in a frame of its own, as on the desktop level.
const Duration streamInterval = Duration(milliseconds: 500);

/// The window in which the fake gateway coalesces run patches (as `FakeInstance` of the
/// integration tests).
const Duration _coalesce = Duration(milliseconds: 40);

/// A command the fake could not carry out, with the error of the fake's isolate.
final class PerfFakeError implements Exception {
  /// The error [message] of the fake.
  const PerfFakeError(this.message);

  /// What went wrong, as the fake's isolate described it.
  final String message;

  @override
  String toString() => 'PerfFakeError: $message';
}

/// The fake server of the measurement on its own isolate: Alex, Sam and Robin of
/// `WorldSeed.standard`, and in its text channel `general` [openThreads] threads of
/// [messagesPerThread] messages from Sam each.
///
/// Example:
/// ```dart
/// final fake = await PerfFake.spawn();
/// await fake.post(fake.threads.first, 10);
/// await fake.stop();
/// ```
final class PerfFake {
  PerfFake._(
    this._isolate,
    this._replies,
    this._commands,
    this._pending,
    this.baseUri,
    this.instanceId,
    this.spaceId,
    this.threads,
  );

  /// Starts the fake on a new isolate and waits until it listens. Throws a [PerfFakeError]
  /// when the isolate fails before that.
  static Future<PerfFake> spawn() async {
    final replies = ReceivePort('perf-fake replies');
    final ready = Completer<Map<Object?, Object?>>();
    final pending = <int, Completer<Object?>>{};
    replies.listen((message) {
      switch (message) {
        case final Map<Object?, Object?> hello when !ready.isCompleted:
          ready.complete(hello);
        case [final int id, final bool ok, final Object? value]:
          final c = pending.remove(id);
          if (ok) {
            c?.complete(value);
          } else {
            c?.completeError(PerfFakeError('$value'));
          }
        // An error of the isolate itself: `onError` sends [error, stack].
        case [final Object? error, final Object? _]:
          final failure = PerfFakeError('$error');
          if (!ready.isCompleted) ready.completeError(failure);
          for (final c in pending.values) {
            c.completeError(failure);
          }
          pending.clear();
      }
    });
    final isolate = await Isolate.spawn(
      _serve,
      replies.sendPort,
      onError: replies.sendPort,
      debugName: 'perf-fake',
    );
    final Map<Object?, Object?> hello;
    try {
      hello = await ready.future;
    } on Object {
      isolate.kill(priority: Isolate.immediate);
      replies.close();
      rethrow;
    }
    return PerfFake._(
      isolate,
      replies,
      hello['commands']! as SendPort,
      pending,
      Uri.parse(hello['base']! as String),
      hello['instance']! as String,
      hello['space']! as String,
      (hello['threads']! as List<Object?>).cast<String>(),
    );
  }

  final Isolate _isolate;
  final ReceivePort _replies;
  final SendPort _commands;
  final Map<int, Completer<Object?>> _pending;
  int _next = 0;

  /// The address of the fake, `http://127.0.0.1:<port>`.
  final Uri baseUri;

  /// The `instance_id` of discovery: the scope of the app's device keys and secrets.
  final String instanceId;

  /// The space "Team" of the seed: its row in the sidebar ends the cold start.
  final String spaceId;

  /// The [openThreads] threads in the channel `general`.
  final List<String> threads;

  Future<T> _call<T>(String command, [List<Object?> args = const []]) {
    final id = _next++;
    final reply = Completer<Object?>();
    _pending[id] = reply;
    _commands.send([id, command, ...args]);
    return reply.future.then((v) => v as T);
  }

  /// Sam posts [count] messages into [thread] (the backlog of the catch-up while the app is
  /// offline); returns the number of messages the thread has then. Throws a [PerfFakeError] for
  /// a thread Sam does not see.
  Future<int> post(String thread, int count) =>
      _call<int>('post', [thread, count]);

  /// Alex posts a message into [thread], a run of Robin for it starts, and its status line
  /// changes [patches] times to `Streaming <n> of <patches>`, every [every], on the fake's
  /// isolate; returns the run ID once the run is running ([streamed] waits for the end).
  Future<String> stream(
    String thread, {
    int patches = streamPatches,
    Duration every = streamInterval,
  }) => _call<String>('stream', [thread, patches, every.inMicroseconds]);

  /// Waits until the stream of [run] has sent its last patch; returns the patches sent.
  Future<int> streamed(String run) => _call<int>('streamed', [run]);

  /// Ends the gateway connections of [deviceId] without a close code, as a lost network does.
  Future<void> disconnect(String deviceId) =>
      _call<Object?>('disconnect', [deviceId]);

  /// The contract violations the fake saw (an empty list when the app spoke the contract).
  Future<List<String>> violations() async =>
      (await _call<List<Object?>>('violations')).cast<String>();

  /// Stops the fake and its isolate.
  Future<void> stop() async {
    try {
      await _call<Object?>('stop');
    } finally {
      _replies.close();
      _isolate.kill();
    }
  }
}

/// The entry of the fake's isolate: starts the fake, seeds the world, sends the hello to
/// [toTest] and serves commands until `stop`.
Future<void> _serve(SendPort toTest) async {
  final server = await FakeServer.start(
    options: const FakeServerOptions(
      timings: GatewayTimings(runPatchCoalesce: _coalesce),
    ),
  );
  final world = server.world;
  final seed = WorldSeed.standard(world);
  // Threads of a text channel: on the phone the page under a thread is then the channel's
  // history. (Under a forum post it is the forum's post list, which throws in a frame callback
  // when it was never laid out; reported with the run, outside this level.)
  final threads = <String>[];
  for (var i = 0; i < openThreads; i++) {
    final thread = world.addThread(
      seed.general,
      'Perf $i',
      createdBy: seed.sam.id,
    );
    for (var m = 0; m < messagesPerThread; m++) {
      seed.post(seed.sam, thread, 'Message $m of $i');
    }
    threads.add(thread);
  }
  final streams = <String, Future<int>>{};
  var posted = 0;

  // Starts a stream (see [PerfFake.stream]) and returns the run ID; the patches follow on
  // this isolate's timers.
  String stream(String thread, int patches, Duration every) {
    final trigger = seed.post(seed.alex, thread, 'Stream please');
    final run = world.createRun(
      agentId: seed.robin.id,
      triggeringMessageId: trigger['id']! as String,
      triggeredBy: seed.alex.id,
    );
    final id = run['id']! as String;
    world.setRunState(id, 'running', triggerState: 'working');
    streams[id] = () async {
      for (var n = 1; n <= patches; n++) {
        await Future<void>.delayed(every);
        world.patchRun(id, [
          {
            'op': 'add',
            'path': '/status_line',
            'value': 'Streaming $n of $patches',
          },
        ]);
      }
      return patches;
    }();
    return id;
  }

  final commands = ReceivePort('perf-fake commands');
  toTest.send({
    'commands': commands.sendPort,
    'base': server.baseUri.toString(),
    'instance': world.instanceId,
    'space': seed.space,
    'threads': threads,
  });
  await for (final message in commands) {
    final [Object? id, ...request] = message as List<Object?>;
    try {
      final Object? value;
      switch (request) {
        case ['post', final String thread, final int count]:
          for (var i = 0; i < count; i++) {
            seed.post(seed.sam, thread, 'Backlog ${posted++}');
          }
          value = world.messagesOf(thread).length;
        case ['stream', final String thread, final int patches, final int us]:
          value = stream(thread, patches, Duration(microseconds: us));
        case ['streamed', final String run]:
          value = await (streams[run] ?? (throw StateError('no stream $run')));
        case ['disconnect', final String device]:
          server.disconnect(deviceId: device);
          value = null;
        case ['violations']:
          value = server.violations;
        case ['stop']:
          await server.stop();
          toTest.send([id, true, null]);
          commands.close();
          return;
        default:
          throw ArgumentError.value(request, 'command');
      }
      toTest.send([id, true, value]);
    } on Object catch (e) {
      toTest.send([id, false, '$e']);
    }
  }
}
