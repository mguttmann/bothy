// SPDX-License-Identifier: Apache-2.0

import 'package:client/telemetry/client_metrics.dart';
import 'package:client/telemetry/metrics_sender.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:yaml/yaml.dart';

import 'repo_files.dart';

/// The rows of signals.md 4.6a: name in the app to kind.
Map<String, String> catalogueRows() {
  final text = readRepoFile('docs/observability/signals.md');
  final start = text.indexOf('### 4.6a');
  final end = text.indexOf('### 4.7', start);
  final kinds = {
    'Histogram': 'histogram',
    'Counter': 'counter',
    'Gauge': 'gauge',
  };
  final rows = <String, String>{};
  for (final line in text.substring(start, end).split('\n')) {
    final cells = line.split('|').map((c) => c.trim()).toList();
    if (cells.length < 4 || !cells[1].startsWith('`client.')) continue;
    rows[cells[1].replaceAll('`', '')] = kinds[cells[2]]!;
  }
  return rows;
}

/// The bounds of the first row of signals.md 4.7.
List<double> catalogueBounds() {
  final text = readRepoFile('docs/observability/signals.md');
  final start = text.indexOf('### 4.7');
  final row = text
      .substring(start)
      .split('\n')
      .firstWhere((l) => l.contains('`http.server.request.duration`'));
  final cells = row.split('|').map((c) => c.trim()).toList();
  return [for (final v in cells[2].split(',')) double.parse(v.trim())];
}

void main() {
  test('the enum equals the table of signals.md 4.6a', () {
    final rows = catalogueRows();
    expect(rows, hasLength(12));
    expect({
      for (final m in ClientMetric.values) m.wireName: m.kind.name,
    }, rows);
  });

  test('the bounds equal the row "durations up to seconds" of 4.7', () {
    expect(histogramBounds, catalogueBounds());
  });

  test('the batch limit equals maxItems of ClientMetricsBatch', () {
    final doc = loadYaml(readRepoFile('docs/api/openapi.yaml')) as YamlMap;
    final schema =
        ((doc['components'] as YamlMap)['schemas']
                as YamlMap)['ClientMetricsBatch']
            as YamlMap;
    final metrics = (schema['properties'] as YamlMap)['metrics'] as YamlMap;
    expect(clientMetricsBatchMaxItems, metrics['maxItems']);
  });

  test('histograms bucket by the bounds and clamp above the top bound', () {
    final m = ClientMetrics()
      ..record(ClientMetric.threadOpenDuration, 0.1)
      ..record(ClientMetric.threadOpenDuration, 0.11)
      ..record(ClientMetric.threadOpenDuration, 0)
      ..record(ClientMetric.threadOpenDuration, 42)
      ..record(ClientMetric.threadOpenDuration, -1)
      ..record(ClientMetric.threadOpenDuration, double.nan)
      ..recordDuration(
        ClientMetric.coldStartDuration,
        const Duration(milliseconds: 2400),
      );
    final entries = {for (final e in m.take()) e.metric: e};
    expect(entries[ClientMetric.threadOpenDuration]!.buckets, {
      0.005: 1,
      0.1: 1,
      0.25: 1,
      10.0: 1,
    });
    expect(entries[ClientMetric.coldStartDuration]!.buckets, {2.5: 1});
    expect(m.isEmpty, isTrue);
    expect(m.take(), isEmpty);
  });

  test(
    'counters add, gauges keep the last value, close codes only for reconnects',
    () {
      final m = ClientMetrics()
        ..record(ClientMetric.frames, 60)
        ..record(ClientMetric.frames, 60)
        ..record(ClientMetric.frames, -5)
        ..record(ClientMetric.reconnects, 1, closeCode: 4009)
        ..record(ClientMetric.reconnects, 1, closeCode: 4009)
        ..record(ClientMetric.reconnects, 1)
        ..record(ClientMetric.resync, 1, closeCode: 4009)
        ..record(ClientMetric.outboxDepth, 3)
        ..record(ClientMetric.outboxDepth, 1);
      final json = [for (final e in m.take()) e.toJson()];
      expect(
        json,
        unorderedEquals([
          {'name': 'client.frames', 'kind': 'counter', 'value': 120.0},
          {
            'name': 'client.reconnects',
            'kind': 'counter',
            'close_code': 4009,
            'value': 2.0,
          },
          {'name': 'client.reconnects', 'kind': 'counter', 'value': 1.0},
          {'name': 'client.resync', 'kind': 'counter', 'value': 1.0},
          {'name': 'client.outbox.depth', 'kind': 'gauge', 'value': 1.0},
        ]),
      );
    },
  );

  test('a batch carries only the attributes of 4.6a', () {
    const batch = MetricsBatch(
      MetricsAttributes(platform: 'macos', appVersion: '1.4.0', theme: 'dark'),
      [MetricEntry(ClientMetric.frames, value: 1)],
    );
    expect(batch.toJson(), {
      'platform': 'macos',
      'app_version': '1.4.0',
      'theme': 'dark',
      'metrics': [
        {'name': 'client.frames', 'kind': 'counter', 'value': 1.0},
      ],
    });
  });
}
