// SPDX-License-Identifier: Apache-2.0

// The forum overview D15 in the desktop shell (brief M4-40 Tests; 01 D1, D15, R-PR-17; Design
// note 4): a click on the forum name opens it in the content column; "Pin" and Cmd+Shift+O move
// it to its own column and back; the pinned state survives a restart on this device; below
// `ui.desktop_min_width_three_columns` a pinned overview returns to the content column.

import 'package:client/app/layout/desktop_shell.dart';
import 'package:client/app/providers/target.dart';
import 'package:client/app/target_resolver.dart';
import 'package:client/keyboard/bindings.dart';
import 'package:client/keyboard/intents.dart';
import 'package:client/sidebar/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/keys.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

final List<Override> overrides = [
  targetStatusProvider.overrideWith((ref, path) async => TargetStatus.visible),
  ownPrincipalProvider.overrideWith(
    (ref) async => Principal(
      id: alex,
      kind: PrincipalKind.user,
      handle: 'alex',
      displayName: 'Alex',
    ),
  ),
];

Future<void> fill(LocalDatabase db) async {
  await db.structureDao.upsertSpaceIfNewer(space(1));
  await db.structureDao.upsertChannelIfNewer(
    channel(1, kind: ChannelKind.forum, name: 'server'),
  );
  await db.threadsDao.upsertIfNewer(thread(1, title: 'Gateway resume'));
}

/// The shell of the router's location.
final Widget shell = Builder(
  builder: (context) => DesktopShell(location: GoRouterState.of(context).uri),
);

/// The binding of "Pin or unpin the forum overview as a column" on the platform of the test
/// (01 7: Cmd+Shift+O on macOS, Ctrl+Shift+O on Windows).
final KeyStroke pin = shortcutRows
    .expand((r) => r.bindings)
    .firstWhere((b) => b.action == ShortcutAction.pinOverview)
    .on(ShortcutPlatform.current)
    .single
    .single;

Finder column(String name) => find.byKey(ValueKey('column.$name'));

/// Whether the overview is in its own column (true) or in the content column (false).
bool pinnedColumn(WidgetTester tester) {
  expect(find.byKey(const ValueKey('postList.newPost')), findsOneWidget);
  return find
          .descendant(
            of: column('post_list'),
            matching: find.byKey(const ValueKey('postList.newPost')),
          )
          .evaluate()
          .isNotEmpty &&
      column('history').evaluate().isNotEmpty;
}

void main() {
  testWidgets(
    'a click on the forum name opens the overview in the content column',
    (tester) async {
      final host = await ColumnHost.open(tester);
      await tester.runAsync(() => fill(host.db));
      await host.pump(tester, shell, overrides: overrides);
      expect(find.byKey(const ValueKey('postList.newPost')), findsNothing);
      await tester.tap(
        find.byKey(ValueKey('sidebar.channel.${channelId(1).value}')),
      );
      await host.settle(tester);
      expect(host.visited.last, '/c/${channelId(1).value}');
      // Three columns: sidebar, the overview in the content column, no history beside it.
      expect(pinnedColumn(tester), isFalse);
      expect(column('history'), findsNothing);
      expect(find.text(en.channelKindSentenceForum), findsWidgets);
      await host.dispose(tester);
    },
  );

  testWidgets('Pin and Cmd+Shift+O move it to its own column and back; '
      'the choice survives a restart', (tester) async {
    final db = await tester.runAsync(
      () async => openInMemoryForTests(clock: () => t0),
    );
    var host = await ColumnHost.open(tester, database: db);
    await tester.runAsync(() => fill(db!));
    final forum = '/c/${channelId(1).value}';
    await host.pump(tester, shell, location: forum, overrides: overrides);
    expect(pinnedColumn(tester), isFalse);

    await tester.tap(find.byKey(const ValueKey('postList.pin')));
    await host.settle(tester);
    expect(pinnedColumn(tester), isTrue);
    expect(find.text(en.overviewUnpin), findsOneWidget);
    expect(await tester.runAsync(db!.forumViewDao.overviewPinned), isTrue);

    // The keys (01 7) unpin it again. A key reaches the app while the
    // focus is in it; Tab puts it there.
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    await pressStroke(tester, pin);
    await host.settle(tester);
    expect(pinnedColumn(tester), isFalse);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    await pressStroke(tester, pin);
    await host.settle(tester);
    expect(pinnedColumn(tester), isTrue);
    await host.dispose(tester);

    // A new start of the app on this device: the same database, new providers.
    host = await ColumnHost.open(tester, database: db);
    await host.pump(
      tester,
      shell,
      location: '/t/${threadId(1).value}',
      overrides: overrides,
    );
    expect(pinnedColumn(tester), isTrue, reason: 'restored from this device');
    await host.dispose(tester);
    await tester.runAsync(db.close);
  });

  testWidgets('a post opens beside the pinned overview, in its place unpinned', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(
      tester,
      shell,
      location: '/t/${threadId(1).value}',
      overrides: overrides,
    );
    // Unpinned: the history of the post fills the content column, no overview.
    expect(find.byKey(const ValueKey('postList.newPost')), findsNothing);
    expect(column('history'), findsOneWidget);
    await host.run(
      tester,
      () => host.db.forumViewDao.setOverviewPinned(pinned: true),
    );
    await host.settle(tester);
    expect(pinnedColumn(tester), isTrue);
    await host.dispose(tester);
  });

  testWidgets(
    'below ui.desktop_min_width_three_columns a pinned overview returns to the content column',
    (tester) async {
      final host = await ColumnHost.open(tester);
      await tester.runAsync(() async {
        await fill(host.db);
        await host.db.forumViewDao.setOverviewPinned(pinned: true);
      });
      await host.pump(
        tester,
        shell,
        location: '/c/${channelId(1).value}',
        overrides: overrides,
      );
      expect(pinnedColumn(tester), isTrue);
      // A window narrower than `ui.desktop_min_width_three_columns`.
      final narrow =
          AppConfig.defaults().desktopMinWidthThreeColumns.toDouble() - 40;
      tester.view
        ..physicalSize = Size(narrow, 800)
        ..devicePixelRatio = 1;
      await host.settle(tester);
      expect(pinnedColumn(tester), isFalse);
      expect(find.byKey(const ValueKey('postList.newPost')), findsOneWidget);
      expect(column('history'), findsNothing);
      await host.dispose(tester);
    },
  );
}
