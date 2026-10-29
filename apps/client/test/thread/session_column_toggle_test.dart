// SPDX-License-Identifier: Apache-2.0

// The session column toggle on the desktop (brief M4-45; 01 5.1 D1, D2, 7): "Hide session" in
// the thread header and Cmd+Shift+I (Ctrl+Shift+I) hide the right session column, so the thread
// takes its width, and "Show session", the same keys or the slim rail at the right edge bring it
// back. The choice is kept on this device per layout (beside a forum post shown by default,
// beside a thread of a text channel collapsed by default) and stays when another thread opens.
// The German interface names the controls of M4-42 and M4-45 and the column borders.

import 'package:client/app/layout/desktop_shell.dart';
import 'package:client/app/providers/target.dart';
import 'package:client/app/target_resolver.dart';
import 'package:client/keyboard/bindings.dart';
import 'package:client/keyboard/intents.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
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

/// The forum `ideas` (channel 2) with its posts 3 and 4.
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
  await db.structureDao.upsertChannelIfNewer(ideas);
  await db.structureDao.putChannelAccess(
    ideas.id,
    const AccessView(level: AccessLevel.write, capabilities: []),
  );
  await db.threadsDao.upsertIfNewer(thread(3, channelN: 2, title: 'Dark mode'));
  await db.threadsDao.upsertIfNewer(thread(4, channelN: 2, title: 'Shortcuts'));
}

List<Override> overrides(LocalDatabase db) => [
  targetStatusProvider.overrideWith((ref, path) async => TargetStatus.visible),
  ...threadOverrides(FakeThreadReads(db)),
];

/// The shell of the router's location.
final Widget shell = Builder(
  builder: (context) => DesktopShell(location: GoRouterState.of(context).uri),
);

/// The binding of "Show and hide the session column" on the platform of the test (01 7:
/// Cmd+Shift+I on macOS, Ctrl+Shift+I on Windows).
final KeyStroke toggle = shortcutRows
    .expand((r) => r.bindings)
    .firstWhere((b) => b.action == ShortcutAction.toggleSessionColumn)
    .on(ShortcutPlatform.current)
    .single
    .single;

Finder column(String name) => find.byKey(ValueKey('column.$name'));

final Finder session = column('session');
final Finder button = find.byKey(const ValueKey('thread.column'));
final Finder rail = find.byKey(const ValueKey('column.session.show'));

String path(int n) => '/t/${threadId(n).value}';

