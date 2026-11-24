// SPDX-License-Identifier: Apache-2.0

// The app widget with its router (brief M4-18 Design note 4; 15 11; 01 6.6, 6.7): onboarding
// before the login, a link before the login opened after it, "Not found or no access" for an
// invisible target, the offline and update banners, the window title.

import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/discovery.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/app/providers/target.dart';
import 'package:client/app/rich_clipboard.dart';
import 'package:client/app/target_resolver.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/phone/providers.dart' show outboxWaitingProvider;
import 'package:client/telemetry/client_metrics.dart';
import 'package:client/telemetry/measurements.dart';
import 'package:client/telemetry/metrics_sender.dart';
import 'package:client/telemetry/telemetry_providers.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show AppConfig, AppConfigValues;
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/fakes.dart';
import '../support/instances.dart';
import '../support/widgets.dart';
import '../telemetry/metrics_sender_test.dart' show ManualTimer, RecordingApi;

const String thread = '0193f2a7-0000-7000-8000-000000000001';

/// A controller whose state the test sets.
class ScriptedOnboarding extends OnboardingController {
  ScriptedOnboarding(this.initial);

  final OnboardingState initial;
  int logouts = 0;

  @override
  OnboardingState build() => initial;

  void show(OnboardingState next) => state = next;

  @override
  Future<void> logout() async => logouts++;
}

