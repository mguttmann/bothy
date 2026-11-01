// SPDX-License-Identifier: Apache-2.0

// "Delete..." in the menus (01 6.18 items 1, 2 and 6; D-DEL-4, D-DEL-14): a space for its owner,
// a channel or forum for `manage` ("Delete project..." with a project binding), a chat for its
// creator or `manage` with `write`, never a main thread and never in an archived or read only
// channel; the entry stands last after a divider; the "More" button of a history has it too; on
// the phone a long press opens the same menu, also on a row that a long press drags; after the
// delete the open view of the item is left and the undo snack bar shows for the shorter of the
// window and ui.delete_undo_toast.

import 'package:client/delete/delete_port.dart' show DeletionView;
import 'package:client/delete/providers.dart';
import 'package:client/menus/channel_menu.dart';
import 'package:client/menus/space_menu.dart';
import 'package:client/menus/thread_menu.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/sidebar/sidebar_arrange.dart';
import 'package:client/sidebar/sidebar_drag.dart';
import 'package:client/thread/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/delete_fakes.dart';
import '../support/structure.dart';

const String rowText = 'the row';

Finder byKey(String key) => find.byKey(ValueKey(key));

Future<void> openMenu(WidgetTester tester) async {
  await tester.tap(find.text(rowText), buttons: kSecondaryMouseButton);
  await tester.pumpAndSettle();
}

