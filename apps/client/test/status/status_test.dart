// SPDX-License-Identifier: Apache-2.0

// The status page of the app against the fake server of M4-17, headless (brief M7-21 Tests):
// the component of an agent shows only when the server returns it, `status.updated` through the
// sync engine reloads the page, and an outage of the server shows "Server unreachable" instead
// of the last page.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show StatusComponentState;
import 'package:client/app/providers/session.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/status/status_route.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';

/// The patience of the repository's test kits (`testkit.Patience`).
const Duration _patience = Duration(minutes: 3);

/// Lets real IO run and advances the fake clock of the test zone a little each round, so the
/// client's own timers (requests, reconnect backoff) fire too, until [finder] finds something;
/// bounded only by the patience.
Future<void> _until(WidgetTester tester, Finder finder, String what) async {
  final watch = Stopwatch()..start();
  while (finder.evaluate().isEmpty) {
    if (watch.elapsed >= _patience) {
      fail('$what did not happen within $_patience');
    }
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 5)),
    );
    await tester.pump(const Duration(milliseconds: 5));
  }
}

void main() {
  HttpOverrides? saved;
  late FakeInstance fake;
  late ProviderContainer c;
  late String lotse;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    fake = await FakeInstance.start();
    c = appContainer(fake);
    final world = fake.server.world;
    world
      ..addStatusComponent('Messages and sync', healthSource: 'api')
      ..addStatusComponent('Push notifications', healthSource: 'push');
    final agent = world.addPerson(
      handle: 'lotse',
      displayName: 'Lotse',
      kind: 'agent',
    );
    lotse = world.addStatusComponent('Lotse', agentId: agent.id);
    world.agentReaders[agent.id] = {fake.seed.alex.id};
  });

  tearDown(() async {
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  Future<StatusController> live(String login) async {
    final control = await connected(c, fake);
    await control.loginWithPassword(login, WorldSeed.password, WorldSeed.totp);
    expect(c.read(onboardingControllerProvider).step, OnboardingStep.signedIn);
    await eventually(
      'live',
      () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
      timeout: _patience,
    );
    final controller = StatusController(c.read(appStatusSourceProvider)!)
      ..start();
    addTearDown(controller.dispose);
    await eventually(
      'the first page',
      () => controller.page != null,
      timeout: _patience,
    );
    return controller;
  }

  test('the component of an agent only when the server returns it', () async {
    final alex = await live(WorldSeed.alexLogin);
    expect(alex.page!.components.map((x) => x.id), contains(lotse));
  });

  test('without agent.read the page has no agent component', () async {
    final sam = await live(WorldSeed.samLogin);
    expect(sam.page!.components.map((x) => x.name), [
      'Messages and sync',
      'Push notifications',
    ]);
  });

  test('status.updated through the sync engine reloads the page', () async {
    final alex = await live(WorldSeed.alexLogin);
    final world = fake.server.world;
    world.statusComponents.first['state'] = 'major_outage';
    world.statusUpdated(
      componentIds: [world.statusComponents.first['id']! as String],
    );
    await eventually(
      'the reloaded page',
      () => alex.page!.overallState == StatusComponentState.majorOutage,
      timeout: _patience,
    );
  });

  test('an outage of the server shows Server unreachable', () async {
    final alex = await live(WorldSeed.alexLogin);
    expect(alex.unreachable, isFalse);
    await fake.server.stop();
    await eventually(
      'Server unreachable',
      () => alex.unreachable,
      timeout: _patience,
    );
  });

  testWidgets('the page shows Server unreachable and no state', (tester) async {
    await tester.runAsync(() async {
      final control = await connected(c, fake);
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
    });
    const ds = DsContext(
      theme: DsThemeMode.light,
      density: DsDensityMode.comfortable,
      motion: DsMotionMode.reduced,
      accent: dsDefaultAccent,
    );
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: c,
        child: MaterialApp(
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          theme: buildDsTheme(ds),
          builder: (context, app) => DsThemeScope(dsContext: ds, child: app!),
          home: const Scaffold(body: AppStatusPage(notFound: SizedBox())),
        ),
      ),
    );
    await _until(
      tester,
      find.byKey(ValueKey('status.component.$lotse')),
      'the page',
    );
    await tester.runAsync(fake.server.stop);
    await _until(tester, find.text('Server unreachable'), 'Server unreachable');
    expect(find.byKey(ValueKey('status.component.$lotse')), findsNothing);
    // The client's timers (in the fake clock) run out before the test ends.
    await tester.pumpWidget(const SizedBox());
    await tester.pump(const Duration(minutes: 5));
  });
}
