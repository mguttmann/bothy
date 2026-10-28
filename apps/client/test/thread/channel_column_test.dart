// SPDX-License-Identifier: Apache-2.0

// The channel column next to an open thread (brief M4-42; 01 5.1 D1, D2, 7): "Hide channel" in
// the thread header and Cmd+Shift+H (Ctrl+Shift+H) hide the main history of a text channel or
// the pinned overview of a forum, so the thread takes the whole content width, and show it
// again; the choice is kept on this device and stays when another thread opens from the
// sidebar. The border between the main history and the thread can be dragged; its width is
// stored as `main_history` in `layout.column_widths`.

import 'package:client/app/layout/column_widths.dart';
import 'package:client/app/layout/desktop_shell.dart';
import 'package:client/app/providers/target.dart';
import 'package:client/app/target_resolver.dart';
import 'package:client/keyboard/bindings.dart';
import 'package:client/keyboard/intents.dart';
import 'package:design_system/design_system.dart';
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
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

/// The text channel `builds` (channel 1) with its main history.
final Channel builds = channel(1, name: 'builds');

/// The main history of [builds].
final Thread mainHistory = Thread(
  id: builds.mainThreadId!,
  channelId: builds.id,
  kind: ThreadKind.main,
  title: '',
  titleSource: TitleSource.auto,
  state: ThreadState.open,
  lastMessageSeq: const ThreadSeq(1),
  unreadFloorSeq: const ThreadSeq(0),
  lastActivityAt: t0,
  messageCount: 1,
  createdBy: sam,
  createdAt: t0,
  version: const ResourceVersion(1),
  tagIds: const [],
);

/// The forum `ideas` (channel 2) and its post 3.
final Channel ideas = channel(2, kind: ChannelKind.forum, name: 'ideas');

Future<void> fill(LocalDatabase db) async {
  await seedThread(
    db,
    ch: builds,
    th: thread(1, title: 'Watch the nightly build', lastSeq: 1),
    messages: [msg(1)],
  );
  await db.threadsDao.upsertIfNewer(mainHistory);
  await db.messagesDao.upsertIfNewer(msg(1, threadN: 0x81));
  await db.messagesDao.addLoadedRange(
    mainHistory.id,
    const ThreadSeq(1),
    const ThreadSeq(1),
  );
  await db.threadsDao.upsertIfNewer(thread(2, title: 'Flaky test'));
  await db.structureDao.upsertChannelIfNewer(ideas);
  await db.structureDao.putChannelAccess(
    ideas.id,
    const AccessView(level: AccessLevel.write, capabilities: []),
  );
  await db.threadsDao.upsertIfNewer(thread(3, channelN: 2, title: 'Dark mode'));
}

List<Override> overrides(LocalDatabase db) => [
  targetStatusProvider.overrideWith((ref, path) async => TargetStatus.visible),
  ...threadOverrides(FakeThreadReads(db)),
];

/// The shell of the router's location.
final Widget shell = Builder(
  builder: (context) => DesktopShell(location: GoRouterState.of(context).uri),
);

/// The binding of "Show and hide the channel next to the thread" on the platform of the test
/// (01 7: Cmd+Shift+H on macOS, Ctrl+Shift+H on Windows).
final KeyStroke toggle = shortcutRows
    .expand((r) => r.bindings)
    .firstWhere((b) => b.action == ShortcutAction.toggleChannelColumn)
    .on(ShortcutPlatform.current)
    .single
    .single;

Finder history(ThreadId thread) =>
    find.byKey(ValueKey('history.${thread.value}'));

Finder column(String name) => find.byKey(ValueKey('column.$name'));

final Finder button = find.byKey(const ValueKey('thread.channelColumn'));

String path(int n) => '/t/${threadId(n).value}';