void main() {
  late ColumnHost host;

  /// Pumps [menu] over a database with [channels], their [access] and [threads], signed in as
  /// [me]; [owner] answers `getMe`.
  Future<ScriptedDeletePort> pumpMenu(
    WidgetTester tester,
    Widget Function() menu, {
    List<Channel> channels = const [],
    AccessView? access,
    List<Thread> threads = const [],
    PrincipalId? me,
    bool owner = false,
    String platform = 'macos',
    String location = '/',
    List<Override> overrides = const [],
  }) async {
    final port = ScriptedDeletePort();
    host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      final db = host.db;
      await db.structureDao.upsertSpaceIfNewer(space(1, name: 'Team'));
      for (final c in channels) {
        await db.structureDao.upsertChannelIfNewer(c);
        if (access != null) {
          await db.structureDao.putChannelAccess(c.id, access);
        }
      }
      for (final t in threads) {
        await db.threadsDao.upsertIfNewer(t);
      }
    });
    await host.pump(
      tester,
      Builder(builder: (_) => menu()),
      platform: platform,
      location: location,
      overrides: [
        deletePortProvider.overrideWithValue(port),
        isOwnerProvider.overrideWith((ref) async => owner),
        myPrincipalIdProvider.overrideWithValue(me),
        ...overrides,
      ],
    );
    return port;
  }

  const write = AccessView(level: AccessLevel.write, capabilities: []);
  const manage = AccessView(
    level: AccessLevel.write,
    capabilities: [Capability.manage],
  );
  final general = channel(1, name: 'general');
  final forum = channel(2, kind: ChannelKind.forum, name: 'ideas');
  final project = forum.copyWith(name: 'website', projectBound: true);
  final post = thread(1, channelN: 2, title: 'Release notes');

  Widget threadMenu(
    Channel c, {
    ThreadId? id,
    String title = 'Release notes',
  }) => ThreadMenu(
    thread: id ?? threadId(1),
    title: title,
    channel: c,
    child: const Text(rowText),
  );

  group('space', () {
    testWidgets('an owner sees Delete space... last after a divider', (
      tester,
    ) async {
      await pumpMenu(
        tester,
        () => SpaceMenu(
          space: space(1, name: 'Team'),
          child: const Text(rowText),
        ),
        owner: true,
      );
      await openMenu(tester);
      expect(find.text('Delete space...'), findsOneWidget);
      final column = tester.widget<Column>(
        find
            .ancestor(of: byKey('menu.delete'), matching: find.byType(Column))
            .first,
      );
      expect(column.children.last.key, const ValueKey('menu.delete'));
      expect(column.children[column.children.length - 2], isA<Divider>());
      await host.dispose(tester);
    });

    testWidgets('a person who is not an owner sees none', (tester) async {
      await pumpMenu(
        tester,
        () => SpaceMenu(
          space: space(1, name: 'Team'),
          child: const Text(rowText),
        ),
      );
      await openMenu(tester);
      expect(byKey('menu.notifications'), findsOneWidget);
      expect(byKey('menu.delete'), findsNothing);
      await host.dispose(tester);
    });
  });

  group('channel', () {
    testWidgets('manage sees Delete channel...', (tester) async {
      await pumpMenu(
        tester,
        () => ChannelMenu(channel: general, child: const Text(rowText)),
        channels: [general],
        access: manage,
      );
      await openMenu(tester);
      expect(find.text('Delete channel...'), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('a member without manage sees none', (tester) async {
      await pumpMenu(
        tester,
        () => ChannelMenu(channel: general, child: const Text(rowText)),
        channels: [general],
        access: write,
      );
      await openMenu(tester);
      expect(byKey('menu.notifications'), findsOneWidget);
      expect(byKey('menu.delete'), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('a forum with a project binding says Delete project...', (
      tester,
    ) async {
      await pumpMenu(
        tester,
        () => ChannelMenu(channel: project, child: const Text(rowText)),
        channels: [project],
        access: manage,
      );
      await openMenu(tester);
      expect(find.text('Delete project...'), findsOneWidget);
      expect(find.text('Delete channel...'), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('the More button of a main history has it', (tester) async {
      await pumpMenu(
        tester,
        () => TargetMenuButton(channel: general),
        channels: [general],
        access: manage,
      );
      await tester.tap(byKey('thread.menu'));
      await tester.pumpAndSettle();
      expect(find.text('Delete channel...'), findsOneWidget);
      await host.dispose(tester);
    });
  });

  group('chat', () {
    testWidgets('its creator with write sees Delete chat...', (tester) async {
      await pumpMenu(
        tester,
        () => threadMenu(forum),
        channels: [forum],
        access: write,
        threads: [post],
        me: sam,
      );
      await openMenu(tester);
      expect(find.text('Delete chat...'), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('manage sees it on a chat of someone else', (tester) async {
      await pumpMenu(
        tester,
        () => threadMenu(forum),
        channels: [forum],
        access: manage,
        threads: [post],
        me: alex,
      );
      await openMenu(tester);
      expect(byKey('menu.delete'), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('someone else without manage sees none', (tester) async {
      await pumpMenu(
        tester,
        () => threadMenu(forum),
        channels: [forum],
        access: write,
        threads: [post],
        me: alex,
      );
      await openMenu(tester);
      expect(byKey('menu.notifications'), findsOneWidget);
      expect(byKey('menu.delete'), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('a creator who may only read sees none', (tester) async {
      await pumpMenu(
        tester,
        () => threadMenu(forum),
        channels: [forum],
        access: const AccessView(level: AccessLevel.read, capabilities: []),
        threads: [post],
        me: sam,
      );
      await openMenu(tester);
      expect(byKey('menu.delete'), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('a main thread has none', (tester) async {
      final main = thread(
        0x81,
        title: 'general',
      ).copyWith(kind: ThreadKind.main);
      await pumpMenu(
        tester,
        () => threadMenu(general, id: threadId(0x81), title: 'general'),
        channels: [general],
        access: manage,
        threads: [main],
        me: sam,
      );
      await openMenu(tester);
      expect(byKey('menu.notifications'), findsOneWidget);
      expect(byKey('menu.delete'), findsNothing);
      await host.dispose(tester);
    });

    for (final (what, closed) in [
      ('a read only channel', forum.copyWith(readOnly: true)),
      ('an archived channel', forum.copyWith(archivedAt: t0)),
    ]) {
      testWidgets('$what has none', (tester) async {
        await pumpMenu(
          tester,
          () => threadMenu(closed),
          channels: [closed],
          access: manage,
          threads: [post],
          me: sam,
        );
        await openMenu(tester);
        expect(byKey('menu.notifications'), findsOneWidget);
        expect(byKey('menu.delete'), findsNothing);
        await host.dispose(tester);
      });
    }

    testWidgets('the More button of a chat history has it', (tester) async {
      await pumpMenu(
        tester,
        () => TargetMenuButton(channel: forum, thread: threadId(1)),
        channels: [forum],
        access: write,
        threads: [post],
        me: sam,
      );
      await tester.tap(byKey('thread.menu'));
      await tester.pumpAndSettle();
      expect(find.text('Delete chat...'), findsOneWidget);
      await host.dispose(tester);
    });
  });

  group('phone', () {
    testWidgets('a long press opens the menu with Delete', (tester) async {
      var taps = 0;
      await pumpMenu(
        tester,
        () => ThreadMenu(
          thread: threadId(1),
          title: 'Release notes',
          channel: forum,
          child: GestureDetector(
            onTap: () => taps++,
            child: const Text(rowText),
          ),
        ),
        channels: [forum],
        access: write,
        threads: [post],
        me: sam,
        platform: 'ios',
      );
      // A short tap opens the row, not the menu.
      await tester.tap(find.text(rowText));
      await tester.pumpAndSettle();
      expect(taps, 1);
      expect(byKey('menu.delete'), findsNothing);
      await tester.longPress(find.text(rowText));
      await tester.pumpAndSettle();
      expect(find.text('Delete chat...'), findsOneWidget);
      expect(taps, 1, reason: 'the long press does not open the row');
      await host.dispose(tester);
    });

    testWidgets('a row that a long press lifts opens the menu when it is not '
        'moved, and a moved one stays a drag', (tester) async {
      final drops = <String>[];
      await pumpMenu(
        tester,
        () => SidebarDragRow(
          data: const SidebarDragData(
            level: SidebarLevel.channels,
            parent: 'space',
            id: 'general',
            index: 0,
          ),
          label: 'general',
          enabled: true,
          touch: true,
          onDrop: (id, to) => drops.add(id),
          child: ChannelMenu(channel: general, child: const Text(rowText)),
        ),
        channels: [general],
        access: manage,
        platform: 'android',
      );
      final gesture = await tester.startGesture(
        tester.getCenter(find.text(rowText)),
      );
      await tester.pump(kLongPressTimeout + const Duration(milliseconds: 100));
      await gesture.moveBy(const Offset(0, 60));
      await tester.pump();
      await gesture.up();
      await tester.pumpAndSettle();
      expect(byKey('menu.delete'), findsNothing, reason: 'a drag');
      await tester.longPress(find.text(rowText));
      await tester.pumpAndSettle();
      expect(find.text('Delete channel...'), findsOneWidget);
      await host.dispose(tester);
    });
  });

  group('after the delete', () {
    testWidgets('the open chat is left and the snack bar keeps clear of the '
        'window', (tester) async {
      final port = await pumpMenu(
        tester,
        () => threadMenu(forum),
        channels: [forum],
        access: write,
        threads: [post],
        me: sam,
        location: '/t/${threadId(1).value}',
      );
      port.deleted = DeletionView(
        undoUntil: t0.add(const Duration(seconds: 5)),
      );
      await openMenu(tester);
      await tester.tap(byKey('menu.delete'));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('delete.dialog')), findsOneWidget);
      await tester.tap(byKey('delete.confirm'));
      await tester.pumpAndSettle();
      expect(port.calls.last, startsWith('delete ${threadId(1).value}'));
      expect(host.visited.last, '/');
      final bar = tester.widget<SnackBar>(byKey('delete.toast'));
      // undo_until minus 2 seconds, shorter than ui.delete_undo_toast.
      expect(bar.duration, const Duration(seconds: 3));
      expect(find.text('Deleted Release notes'), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('a long window shows the snack bar for ui.delete_undo_toast', (
      tester,
    ) async {
      final port = await pumpMenu(
        tester,
        () => ChannelMenu(channel: general, child: const Text(rowText)),
        channels: [general],
        access: manage,
        location: '/c/${channelId(3).value}',
      );
      port.deleted = DeletionView(
        undoUntil: t0.add(const Duration(minutes: 1)),
      );
      await openMenu(tester);
      await tester.tap(byKey('menu.delete'));
      await tester.pumpAndSettle();
      await tester.tap(byKey('delete.confirm'));
      await tester.pumpAndSettle();
      expect(port.calls.last, contains('purpose=Delete the channel general'));
      expect(host.visited.last, '/c/${channelId(3).value}', reason: 'stays');
      expect(
        tester.widget<SnackBar>(byKey('delete.toast')).duration,
        const Duration(seconds: 10),
      );
      await host.dispose(tester);
    });
  });
}
