// SPDX-License-Identifier: Apache-2.0

// The app wires the telemetry of brief M4-31 (Scope, Design notes 1, 2 and 4): the sidebar,
// the thread view and the composer report their measurement points, the sender runs while a
// person is signed in and follows `privacy.client_metrics`, and the crash path follows
// `privacy.crash_reports`.

import 'dart:async';
import 'dart:io';

import 'package:client/app/branding.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/app/providers/settings.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:client/telemetry/client_metrics.dart';
import 'package:client/telemetry/crash_reports.dart';
import 'package:client/telemetry/measurements.dart';
import 'package:client/telemetry/telemetry_providers.dart';
import 'package:client/thread/history_column.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/fakes.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import 'crash_reports_test.dart' show FakeSentry;
import 'metrics_sender_test.dart' show RecordingApi;

Set<ClientMetric> recorded(ClientMetrics m) => {
  for (final e in m.take()) e.metric,
};

AppMeasurements immediate(ClientMetrics metrics) =>
    AppMeasurements(metrics, afterFrame: (callback) => callback());

void main() {
  testWidgets('the sidebar ends the cold start once it shows spaces', (
    tester,
  ) async {
    final metrics = ClientMetrics();
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await host.db.structureDao.upsertSpaceIfNewer(space(1));
      await host.db.structureDao.upsertChannelIfNewer(channel(1));
    });
    await host.pump(
      tester,
      const SidebarColumn(),
      overrides: [
        appMeasurementsProvider.overrideWithValue(immediate(metrics)),
      ],
    );
    expect(recorded(metrics), {ClientMetric.coldStartDuration});
    await host.dispose(tester);
  });

  testWidgets('the thread view reports its open and the composer its send', (
    tester,
  ) async {
    final metrics = ClientMetrics();
    final host = await ColumnHost.open(tester);
    await host.run(
      tester,
      () => seedThread(
        host.db,
        messages: [msg(1, author: sam, text: 'Hi')],
      ),
    );
    final reads = FakeThreadReads(host.db);
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      overrides: [
        ...threadOverrides(reads),
        appMeasurementsProvider.overrideWithValue(immediate(metrics)),
      ],
    );
    expect(recorded(metrics), {ClientMetric.threadOpenDuration});
    await tester.enterText(composerField, 'Hello Sam');
    await host.settle(tester);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await host.settle(tester);
    expect(recorded(metrics), contains(ClientMetric.sendLocalDuration));
    await host.dispose(tester);
  });

  group('providers', () {
    late ClientMetrics metrics;
    late RecordingApi api;
    late FakeSentry sentry;

    late LocalDatabase db;

    ProviderContainer container({
      required bool clientMetrics,
      required bool crashReports,
      bool signedIn = true,
      Stream<Map<String, Object?>>? stored,
    }) => ProviderContainer(
      overrides: [
        localDatabaseProvider.overrideWithValue(signedIn ? db : null),
        userSettingsProvider.overrideWith(
          (ref) =>
              stored ??
              Stream.value({
                SettingKeys.clientMetrics: clientMetrics,
                SettingKeys.crashReports: crashReports,
              }),
        ),
        appPortsProvider.overrideWithValue(
          testPorts(
            dataRoot: Directory.systemTemp,
            branding: const BuildBranding(
              appName: 'ExampleChat',
              urlScheme: 'chat-example',
              bundleId: 'com.example.chat',
              defaultServer: 'https://chat.example.com',
              appVersion: '1.0.0',
              crashDsn: 'https://public@crash.example.com/1',
            ),
          ),
        ),
        appMeasurementsProvider.overrideWithValue(immediate(metrics)),
        currentInstanceProvider.overrideWithValue(null),
        clientMetricsApiProvider.overrideWithValue(signedIn ? api : null),
        crashReportsProvider.overrideWithValue(
          CrashReports(
            dsn: 'https://public@crash.example.com/1',
            release: 'client@1.0.0',
            environment: 'test',
            start: sentry.start,
            close: sentry.close,
          ),
        ),
      ],
    );

    setUp(() {
      metrics = ClientMetrics();
      api = RecordingApi();
      sentry = FakeSentry();
      db = openInMemoryForTests(clock: () => t0);
    });

    tearDown(() => db.close());

    test('signed in with the setting on, a flush sends the batch', () async {
      final c = container(clientMetrics: true, crashReports: false);
      c.listen(appTelemetryProvider, (_, _) {});
      await pumpEventQueue();
      metrics.record(ClientMetric.frames, 3);
      await c.read(metricsSenderProvider)!.flush();
      expect(api.sent, hasLength(1));
      expect(api.sent.single['platform'], 'macos');
      expect(api.sent.single['app_version'], '1.0.0');
      expect(api.sent.single['density'], 'comfortable');
      expect(['light', 'dark'], contains(api.sent.single['theme']));
      expect(sentry.started, isEmpty);
      c.dispose();
    });

    test('with the setting off nothing is sent', () async {
      final c = container(clientMetrics: false, crashReports: false);
      c.listen(appTelemetryProvider, (_, _) {});
      await pumpEventQueue();
      metrics.record(ClientMetric.frames, 3);
      await c.read(metricsSenderProvider)!.flush();
      expect(api.sent, isEmpty);
      c.dispose();
    });

    test('no sender before the login', () {
      final c = container(
        clientMetrics: true,
        crashReports: false,
        signedIn: false,
      );
      expect(c.read(metricsSenderProvider), isNull);
      c.dispose();
    });

    test('crash reports start with the switch on', () async {
      final c = container(clientMetrics: true, crashReports: true);
      c.listen(appTelemetryProvider, (_, _) {});
      await pumpEventQueue();
      await pumpEventQueue();
      expect(sentry.started, hasLength(1));
      c.dispose();
    });

    test('an opt-out still loading never starts crash reports', () async {
      final stored = StreamController<Map<String, Object?>>();
      final c = container(
        clientMetrics: true,
        crashReports: false,
        stored: stored.stream,
      );
      c.listen(appTelemetryProvider, (_, _) {});
      await pumpEventQueue();
      await pumpEventQueue();
      expect(sentry.started, isEmpty, reason: 'started before the setting');
      stored.add({SettingKeys.crashReports: false});
      await pumpEventQueue();
      expect(sentry.started, isEmpty);
      c.dispose();
      await stored.close();
    });

    test('signed out, crash reports do not start', () async {
      final c = container(
        clientMetrics: true,
        crashReports: true,
        signedIn: false,
        stored: Stream.value(const {}),
      );
      c.listen(appTelemetryProvider, (_, _) {});
      await pumpEventQueue();
      await pumpEventQueue();
      expect(sentry.started, isEmpty);
      c.dispose();
    });
  });
}