void main() {
  testWidgets('Hide session gives the thread the width, the rail brings the '
      'column back; the choice survives a restart', (tester) async {
    final db = await tester.runAsync(
      () async => openInMemoryForTests(clock: () => t0),
    );
    var host = await ColumnHost.open(tester, database: db);
    await tester.runAsync(() => fill(db!));
    await host.pump(
      tester,
      shell,
      location: path(3),
      overrides: overrides(db!),
    );
    // 01 D1: beside a forum post the session column shows.
    expect(session, findsOneWidget);
    expect(rail, findsNothing);
    expect(find.text(en.threadHideSession), findsOneWidget);
    final beside = tester.getSize(column('history')).width;

    await tester.tap(button);
    await host.settle(tester);
    expect(session, findsNothing);
    expect(tester.getSize(column('history')).width, greaterThan(beside));
    expect(find.text(en.threadShowSession), findsOneWidget);
    expect(rail, findsOneWidget);
    expect(
      await tester.runAsync(
        () => db.forumViewDao.sessionColumnHidden(textThread: false),
      ),
      isTrue,
    );
    await host.dispose(tester);
    await tester.runAsync(() async {
      for (final e in await db.outboxDao.allEntries()) {
        await db.outboxDao.delete(e.id);
      }
    });

    // A new start of the app on this device: the same database, new providers.
    host = await ColumnHost.open(tester, database: db);
    await host.pump(tester, shell, location: path(3), overrides: overrides(db));
    expect(session, findsNothing, reason: 'kept per device');
    await tester.tap(rail);
    await host.settle(tester);
    expect(session, findsOneWidget);
    expect(rail, findsNothing);
    expect(tester.getSize(column('history')).width, beside);
    expect(
      await tester.runAsync(
        () => db.forumViewDao.sessionColumnHidden(textThread: false),
      ),
      isFalse,
    );
    await host.dispose(tester);
    await tester.runAsync(db.close);
  });

  testWidgets('Cmd+Shift+I hides and shows the session column; Pin shows it', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(
      tester,
      shell,
      location: path(3),
      overrides: overrides(host.db),
    );
    expect(session, findsOneWidget);
    // A key reaches the app while the focus is in it; Tab puts it there.
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    await pressStroke(tester, toggle);
    await host.settle(tester);
    expect(session, findsNothing);
    await pressStroke(tester, toggle);
    await host.settle(tester);
    expect(session, findsOneWidget);

    await tester.tap(button);
    await host.settle(tester);
    expect(session, findsNothing);
    await tester.tap(find.byKey(const ValueKey('thread.pin')));
    await host.settle(tester);
    expect(session, findsOneWidget, reason: 'Pin shows the pinned messages');
    await host.dispose(tester);
  });

  testWidgets('beside a thread of a text channel the column is collapsed until '
      'shown; that choice is its own and is kept', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(
      tester,
      shell,
      location: path(1),
      overrides: overrides(host.db),
    );
    // 01 D2: collapsed beside the main history; both headers offer to show it.
    expect(session, findsNothing);
    expect(rail, findsOneWidget);
    expect(find.text(en.threadShowSession), findsNWidgets(2));
    await tester.tap(button.last);
    await host.settle(tester);
    expect(session, findsOneWidget);
    expect(rail, findsNothing);
    expect(find.text(en.threadHideSession), findsNWidgets(2));
    expect(
      await tester.runAsync(
        () => host.db.forumViewDao.sessionColumnHidden(textThread: true),
      ),
      isFalse,
    );
    expect(
      await tester.runAsync(
        () => host.db.forumViewDao.sessionColumnHidden(textThread: false),
      ),
      isFalse,
      reason: 'the forum layout keeps its own value',
    );
    // The header of the main history toggles the same column.
    await tester.tap(button.first);
    await host.settle(tester);
    expect(session, findsNothing);
    await host.dispose(tester);
  });

  testWidgets('a main history opened as a thread route stands alone: the '
      'session column shows and the toggle keeps the text-thread choice', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    // A mention, a search hit or a push opens the main history as `/t/<mainThreadId>`.
    await host.pump(
      tester,
      shell,
      location: '/t/${mainHistory.id.value}',
      overrides: overrides(host.db),
    );
    // 01 D2, D-M4-45-2: a main history alone shows the session column.
    expect(session, findsOneWidget);
    expect(rail, findsNothing);
    expect(find.text(en.threadHideSession), findsOneWidget);
    expect(find.byKey(const ValueKey('thread.channelColumn')), findsNothing);
    await tester.tap(button);
    await host.settle(tester);
    expect(session, findsNothing);
    expect(find.text(en.threadShowSession), findsOneWidget);
    expect(
      await tester.runAsync(
        () => host.db.forumViewDao.sessionColumnHidden(textThread: false),
      ),
      isTrue,
    );
    expect(
      await tester.runAsync(
        () => host.db.forumViewDao.sessionColumnHidden(textThread: true),
      ),
      isTrue,
      reason: 'the text-thread layout keeps its default',
    );
    await host.dispose(tester);
  });

  testWidgets('a post opened from the sidebar keeps the session hidden', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await fill(host.db);
      await host.db.forumViewDao.setSessionColumnHidden(
        textThread: false,
        hidden: true,
      );
    });
    await host.pump(
      tester,
      shell,
      location: path(3),
      overrides: overrides(host.db),
    );
    expect(session, findsNothing);
    await tester.tap(
      find.byKey(ValueKey('sidebar.thread.${threadId(4).value}')),
    );
    await host.settle(tester);
    expect(host.visited.last, path(4));
    expect(session, findsNothing);
    expect(find.text(en.threadShowSession), findsOneWidget);
    expect(rail, findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('a window too narrow for the session column has no rail', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await fill(host.db);
      await host.db.forumViewDao.setSessionColumnHidden(
        textThread: false,
        hidden: true,
      );
    });
    await host.pump(
      tester,
      shell,
      location: path(3),
      overrides: overrides(host.db),
    );
    expect(rail, findsOneWidget);
    tester.view.physicalSize = const Size(1100, 1000);
    await host.settle(tester);
    expect(rail, findsNothing);
    expect(session, findsNothing);
    await host.dispose(tester);
  });

  testWidgets('the German interface names the column controls and borders', (
    tester,
  ) async {
    final de = lookupAppLocalizations(const Locale('de'));
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(
      tester,
      shell,
      location: path(1),
      locale: const Locale('de'),
      overrides: overrides(host.db),
    );
    expect(de.threadHideSession, 'Sitzung ausblenden');
    expect(de.threadShowSession, 'Sitzung einblenden');
    expect(de.threadHideChannel, 'Kanal ausblenden');
    expect(de.shellResizeColumn, 'Spaltenbreite ändern');
    expect(find.text(de.threadShowSession), findsNWidgets(2));
    expect(find.text(de.threadHideChannel), findsOneWidget);
    expect(find.bySemanticsLabel(de.threadShowSession), findsWidgets);
    // Every border of this layout is a drag handle with the German label: sidebar and main
    // history beside the thread.
    expect(
      find.byKey(const ValueKey('column.handle.main_history')),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel(de.shellResizeColumn),
      findsAtLeastNWidgets(2),
    );

    await tester.tap(find.byKey(const ValueKey('thread.channelColumn')));
    await host.settle(tester);
    expect(find.text(de.threadShowChannel), findsOneWidget);
    expect(de.threadShowChannel, 'Kanal einblenden');
    await host.dispose(tester);
  });
}
