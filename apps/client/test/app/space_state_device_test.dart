// SPDX-License-Identifier: Apache-2.0

// The collapse state of a space stays on its device (01 6.7, R-PR-24; 04 14, R-SY-29; brief
// FIX-device-mirroring-2). Live mechanism of the owner report: the person expanded or collapsed
// a space on one device to open a chat, and the sidebar of the other device expanded or
// collapsed it at the same moment (`PUT /me/sidebar/spaces/{id}`, then `sidebar.updated`).
// Device A is the desktop app, device B the phone app beside it, both of Alex against one fake
// server. Each taps the row of the space "Team" in turn; the other keeps its own state, while a
// setting written after the tap still reaches both (data syncs).

import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/sidebar/providers.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';

/// The patience of the repository's test kits (`testkit.Patience`).
const Duration _patience = Duration(minutes: 3);

/// Lets real IO run and advances the fake clock of the test zone a little each round, until
/// [condition] holds; bounded by the patience.
Future<void> _until(
  WidgetTester tester,
  String what,
  Future<bool> Function() condition,
) async {
  final watch = Stopwatch()..start();
  while (!await condition()) {
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
  late FakeInstance first;
  late FakeInstance second;
  late ProviderContainer a;
  late ProviderContainer b;
  var disposed = false;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    first = await FakeInstance.start();
    second = await first.sibling(platform: 'ios');
    a = appContainer(first);
    b = appContainer(second);
    disposed = false;
  });

  tearDown(() async {
    if (!disposed) {
      a.dispose();
      b.dispose();
    }
    await second.stopSibling();
    await first.stop();
    HttpOverrides.global = saved;
  });

  Future<void> signIn(ProviderContainer c, FakeInstance f) async {
    final control = await connected(c, f);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
    );
  }

  testWidgets(
    'the phone collapses a space: the desktop sidebar beside it keeps its own state',
    (tester) async {
      tester.view
        ..physicalSize = const Size(1440 + 390, 1000)
        ..devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      final seed = first.seed;
      await tester.runAsync(() async {
        await signIn(a, first);
        await signIn(b, second);
      });
      // Two processes in one test: the desktop app of device A and the phone app of device B,
      // whose start page is the Spaces page (the same `SidebarColumn`).
      await tester.pumpWidget(
        Directionality(
          textDirection: TextDirection.ltr,
          child: Row(
            children: [
              SizedBox(
                width: 1440,
                child: UncontrolledProviderScope(
                  container: a,
                  child: const ClientApp(),
                ),
              ),
              SizedBox(
                width: 390,
                child: UncontrolledProviderScope(
                  container: b,
                  child: const ClientApp(),
                ),
              ),
            ],
          ),
        ),
      );
      Finder on(ProviderContainer c, Finder f) => find.descendant(
        of: find.byWidgetPredicate(
          (w) => w is UncontrolledProviderScope && w.container == c,
        ),
        matching: f,
      );
      final team = find.byKey(ValueKey('sidebar.space.${seed.space}'));
      final general = find.byKey(ValueKey('sidebar.channel.${seed.general}'));
      bool shows(ProviderContainer c, Finder f) =>
          on(c, f).evaluate().isNotEmpty;
      bool expanded(ProviderContainer c) => c
          .read(spacesWithStateProvider)
          .any((s) => s.space.id.value == seed.space && !s.collapsed);

      await _until(
        tester,
        'Team expanded on both devices',
        () async => shows(a, general) && shows(b, general),
      );

      // A personal event written after a tap reaches a device only after every earlier event
      // of its log (04 7.1), so once a setting changed by the server is on both devices, a
      // `sidebar.updated` of the tap would have been applied too. The tap writes its outbox
      // entry right after the local row, so a few rounds of real IO come first.
      var marker = false;
      Future<void> afterTheTap(ProviderContainer tapped) async {
        for (var i = 0; i < 20; i++) {
          await tester.runAsync(
            () => Future<void>.delayed(const Duration(milliseconds: 5)),
          );
          await tester.pump(const Duration(milliseconds: 5));
        }
        await _until(
          tester,
          'the outbox of the tapping device is empty',
          () async {
            final db = tapped.read(localDatabaseProvider)!;
            return (await tester.runAsync(db.outboxDao.allEntries))!.isEmpty;
          },
        );
        marker = !marker;
        first.server.world.putSetting(
          seed.alex.id,
          'composer.enter_sends',
          marker,
        );
        for (final c in [a, b]) {
          await _until(tester, 'the setting on both devices', () async {
            final db = c.read(localDatabaseProvider)!;
            final settings = await tester.runAsync(
              () => db.personalDao.watchSettings().first,
            );
            return settings!.any(
              (s) => s.key == 'composer.enter_sends' && s.value == marker,
            );
          });
        }
        await tester.pump();
      }

      // The phone collapses Team; the desktop keeps it expanded.
      await tester.tap(on(b, team));
      await _until(tester, 'Team collapsed on the phone', () async {
        await tester.pump();
        return !shows(b, general);
      });
      await afterTheTap(b);
      expect(expanded(a), isTrue, reason: 'A collapsed Team because B did');
      expect(shows(a, general), isTrue);
      expect(expanded(b), isFalse);

      // The desktop collapses Team as well, then expands it; the phone keeps it collapsed.
      await tester.tap(on(a, team));
      await _until(tester, 'Team collapsed on the desktop', () async {
        await tester.pump();
        return !shows(a, general);
      });
      await tester.tap(on(a, team));
      await _until(tester, 'Team expanded again on the desktop', () async {
        await tester.pump();
        return shows(a, general);
      });
      await afterTheTap(a);
      expect(expanded(b), isFalse, reason: 'B expanded Team because A did');
      expect(shows(b, general), isFalse);
      expect(first.server.violations, isEmpty);

      // Ends both devices inside the test zone, where the apps' periodic timers live.
      await tester.pumpWidget(const SizedBox());
      a.dispose();
      b.dispose();
      disposed = true;
      await tester.pump(const Duration(minutes: 5));
    },
  );
}
