// SPDX-License-Identifier: Apache-2.0

// The row "Export of your data" of the settings section Privacy
// (docs/decisions.d/AUD-client-settings-01.md; 01 5.1 D8 "[Export]"; 20 5.1). First the
// section over the client the app builds with the answers of the real server
// (`TestExport_AsTheAppAsksForIt` in `services/server/test/privacy`): the person requests the
// export and follows it to the archive. Then every state of a request, the save dialog and
// the failures over a scripted port.

import 'dart:io';

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:auth/auth.dart' show StepUpCancelled, StepUpUnavailable;
import 'package:client/app/providers/ports.dart';
import 'package:client/settings/data_export.dart';
import 'package:client/settings/data_export_row.dart';
import 'package:client/settings/privacy_section.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:dio/dio.dart' show CancelToken;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/fakes.dart';
import '../support/file_fakes.dart';
import '../support/widgets.dart';
import 'data_export_support.dart';

/// A port that answers what the test sets and records what the row asked for.
class FakeExport implements DataExportPort {
  FakeExport([this.current]);

  /// What [latest] returns.
  DataExport? current;

  /// What [request] returns and [latest] returns from then on.
  DataExport? requested;

  /// What the next call of each method throws, once.
  Object? latestFails;
  Object? requestFails;
  Object? downloadFails;

  /// What [latest] returns after a failed download.
  DataExport? afterFailedDownload;

  final List<String> calls = [];

  @override
  Future<DataExport?> latest() async {
    calls.add('latest');
    if (latestFails case final Object error) {
      latestFails = null;
      throw error;
    }
    return current;
  }

  @override
  Future<DataExport> request({
    required String reason,
    required String purpose,
  }) async {
    calls.add('request | $reason | $purpose');
    if (requestFails case final Object error) {
      requestFails = null;
      throw error;
    }
    return current = requested!;
  }

  @override
  Future<int> download(
    DataExport export, {
    required String path,
    required String purpose,
    CancelToken? cancel,
  }) async {
    calls.add('download ${export.id} | $path | $purpose');
    if (downloadFails case final Object error) {
      downloadFails = null;
      current = afterFailedDownload ?? current;
      throw error;
    }
    return 4867;
  }
}

/// The time of the tests: a few minutes after the request of [RealAnswers].
final DateTime now = RealAnswers.requestedAt.add(const Duration(minutes: 5));

DataExport export(DataExportPhase phase) => DataExport(
  id: 'r1',
  phase: phase,
  requestedAt: RealAnswers.requestedAt,
  availableUntil: phase == DataExportPhase.ready
      ? RealAnswers.availableUntil
      : null,
);

