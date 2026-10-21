// SPDX-License-Identifier: Apache-2.0

import 'dart:async';

import 'package:client/telemetry/client_metrics.dart';
import 'package:client/telemetry/measurements.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

/// A stopwatch whose elapsed time the test sets.
final class FakeWatch implements Stopwatch {
  Duration now = Duration.zero;
  @override
  Duration get elapsed => now;
  @override
  int get elapsedMicroseconds => now.inMicroseconds;
  @override
  int get elapsedMilliseconds => now.inMilliseconds;
  @override
  int get elapsedTicks => now.inMicroseconds;
  @override
  int get frequency => 1000000;
  @override
  bool get isRunning => true;
  @override
  void reset() {}
  @override
  void start() {}
  @override
  void stop() {}
}

/// A channel the test feeds and closes.
final class FakeChannel implements GatewayChannel {
  final StreamController<String> _frames = StreamController();
  int? code;
  @override
  Stream<String> get frames => _frames.stream;
  void add(String frame) => _frames.add(frame);
  Future<void> drop(int? closeCode) async {
    code = closeCode;
    await _frames.close();
  }

  @override
  void send(String text) {}
  @override
  Future<void> close(int code, [String? reason]) => drop(code);
  @override
  int? get closeCode => code;
  @override
  String? get closeReason => null;
}

final class FakeTransport implements GatewayTransport {
  final List<FakeChannel> channels = [];
  @override
  Future<GatewayChannel> connect(Uri url) async {
    final c = FakeChannel();
    channels.add(c);
    return c;
  }
}

Map<ClientMetric, MetricEntry> entries(ClientMetrics m) => {
  for (final e in m.take()) e.metric: e,
};

void main() {
  late ClientMetrics metrics;
  late FakeWatch clock;
  late AppMeasurements m;

  setUp(() {
    metrics = ClientMetrics();
    clock = FakeWatch();
    m = AppMeasurements(metrics, clock: clock, afterFrame: (f) => f());
  });

  test(
    'cold start stops once, when the sidebar first shows from the cache',
    () {
      clock.now = const Duration(milliseconds: 1800);
      m.sidebarShown();
      clock.now = const Duration(seconds: 9);
      m.sidebarShown();
      expect(entries(metrics)[ClientMetric.coldStartDuration]!.buckets, {
        2.5: 1,
      });
    },
  );

  test(
    'thread open runs from opening until the first messages are visible',
    () {
      clock.now = const Duration(seconds: 1);
      m.threadOpening('t1');
      clock.now = const Duration(milliseconds: 1080);
      m.threadShown('t1');
      m.threadShown('t1');
      m.threadShown('t2');
      expect(entries(metrics)[ClientMetric.threadOpenDuration]!.buckets, {
        0.1: 1,
      });
    },
  );

  test('send runs until the frame after the local write', () {
    clock.now = const Duration(seconds: 2);
    final send = m.sendStarted();
    clock.now = const Duration(milliseconds: 2030);
    m.sendShown(send);
    expect(entries(metrics)[ClientMetric.sendLocalDuration]!.buckets, {
      0.05: 1,
    });
  });

  test('frames count and those over the budget', () {
    m.recordFrames([
      const Duration(milliseconds: 8),
      const Duration(milliseconds: 16),
      const Duration(milliseconds: 40),
    ], budget: const Duration(microseconds: 16667));
    final e = entries(metrics);
    expect(e[ClientMetric.frames]!.value, 3);
    expect(e[ClientMetric.framesOverBudget]!.value, 1);
  });

  test(
    'engine: catch-up, events, resyncs, reconnects by close code, outbox depth',
    () async {
      final states = StreamController<ConnectionState>(sync: true);
      final notices = StreamController<SyncNotice>(sync: true);
      final depth = StreamController<int>(sync: true);
      final stop = m.watchEngine(
        states: states.stream,
        notices: notices.stream,
        outboxDepth: depth.stream,
      );
      final transport = FakeTransport();
      final measured = m.wrapTransport(transport);

      // First connection: catch-up with two dispatch frames and a cursor.
      final first = await measured.connect(
        Uri.parse('wss://chat.example.com/api/v1/gateway'),
      );
      final seen = <String>[];
      final sub = first.frames.listen(seen.add);
      clock.now = const Duration(seconds: 3);
      states.add(const ConnectionState(ConnectionPhase.catchingUp));
      transport.channels[0]
        ..add('{"op":"dispatch","d":{"seq":1}}')
        ..add('{"op":"dispatch","d":{"seq":2}}')
        ..add('{"op":"cursor","d":{"seq":2}}');
      await pumpEventQueue();
      clock.now = const Duration(milliseconds: 3400);
      states.add(const ConnectionState(ConnectionPhase.live));
      transport.channels[0].add('{"op":"dispatch","d":{"seq":3}}');
      await pumpEventQueue();
      expect(seen, hasLength(4));
      await transport.channels[0].drop(4009);
      await sub.cancel();
      // The reconnect counts the close code of the previous connection.
      await measured.connect(
        Uri.parse('wss://chat.example.com/api/v1/gateway'),
      );
      notices.add(const Resynced('retention'));
      depth
        ..add(4)
        ..add(2);
      final e = [for (final x in metrics.take()) x.toJson()];
      expect(
        e,
        unorderedEquals([
          {
            'name': 'client.catchup.duration',
            'kind': 'histogram',
            'buckets': [
              {'le': 0.5, 'count': 1},
            ],
          },
          {'name': 'client.catchup.events', 'kind': 'counter', 'value': 2.0},
          {
            'name': 'client.reconnects',
            'kind': 'counter',
            'close_code': 4009,
            'value': 1.0,
          },
          {'name': 'client.resync', 'kind': 'counter', 'value': 1.0},
          {'name': 'client.outbox.depth', 'kind': 'gauge', 'value': 2.0},
        ]),
      );
      await stop();
    },
  );
}
