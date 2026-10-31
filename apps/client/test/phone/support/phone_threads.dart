// SPDX-License-Identifier: Apache-2.0

// The phone shell over the local database for the thread group and new chat tests of M1 (brief
// M5-15 Tests): the column host's in-memory database and idle engine, a fake clock, the phone's
// density `touch` and a router that shows every path in one phone shell, as the app does.
// White label: Alex, Sam, Robin and `example.com`.

import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/phone/phone_shell.dart';
import 'package:client/sync/thread_ops.dart' show CreatedPosts;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';

import '../../support/column_host.dart';
import '../../support/fakes.dart';
import '../../support/new_chat_fixtures.dart';
import '../../support/structure.dart';
import 'phone_app.dart';

/// Post [n] of the forum `server` (channel 1), created [created] minutes into the day before
/// the fake clock's [t0] and last active [ageHours] before it.
Thread serverPost(
  int n, {
  required int created,
  int ageHours = 500,
  String? title,
  ThreadState state = ThreadState.open,
  int version = 1,
}) => Thread(
  id: threadId(n),
  channelId: server.id,
  kind: ThreadKind.thread,
  title: title ?? 'Post $n',
  titleSource: TitleSource.manual,
  state: state,
  lastMessageSeq: const ThreadSeq(4),
  unreadFloorSeq: const ThreadSeq(0),
  lastActivityAt: t0.subtract(Duration(hours: ageHours)),
  messageCount: 4,
  createdBy: sam,
  createdAt: t0.subtract(Duration(days: 30, minutes: -created)),
  version: ResourceVersion(version),
  tagIds: const [],
);

/// The forums of [fillForums] and two open posts of `server`: post 1 is old and unread (3
/// unread), post 2 is recent; a forum lists both (01 6.8 item 2, M4-46).
Future<void> fillThreads(LocalDatabase db) async {
  await fillForums(db);
  await db.threadsDao.upsertIfNewer(serverPost(1, created: 1));
  await db.threadsDao.upsertIfNewer(serverPost(2, created: 2, ageHours: 1));
  await db.personalDao.applyReadState(readState(1, lastRead: 1, unread: 3));
}

/// Post 1 marked done: it no longer qualifies (01 6.8 item 2).
Thread post1Done() =>
    serverPost(1, created: 1, state: ThreadState.closed, version: 2);

/// The thread row of post [n].
Finder postRow(int n) =>
    find.byKey(ValueKey('sidebar.thread.${threadId(n).value}'));

/// "All posts" of [forum].
Finder allPostsRow(Channel forum) =>
    find.byKey(ValueKey('sidebar.allPosts.${forum.id.value}'));

/// The "+" action of [forum] by its label (components.md 7).
Finder addAction(Channel forum, AppLocalizations l) => find.byTooltip(
  forum.projectBound
      ? l.sidebarNewChatIn(forum.name)
      : l.sidebarNewPostIn(forum.name),
);

/// Pumps the phone shell at [location] on [platform] (`ios` or `android`) over [host]; every
/// path the router goes to is recorded in [ColumnHost.visited]. The shell stays while the path
/// changes (one page key, as the app's phone routes), so a tab keeps its stack. The target
/// platform of the routes is the test's ([TargetPlatformVariant] or the default); [created]
/// records the threads of sent posts; [theme] is the theme of the app (light by default).
Future<void> pumpPhone(
  WidgetTester tester,
  ColumnHost host, {
  String location = '/',
  String platform = 'ios',
  Locale locale = const Locale('en'),
  List<Override> overrides = const [],
  CreatedPosts? created,
  DsThemeMode theme = DsThemeMode.light,
}) async {
  phoneWindow(tester);
  final router = GoRouter(
    initialLocation: location,
    routes: [
      GoRoute(
        path: '/:rest(.*)',
        pageBuilder: (context, state) {
          host.visited.add(state.uri.toString());
          return NoTransitionPage<void>(
            key: const ValueKey('phone.shell.page'),
            child: PhoneShell(location: state.uri),
          );
        },
      ),
    ],
  );
  final ds = DsContext(
    theme: theme,
    density: DsDensityMode.touch,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        appPortsProvider.overrideWithValue(
          testPorts(
            dataRoot: host.root,
            clock: TestClock(t0),
            platform: platform,
          ),
        ),
        localDatabaseProvider.overrideWithValue(host.db),
        syncEngineProvider.overrideWithValue(host.engine),
        ...shellOverrides(created: created, more: overrides),
      ],
      child: MaterialApp.router(
        locale: locale,
        theme: buildDsTheme(ds),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        routerConfig: router,
        builder: (context, child) =>
            DsThemeScope(dsContext: ds, child: child ?? const SizedBox()),
      ),
    ),
  );
  await settlePhone(tester, host);
}

/// The phone shell's state.
PhoneShellState phoneShell(WidgetTester tester) =>
    tester.state<PhoneShellState>(find.byType(PhoneShell));

/// A widget test of a phone build of [platform] (`ios` or `android`) with the routes of that
/// platform (Cupertino pages with the edge swipe, or Material pages and system back); the
/// override is reset inside the test, which the binding checks before the tear-down.
void testPhone(
  String name,
  String platform,
  Future<void> Function(WidgetTester tester) body,
) => testWidgets('$name on $platform', (tester) async {
  debugDefaultTargetPlatformOverride = platform == 'android'
      ? TargetPlatform.android
      : TargetPlatform.iOS;
  try {
    await body(tester);
  } finally {
    debugDefaultTargetPlatformOverride = null;
  }
});

/// Lets the database streams deliver, then the page transitions of the shell end (the
/// Cupertino route takes its time, and a page in transition takes no tap), then settles again.
Future<void> settlePhone(WidgetTester tester, ColumnHost host) async {
  await host.settle(tester);
  await tester.pump(const Duration(seconds: 1));
  await host.settle(tester);
}
