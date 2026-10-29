// SPDX-License-Identifier: Apache-2.0

// The personal sidebar across two devices of Alex and the counters of a channel (brief M4-19
// Tests; 04 14): collapsing in the app stays on this device (01 6.7, R-PR-24), a new order from
// the other device reaches the app through `sidebar.updated`, and the counters follow messages
// from Sam (on the thread row of the thread, M4-40).

import 'package:api_client/api_client.dart';
import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';

import 'support/test_app.dart';

final AppLocalizations en = lookupAppLocalizations(const Locale('en'));

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  Future<void> until(
    WidgetTester tester,
    String what,
    bool Function() condition,
  ) async {
    final end = DateTime.now().add(const Duration(seconds: 20));
    while (!condition()) {
      if (DateTime.now().isAfter(end)) fail('timed out: $what');
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  testWidgets('order syncs across devices, collapse stays; counters follow Sam', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final ops = world.addSpace('Ops');
    world.addChannel(ops, 'alerts', {seed.alex.id: 'write'});
    final other = await app.otherDevice();
    try {
      await app.connect();
      await app.waitFor(find.byKey(const ValueKey('e2.loginName')));
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();
      final team = find.byKey(ValueKey('sidebar.space.${seed.space}'));
      final opsRow = find.byKey(ValueKey('sidebar.space.$ops'));
      await app.waitFor(opsRow);
      expect(
        tester.getTopLeft(team).dy,
        lessThan(tester.getTopLeft(opsRow).dy),
        reason: 'the global order first',
      );

      // Counters: a message of Sam, then one that mentions Alex.
      final general = find.byKey(ValueKey('sidebar.channel.${seed.general}'));
      await app.waitFor(general);
      final semantics = tester.ensureSemantics();
      seed.post(seed.sam, seed.releaseNotes, 'Build is green');
      // The thread is unread now, so it is a row below `general` with its own counters, and
      // the channel row does not count it a second time (01 6.8 item 6, M4-40).
      await app.waitFor(
        find.bySemanticsLabel(
          'Release notes, ${en.threadRowStateOpen}, ${en.threadRowUnread(1)}',
        ),
      );
      expect(
        find.bySemanticsLabel('general, ${en.channelKindNameText}'),
        findsOneWidget,
      );
      world.createMessage(seed.sam.id, seed.releaseNotes, {
        'client_msg_id': world.nextId(),
        'content_md': '@alex please check',
        'mention_ids': [seed.alex.id],
      });
      await app.waitFor(
        find.bySemanticsLabel(
          'Release notes, ${en.threadRowStateOpen}, ${en.threadRowUnread(2)}, '
          '${en.threadRowMentions(1)}',
        ),
      );
      semantics.dispose();

      // Collapse in the app: it stays on this device, the server keeps no collapse of Team.
      await tester.tap(team);
      await until(
        tester,
        'collapsed at once',
        () => general.evaluate().isEmpty,
      );
      final prefs = other.auth.api.getPreferencesApi();
      final list = (await tester.runAsync(prefs.getMySidebar))!.data!;
      expect(
        list.items.where((s) => s.spaceId == seed.space && s.collapsed),
        isEmpty,
        reason: 'the other device sees no collapse',
      );

      // A new order on the other device: Ops first, with a key the server accepts.
      await tester.runAsync(
        () => prefs.putMySidebarSpace(
          spaceId: ops,
          sidebarStatePut: SidebarStatePut(
            (b) => b
              ..collapsed = false
              ..sortKey = FractionalIndex.between(null, 'a0'),
          ),
        ),
      );
      await until(
        tester,
        'Ops above Team',
        () => tester.getTopLeft(opsRow).dy < tester.getTopLeft(team).dy,
      );
      expect(general.evaluate(), isEmpty, reason: 'Team stays collapsed');
    } finally {
      await tester.runAsync(other.dispose);
      await app.stop();
    }
  });
}
