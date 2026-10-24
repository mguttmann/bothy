// SPDX-License-Identifier: Apache-2.0

// The quick switcher D6 (brief M4-23 Design note 3, fixed requirements 9, 10, 15): recently
// visited targets first, at most `ui.quick_switch_max_results` rows, only what the local
// database holds plus the server's thread titles merged by ID, the filter row with Tab, Enter
// navigates and Esc closes; the shell records a visit for every route it shows.

import 'dart:io' as io;

import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/switcher/providers.dart';
import 'package:client/switcher/quick_switcher.dart';
import 'package:client/switcher/recent_targets.dart';
import 'package:client/switcher/switcher_sources.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/fakes.dart';
import '../support/structure.dart';
import '../support/until.dart';
import '../support/widgets.dart';

/// A DM channel [n] between [participants] (no space).
Channel dm(int n, List<PrincipalId> participants) => Channel(
  id: channelId(n),
  kind: ChannelKind.dm,
  name: '',
  topic: '',
  sortKey: 'a$n',
  operator: const ChannelOperator(mode: OperatorMode.inherit),
  readOnly: false,
  mainThreadId: threadId(0x80 + n),
  dmParticipantIds: participants,
  createdAt: t0,
  version: const ResourceVersion(1),
  settingsVersion: const ResourceVersion(1),
);

Principal person(PrincipalId id, String name, {PrincipalKind? kind}) =>
    Principal(
      id: id,
      kind: kind ?? PrincipalKind.user,
      handle: name.toLowerCase(),
      displayName: name,
    );

AgentCard agentCard(PrincipalId id, String name) => AgentCard(
  id: id,
  handle: name,
  displayName: name,
  description: '',
  billingMode: BillingMode.organization,
  toolClass: ToolClass.chat,
  presence: const AgentPresence(state: AgentRuntimeState.online),
  version: const ResourceVersion(1),
);

Command command(int n, String name, {TargetKind? scope, String? scopeId}) =>
    Command(
      id: CommandId.parse(uuid(0x900 + n)),
      ownerPrincipalId: robin,
      name: name,
      description: '',
      scopeKind: scope ?? TargetKind.instance,
      scopeId: scopeId,
      options: const [],
      requiredLevel: CommandLevel.write,
      control: false,
      version: const ResourceVersion(1),
    );

/// The server's prefix search with scripted hits; records every query.
final class ScriptedTitles implements ThreadTitleSearch {
  ScriptedTitles(this.hits);

  final List<ThreadTitleHit> hits;
  final List<String> queries = [];

  @override
  Future<List<ThreadTitleHit>> prefix(String q, {required int limit}) async {
    queries.add(q);
    return hits.take(limit).toList();
  }
}

/// The local data of the pure ranking tests: two spaces, three channels, two threads, Sam,
/// the agent Robin with a DM and two commands.
SwitcherLocalData data({ChannelId? current}) => SwitcherLocalData(
  spaces: [
    space(1, name: 'Support'),
    space(2, name: 'Engineering'),
  ],
  channels: [
    channel(1, name: 'general'),
    channel(2, name: 'server', spaceN: 2),
    channel(3, name: 'service-desk'),
    dm(9, [alex, robin]),
  ],
  threads: [
    thread(1, channelN: 2, title: 'Gateway: resume after network change'),
    thread(2, channelN: 1, title: 'Release notes', activeMinutes: 5),
  ],
  principals: [
    person(alex, 'Alex'),
    person(sam, 'Sam'),
    person(robin, 'Robin', kind: PrincipalKind.agent),
  ],
  agents: [agentCard(robin, 'dev-runner')],
  commands: [
    command(1, 'status'),
    command(
      2,
      'deploy',
      scope: TargetKind.channel,
      scopeId: channelId(2).value,
    ),
  ],
  me: alex,
  currentChannel: current,
);

RecentTarget recent(RecentTargetKind kind, String id, int minutes) =>
    RecentTarget(
      kind: kind,
      targetId: id,
      visitedAt: t0.add(Duration(minutes: minutes)),
    );