void main() {
  testWidgets('Hide channel gives the thread the whole width, Show brings the '
      'main history back; the choice survives a restart', (tester) async {
    final db = await tester.runAsync(
      () async => openInMemoryForTests(clock: () => t0),
    );
    var host = await ColumnHost.open(tester, database: db);
    await tester.runAsync(() => fill(db!));
    await host.pump(
      tester,
      shell,
      location: path(1),
      overrides: overrides(db!),
    );
    expect(history(mainHistory.id), findsOneWidget);
    expect(history(threadId(1)), findsOneWidget);
    final beside = tester.getSize(history(threadId(1))).width;
    expect(find.text(en.threadHideChannel), findsOneWidget);

    await tester.tap(button);
    await host.settle(tester);
    expect(history(mainHistory.id), findsNothing);
    expect(history(threadId(1)), findsOneWidget);
    expect(tester.getSize(history(threadId(1))).width, greaterThan(beside));
    expect(find.text(en.threadShowChannel), findsOneWidget);
    expect(await tester.runAsync(db.forumViewDao.channelColumnHidden), isTrue);
    await host.dispose(tester);
    // The server confirmed the writes of the first start (the test ports count IDs anew).
    await tester.runAsync(() async {
      for (final e in await db.outboxDao.allEntries()) {
        await db.outboxDao.delete(e.id);
      }
    });

    // A new start of the app on this device: the same database, new providers.
    host = await ColumnHost.open(tester, database: db);
    await host.pump(tester, shell, location: path(1), overrides: overrides(db));
    expect(history(mainHistory.id), findsNothing, reason: 'kept per device');
    await tester.tap(button);
    await host.settle(tester);
    expect(history(mainHistory.id), findsOneWidget);
    expect(tester.getSize(history(threadId(1))).width, beside);
    expect(await tester.runAsync(db.forumViewDao.channelColumnHidden), isFalse);
    await host.dispose(tester);
    await tester.runAsync(db.close);
  });

  testWidgets('Cmd+Shift+H hides and shows the channel next to the thread', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(
      tester,
      shell,
      location: path(1),
      overrides: overrides(host.db),
    );
    expect(history(mainHistory.id), findsOneWidget);
    // A key reaches the app while the focus is in it; Tab puts it there.
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    await pressStroke(tester, toggle);
    await host.settle(tester);
    expect(history(mainHistory.id), findsNothing);
    expect(history(threadId(1)), findsOneWidget);
    await pressStroke(tester, toggle);
    await host.settle(tester);
    expect(history(mainHistory.id), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('a thread opened from the sidebar keeps the channel hidden', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await fill(host.db);
      await host.db.forumViewDao.setChannelColumnHidden(hidden: true);
    });
    await host.pump(
      tester,
      shell,
      location: path(1),
      overrides: overrides(host.db),
    );
    expect(history(mainHistory.id), findsNothing);
    await tester.tap(
      find.byKey(ValueKey('sidebar.thread.${threadId(2).value}')),
    );
    await host.settle(tester);
    expect(host.visited.last, path(2));
    expect(history(threadId(2)), findsOneWidget);
    expect(history(mainHistory.id), findsNothing);
    expect(find.text(en.threadShowChannel), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('the border between main history and thread can be dragged; the '
      'width is stored as main_history and the thread keeps its minimum', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(
      tester,
      shell,
      location: path(1),
      overrides: overrides(host.db),
    );
    final handle = find.byKey(const ValueKey('column.handle.main_history'));
    expect(handle, findsOneWidget);
    final before = tester.getSize(history(mainHistory.id)).width;
    await tester.drag(handle, const Offset(-120, 0));
    await host.settle(tester);
    final after = tester.getSize(history(mainHistory.id)).width;
    expect(after, closeTo(before - 120, 1));
    final stored = await tester.runAsync(
      () => host.db.personalDao.watchSettings().first,
    );
    final value = stored!
        .firstWhere((s) => s.key == ColumnWidths.settingKey)
        .value;
    expect(value, {'main_history': after.round()});

    // Far to the right: the thread keeps `DsLayout.columnMin`.
    await tester.drag(handle, const Offset(5000, 0));
    await host.settle(tester);
    expect(
      tester.getSize(history(threadId(1))).width,
      greaterThanOrEqualTo(DsLayout.columnMin - 1),
    );
    // Far to the left: the main history keeps it too.
    await tester.drag(handle, const Offset(-5000, 0));
    await host.settle(tester);
    expect(
      tester.getSize(history(mainHistory.id)).width,
      greaterThanOrEqualTo(DsLayout.columnMin - 1),
    );
    await host.dispose(tester);
  });

  testWidgets('in a forum Hide channel hides the pinned overview; without the '
      'pin and on the main history there is nothing to hide', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await fill(host.db);
      await host.db.forumViewDao.setOverviewPinned(pinned: true);
    });
    await host.pump(
      tester,
      shell,
      location: path(3),
      overrides: overrides(host.db),
    );
    expect(column('post_list'), findsOneWidget);
    final beside = tester.getSize(column('history')).width;
    await tester.tap(button);
    await host.settle(tester);
    expect(column('post_list'), findsNothing);
    expect(history(threadId(3)), findsOneWidget);
    expect(tester.getSize(column('history')).width, greaterThan(beside));
    expect(
      await tester.runAsync(host.db.forumViewDao.overviewPinned),
      isTrue,
      reason: 'the pin stays',
    );
    await tester.tap(button);
    await host.settle(tester);
    expect(column('post_list'), findsOneWidget);

    await host.run(
      tester,
      () => host.db.forumViewDao.setOverviewPinned(pinned: false),
    );
    await host.settle(tester);
    expect(button, findsNothing, reason: 'no overview next to the post');

    // The main history of a text channel alone has no channel column to hide.
    host.visited.clear();
    await tester.tap(
      find.byKey(ValueKey('sidebar.channel.${builds.id.value}')),
    );
    await host.settle(tester);
    expect(history(mainHistory.id), findsOneWidget);
    expect(button, findsNothing);
    await host.dispose(tester);
  });
}
