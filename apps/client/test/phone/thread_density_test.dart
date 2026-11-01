// SPDX-License-Identifier: Apache-2.0

// The density of the thread on the phone (brief M5-17; 01 5.3 M3, addendum of 2026-10-31): the
// message rows use the full width, the header is one line with the back button, the title and
// the overflow menu (the operator inside it) and collapses while the person reads older
// messages, the live box is one line, and the input's text field fills the field between its
// buttons with the microphone inside it, starts at the left, grows and then scrolls.

import 'package:client/agents/providers.dart';
import 'package:client/phone/phone_back_scope.dart';
import 'package:client/phone/phone_navigation.dart';
import 'package:client/phone/phone_shell.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/agent_fakes.dart';
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

const String _long =
    'The migration of the sync tables is done. I ran the whole suite twice, '
    'once with the old schema and once with the new one, and both runs are green. '
    'Next I will look at the expired threads you mentioned.';

/// A thread of [count] messages by Alex and Robin with a running run of Robin at the end,
/// in a phone page with a back button to "requests", or in [page] instead.
Future<ColumnHost> openThread(
  WidgetTester tester, {
  int count = 3,
  Widget Function(Widget history)? page,
}) async {
  final host = await ColumnHost.open(tester);
  final source = StoreRunSource();
  source.fetcher.docs[runId(1)] = runDoc(
    1,
    statusLine: 'compacting the conversation',
  );
  final messages = [
    for (var i = 1; i < count; i++)
      msg(
        i,
        author: i.isOdd ? alex : robin,
        origin: i.isOdd ? MessageOrigin.human : MessageOrigin.agent,
        text: i == 1 ? _long : 'Message $i',
        minutes: i * 10,
      ),
    msg(
      count,
      author: robin,
      text: '',
      kind: MessageKind.run,
      origin: MessageOrigin.agent,
      minutes: count * 10,
    ).copyWith(runId: runId(1)),
  ];
  await host.run(tester, () async {
    await seedThread(
      host.db,
      th: thread(
        1,
        title: 'Gateway: resume after a network loss on the phone',
        lastSeq: count,
        operator: robin,
      ),
      messages: messages,
      level: AccessLevel.trigger,
    );
    await host.db.runsDao.upsertIfNewer(run(1, RunState.running));
  });
  await host.pump(
    tester,
    page?.call(HistoryColumn(threadId: threadId(1))) ??
        PhoneBackScope(
          label: 'requests',
          onBack: () {},
          child: HistoryColumn(threadId: threadId(1)),
        ),
    location: '/t/${threadId(1).value}',
    platform: 'ios',
    density: DsDensityMode.touch,
    overrides: [
      ...threadOverrides(FakeThreadReads(host.db)),
      runSourceProvider.overrideWithValue(source),
      agentReadsProvider.overrideWithValue(FakeAgentReads()),
    ],
  );
  return host;
}

final Finder _header = find.byKey(const ValueKey('thread.header'));

/// The space the header takes in the column (it collapses to none).
final Finder _slot = find.byKey(const ValueKey('thread.header.slot'));