List<String> ids(List<SwitcherEntry> rows) => [
  for (final r in rows) '${r.kind.name}.${r.id}',
];

void main() {
  group('ranking', () {
    test('an empty input lists the recent targets, newest first', () {
      final rows = rankSwitcher(
        local: localEntries(data()),
        server: const [],
        recent: [
          recent(RecentTargetKind.thread, threadId(1).value, 9),
          recent(RecentTargetKind.channel, channelId(1).value, 3),
          recent(RecentTargetKind.space, spaceId(2).value, 6),
        ],
        query: '',
        filter: SwitcherFilter.all,
        max: 20,
      );
      expect(ids(rows), [
        'thread.${threadId(1).value}',
        'space.${spaceId(2).value}',
        'channel.${channelId(1).value}',
      ]);
    });

    test('recent targets rank before prefix matches, then the rest', () {
      final rows = rankSwitcher(
        local: localEntries(data()),
        server: const [],
        recent: [recent(RecentTargetKind.channel, channelId(3).value, 1)],
        query: 'se',
        filter: SwitcherFilter.all,
        max: 20,
      );
      // service-desk was visited; server starts with "se"; the "Release notes" thread and
      // the space have no word starting with "se"; "desk" does not start with "se".
      expect(ids(rows), [
        'channel.${channelId(3).value}',
        'channel.${channelId(2).value}',
      ]);
    });

    test('a word anywhere in the name matches, case-insensitive', () {
      final rows = rankSwitcher(
        local: localEntries(data()),
        server: const [],
        recent: const [],
        query: 'NETW gate',
        filter: SwitcherFilter.all,
        max: 20,
      );
      expect(ids(rows), ['thread.${threadId(1).value}']);
      expect(rows.single.context, 'server');
      expect(rows.single.path, '/t/${threadId(1).value}');
    });

    test('at most max rows', () {
      final rows = rankSwitcher(
        local: localEntries(data()),
        server: const [],
        recent: const [],
        query: 's',
        filter: SwitcherFilter.all,
        max: 2,
      );
      expect(rows, hasLength(2));
    });

    test('server titles merge by thread ID; an unstored one is listed', () {
      final local = data();
      final rows = rankSwitcher(
        local: localEntries(local),
        server: serverEntries([
          ThreadTitleHit(
            threadId: threadId(1),
            channelId: channelId(2),
            title: 'Gateway: resume after network change',
            lastActivityAt: t0,
          ),
          ThreadTitleHit(
            threadId: threadId(40),
            channelId: channelId(2),
            title: 'Gateway timeouts',
            lastActivityAt: t0,
          ),
        ], local),
        recent: const [],
        query: 'gate',
        filter: SwitcherFilter.all,
        max: 20,
      );
      expect(ids(rows), [
        'thread.${threadId(1).value}',
        'thread.${threadId(40).value}',
      ]);
      expect(rows.last.path, '/t/${threadId(40).value}');
      expect(rows.last.context, 'server');
    });

    test(
      'people and agents open their stored DM, else they do not navigate',
      () {
        final rows = localEntries(data());
        final robinRow = rows.singleWhere((r) => r.kind == SwitcherKind.agent);
        final samRow = rows.singleWhere((r) => r.kind == SwitcherKind.person);
        expect(robinRow.name, 'dev-runner');
        expect(robinRow.path, '/c/${channelId(9).value}');
        expect(samRow.name, 'Sam');
        expect(samRow.path, isNull);
        // Alex is the signed-in person and no target of a jump.
        expect(rows.where((r) => r.id == alex.value), isEmpty);
      },
    );

    test('a visit of a DM channel ranks the person of that DM as recent', () {
      final rows = rankSwitcher(
        local: localEntries(data()),
        server: const [],
        recent: [recent(RecentTargetKind.channel, channelId(9).value, 1)],
        query: '',
        filter: SwitcherFilter.all,
        max: 20,
      );
      expect(ids(rows), ['agent.${robin.value}']);
    });

    test('commands of the current channel only', () {
      List<String> commands(ChannelId? current) => [
        for (final r in localEntries(data(current: current)))
          if (r.kind == SwitcherKind.command) r.name,
      ];
      expect(commands(channelId(2)), ['/deploy', '/status']);
      expect(commands(channelId(1)), ['/status']);
      expect(commands(null), ['/status']);
    });

    test('the filter keeps only its kinds; Channels includes spaces', () {
      List<SwitcherKind> kinds(SwitcherFilter f) => {
        for (final r in rankSwitcher(
          local: localEntries(data(current: channelId(2))),
          server: const [],
          recent: const [],
          query: '',
          filter: f,
          max: 50,
          showAllWhenEmpty: true,
        ))
          r.kind,
      }.toList();
      expect(kinds(SwitcherFilter.channels), [
        SwitcherKind.space,
        SwitcherKind.channel,
      ]);
      expect(kinds(SwitcherFilter.threads), [SwitcherKind.thread]);
      expect(kinds(SwitcherFilter.people), [SwitcherKind.person]);
      expect(kinds(SwitcherFilter.agents), [SwitcherKind.agent]);
      expect(kinds(SwitcherFilter.commands), [SwitcherKind.command]);
    });

    test('Tab and Shift+Tab cycle the filter row', () {
      expect(SwitcherFilter.all.next, SwitcherFilter.channels);
      expect(SwitcherFilter.commands.next, SwitcherFilter.all);
      expect(SwitcherFilter.all.previous, SwitcherFilter.commands);
    });
  });

  group('recent targets', () {
    test('a route of a space, channel or thread is a target', () {
      expect(recentTargetOf(Uri.parse('/s/${spaceId(1).value}')), (
        RecentTargetKind.space,
        spaceId(1).value,
      ));
      expect(recentTargetOf(Uri.parse('/c/${channelId(1).value}')), (
        RecentTargetKind.channel,
        channelId(1).value,
      ));
      expect(
        recentTargetOf(Uri.parse('/t/${threadId(1).value}?m=${uuid(0x500)}')),
        (RecentTargetKind.thread, threadId(1).value),
      );
      expect(recentTargetOf(Uri.parse('/search?q=x')), isNull);
      expect(recentTargetOf(Uri.parse('/')), isNull);
      expect(recentTargetOf(Uri.parse('/c/not-an-id')), isNull);
    });

    test('the shell provider records a visit with the clock and the limit '
        'of AppConfig', () async {
      final db = openInMemoryForTests(clock: () => t0);
      addTearDown(db.close);
      final clock = TestClock(t0);
      final c = ProviderContainer(
        overrides: [
          localDatabaseProvider.overrideWithValue(db),
          appPortsProvider.overrideWithValue(
            testPorts(
              dataRoot: io.Directory.systemTemp,
              clock: clock,
              config: {...fastConfig, 'ui.quick_switch_max_results': '2'},
            ),
          ),
        ],
      );
      addTearDown(c.dispose);
      for (final (i, path) in [
        '/c/${channelId(1).value}',
        '/t/${threadId(1).value}',
        '/s/${spaceId(1).value}',
        '/running',
      ].indexed) {
        clock.start = t0.add(Duration(minutes: i));
        await c.read(recordVisitProvider(path).future);
      }
      final kept = await db.recentTargetsDao.watchRecent(limit: 10).first;
      expect(
        [for (final r in kept) (r.kind, r.targetId, r.visitedAt)],
        [
          (
            RecentTargetKind.space,
            spaceId(1).value,
            t0.add(const Duration(minutes: 2)),
          ),
          (
            RecentTargetKind.thread,
            threadId(1).value,
            t0.add(const Duration(minutes: 1)),
          ),
        ],
      );
    });
  });

  group('overlay', () {
    late ColumnHost host;
    late ScriptedTitles titles;

    Future<void> open(
      WidgetTester tester, {
      Map<String, String> config = const {},
      ConnectionPhase phase = ConnectionPhase.offline,
      String location = '/',
    }) async {
      host = await ColumnHost.open(tester);
      final db = host.db;
      await host.run(tester, () async {
        await db.structureDao.upsertSpaceIfNewer(space(1, name: 'Support'));
        await db.structureDao.upsertSpaceIfNewer(space(2, name: 'Engineering'));
        await db.structureDao.upsertChannelIfNewer(channel(1, name: 'general'));
        await db.structureDao.upsertChannelIfNewer(
          channel(2, name: 'server', spaceN: 2),
        );
        await db.structureDao.upsertChannelIfNewer(
          channel(3, name: 'service-desk'),
        );
        await db.threadsDao.upsertIfNewer(
          thread(1, channelN: 2, title: 'Gateway: resume after network change'),
        );
        await db.threadsDao.upsertIfNewer(
          thread(2, channelN: 1, title: 'Release notes'),
        );
        await db.principalsDao.put(person(sam, 'Sam'));
        await db.recentTargetsDao.visit(
          RecentTargetKind.channel,
          channelId(3).value,
          at: t0,
          keep: 20,
        );
        await db.recentTargetsDao.visit(
          RecentTargetKind.thread,
          threadId(2).value,
          at: t0.add(const Duration(minutes: 1)),
          keep: 20,
        );
      });
      titles = ScriptedTitles([
        ThreadTitleHit(
          threadId: threadId(1),
          channelId: channelId(2),
          title: 'Gateway: resume after network change',
          lastActivityAt: t0,
        ),
        ThreadTitleHit(
          threadId: threadId(40),
          channelId: channelId(2),
          title: 'Gateway timeouts',
          lastActivityAt: t0,
        ),
      ]);
      desktopWindow(tester);
      final router = GoRouter(
        initialLocation: location,
        routes: [
          GoRoute(
            path: '/:rest(.*)',
            builder: (context, state) {
              host.visited.add(state.uri.toString());
              return Scaffold(
                body: Builder(
                  builder: (context) => TextButton(
                    key: const ValueKey('open'),
                    onPressed: () => showQuickSwitcher(context),
                    child: const Text('open'),
                  ),
                ),
              );
            },
          ),
        ],
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            appPortsProvider.overrideWithValue(
              testPorts(
                dataRoot: host.root,
                clock: TestClock(t0),
                config: {...fastConfig, ...config},
              ),
            ),
            localDatabaseProvider.overrideWithValue(db),
            syncEngineProvider.overrideWithValue(host.engine),
            connectionStateProvider.overrideWith(
              (ref) => Stream.value(ConnectionState(phase)),
            ),
            threadTitleSearchProvider.overrideWithValue(titles),
          ],
          child: hostRouter(router),
        ),
      );
      await host.settle(tester);
      await tester.tap(find.byKey(const ValueKey('open')));
      await pumpUntil(
        tester,
        () =>
            find.byKey(const ValueKey('switcher.dialog')).evaluate().isNotEmpty,
        what: 'the switcher',
      );
      await host.settle(tester);
    }

    Future<void> type(WidgetTester tester, String text) async {
      await tester.enterText(
        find.byKey(const ValueKey('switcher.field')),
        text,
      );
      await host.settle(tester);
    }

    List<String> rowKeys(WidgetTester tester) => [
      for (final e
          in find
              .byWidgetPredicate(
                (w) =>
                    w.key is ValueKey<String> &&
                    (w.key! as ValueKey<String>).value.startsWith(
                      'switcher.row.',
                    ),
              )
              .evaluate())
        (e.widget.key! as ValueKey<String>).value,
    ];

    testWidgets('an empty input shows the recent targets, newest first', (
      tester,
    ) async {
      await open(tester);
      expect(rowKeys(tester), [
        'switcher.row.thread.${threadId(2).value}',
        'switcher.row.channel.${channelId(3).value}',
      ]);
      await host.dispose(tester);
    });

    testWidgets('the limit comes from AppConfig', (tester) async {
      await open(tester, config: {'ui.quick_switch_max_results': '1'});
      expect(rowKeys(tester), ['switcher.row.thread.${threadId(2).value}']);
      await type(tester, 's');
      expect(rowKeys(tester), hasLength(1));
      await host.dispose(tester);
    });

    testWidgets(
      'offline only local rows; online the server titles join by ID',
      (tester) async {
        await open(tester, phase: ConnectionPhase.live);
        await type(tester, 'gate');
        await pumpUntil(
          tester,
          () => rowKeys(tester).length == 2,
          what: 'the server titles',
        );
        expect(rowKeys(tester), [
          'switcher.row.thread.${threadId(1).value}',
          'switcher.row.thread.${threadId(40).value}',
        ]);
        expect(titles.queries, contains('gate'));
        await host.dispose(tester);
      },
    );

    testWidgets('offline the server is not asked', (tester) async {
      await open(tester);
      await type(tester, 'gate');
      expect(rowKeys(tester), ['switcher.row.thread.${threadId(1).value}']);
      expect(titles.queries, isEmpty);
      await host.dispose(tester);
    });

    testWidgets('Tab and Shift+Tab cycle the filter row', (tester) async {
      await open(tester);
      await type(tester, 's');
      expect(
        rowKeys(tester),
        containsAll([
          'switcher.row.channel.${channelId(2).value}',
          'switcher.row.person.${sam.value}',
        ]),
      );
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await host.settle(tester);
      expect(selectedFilter(tester), 'channels');
      expect(
        rowKeys(tester).every(
          (k) =>
              k.startsWith('switcher.row.channel.') ||
              k.startsWith('switcher.row.space.'),
        ),
        isTrue,
      );
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await host.settle(tester);
      expect(selectedFilter(tester), 'people');
      expect(rowKeys(tester), ['switcher.row.person.${sam.value}']);
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await host.settle(tester);
      expect(selectedFilter(tester), 'threads');
      await tester.tap(find.byKey(const ValueKey('switcher.filter.all')));
      await host.settle(tester);
      expect(selectedFilter(tester), 'all');
      await host.dispose(tester);
    });

    testWidgets('arrow keys move the selection and Enter navigates', (
      tester,
    ) async {
      await open(tester);
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
      await host.settle(tester);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await pumpUntil(
        tester,
        () => find.byKey(const ValueKey('switcher.dialog')).evaluate().isEmpty,
        what: 'the close',
      );
      await host.settle(tester);
      expect(host.visited.last, '/c/${channelId(3).value}');
      await host.dispose(tester);
    });

    testWidgets('Enter on a server title opens the thread', (tester) async {
      await open(tester, phase: ConnectionPhase.live);
      await type(tester, 'gateway time');
      await pumpUntil(
        tester,
        () => rowKeys(tester).isNotEmpty,
        what: 'the server title',
      );
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await host.settle(tester);
      expect(host.visited.last, '/t/${threadId(40).value}');
      await host.dispose(tester);
    });

    testWidgets('Esc closes without navigating', (tester) async {
      await open(tester);
      final before = List.of(host.visited);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await host.settle(tester);
      expect(find.byKey(const ValueKey('switcher.dialog')), findsNothing);
      expect(host.visited, before);
      await host.dispose(tester);
    });

    testWidgets('nothing found shows the note', (tester) async {
      await open(tester);
      await type(tester, 'zzz');
      expect(rowKeys(tester), isEmpty);
      expect(find.text(en.switcherNothing), findsOneWidget);
      await host.dispose(tester);
    });
  });
}

/// The name of the selected filter chip (`switcher.filter.<name>`).
String selectedFilter(WidgetTester tester) => [
  for (final f in SwitcherFilter.values)
    if (tester
        .widget<ChoiceChip>(find.byKey(ValueKey('switcher.filter.${f.name}')))
        .selected)
      f.name,
].single;
