// SPDX-License-Identifier: Apache-2.0

// A7 "Statistics" of the app against the fake server of M4-17, headless (brief M6-06 Tests:
// integration): the REST source of admin_ui reads the statistics operations through the client
// of the signed-in session, every answer validates against the contract, the telemetry link
// reaches the owner only, and the screen of `/admin/statistics` shows the numbers the server
// answered.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:client/admin/admin_route.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import 'admin_step_up_test.dart' show RecordingPrompt;

const String _url = 'https://telemetry.example.com/d/bothy';

void main() {
  HttpOverrides? saved;
  late FakeInstance fake;
  late ProviderContainer c;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    fake = await FakeInstance.start(stepUpPrompt: RecordingPrompt());
    c = appContainer(fake);
    final world = fake.server.world;
    world.telemetryDashboardUrl = _url;
    world.stats['getRunStats'] = {
      'buckets': [
        {
          'start': '2026-10-16T00:00:00.000Z',
          'group_key': '0192f4a0-7b1c-7000-8a00-000000000201',
          'metrics': {'runs': 7.0, 'completed': 6.0, 'failed': 1.0},
        },
      ],
    };
    world.stats['getUsageStats'] = {
      'buckets': [
        {
          'start': '2026-10-12T00:00:00.000Z',
          'group_key': '0192f4a0-7b1c-7000-8a00-000000000001',
          'metrics': {'messages': 3.0},
          'below_min_group': ['active_people'],
        },
      ],
    };
  });

  tearDown(() async {
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  Future<AdminRepository> signedIn(String login) async {
    final control = await connected(c, fake);
    await control.loginWithPassword(login, WorldSeed.password, WorldSeed.totp);
    expect(c.read(onboardingControllerProvider).step, OnboardingStep.signedIn);
    // Read into a local first (see hints_test.dart).
    final admin = c.read(adminRepositoryProvider)!;
    return admin;
  }

  final query = StatsQuery(
    from: DateTime.utc(2026, 10, 10, 9),
    to: DateTime.utc(2026, 10, 17, 9),
    bucket: StatsBucketSize.day,
  );

  test('the owner reads the statistics and the telemetry link', () async {
    final admin = await signedIn(WorldSeed.alexLogin);
    final runs = await admin.runStats(query);
    expect(runs.buckets.single.metrics['runs'], 7);
    final usage = await admin.usageStats(
      query.withBucket(StatsBucketSize.week),
    );
    expect(usage.buckets.single.belowMinGroup, ['active_people']);
    final costs = await admin.costStats(
      query,
      groupBy: StatsCostGrouping.agent,
    );
    expect(costs.buckets, isEmpty);
    final instance = await admin.statsInstance();
    expect(instance.telemetryDashboardUrl, Uri.parse(_url));
    expect(instance.timeZone, 'UTC');
  });

  test('a person who is no owner: no link, the instance refused', () async {
    final admin = await signedIn(WorldSeed.samLogin);
    expect((await admin.statsInstance()).telemetryDashboardUrl, isNull);
    final error = await admin
        .runStats(query)
        .then<Object?>((_) => null, onError: (Object e) => e);
    expect((error! as SelfActionFailed).problem?.status, 403);
  });

  testWidgets('the administration opens A7 against the fake server', (
    tester,
  ) async {
    final admin = (await tester.runAsync(() => signedIn(WorldSeed.alexLogin)))!;
    final opened = <Uri>[];
    const ds = DsContext(
      theme: DsThemeMode.light,
      density: DsDensityMode.comfortable,
      motion: DsMotionMode.reduced,
      accent: dsDefaultAccent,
    );
    tester.view
      ..physicalSize = const Size(1400, 1000)
      ..devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      MaterialApp(
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        theme: buildDsTheme(ds),
        builder: (context, app) => DsThemeScope(dsContext: ds, child: app!),
        home: Scaffold(
          body: AdminBody(
            repository: admin,
            location: adminLocationOf(['admin', 'statistics'])!,
            onNavigate: (_) {},
            openExternal: opened.add,
          ),
        ),
      ),
    );
    // The requests travel over real sockets to the fake: let the real IO run and advance the
    // fake clock of the test zone a little each round, so the client's own timers fire too;
    // the condition ends the wait, bounded only by the patience.
    final watch = Stopwatch()..start();
    while (find.byKey(const ValueKey('stats.tiles')).evaluate().isEmpty) {
      if (watch.elapsed > const Duration(minutes: 3)) {
        fail(
          'A7 did not load: ${find.byType(Text).evaluate().map((e) => (e.widget as Text).data).toList()}',
        );
      }
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 5)),
      );
      await tester.pump(const Duration(milliseconds: 5));
    }
    expect(find.byKey(const ValueKey('stats.tiles')), findsOneWidget);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('stats.tile.0')),
        matching: find.text('7'),
      ),
      findsOneWidget,
    );
    await tester.tap(find.byKey(const ValueKey('stats.telemetry')));
    expect(opened, [Uri.parse(_url)]);
    // The client's timers of the requests (in the fake clock) run out before the test ends.
    await tester.pumpWidget(const SizedBox());
    await tester.pump(const Duration(minutes: 5));
  });
}
