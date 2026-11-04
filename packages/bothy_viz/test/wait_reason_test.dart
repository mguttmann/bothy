// SPDX-License-Identifier: Apache-2.0

// Why a queued run waits in the live topology and its table (F-WARTESCHLANGE-GRUND;
// docs/spec/25-visualization.md 6.1, 09 5.2.1).

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:bothy_viz/src/views/table/live_topology_table.dart';
import 'package:design_system/design_system.dart' show DsWaitReason;
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support/fixtures.dart';

const _space = '0192f4a0-7b1c-7000-8a00-000000000002';
const _channel = '0192f4a0-7b1c-7000-8a00-000000000102';
const _runner = '0192f4a0-7b1c-7000-8a00-000000000202';
const _waiting = '0192f4a0-7b1c-7000-8a00-000000000503';
const _shown = '0192f4a0-7b1c-7000-8a00-000000000501';
const _shownThread = '0192f4a0-7b1c-7000-8a00-000000000601';
const _unnamed = '0192f4a0-7b1c-7000-8a00-000000000502';

Map<String, Object?> _run(
  String id,
  String state, {
  String? reason,
  Map<String, Object?>? chat,
}) => {
  'run_id': id,
  'thread_id':
      '0192f4a0-7b1c-7000-8a00-0000000006${id.substring(id.length - 2)}',
  'channel_id': _channel,
  'space_id': _space,
  'agent_id': _runner,
  'state': state,
  'state_reason': ?reason,
  'queued_at': '2026-11-04T09:00:00.000Z',
  'billing_mode': 'organization',
  'approval_pending': false,
  'chat': ?chat,
};

api.StatsLiveFrame _frame({
  String presence = 'online',
  Map<String, Object?>? slots = const {'used': 3, 'total': 3},
}) => api.standardSerializers.deserializeWith(api.StatsLiveFrame.serializer, {
  'at': '2026-11-04T10:00:00.000Z',
  'interval_ms': 1000,
  'scope': {'kind': 'instance'},
  'runs': [
    _run(
      _shown,
      'running',
      chat: {
        'thread_id': _shownThread,
        'channel_name': 'backend',
        'thread_title': 'Payment',
      },
    ),
    _run(_unnamed, 'running'),
    _run(_waiting, 'queued', reason: 'slots_full'),
  ],
  'runs_truncated': false,
  'agents': [
    {
      'agent_id': _runner,
      'presence': presence,
      'billing_mode': 'organization',
      'run_slots': ?slots,
    },
  ],
  'rates': <Object?>[],
})!;

void main() {
  late AppLocalizations l;
  setUpAll(() async {
    l = await AppLocalizations.delegate.load(const Locale('en'));
  });

  RunCardModel waiting(api.StatsLiveFrame frame) => LiveTopologyModel.fromFrame(
    frame,
    names: names,
  ).cards.firstWhere((c) => c.runId == _waiting);

  test('a slots_full run names the slot use and the runs that hold it', () {
    final wait = waiting(_frame()).wait!;
    expect(wait.reason, DsWaitReason.slotsFull);
    expect((wait.used, wait.total), (3, 3));
    expect([for (final b in wait.blockers) b.threadId], [_shownThread, null]);
    expect(wait.hidden, 1);
  });

  test('an offline runner is the reason, ahead of slots_full', () {
    for (final presence in ['offline', 'unknown']) {
      final wait = waiting(_frame(presence: presence)).wait!;
      expect(wait.reason, DsWaitReason.agentOffline, reason: presence);
      expect(wait.blockers, isEmpty);
    }
  });

  test('a running card has no wait', () {
    final model = LiveTopologyModel.fromFrame(_frame(), names: names);
    expect(model.cards.firstWhere((c) => c.runId == _shown).wait, isNull);
  });

  testWidgets('the card says why it waits and links the runs that hold the '
      'slots', (tester) async {
    final opened = <String>[];
    await tester.pumpWidget(
      app(RunCardTreeMark(model: waiting(_frame()), onTap: opened.add)),
    );
    await dsPumpIcons(tester);
    expect(find.text(l.liveBoxWaitSlotsFull(3, 3)), findsOneWidget);
    expect(find.text(l.liveBoxStateQueued), findsNothing);
    expect(find.text(l.liveBoxWaitHeldBy), findsOneWidget);
    expect(find.text(l.liveBoxWaitHiddenRuns(1)), findsOneWidget);
    await tester.tap(
      find.bySemanticsLabel(l.liveBoxWaitOpenRun('backend / Payment')),
    );
    expect(opened, [_shownThread]);
  });

  testWidgets('an offline runner on the card', (tester) async {
    await tester.pumpWidget(
      app(RunCardTreeMark(model: waiting(_frame(presence: 'offline')))),
    );
    await dsPumpIcons(tester);
    expect(find.text(l.liveBoxWaitAgentOffline), findsOneWidget);
    expect(find.text(l.liveBoxWaitHeldBy), findsNothing);
  });

  test('the table has the wait text and one row per run that holds a slot', () {
    final model = LiveTopologyModel.fromFrame(_frame(), names: names);
    final opened = <String>[];
    final table = liveTopologyTables(
      VizTexts(l, const Locale('en')),
      lanes: model.lanes,
      rates: model.rates,
      onOpenThread: opened.add,
    ).first;
    expect(table.texts, contains(l.liveBoxWaitSlotsFull(3, 3)));
    expect(table.texts, contains(l.liveBoxWaitHiddenRuns(1)));
    final holder = table.rows.firstWhere(
      (r) => r.any((c) => c.text == l.liveBoxWaitHeldBy) && r[1].onTap != null,
    );
    holder[1].onTap!();
    expect(opened, [_shownThread]);
  });

  test('without run_slots the text has no count', () {
    final wait = waiting(_frame(slots: null)).wait!;
    expect(wait.reason, DsWaitReason.slotsFull);
    expect(wait.total, isNull);
    expect(
      VizTexts(l, const Locale('en')).cardState(waiting(_frame(slots: null))),
      l.liveBoxWaitSlotsFullNoCount,
    );
  });
}
