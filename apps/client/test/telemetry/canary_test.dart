// SPDX-License-Identifier: Apache-2.0

// The canary test of the client paths (brief M4-31 Design note 5; PLAN M4-31; 15 16 and 17;
// ADR 0057): while the app shows a thread with a canary message text, a canary channel name, a
// canary thread title and a canary file name, and while its HTTP layer carries a canary access
// token, refresh token and DPoP proof, the collector, the sender (with a recording API) and the
// crash path (the Sentry pipeline with a recording transport) run; no canary value may appear in
// any sent batch, crash event, breadcrumb or log line, and no reported URL keeps a parameter.

import 'dart:async';
import 'dart:convert';

import 'package:client/telemetry/client_metrics.dart';
import 'package:client/telemetry/crash_reports.dart';
import 'package:client/telemetry/measurements.dart';
import 'package:client/telemetry/metrics_sender.dart';
import 'package:client/telemetry/telemetry_providers.dart';
import 'package:client/thread/history_column.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sentry_flutter/sentry_flutter.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import 'metrics_sender_test.dart' show ManualTimer, RecordingApi;

const canaryText = 'CANARYTEXT7f3a';
const canaryChannel = 'canarychannel9b2c';
const canaryTitle = 'CANARYTITLE4d1e';
const canaryFile = 'CANARYFILE5e6f.pdf';
const canaryAccess = 'CANARYACCESS0a1b2c3d4e5f6';
const canaryRefresh = 'CANARYREFRESH6f5e4d3c2b1a';
const canaryProof = 'CANARYPROOF.eyJhbGciOiJFUzI1NiJ9.sig';
const canaryQuery = 'CANARYQUERY8c7d';

const canaries = [
  canaryText,
  canaryChannel,
  canaryTitle,
  canaryFile,
  'CANARYFILE5e6f',
  canaryAccess,
  canaryRefresh,
  canaryProof,
  'CANARYPROOF',
  canaryQuery,
];

/// Keeps every envelope Sentry would send, serialized as it would leave the device.
final class RecordingTransport implements Transport {
  RecordingTransport(this.options);
  final SentryOptions options;
  final List<String> sent = [];

  @override
  Future<SentryId?> send(SentryEnvelope envelope) async {
    final bytes = <int>[];
    await envelope.envelopeStream(options).forEach(bytes.addAll);
    sent.add(utf8.decode(bytes));
    return SentryId.newId();
  }
}

void expectNoCanary(String where, Iterable<String> texts) {
  for (final text in texts) {
    for (final c in canaries) {
      expect(text, isNot(contains(c)), reason: '$c leaked into $where');
    }
  }
}

void main() {
  testWidgets('no canary value leaves the device', (tester) async {
    final logs = <String>[];
    final previousPrint = debugPrint;
    debugPrint = (message, {wrapWidth}) => logs.add(message ?? '');

    // The app shows a thread with canary content and measures it.
    final metrics = ClientMetrics();
    final measurements = AppMeasurements(
      metrics,
      afterFrame: (callback) => callback(),
    );
    final host = await ColumnHost.open(tester);
    await host.run(
      tester,
      () => seedThread(
        host.db,
        ch: channel(1, name: canaryChannel),
        th: thread(1, channelN: 1, title: canaryTitle, lastSeq: 1),
        messages: [msg(1, author: sam, text: '$canaryText see $canaryFile')],
      ),
    );
    final reads = FakeThreadReads(host.db);
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      overrides: [
        ...threadOverrides(reads),
        appMeasurementsProvider.overrideWithValue(measurements),
      ],
    );
    expect(find.textContaining(canaryText), findsWidgets);
    await tester.enterText(composerField, '$canaryText again');
    await host.settle(tester);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await host.settle(tester);
    measurements
      ..recordFrames([
        const Duration(milliseconds: 9),
        const Duration(milliseconds: 30),
      ], budget: const Duration(microseconds: 16667))
      ..metrics.record(ClientMetric.reconnects, 1, closeCode: 4009);

    // The sender sends what the app measured.
    final api = RecordingApi();
    late ManualTimer timer;
    final sender = MetricsSender(
      metrics: metrics,
      api: api,
      interval: const Duration(minutes: 5),
      enabled: () => true,
      attributes: () => const MetricsAttributes(
        platform: 'macos',
        appVersion: '1.4.0',
        density: 'comfortable',
        theme: 'dark',
      ),
      timer: (interval, tick) => timer = ManualTimer(interval, tick),
      log: logs.add,
    )..start();
    await tester.runAsync(timer.fire);
    expect(api.sent, hasLength(1));
    expect(
      {for (final m in api.sent.single['metrics']! as List) (m as Map)['name']},
      containsAll([
        'client.thread_open.duration',
        'client.send_local.duration',
        'client.frames',
        'client.reconnects',
      ]),
    );
    expectNoCanary('a batch', [jsonEncode(api.sent)]);

    // A crash while the thread shows, with breadcrumbs of the HTTP layer carrying the tokens.
    late RecordingTransport transport;
    await tester.runAsync(() async {
      await Sentry.init((o) {
        CrashReports.configureCommon(
          o,
          dsn: 'https://public@crash.example.com/1',
          release: 'client@1.4.0',
          environment: 'test',
        );
        transport = RecordingTransport(o);
        o.transport = transport;
      });
      Sentry.configureScope((scope) {
        scope.setTag('thread_title', canaryTitle);
        scope.setContexts('thread', {'channel': canaryChannel});
      });
      await Sentry.addBreadcrumb(
        Breadcrumb(
          type: 'http',
          category: 'http',
          data: {
            'url':
                'https://chat.example.com/api/v1/files/$canaryFile'
                '?access_token=$canaryAccess&q=$canaryQuery#$canaryText',
            'method': 'GET',
            'status_code': 200,
            'request_headers': {
              'Authorization': 'DPoP $canaryAccess',
              'DPoP': canaryProof,
              'DPoP-Nonce': canaryProof,
              'Cookie': 'refresh=$canaryRefresh',
            },
            'request_body': '{"refresh_token":"$canaryRefresh"}',
            'response_body': canaryText,
          },
        ),
      );
      await Sentry.addBreadcrumb(
        Breadcrumb(
          type: 'navigation',
          category: 'navigation',
          data: {
            'from': '/c/$canaryChannel',
            'to': '/t/${threadId(1).value}?q=$canaryQuery',
          },
        ),
      );
      await Sentry.addBreadcrumb(
        Breadcrumb(message: 'typed $canaryText', category: 'ui.input'),
      );
      try {
        throw StateError('failed on $canaryText in $canaryFile');
      } on StateError catch (e, st) {
        await Sentry.captureException(e, stackTrace: st);
      }
      await Sentry.captureMessage('thread $canaryTitle');
      await Sentry.close();
    });
    expect(transport.sent, isNotEmpty, reason: 'the crash path sent nothing');
    expectNoCanary('a crash report', transport.sent);
    final urls = RegExp(r'https?://[^"\s]+');
    for (final report in transport.sent) {
      for (final url in urls.allMatches(report)) {
        expect(url[0], isNot(contains('?')), reason: 'a URL with parameters');
      }
    }

    // And no log line names a canary.
    debugPrint = previousPrint;
    expectNoCanary('a log line', logs);
    await sender.stop();
    await host.dispose(tester);
  });
}
