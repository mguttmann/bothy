// SPDX-License-Identifier: Apache-2.0

// FIX-TEAM-MEMBER-AUTHOR-DISPLAY (owner report of 2026-11-22): in a team channel every member's
// mirrored reply shows the member as its author (handle, own avatar, the agent only as a small
// note), and consecutive replies of two members of one agent are never one group.

import 'package:client/agents/providers.dart';
import 'package:client/notifications_settings/providers.dart'
    show isOwnerProvider;
import 'package:client/team/providers.dart';
import 'package:client/thread/history_column.dart';
import 'package:client/thread/message_tile.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
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

Message mirror(int seq, String handle, String text) => msg(
  seq,
  author: robin,
  origin: MessageOrigin.agent,
  text: text,
).copyWith(teamHandle: handle);

Future<ColumnHost> open(WidgetTester tester) async {
  final host = await ColumnHost.open(tester);
  await host.run(tester, () async {
    await seedThread(
      host.db,
      ch: channel(1, name: 'team'),
      th: thread(1, title: 'Release', lastSeq: 5),
      messages: [
        msg(1, author: alex, text: 'Please ship the release'),
        mirror(2, 'codingagent', 'Built it'),
        mirror(3, 'reviewagent', 'Reviewed it'),
        mirror(4, 'reviewagent', 'One more note'),
        msg(5, author: robin, origin: MessageOrigin.agent, text: 'Old copy'),
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
      isOwnerProvider.overrideWith((ref) async => true),
      channelTeamProvider(channelId(1).value).overrideWith(
        (ref) async => team(
          members: [
            member(1, 'codingagent'),
            member(2, 'reviewagent', homeN: 3, threadN: 3),
          ],
        ),
      ),
    ],
  );
  return host;
}

DsMessage messageOf(WidgetTester tester, int seq) => tester.widget<DsMessage>(
  find.descendant(
    of: find.byKey(ValueKey('message.${messageId(seq).value}')),
    matching: find.byType(DsMessage),
  ),
);

void main() {
  testWidgets('each member writes under its own name, the agent is a note', (
    tester,
  ) async {
    final host = await open(tester);
    expect(messageOf(tester, 2).data.authorName, 'codingagent');
    expect(messageOf(tester, 2).data.authorVia, 'Robin');
    expect(messageOf(tester, 3).data.authorName, 'reviewagent');
    expect(find.text(dsIsolate('codingagent')), findsOneWidget);
    expect(find.text(dsIsolate('reviewagent')), findsOneWidget);
    expect(
      find.text(en.messageAuthorVia(dsIsolate('Robin'))),
      findsNWidgets(2),
    );
    // The header names the member, so the small "from @handle" line is gone.
    expect(
      find.byKey(ValueKey('team.label.${messageId(2).value}')),
      findsNothing,
    );
    expect(find.text(en.teamFromMember('@codingagent')), findsNothing);
    // A copy stored without a member handle keeps the agent and the earlier line.
    expect(messageOf(tester, 5).data.authorName, 'Robin');
    expect(messageOf(tester, 5).data.authorVia, isNull);
    expect(
      find.text(en.teamFromMember('@codingagent, @reviewagent')),
      findsOneWidget,
    );
    await host.dispose(tester);
  });

  testWidgets('two members of one agent are two groups, one member one group', (
    tester,
  ) async {
    final host = await open(tester);
    expect(messageOf(tester, 2).isContinuation, isFalse);
    expect(messageOf(tester, 3).isContinuation, isFalse);
    expect(messageOf(tester, 4).isContinuation, isTrue);
    expect(messageOf(tester, 5).isContinuation, isFalse);
    await host.dispose(tester);
  });

  test(
    'a reply quotes the member, German names the agent in its own words',
    () {
      final principals = {for (final p in people) p.id: p};
      final data = messageData(
        msg(9, author: alex, text: 'Thanks'),
        principals: principals,
        me: alex,
        timeLabel: '10:42',
        replied: mirror(2, 'codingagent', 'Built it'),
      );
      expect(data.replyToName, 'codingagent');
      // language-check: off
      expect(
        lookupAppLocalizations(const Locale('de')).messageAuthorVia('Robin'),
        'über Robin',
      );
      // language-check: on
    },
  );
}
