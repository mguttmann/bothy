// SPDX-License-Identifier: Apache-2.0

// A team channel in the history column (09 3.9; design cross-project-handoff.md MVP item 6):
// an owner sees the member bar, "from @handle" under a member's mirrored reply and "to all
// members" under an owner's message, and the composer offers `/team`; a person who is no owner
// sees none of it.

import 'package:client/agents/providers.dart';
import 'package:client/notifications_settings/providers.dart'
    show isOwnerProvider;
import 'package:client/team/providers.dart';
import 'package:client/team/team_port.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
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
      th: thread(1, title: 'Release', lastSeq: 3),
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
