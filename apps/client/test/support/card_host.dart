// SPDX-License-Identifier: Apache-2.0

// Hosts thread 1 with a card of the agent Robin (kind card) bound to run 1 for the card tests
// of M4-21 (SEC-T14, components.md 8).

import 'package:client/agents/providers.dart';
import 'package:client/thread/history_column.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import 'agent_fakes.dart';
import 'column_host.dart';
import 'structure.dart';
import 'thread_fixtures.dart';
import 'thread_host.dart';

/// The ID of the card message.
final MessageId cardId = messageId(2);

/// The card message of Robin with [text].
Message cardMessage(String text, {int version = 1}) => msg(
  2,
  author: robin,
  text: text,
  kind: MessageKind.card,
  origin: MessageOrigin.agent,
  minutes: 1,
  version: version,
);

/// Opens thread 1 with Alex's triggering message, run 1 of Robin (started by [starter]) and
/// the card [text] with [components]; Alex has [level] and [capabilities] in the channel.
Future<ColumnHost> openCard(
  WidgetTester tester, {
  required String text,
  required List<MessageComponent> components,
  AccessLevel level = AccessLevel.trigger,
  List<Capability> capabilities = const [],
  PrincipalId? starter,
}) async {
  final host = await ColumnHost.open(tester);
  await host.run(tester, () async {
    await seedThread(
      host.db,
      messages: [
        msg(1, author: alex, text: '@robin deploy'),
        cardMessage(text),
      ],
      level: level,
      capabilities: capabilities,
    );
    await host.db.runsDao.upsertIfNewer(
      run(1, RunState.waitingInput, triggeredBy: starter ?? alex),
    );
    for (final c in components) {
      await host.db.messagesDao.upsertComponentIfNewer(c);
    }
  });
  await host.pump(
    tester,
    HistoryColumn(threadId: threadId(1)),
    location: '/t/${threadId(1).value}',
    overrides: [
      ...threadOverrides(FakeThreadReads(host.db)),
      runSourceProvider.overrideWithValue(StoreRunSource()),
      agentReadsProvider.overrideWithValue(FakeAgentReads()),
    ],
  );
  return host;
}

/// The outbox entries of [kind].
Future<List<OutboxEntry>> entriesOf(
  WidgetTester tester,
  ColumnHost host,
  String kind,
) async => [
  for (final e in await host.run(tester, () => host.db.outboxDao.allEntries()))
    if (e.kind == kind) e,
];

/// The card body of the card message.
final Finder cardFinder = find.byKey(ValueKey('card.${cardId.value}'));
