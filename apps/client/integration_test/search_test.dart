// SPDX-License-Identifier: Apache-2.0

// Search D7 against the fake server (brief M4-23 Tests; 14 4, 6, 12 client row): the server
// search finds a message of a visible channel; offline the local index answers with the note
// "Messages stored only on this device"; after the fake removed the channel, the offline search
// finds nothing there.

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart' show ConnectionPhase;

import '../test/support/fakes.dart';
import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('online, offline with the note, and nothing after the removal', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final en = lookupAppLocalizations(const Locale('en'));
    Future<void> until(String what, bool Function() condition) async {
      final end = DateTime.now().add(const Duration(seconds: 30));
      while (!condition()) {
        if (DateTime.now().isAfter(end)) fail('timed out: $what');
        await tester.pump(const Duration(milliseconds: 50));
      }
    }

    Future<void> search(String text) async {
      await tester.enterText(find.byKey(const ValueKey('search.field')), text);
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('search.submit')));
      await tester.pump();
    }

    Future<void> offline() async {
      (app.fake.ports.network as SwitchNetwork).set(false);
      app.fake.server.disconnect(deviceId: app.deviceId);
      await until(
        'offline',
        () => app.state.services!.engine.state.phase != ConnectionPhase.live,
      );
    }

    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();
      final message = seed.post(
        seed.sam,
        seed.releaseNotes,
        'The resume window is measured from the last heartbeat.',
      );
      final hit = find.byKey(ValueKey('search.hit.${message['id']}'));
      final db = app.state.services!.database;
      Future<void> rowsUntil(String what, bool Function(int) done) async {
        final end = DateTime.now().add(const Duration(seconds: 30));
        while (true) {
          final rows = (await tester.runAsync(
            () => db.searchDao.search('resume', limit: 10),
          ))!.length;
          if (done(rows)) return;
          if (DateTime.now().isAfter(end)) fail('timed out: $what');
          await tester.pump(const Duration(milliseconds: 50));
        }
      }

      // The live event fills the local index (14 6).
      await rowsUntil('the local index row', (rows) => rows > 0);

      GoRouter.of(tester.element(find.byKey(const ValueKey('column.sidebar'))))
          .go('/search');
      await app.waitFor(find.byKey(const ValueKey('search.field')));
      await search('resume window');
      await until('the server hit', () => hit.evaluate().isNotEmpty);
      await until(
        'the server answer',
        () => find.text(en.searchResultCount(1)).evaluate().isNotEmpty,
      );
      expect(find.byKey(const ValueKey('search.localNote')), findsNothing);

      // Offline only the local index answers, with the note.
      await offline();
      await search('resume');
      await app.waitFor(find.byKey(const ValueKey('search.localNote')));
      await until('the local hit', () => hit.evaluate().isNotEmpty);

      // The channel goes; the device learns it on the reconnect (04 8.3) and its rows go.
      world.setAccess(seed.alex.id, seed.general, null);
      (app.fake.ports.network as SwitchNetwork).set(true);
      await app.waitLive();
      await rowsUntil('the removal', (rows) => rows == 0);
      await offline();
      await search('resume');
      await app.waitFor(find.byKey(const ValueKey('search.localNote')));
      await until(
        'no local hit',
        () => find.text(en.searchNoResults).evaluate().isNotEmpty,
      );
      expect(hit, findsNothing);
    } finally {
      await app.stop();
    }
  });
}