void main() {
  /// A day and time as the row writes it, in UTC.
  String when(DateTime at, {Locale locale = const Locale('en')}) {
    final format = L10nFormat(locale);
    return '${format.relativeDay(at, now: now, timeZone: 'UTC')} '
        '${format.time(at, timeZone: 'UTC')}';
  }

  final requested = when(RealAnswers.requestedAt);
  final until = when(RealAnswers.availableUntil);

  Widget host(
    DataExportPort? port, {
    FakePicker? picker,
    String platform = 'macos',
    DateTime? at,
    Locale locale = const Locale('en'),
  }) => ProviderScope(
    overrides: [
      appPortsProvider.overrideWithValue(
        testPorts(
          dataRoot: Directory.systemTemp,
          platform: platform,
          clock: TestClock(at ?? now),
          files: fakeFilePorts(picker: picker),
        ),
      ),
      personalSettingsProvider.overrideWithValue(
        PersonalSettings.resolve(
          const {},
          desktop: platform == 'macos',
          hasCrashTarget: false,
        ),
      ),
      settingsRepositoryProvider.overrideWithValue(null),
      personTimeZoneProvider.overrideWithValue('UTC'),
      dataExportPortProvider.overrideWithValue(port),
    ],
    child: hostScreen(
      const Scaffold(body: SingleChildScrollView(child: PrivacySection())),
      locale: locale,
    ),
  );

  Future<void> settle(WidgetTester tester) async {
    for (var i = 0; i < 6; i++) {
      await tester.pump(const Duration(milliseconds: 10));
    }
  }

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.tap(find.byKey(ValueKey(key)));
    await settle(tester);
  }

  Finder key(String key) => find.byKey(ValueKey(key));

  group('over the client of the app with the answers of the real server', () {
    testWidgets(
      'the section requests the export and follows it to the archive',
      (tester) async {
        desktopWindow(tester);
        final (:port, :network) = exportPort(now: now);
        network.answers
          ..add(Answer.json(200, RealAnswers.getMe))
          ..add(Answer.json(200, RealAnswers.listBefore))
          ..add(Answer.json(201, RealAnswers.created))
          ..add(Answer.json(200, RealAnswers.listCompleted));

        await tester.pumpWidget(host(port));
        await settle(tester);
        // A request of another kind about the person is no export: the row offers one.
        expect(find.text(en.settingsDataExport), findsOneWidget);
        expect(key('settings.dataExport.state'), findsNothing);

        await tap(tester, 'settings.dataExport.request');
        expect(
          find.text(en.settingsDataExportPreparing(requested)),
          findsOneWidget,
        );
        expect(key('settings.dataExport.request'), findsNothing);

        await tap(tester, 'settings.dataExport.check');
        expect(find.text(en.settingsDataExportReady(until)), findsOneWidget);
        expect(key('settings.dataExport.download'), findsOneWidget);
        expect(key('settings.dataExport.error'), findsNothing);

        expect(network.lines, [
          'GET /api/v1/me',
          'GET /api/v1/gdpr-requests?subject_user_id=${RealAnswers.alex}',
          'POST /api/v1/gdpr-requests',
          'GET /api/v1/gdpr-requests?subject_user_id=${RealAnswers.alex}',
        ]);
        expect(network.requests[2].data, {
          'kind': 'export',
          'reason': en.settingsDataExportReason,
        });
      },
    );

    testWidgets('after the time of the archive the row says so and offers a '
        'new request', (tester) async {
      desktopWindow(tester);
      final late = RealAnswers.availableUntil.add(const Duration(minutes: 1));
      final (:port, :network) = exportPort(now: late);
      network.answers
        ..add(Answer.json(200, RealAnswers.getMe))
        ..add(Answer.json(200, RealAnswers.listCompleted));

      await tester.pumpWidget(host(port, at: late));
      await settle(tester);

      expect(key('settings.dataExport.state'), findsOneWidget);
      expect(key('settings.dataExport.download'), findsNothing);
      expect(key('settings.dataExport.request'), findsOneWidget);
    });

    testWidgets('a request another device filed meanwhile is the state', (
      tester,
    ) async {
      desktopWindow(tester);
      final (:port, :network) = exportPort(now: now);
      network.answers
        ..add(Answer.json(200, RealAnswers.getMe))
        ..add(Answer.json(200, RealAnswers.listBefore))
        ..add(Answer.json(409, RealAnswers.openConflict))
        ..add(Answer.json(200, RealAnswers.listApproved));

      await tester.pumpWidget(host(port));
      await settle(tester);
      await tap(tester, 'settings.dataExport.request');

      expect(
        find.text(en.settingsDataExportPreparing(requested)),
        findsOneWidget,
      );
      expect(key('settings.dataExport.error'), findsNothing);
    });
  });

  group('states', () {
    testWidgets('nothing shows before the login', (tester) async {
      await tester.pumpWidget(host(null));
      await settle(tester);
      expect(find.text(en.settingsDataExport), findsNothing);
      // The switches of the section stay.
      expect(find.text(en.settingsClientMetrics), findsOneWidget);
    });

    testWidgets('without a request the row offers one; the request carries '
        'its reason and the purpose of the step-up', (tester) async {
      desktopWindow(tester);
      final port = FakeExport()..requested = export(DataExportPhase.preparing);
      await tester.pumpWidget(host(port));
      await settle(tester);
      expect(find.text(en.settingsDataExportHint), findsOneWidget);

      await tap(tester, 'settings.dataExport.request');

      expect(port.calls, [
        'latest',
        'request | ${en.settingsDataExportReason} | '
            '${en.stepUpPurposeDataExportRequest}',
      ]);
      expect(
        find.text(en.settingsDataExportPreparing(requested)),
        findsOneWidget,
      );
      expect(key('settings.dataExport.check'), findsOneWidget);
    });

    testWidgets('a request that waits for an owner says so; "Check again" '
        'reads it once more', (tester) async {
      desktopWindow(tester);
      final port = FakeExport(export(DataExportPhase.waiting));
      await tester.pumpWidget(host(port));
      await settle(tester);
      expect(
        find.text(en.settingsDataExportWaiting(requested)),
        findsOneWidget,
      );
      expect(key('settings.dataExport.request'), findsNothing);

      port.current = export(DataExportPhase.ready);
      await tap(tester, 'settings.dataExport.check');

      expect(port.calls, ['latest', 'latest']);
      expect(find.text(en.settingsDataExportReady(until)), findsOneWidget);
    });

    for (final (phase, text) in [
      (DataExportPhase.expired, en.settingsDataExportExpired),
      (DataExportPhase.rejected, en.settingsDataExportRejected),
      (DataExportPhase.cancelled, en.settingsDataExportCancelled),
      (DataExportPhase.failed, en.settingsDataExportFailed),
    ]) {
      testWidgets('a request that is ${phase.name} says so and a new one can '
          'be filed', (tester) async {
        desktopWindow(tester);
        await tester.pumpWidget(host(FakeExport(export(phase))));
        await settle(tester);
        expect(find.text(text(requested)), findsOneWidget);
        expect(key('settings.dataExport.request'), findsOneWidget);
        expect(key('settings.dataExport.download'), findsNothing);
      });
    }

    testWidgets('the texts are German for a German app', (tester) async {
      desktopWindow(tester);
      final de = lookupAppLocalizations(const Locale('de'));
      await tester.pumpWidget(
        host(
          FakeExport(export(DataExportPhase.ready)),
          locale: const Locale('de'),
        ),
      );
      await settle(tester);
      expect(find.text(de.settingsDataExport), findsOneWidget);
      expect(
        find.text(
          de.settingsDataExportReady(
            when(RealAnswers.availableUntil, locale: const Locale('de')),
          ),
        ),
        findsOneWidget,
      );
      expect(find.text(de.settingsDataExportDownload), findsOneWidget);
    });
  });

  group('the archive', () {
    testWidgets('"Download" asks where to save and writes the archive there', (
      tester,
    ) async {
      desktopWindow(tester);
      final picker = FakePicker()..saveTo = '/tmp/chosen.zip';
      final port = FakeExport(export(DataExportPhase.ready));
      await tester.pumpWidget(host(port, picker: picker));
      await settle(tester);

      await tap(tester, 'settings.dataExport.download');

      expect(picker.asked, ['export-2026-11-12.zip']);
      expect(port.calls, [
        'latest',
        'download r1 | /tmp/chosen.zip | ${en.stepUpPurposeDataExportDownload}',
      ]);
      expect(key('settings.dataExport.saved'), findsOneWidget);
      expect(key('settings.dataExport.error'), findsNothing);
      // The archive stays available: the button too.
      expect(key('settings.dataExport.download'), findsOneWidget);
    });

    testWidgets('a cancelled save dialog downloads nothing', (tester) async {
      desktopWindow(tester);
      final picker = FakePicker();
      final port = FakeExport(export(DataExportPhase.ready));
      await tester.pumpWidget(host(port, picker: picker));
      await settle(tester);

      await tap(tester, 'settings.dataExport.download');

      expect(picker.asked, hasLength(1));
      expect(port.calls, ['latest']);
      expect(key('settings.dataExport.saved'), findsNothing);
    });

    testWidgets('a phone shows the state and names the desktop app for the '
        'download', (tester) async {
      final port = FakeExport(export(DataExportPhase.ready));
      await tester.pumpWidget(host(port, platform: 'ios'));
      await settle(tester);

      expect(
        find.text(en.settingsDataExportReadyNoSave(until)),
        findsOneWidget,
      );
      expect(key('settings.dataExport.download'), findsNothing);
      expect(key('settings.dataExport.request'), findsNothing);
    });

    testWidgets('an archive that expired meanwhile is named and the row shows '
        'the new state', (tester) async {
      desktopWindow(tester);
      final picker = FakePicker()..saveTo = '/tmp/chosen.zip';
      final port = FakeExport(export(DataExportPhase.ready))
        ..downloadFails = const DataExportRefused(
          ApiProblem(status: 410, code: 'gone'),
        )
        ..afterFailedDownload = export(DataExportPhase.expired);
      await tester.pumpWidget(host(port, picker: picker));
      await settle(tester);

      await tap(tester, 'settings.dataExport.download');

      expect(
        find.text(en.settingsDataExportDownloadFailed(problemText(en, 'gone'))),
        findsOneWidget,
      );
      expect(
        find.text(en.settingsDataExportExpired(requested)),
        findsOneWidget,
      );
      expect(key('settings.dataExport.request'), findsOneWidget);
    });

    testWidgets('a download that broke off says that the file was deleted', (
      tester,
    ) async {
      desktopWindow(tester);
      final picker = FakePicker()..saveTo = '/tmp/chosen.zip';
      final port = FakeExport(export(DataExportPhase.ready))
        ..downloadFails = const DataExportBrokeOff();
      await tester.pumpWidget(host(port, picker: picker));
      await settle(tester);

      await tap(tester, 'settings.dataExport.download');

      expect(find.text(en.exportBrokeOff), findsOneWidget);
      expect(key('settings.dataExport.download'), findsOneWidget);
    });
  });

  group('failures', () {
    testWidgets('a first read without an answer says so; "Check again" reads '
        'once more', (tester) async {
      desktopWindow(tester);
      final port = FakeExport(export(DataExportPhase.preparing))
        ..latestFails = const DataExportRefused(null);
      await tester.pumpWidget(host(port));
      await settle(tester);
      expect(
        find.text(en.settingsDataExportLoadFailed(en.selfOffline)),
        findsOneWidget,
      );
      // What the instance stores is unknown: no request is offered.
      expect(key('settings.dataExport.request'), findsNothing);

      await tap(tester, 'settings.dataExport.check');

      expect(key('settings.dataExport.error'), findsNothing);
      expect(
        find.text(en.settingsDataExportPreparing(requested)),
        findsOneWidget,
      );
    });

    testWidgets('a refused request names the reason and can be repeated', (
      tester,
    ) async {
      desktopWindow(tester);
      final port = FakeExport()
        ..requested = export(DataExportPhase.waiting)
        ..requestFails = const DataExportRefused(
          ApiProblem(
            status: 403,
            code: 'forbidden',
            reason: 'step_up_not_possible',
          ),
        );
      await tester.pumpWidget(host(port));
      await settle(tester);

      await tap(tester, 'settings.dataExport.request');
      expect(
        find.text(
          en.settingsDataExportRequestFailed(
            problemText(en, 'forbidden', reason: 'step_up_not_possible'),
          ),
        ),
        findsOneWidget,
      );

      await tap(tester, 'settings.dataExport.request');
      expect(key('settings.dataExport.error'), findsNothing);
      expect(
        find.text(en.settingsDataExportWaiting(requested)),
        findsOneWidget,
      );
    });

    testWidgets('a device that cannot confirm a step-up reads why', (
      tester,
    ) async {
      desktopWindow(tester);
      final port = FakeExport()..requestFails = const StepUpUnavailable();
      await tester.pumpWidget(host(port));
      await settle(tester);

      await tap(tester, 'settings.dataExport.request');

      expect(
        find.text(
          en.settingsDataExportRequestFailed(
            problemText(en, 'forbidden', reason: 'step_up_not_possible'),
          ),
        ),
        findsOneWidget,
      );
      expect(key('settings.dataExport.request'), findsOneWidget);
    });

    testWidgets('a cancelled step-up says nothing and changes nothing', (
      tester,
    ) async {
      desktopWindow(tester);
      final port = FakeExport()..requestFails = const StepUpCancelled();
      await tester.pumpWidget(host(port));
      await settle(tester);

      await tap(tester, 'settings.dataExport.request');

      expect(key('settings.dataExport.error'), findsNothing);
      expect(key('settings.dataExport.state'), findsNothing);
      expect(
        tester
            .widget<OutlinedButton>(key('settings.dataExport.request'))
            .onPressed,
        isNotNull,
      );
    });
  });

  testWidgets(
    'a login after the row appeared reads the requests of that session',
    (tester) async {
      desktopWindow(tester);
      final port = FakeExport(export(DataExportPhase.waiting));
      final current = _Port(null);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            appPortsProvider.overrideWithValue(
              testPorts(dataRoot: Directory.systemTemp, clock: TestClock(now)),
            ),
            personalSettingsProvider.overrideWithValue(
              PersonalSettings.resolve(
                const {},
                desktop: true,
                hasCrashTarget: false,
              ),
            ),
            settingsRepositoryProvider.overrideWithValue(null),
            personTimeZoneProvider.overrideWithValue('UTC'),
            dataExportPortProvider.overrideWith(
              (ref) => ref.watch(_portProvider),
            ),
            _portProvider.overrideWith(() => current),
          ],
          child: hostScreen(
            const Scaffold(
              body: SingleChildScrollView(child: PrivacySection()),
            ),
          ),
        ),
      );
      await settle(tester);
      expect(find.text(en.settingsDataExport), findsNothing);

      current.set(port);
      await settle(tester);

      expect(port.calls, ['latest']);
      expect(
        find.text(en.settingsDataExportWaiting(requested)),
        findsOneWidget,
      );
    },
  );

  test('the suggested file name is the day of the request', () {
    expect(
      dataExportFileName(export(DataExportPhase.ready)),
      'export-2026-11-12.zip',
    );
  });
}

/// The port of the session, which a test replaces after the row appeared.
class _Port extends Notifier<DataExportPort?> {
  _Port(this._first);

  final DataExportPort? _first;

  @override
  DataExportPort? build() => _first;

  void set(DataExportPort? port) => state = port;
}

final _portProvider = NotifierProvider<_Port, DataExportPort?>(
  () => _Port(null),
);
