// SPDX-License-Identifier: Apache-2.0

// A team channel in the history column (09 3.9; design cross-project-handoff.md MVP item 6):
// an owner sees the member bar, "from @handle" under a member's mirrored reply, "to @handle"
// under an owner's message that mentions members and "to all members" under one that does not,
// the composer offers `/team` and, after `@`, every member; a person who is no owner sees none
// of it.

import 'package:client/agents/providers.dart';
import 'package:client/notifications_settings/providers.dart'
    show isOwnerProvider;
import 'package:client/team/providers.dart';
import 'package:client/team/team_port.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';

import '../support/agent_fakes.dart';
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/team_fakes.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

Future<ColumnHost> open(
  WidgetTester tester, {
  required bool owner,
  TeamView? teamOf,
}) async {
  final host = await ColumnHost.open(tester);
  await host.run(tester, () async {
    await seedThread(
      host.db,
      ch: channel(1, name: 'team'),
      th: thread(1, title: 'Release', lastSeq: 4),
      messages: [
        msg(1, author: alex, text: 'Please ship the release'),
        msg(2, author: robin, origin: MessageOrigin.agent, text: 'Done, @qa'),
        msg(
          3,
          author: robin,
          origin: MessageOrigin.system,
          kind: MessageKind.system,
          text: 'Team stopped',
        ),
        msg(4, author: alex, text: '@Coding please review', minutes: 1),
      ],
      level: AccessLevel.trigger,
    );
  });
  await host.pump(
    tester,
    HistoryColumn(threadId: threadId(1)),
    location: '/t/${threadId(1).value}',
    overrides: [
      ...threadOverrides(FakeThreadReads(host.db)),
      runSourceProvider.overrideWithValue(StoreRunSource()),
      agentReadsProvider.overrideWithValue(FakeAgentReads()),
      isOwnerProvider.overrideWith((ref) async => owner),
      channelTeamProvider(channelId(1).value)
          .overrideWith((ref) async => owner ? teamOf : null),
    ],
  );
  return host;
}

List<String> suggested(WidgetTester tester) => [
  for (final s
      in tester
          .widget<DsSuggestionList>(find.byType(DsSuggestionList))
          .suggestions)
    s.command,
];

void main() {
  testWidgets('an owner sees the members, the labels and /team', (
    tester,
  ) async {
    final host = await open(
      tester,
      owner: true,
      teamOf: team(members: [member(1, 'coding')]),
    );
    expect(find.byKey(const ValueKey('team.header')), findsOneWidget);
    expect(
      find.text(en.teamMemberSemantics('coding', en.teamStateIdle)),
      findsOneWidget,
    );
    expect(
      find.byKey(ValueKey('team.label.${messageId(2).value}')),
      findsOneWidget,
    );
    expect(find.text(en.teamFromMember('@coding')), findsOneWidget);
    expect(
      find.byKey(ValueKey('team.label.${messageId(1).value}')),
      findsOneWidget,
    );
    expect(find.text(en.teamToAllMembers), findsOneWidget);
    await tester.enterText(composerField, '/te');
    await host.settle(tester);
    expect(suggested(tester), contains('/team'));
    await host.dispose(tester);
  });

  testWidgets('an owner message that mentions a member is labelled to it', (
    tester,
  ) async {
    final host = await open(
      tester,
      owner: true,
      teamOf: team(members: [member(1, 'coding'), member(2, 'qa')]),
    );
    expect(find.text(en.teamToMembers('@coding')), findsOneWidget);
    expect(find.text(en.teamToAllMembers), findsOneWidget);
    expect(
      lookupAppLocalizations(const Locale('de')).teamToMembers('@coding'),
      'an @coding',
    );
    await host.dispose(tester);
  });

  testWidgets('@ offers every member; letters filter, keys and taps choose', (
    tester,
  ) async {
    final host = await open(
      tester,
      owner: true,
      teamOf: team(
        members: [
          member(1, 'coding'),
          member(2, 'qa', agent: nova),
          member(3, 'docs'),
        ],
      ),
    );
    String text() => tester.widget<TextField>(composerField).controller!.text;
    await tester.enterText(composerField, '@');
    await host.settle(tester);
    expect(suggested(tester), [
      '@coding',
      '@docs',
      '@qa',
    ], reason: 'all members at once, people are no team target');
    await tester.enterText(composerField, 'Hi @d');
    await host.settle(tester);
    expect(suggested(tester), ['@docs']);
    expect(
      tester
          .widget<DsSuggestionList>(find.byType(DsSuggestionList))
          .suggestions
          .single
          .description,
      'Robin',
      reason: 'the agent of the member',
    );
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await host.settle(tester);
    expect(text(), 'Hi @docs ');
    expect(find.byType(DsSuggestionList), findsNothing);

    await tester.enterText(composerField, '@');
    await host.settle(tester);
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await host.settle(tester);
    expect(text(), '@qa ', reason: 'arrow down twice is the third member');

    await tester.enterText(composerField, '@');
    await host.settle(tester);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await host.settle(tester);
    expect(find.byType(DsSuggestionList), findsNothing);

    await tester.enterText(composerField, 'see @');
    await host.settle(tester);
    await tester.tap(find.byKey(const ValueKey('ds-suggestion-team.coding')));
    await host.settle(tester);
    expect(text(), 'see @coding ');
    await host.dispose(tester);
  });

  testWidgets('an owner in a channel without team mode gets /team but no bar', (
    tester,
  ) async {
    final host = await open(tester, owner: true);
    expect(find.byKey(const ValueKey('team.header')), findsNothing);
    expect(find.text(en.teamToAllMembers), findsNothing);
    await tester.enterText(composerField, '/te');
    await host.settle(tester);
    expect(suggested(tester), contains('/team'));
    await host.dispose(tester);
  });

  testWidgets('a person who is no owner sees no team and no /team', (
    tester,
  ) async {
    final host = await open(
      tester,
      owner: false,
      teamOf: team(members: [member(1, 'coding')]),
    );
    expect(find.byKey(const ValueKey('team.header')), findsNothing);
    expect(find.text(en.teamFromMember('@coding')), findsNothing);
    await tester.enterText(composerField, '/');
    await host.settle(tester);
    expect(suggested(tester), isNot(contains('/team')));
    await host.dispose(tester);
  });
}