void main() {
  late Directory root;
  setUp(() async => root = Directory.systemTemp.createTempSync('app'));
  tearDown(() => root.deleteSync(recursive: true));

  Future<(ScriptedOnboarding, ManualDeepLinks)> pumpApp(
    WidgetTester tester, {
    OnboardingState initial = const OnboardingState(
      step: OnboardingStep.server,
    ),
    List<Override> extra = const [],
    List<Uri> pending = const [],
  }) async {
    desktopWindow(tester);
    final control = ScriptedOnboarding(initial);
    final links = ManualDeepLinks()..pending.addAll(pending);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appPortsProvider.overrideWithValue(
            testPorts(dataRoot: root, deepLinks: links),
          ),
          onboardingControllerProvider.overrideWith(() => control),
          ...extra,
        ],
        child: const ClientApp(),
      ),
    );
    await tester.pumpAndSettle();
    return (control, links);
  }

  testWidgets('before the login E1 shows and the title is the build name', (
    tester,
  ) async {
    await pumpApp(tester);
    expect(find.byKey(const ValueKey('screen.e1')), findsOneWidget);
    expect(
      tester.widget<MaterialApp>(find.byType(MaterialApp)).title,
      testBranding.appName,
    );
  });

  testWidgets(
    'until the first screen is decided the start screen shows, not E1',
    (tester) async {
      final (control, _) = await pumpApp(
        tester,
        initial: const OnboardingState(
          step: OnboardingStep.server,
          starting: true,
        ),
      );
      expect(find.byKey(const ValueKey('screen.start')), findsOneWidget);
      // A screen reader hears the name of the build for the mark.
      expect(find.bySemanticsLabel(testBranding.appName), findsOneWidget);
      expect(find.byKey(const ValueKey('screen.e1')), findsNothing);
      expect(find.byKey(const ValueKey('e1.address')), findsNothing);
      expect(find.byKey(const ValueKey('e1.connect')), findsNothing);
      control.show(const OnboardingState(step: OnboardingStep.server));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('screen.start')), findsNothing);
      expect(find.byKey(const ValueKey('screen.e1')), findsOneWidget);
    },
  );

  testWidgets('block copies go to the rich clipboard of the runner (M4-48)', (
    tester,
  ) async {
    await pumpApp(tester);
    final scope = tester.widget<DsClipboardScope>(
      find.byType(DsClipboardScope),
    );
    expect(scope.clipboard, isA<PlatformRichClipboard>());
  });

  testWidgets('code blocks follow the choice of the device and the limit of '
      'the configuration (M4-49)', (tester) async {
    await pumpApp(tester);
    final scope = tester.widget<DsCodeBlockScope>(
      find.byType(DsCodeBlockScope),
    );
    // Signed out there is no local database: blocks wrap, the toggle stores nothing.
    expect(scope.wrap, isTrue);
    expect(scope.collapseLines, AppConfig.defaults().codeBlockCollapseLines);
    expect(scope.onWrapChanged, isNotNull);
  });

  testWidgets('each step of the controller has its screen', (tester) async {
    final (control, _) = await pumpApp(tester);
    final steps = {
      OnboardingStep.login: 'screen.e2',
      OnboardingStep.enroll: 'screen.e3',
      OnboardingStep.pending: 'screen.e4',
      OnboardingStep.pair: 'screen.e5',
      OnboardingStep.signedOut: 'screen.e7',
    };
    for (final MapEntry(key: step, value: key) in steps.entries) {
      control.show(
        OnboardingState(
          step: step,
          code: '481207',
          deadline: DateTime.now().add(const Duration(hours: 1)),
        ).withInstance(oidcOnly),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(ValueKey(key)), findsOneWidget, reason: step.name);
    }
    expect(
      tester.widget<MaterialApp>(find.byType(MaterialApp)).title,
      'Team Chat',
    );
  });

  testWidgets('a link before the login is kept and opened after it', (
    tester,
  ) async {
    final (control, links) = await pumpApp(
      tester,
      pending: [Uri.parse('chat-example://t/$thread')],
      extra: [
        targetStatusProvider.overrideWith(
          (ref, path) async => TargetStatus.notFound,
        ),
      ],
    );
    expect(find.byKey(const ValueKey('screen.e1')), findsOneWidget);
    links.send(Uri.parse('https://elsewhere.example.com'));
    await tester.pumpAndSettle();
    control.show(const OnboardingState(step: OnboardingStep.signedIn));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('column.sidebar')), findsOneWidget);
    expect(find.byKey(const ValueKey('target.notFound')), findsOneWidget);
    expect(find.text(en.accessNotFoundOrNoAccess), findsOneWidget);
  });

  testWidgets(
    'signed in, a link goes to its target; an invisible one is not found',
    (tester) async {
      final (_, links) = await pumpApp(
        tester,
        initial: const OnboardingState(step: OnboardingStep.signedIn),
        extra: [
          targetStatusProvider.overrideWith(
            (ref, path) async => path == '/' || path.contains(thread)
                ? TargetStatus.visible
                : TargetStatus.notFound,
          ),
        ],
      );
      expect(find.byKey(const ValueKey('column.history')), findsOneWidget);
      expect(find.byKey(const ValueKey('target.notFound')), findsNothing);
      links.send(
        Uri.parse('chat-example://t/0193f2a7-0000-7000-8000-0000000000ff'),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('target.notFound')), findsOneWidget);
      links.send(Uri.parse('chat-example://t/$thread'));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('target.notFound')), findsNothing);
      links.send(Uri.parse('chat-example://nowhere'));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('target.notFound')), findsOneWidget);
    },
  );

  testWidgets('a start through a login screen reports no cold start', (
    tester,
  ) async {
    // The wait for the person landed in the cold start (AUD-PKG-P02-client-b, #1285).
    final metrics = ClientMetrics();
    final measurements = AppMeasurements(
      metrics,
      afterFrame: (callback) => callback(),
    );
    final (control, _) = await pumpApp(
      tester,
      initial: const OnboardingState(
        step: OnboardingStep.server,
        starting: true,
      ),
      extra: [appMeasurementsProvider.overrideWithValue(measurements)],
    );
    control.show(const OnboardingState(step: OnboardingStep.login));
    await tester.pumpAndSettle();
    measurements.sidebarShown();
    expect(metrics.take(), isEmpty);
  });

  testWidgets('the app metrics go out when the app moves to the background', (
    tester,
  ) async {
    // iOS and Android never ask to quit, so the last values were lost with the process
    // (AUD-PKG-P02-client-b, #1282).
    final metrics = ClientMetrics();
    final api = RecordingApi();
    final sender = MetricsSender(
      metrics: metrics,
      api: api,
      interval: const Duration(minutes: 5),
      enabled: () => true,
      attributes: () =>
          const MetricsAttributes(platform: 'ios', appVersion: '1.4.0'),
      timer: ManualTimer.new,
    );
    await pumpApp(
      tester,
      extra: [metricsSenderProvider.overrideWithValue(sender)],
    );
    metrics.record(ClientMetric.frames, 3);
    for (final step in [AppLifecycleState.inactive, AppLifecycleState.hidden]) {
      tester.binding.handleAppLifecycleStateChanged(step);
    }
    await tester.pump();
    expect(api.sent, hasLength(1));
    for (final step in [
      AppLifecycleState.inactive,
      AppLifecycleState.resumed,
    ]) {
      tester.binding.handleAppLifecycleStateChanged(step);
    }
    await tester.pump();
  });

  testWidgets('the offline banner, the update banner and the logout button', (
    tester,
  ) async {
    final (control, _) = await pumpApp(
      tester,
      initial: const OnboardingState(step: OnboardingStep.signedIn),
    );
    expect(find.byKey(const ValueKey('shell.offline')), findsOneWidget);
    // AUD-PKG-P02-client-b: "Log out" in the foot asks once, like the phone; Cancel keeps
    // the login.
    await tester.tap(find.byKey(const ValueKey('shell.logout')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('logout.dialog')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('logout.cancel')));
    await tester.pumpAndSettle();
    expect(control.logouts, 0);
    await tester.tap(find.byKey(const ValueKey('shell.logout')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('logout.confirm')));
    await tester.pumpAndSettle();
    expect(control.logouts, 1);
  });

  testWidgets('"Log out" in the foot names the changes that still wait', (
    tester,
  ) async {
    await pumpApp(
      tester,
      initial: const OnboardingState(step: OnboardingStep.signedIn),
      extra: [outboxWaitingProvider.overrideWith((ref) => Stream.value(2))],
    );
    await tester.tap(find.byKey(const ValueKey('shell.logout')));
    await tester.pumpAndSettle();
    expect(
      tester.widget<Text>(find.byKey(const ValueKey('logout.waiting'))).data,
      en.phoneLogoutWaiting(2),
    );
  });

  testWidgets('live hides the banner', (tester) async {
    await pumpApp(
      tester,
      initial: const OnboardingState(step: OnboardingStep.signedIn),
      extra: [
        connectionStateProvider.overrideWith(
          (ref) => Stream.value(const ConnectionState(ConnectionPhase.live)),
        ),
      ],
    );
    expect(find.byKey(const ValueKey('shell.offline')), findsNothing);
  });

  testWidgets('4007 shows "Update required"', (tester) async {
    await pumpApp(
      tester,
      initial: const OnboardingState(step: OnboardingStep.signedIn),
      extra: [
        connectionStateProvider.overrideWith(
          (ref) => Stream.value(
            const ConnectionState(ConnectionPhase.updateRequired),
          ),
        ),
      ],
    );
    expect(find.byKey(const ValueKey('shell.updateRequired')), findsOneWidget);
  });

  test('the instance providers are empty before the login', () {
    final c = ProviderContainer(
      overrides: [
        appPortsProvider.overrideWithValue(testPorts(dataRoot: root)),
        onboardingControllerProvider.overrideWith(
          () => ScriptedOnboarding(
            const OnboardingState(step: OnboardingStep.login)
                .withInstance(nothing),
          ),
        ),
      ],
    );
    addTearDown(c.dispose);
    expect(c.read(signedInServicesProvider), isNull);
    expect(c.read(localDatabaseProvider), isNull);
    expect(c.read(syncEngineProvider), isNull);
    expect(c.read(currentInstanceProvider), same(nothing));
    expect(nothing, isA<InstanceInfo>());
  });
}
