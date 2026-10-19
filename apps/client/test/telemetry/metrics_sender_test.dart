// SPDX-License-Identifier: Apache-2.0

import 'dart:async';
import 'dart:io';

import 'package:client/telemetry/client_metrics.dart';
import 'package:client/telemetry/metrics_sender.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// Records every batch as its JSON and answers with the next scripted outcome.
final class RecordingApi implements ClientMetricsApi {
  final List<Map<String, Object?>> sent = [];
  final List<Object> failures = [];

  @override
  Future<void> submit(MetricsBatch batch) async {
    sent.add(batch.toJson());
    if (failures.isNotEmpty) throw failures.removeAt(0);
  }
}

/// A timer the test fires by hand.
final class ManualTimer implements Timer {
  ManualTimer(this.interval, this.callback);
  final Duration interval;
  final void Function(Timer) callback;
  bool cancelled = false;
  int ticks = 0;

  Future<void> fire() async {
    ticks++;
    callback(this);
    // Let the flush finish.
    await pumpEventQueue();
  }

  @override
  void cancel() => cancelled = true;
  @override
  bool get isActive => !cancelled;
  @override
  int get tick => ticks;
}

DioException networkError() => DioException(
  requestOptions: RequestOptions(path: '/client-metrics'),
  type: DioExceptionType.connectionError,
  error: const SocketException('down'),
);

DioException problem(int status, String code) {
  final options = RequestOptions(path: '/client-metrics');
  return DioException(
    requestOptions: options,
    type: DioExceptionType.badResponse,
    response: Response<Object?>(
      requestOptions: options,
      statusCode: status,
      data: {
        'code': code,
        'status': status,
        'title': 'x',
        'type': 'about:blank',
      },
    ),
  );
}

void main() {
  late ClientMetrics metrics;
  late RecordingApi api;
  late ManualTimer timer;
  late bool enabled;
  late MetricsSender sender;

  setUp(() {
    metrics = ClientMetrics();
    api = RecordingApi();
    enabled = true;
    sender = MetricsSender(
      metrics: metrics,
      api: api,
      interval: const Duration(minutes: 5),
      enabled: () => enabled,
      attributes: () => const MetricsAttributes(
        platform: 'macos',
        appVersion: '1.4.0',
        density: 'compact',
        theme: 'light',
      ),
      timer: (interval, cb) => timer = ManualTimer(interval, cb),
    )..start();
  });

  test('one send per interval at most, nothing for an empty batch', () async {
    expect(timer.interval, const Duration(minutes: 5));
    metrics.record(ClientMetric.frames, 60);
    metrics.record(ClientMetric.frames, 60);
    await pumpEventQueue();
    expect(api.sent, isEmpty);
    await timer.fire();
    expect(api.sent, hasLength(1));
    expect(api.sent.single['app_version'], '1.4.0');
    await timer.fire();
    expect(api.sent, hasLength(1));
    metrics.record(ClientMetric.resync, 1);
    await timer.fire();
    expect(api.sent, hasLength(2));
  });

  test(
    'nothing is sent while the setting is off, and nothing is kept for later',
    () async {
      enabled = false;
      metrics.record(ClientMetric.frames, 60);
      await timer.fire();
      expect(api.sent, isEmpty);
      enabled = true;
      sender.settingChanged();
      await timer.fire();
      expect(api.sent, isEmpty);
    },
  );

  test('stops after 403 feature_disabled until the setting changes', () async {
    api.failures.add(problem(403, 'feature_disabled'));
    metrics.record(ClientMetric.frames, 60);
    await timer.fire();
    expect(api.sent, hasLength(1));
    expect(sender.stoppedByServer, isTrue);
    metrics.record(ClientMetric.frames, 60);
    await timer.fire();
    expect(api.sent, hasLength(1));
    sender.settingChanged();
    metrics.record(ClientMetric.frames, 60);
    await timer.fire();
    expect(api.sent, hasLength(2));
  });

  test('splits above maxItems', () async {
    for (var code = 0; code < clientMetricsBatchMaxItems + 5; code++) {
      metrics.record(ClientMetric.reconnects, 1, closeCode: 4000 + code);
    }
    await timer.fire();
    expect(api.sent, hasLength(2));
    expect(
      (api.sent[0]['metrics']! as List),
      hasLength(clientMetricsBatchMaxItems),
    );
    expect((api.sent[1]['metrics']! as List), hasLength(5));
  });

  test(
    'a network error keeps one batch for the next interval, never more',
    () async {
      api.failures.addAll([networkError(), networkError()]);
      metrics.record(ClientMetric.frames, 1);
      await timer.fire();
      metrics.record(ClientMetric.frames, 2);
      await timer.fire();
      metrics.record(ClientMetric.frames, 3);
      await timer.fire();
      // First: batch 1 fails. Second: batch 1 again and fails, batch 2 is kept instead.
      // Third: batch 2 again, then batch 3.
      final values = [
        for (final b in api.sent)
          ((b['metrics']! as List).single as Map)['value'],
      ];
      expect(values, [1.0, 1.0, 2.0, 3.0]);
    },
  );

  test('any other error drops the batch', () async {
    api.failures.add(problem(500, 'internal_error'));
    metrics.record(ClientMetric.frames, 1);
    await timer.fire();
    await timer.fire();
    expect(api.sent, hasLength(1));
  });

  test('stop sends what is left and cancels the timer', () async {
    metrics.record(ClientMetric.outboxDepth, 2);
    await sender.stop();
    expect(api.sent, hasLength(1));
    expect(timer.cancelled, isTrue);
  });
}