void main() {
  testWidgets('message rows use the full width of the phone', (tester) async {
    final host = await openThread(tester, count: 4);
    final first = tester.getTopLeft(find.textContaining('The migration'));
    // Message 3 is Alex again, after Robin: its own group with the small avatar.
    final other = tester.getTopLeft(find.text('Message 3'));
    expect(first.dx, lessThanOrEqualTo(DsSpace.s16));
    expect(other.dx, first.dx);
    await host.dispose(tester);
  });

  testWidgets('the header is one line: back, title, overflow menu', (
    tester,
  ) async {
    final host = await openThread(tester);
    final header = tester.getRect(_header);
    expect(header.height, lessThanOrEqualTo(DsDensity.touch.targetMin + 2));
    // The back button sits in the header row, before the title.
    final back = find.descendant(
      of: _header,
      matching: find.byKey(const ValueKey('phone.back')),
    );
    expect(back, findsOneWidget);
    expect(tester.getSize(back).width, greaterThanOrEqualTo(48));
    expect(find.bySemanticsLabel(en.phoneBackTo('requests')), findsOneWidget);
    final title = tester.widget<Text>(
      find.byKey(const ValueKey('thread.title')),
    );
    expect(title.maxLines, 1);
    expect(title.overflow, TextOverflow.ellipsis);
    // The operator moved into the overflow menu.
    expect(find.byKey(const ValueKey('thread.operator')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('thread.more')));
    await host.settle(tester);
    expect(find.byKey(const ValueKey('thread.operator')), findsOneWidget);
    expect(find.text(en.threadOperator(dsIsolate('Robin'))), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('the header collapses while reading older messages', (
    tester,
  ) async {
    final host = await openThread(tester, count: 40);
    final list = find.byKey(const ValueKey('history.list'));
    final height = tester.getSize(_slot).height;
    // A drag down shows older messages (the list is reversed).
    await tester.drag(list, const Offset(0, 300));
    await host.settle(tester);
    await tester.pumpAndSettle();
    expect(tester.getSize(_slot).height, lessThan(1));
    expect(
      find.byKey(const ValueKey('phone.back')).hitTestable(),
      findsNothing,
    );
    // Back towards the newest message: the header returns.
    await tester.drag(list, const Offset(0, -100));
    await host.settle(tester);
    await tester.pumpAndSettle();
    expect(tester.getSize(_slot).height, height);
    await host.dispose(tester);
  });

  testWidgets('with a screen reader the header never collapses', (
    tester,
  ) async {
    tester.platformDispatcher.accessibilityFeaturesTestValue =
        const FakeAccessibilityFeatures(accessibleNavigation: true);
    addTearDown(tester.platformDispatcher.clearAccessibilityFeaturesTestValue);
    final host = await openThread(tester, count: 40);
    final height = tester.getSize(_slot).height;
    await tester.drag(
      find.byKey(const ValueKey('history.list')),
      const Offset(0, 300),
    );
    await host.settle(tester);
    expect(tester.getSize(_slot).height, height);
    await host.dispose(tester);
  });

  testWidgets('the live box is one line', (tester) async {
    final host = await openThread(tester);
    final line = find.byKey(ValueKey('livebox.line.${runId(1).value}'));
    expect(tester.getSize(line).height, lessThanOrEqualTo(50));
    await tester.tap(line);
    await host.settle(tester);
    expect(
      find.byKey(ValueKey('livebox.sheet.${runId(1).value}')),
      findsOneWidget,
    );
    await host.dispose(tester);
  });

  group('composer', () {
    final field = find.byType(TextField).last;
    final box = find.byKey(const ValueKey('ds-composer-field'));

    testWidgets('the text field fills the field, the microphone inside it', (
      tester,
    ) async {
      final host = await openThread(tester);
      final outer = tester.getRect(box);
      final text = tester.getRect(field);
      final mic = tester.getRect(find.byKey(const ValueKey('voice.mic')));
      expect(outer.contains(mic.center), isTrue);
      // The field spans the column but its compact inset; three buttons sit in it.
      expect(outer.width, greaterThanOrEqualTo(390 - 2 * DsSpace.s8));
      expect(text.width, greaterThanOrEqualTo(outer.width - 3 * 48 - 2));
      expect(mic.width, greaterThanOrEqualTo(48));
      expect(mic.height, greaterThanOrEqualTo(48));
      // Templates moved into the attachment menu.
      expect(find.byKey(const ValueKey('composer.templates')), findsNothing);
      await tester.tap(find.bySemanticsLabel(en.composerAttach));
      await host.settle(tester);
      expect(
        find.byKey(const ValueKey('composer.attach.templates')),
        findsOneWidget,
      );
      await host.dispose(tester);
    });

    testWidgets('a tap at the left edge of the text field focuses it', (
      tester,
    ) async {
      final host = await openThread(tester);
      final text = tester.getRect(field);
      await tester.tapAt(Offset(text.left + 2, text.top + 2));
      await host.settle(tester);
      expect(
        tester
            .widget<EditableText>(find.byType(EditableText).last)
            .focusNode
            .hasFocus,
        isTrue,
      );
      final hint = tester.getRect(find.textContaining('Follow-up to').last);
      expect(hint.left - text.left, lessThanOrEqualTo(DsSpace.s8));
      await host.dispose(tester);
    });

    testWidgets('typing grows the field up to its lines, then it scrolls', (
      tester,
    ) async {
      final host = await openThread(tester);
      final one = tester.getSize(box).height;
      await tester.enterText(field, 'one\ntwo\nthree');
      await host.settle(tester);
      final three = tester.getSize(box).height;
      expect(three, greaterThan(one));
      await tester.enterText(
        field,
        List.generate(40, (i) => 'line $i').join('\n'),
      );
      await host.settle(tester);
      final capped = tester.getSize(box).height;
      expect(capped, greaterThan(three));
      // Capped below half the screen; the rest scrolls inside the field.
      expect(capped, lessThan(844 / 2));
      final scrollable = tester.state<ScrollableState>(
        find.descendant(of: field, matching: find.byType(Scrollable)),
      );
      expect(scrollable.position.maxScrollExtent, greaterThan(0));
      expect(tester.takeException(), isNull);
      await host.dispose(tester);
    });
  });

  group('the page frame of a thread', () {
    Future<ColumnHost> openFrame(WidgetTester tester, {bool header = true}) =>
        openThread(
          tester,
          page: (history) => PhoneHeaderBackFrame(
            page: PhonePage(PhonePageKind.thread, '/t/${threadId(1).value}'),
            below: const PhonePage(PhonePageKind.spaces, '/'),
            android: false,
            child: header ? history : const SizedBox.expand(),
          ),
        );

    testWidgets('the header shows the one back button', (tester) async {
      final host = await openFrame(tester);
      final back = find.byKey(const ValueKey('phone.back'));
      expect(back, findsOneWidget);
      expect(find.descendant(of: _header, matching: back), findsOneWidget);
      expect(find.bySemanticsLabel(en.phoneBackTo('Spaces')), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('without a header (loading, not found) the bar stays', (
      tester,
    ) async {
      final host = await openFrame(tester, header: false);
      expect(find.byKey(const ValueKey('phone.back')), findsOneWidget);
      expect(_header, findsNothing);
      await host.dispose(tester);
    });
  });
}
