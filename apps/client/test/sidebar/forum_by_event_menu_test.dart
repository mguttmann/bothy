// SPDX-License-Identifier: Apache-2.0

// The menu of a project forum that reached this device only through `channel.created` (brief
// FIX-access-on-created; 01 6.11, 01 6.18 item 1): the server sends no level and no capabilities
// in the event and no `access.changed` to its owner, so before the fix the forum row came with
// no access and its menu hid "Rename project" and "Delete project...". The desktop app of Alex
// runs against the fake server; the forum is created while it is connected, as on the live
// instance, and its sidebar menu offers both entries once the engine loaded the view.

import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/thread/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter/services.dart';
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
  late FakeInstance f;
  late ProviderContainer c;
  var disposed = false;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    f = await FakeInstance.start();
    c = appContainer(f);
    disposed = false;
  });

  tearDown(() async {
    if (!disposed) c.dispose();
    await f.stop();
    HttpOverrides.global = saved;
  });

  testWidgets('a project forum created while connected offers Rename project '
      'and Delete project to its owner', (tester) async {
    tester.view
      ..physicalSize = const Size(1440, 1000)
      ..devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final seed = f.seed;
    final world = f.server.world;
    await tester.runAsync(() async {
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
    });
    await tester.pumpWidget(
      UncontrolledProviderScope(container: c, child: const ClientApp()),
    );
    await _until(
      tester,
      'the sidebar',
      () async => find
          .byKey(ValueKey('sidebar.channel.${seed.general}'))
          .evaluate()
          .isNotEmpty,
    );

    // As the server: channel.created with the channel only; the owner holds `manage` through
    // the owner rule and gets no access.changed.
    final forum = world.createChannel(
      seed.alex.id,
      seed.space,
      'website',
      {seed.alex.id: 'trigger'},
      kind: 'forum',
      projectBound: true,
      capabilities: {
        seed.alex.id: {'manage'},
      },
    );
    final row = find.byKey(ValueKey('sidebar.channel.$forum'));
    await _until(tester, 'the forum row', () async {
      await tester.pump();
      return row.evaluate().isNotEmpty;
    });
    // The provider the menu watches, held while the menu is open, so the wait runs in the zone
    // of the app and the menu reads the loaded value.
    final access = c.listen(channelAccessProvider(forum), (_, _) {});
    await _until(tester, 'the access of the forum', () async {
      await tester.pump();
      return access.read().value?.capabilities.contains(Capability.manage) ??
          false;
    });
    await tester.tap(row, buttons: kSecondaryMouseButton);
    await _until(tester, 'the menu', () async {
      await tester.pump();
      return find
          .byKey(const ValueKey('menu.notifications'))
          .evaluate()
          .isNotEmpty;
    });
    expect(find.byKey(const ValueKey('menu.renameProject')), findsOneWidget);
    expect(find.byKey(const ValueKey('menu.delete')), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    access.close();
    expect(f.server.violations, isEmpty);

    // Ends the app inside the test zone, where its periodic timers live.
    await tester.pumpWidget(const SizedBox());
    c.dispose();
    disposed = true;
    await tester.pump(const Duration(minutes: 5));
  });
}
