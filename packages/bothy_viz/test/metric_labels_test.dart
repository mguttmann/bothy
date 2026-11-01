// SPDX-License-Identifier: Apache-2.0

// Metric labels never show a raw enum value (owner request 2026-11-01): every dimension code of the
// closed lists of the server (services/server/internal/kernel/measure/measure.go, 25 3.2) and every
// metric name of the query responses is translated through l10n in one place (VizTexts), in every
// locale of the app. The dimension lists are read from the server source, so a code added there
// without a translation fails here.
import 'dart:io';

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:built_value/serializer.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support/views.dart';

/// The closed dimension lists per metric of the server, parsed from its source.
Map<String, List<String>> serverDimensions() {
  final source = File(
    '../../services/server/internal/kernel/measure/measure.go',
  ).readAsStringSync();
  final shared = <String, List<String>>{};
  for (final m in RegExp(
    r'^\s*(\w+)\s*=\s*\[\]string\{([^}]*)\}',
    multiLine: true,
  ).allMatches(source)) {
    shared[m.group(1)!] = _codes(m.group(2)!);
  }
  final names = <String, String>{};
  for (final m in RegExp(
    r'^\s*(\w+)\s+Metric\s*=\s*"([^"]+)"',
    multiLine: true,
  ).allMatches(source)) {
    names[m.group(1)!] = m.group(2)!;
  }
  final out = <String, List<String>>{};
  for (final m in RegExp(
    r'^\s*(\w+):\s*\{dimensions:\s*(\[\]string\{([^}]*)\}|\w+)',
    multiLine: true,
  ).allMatches(source)) {
    final metric = names[m.group(1)!];
    if (metric == null) continue;
    final list = m.group(3) != null ? _codes(m.group(3)!) : shared[m.group(2)!];
    out[metric] = [
      for (final c in list!)
        if (c.isNotEmpty) c,
    ];
  }
  return out;
}

List<String> _codes(String body) => [
  for (final m in RegExp(r'"([^"]*)"').allMatches(body)) m.group(1)!,
];

/// Whether [label] shows [code] as a word of its own, case-sensitively: "Tool calls completed"
/// does, "Tool calls, Completed", "Werkzeugaufrufe, abgeschlossen" and the agent name
/// "support-agent" do not.
bool showsCode(String label, String code) =>
    RegExp('(^|[^A-Za-z0-9_-])${RegExp.escape(code)}(\$|[^A-Za-z0-9_-])')
        .hasMatch(label);

api.StatsMetric metricOf(String wire) =>
    api.standardSerializers.deserialize(
          wire,
          specifiedType: const FullType(api.StatsMetric),
        )!
        as api.StatsMetric;

/// The metric names of getUsageStats, getRunStats and getErrorStats (the operation descriptions
/// of docs/api/openapi.yaml), with one example of each name pattern.
const queryMetrics = [
  'messages',
  'messages_human',
  'messages_agent',
  'messages_app',
  'messages_import',
  'threads_created',
  'threads_done',
  'runs',
  'active_people',
  'storage_bytes',
  'completed',
  'failed',
  'cancelled',
  'duration_ms_p50',
  'duration_ms_p95',
  'failed_reason_timeout',
  'run_failure_rate',
  'invocation_denials',
  'webhook_failure_rate',
  'push_failure_rate_apns',
  'scan_errors',
  'scan_infected',
];

void main() {
  final dimensions = serverDimensions();

  test('the dimension lists are read from the server', () {
    expect(dimensions['tool_calls'], ['completed', 'failed']);
    expect(dimensions['runs_finished'], ['completed', 'failed', 'cancelled']);
    expect(dimensions['messages_created'], ['human', 'agent', 'app', 'import']);
    expect(dimensions['runs_started'], ['personal', 'organization']);
    expect(dimensions.keys, hasLength(greaterThanOrEqualTo(10)));
  });

  for (final locale in AppLocalizations.supportedLocales) {
    group('locale ${locale.languageCode}', () {
      late VizTexts texts;
      setUpAll(() async {
        texts = VizTexts(await AppLocalizations.delegate.load(locale), locale);
      });

      test('every dimension code has a translation', () {
        for (final MapEntry(key: metric, value: codes) in dimensions.entries) {
          for (final code in codes) {
            expect(
              texts.dimension(metricOf(metric), code),
              isNotNull,
              reason: '$metric $code',
            );
          }
        }
      });

      test('no rate, series or legend label shows a raw dimension code', () {
        for (final MapEntry(key: metric, value: codes) in dimensions.entries) {
          for (final code in codes) {
            final m = metricOf(metric);
            final labels = [
              texts.metricLabel(m, code),
              AreaMark.seriesLabel(
                texts,
                SeriesModel(
                  metric: m,
                  dimension: code,
                  colorIndex: 0,
                  points: const [],
                ),
              ),
            ];
            for (final label in labels) {
              expect(
                showsCode(label, code),
                isFalse,
                reason: '"$label" shows the code $code of $metric',
              );
            }
          }
        }
      });

      test('no metric name of a query response is shown raw', () {
        for (final name in queryMetrics) {
          final label = texts.queryMetric(name);
          expect(
            showsCode(label, name),
            isFalse,
            reason: '"$label" shows the name $name',
          );
          expect(label, isNot(texts.l10n.vizMetricUnknown(name)));
        }
      });
    });
  }

  // The rendered views of "Live and statistics" in German: no text and no semantics label shows a
  // dimension code of the server (the rates of the last minute, the legends, the tables).
  for (final kind in [
    VizViewKind.liveTopology,
    VizViewKind.timeline,
    VizViewKind.delivery,
    VizViewKind.backend,
    VizViewKind.executive,
  ]) {
    for (final table in [false, true]) {
      testWidgets('${kind.name}${table ? ' as table' : ''} shows no raw '
          'dimension code in German', (tester) async {
        final world = ViewWorld();
        await tester.runAsync(world.load);
        await pumpWorld(
          tester,
          world,
          world.view(kind),
          locale: const Locale('de'),
        );
        if (table) {
          await tester.tap(find.byKey(const ValueKey('viz-as-table')));
          await tester.pumpAndSettle();
        }
        final codes = {for (final c in dimensions.values) ...c};
        for (final text in renderedTexts(tester)) {
          for (final code in codes) {
            expect(
              showsCode(text, code),
              isFalse,
              reason: '"$text" shows the code $code',
            );
          }
        }
      });
    }
  }

  test('the German labels of the owner report are translated', () async {
    const de = Locale('de');
    final texts = VizTexts(await AppLocalizations.delegate.load(de), de);
    String rate(api.StatsMetric m, String d) => texts.metricLabel(m, d);
    // language-check: off
    expect(
      rate(api.StatsMetric.toolCalls, 'completed'),
      'Werkzeugaufrufe, abgeschlossen',
    );
    expect(
      rate(api.StatsMetric.toolCalls, 'failed'),
      'Werkzeugaufrufe, fehlgeschlagen',
    );
    expect(
      rate(api.StatsMetric.runsFinished, 'cancelled'),
      'Beendete Läufe, abgebrochen',
    );
    expect(
      rate(api.StatsMetric.runsStarted, 'personal'),
      'Gestartete Läufe, Persönlich',
    );
    expect(
      rate(api.StatsMetric.messagesCreated, 'human'),
      'Nachrichten, Mensch',
    );
    // language-check: on
  });
}
