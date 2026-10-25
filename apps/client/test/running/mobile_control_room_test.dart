// SPDX-License-Identifier: Apache-2.0

// The control room M11 and the run graph M12 on the phone (brief M5-14 Tests; 01 5.3 M11, M12,
// 5.4; 04 6.2 `background_thread_ids`): the tab Running opens M11 with the cards in one column,
// a tap on a card opens M12 on the Running stack, Stop only with the capability and at full
// width, the filters as a sheet, and the focus of the visible cards is released when the app
// goes to the background and requested again when it returns.

import 'dart:io';

import 'package:client/phone/phone_navigation.dart';
import 'package:client/running/providers.dart';
import 'package:client/running/running_now_view.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

import '../phone/support/phone_app.dart';
import '../support/agent_fakes.dart' show runDoc;
import '../support/column_host.dart';
import '../support/fakes.dart';
import '../support/structure.dart';
import '../support/until.dart';
import '../support/widgets.dart';
import 'run_graph_screen_test.dart' show pumpGraph;
import 'running_now_test.dart' show active, fill, overrides;

Finder card(int n) => find.byKey(ValueKey('running.card.${uuid(0x700 + n)}'));
Finder cardStop(int n) =>
    find.byKey(ValueKey('runCard.stop.${uuid(0x700 + n)}'));

/// Every live card draws the document of its run (the run store of a connected engine).
final documents = cardDocumentProvider.overrideWith(
  (ref, id) => Stream<RunDocument?>.value(
    RunDocument.fromJson({...runDoc(1), 'id': id}),
  ),
);

/// Pumps past the transitions of pages and sheets: the running state icons of the cards turn,
/// so the frames never settle.
Future<void> pumpTransitions(WidgetTester tester) async {
  for (var i = 0; i < 10; i++) {
    await tester.pump(const Duration(milliseconds: 100));
  }
}

Future<ColumnHost> pumpPhoneCards(
  WidgetTester tester, {
  String platform = 'ios',
  SwitchLifecycle? lifecycle,
}) async {
  final host = await ColumnHost.open(tester);
  await tester.runAsync(() => fill(host.db));
  await host.pump(
    tester,
    const RunningNowView(),
    platform: platform,
    lifecycle: lifecycle,
    overrides: [...overrides, documents],
  );
  await pumpUntil(tester, () => card(4).evaluate().isNotEmpty, what: 'cards');
  await host.settle(tester);
  return host;
}

void main() {
  testWidgets('the tab Running opens M11; a tap on a card opens M12 on the '
      'Running stack and back returns', (tester) async {
    final root = Directory.systemTemp.createTempSync('m11');
    addTearDown(() => root.deleteSync(recursive: true));
    final row = RunningRow(
      run: active(1, RunState.running, channelN: 1),
      agentName: 'Robin',
      path: const ['Engineering', 'server-1'],
      spaceId: null,
      canStop: true,
    );
    final app = await PhoneApp.start(
      tester,
      root: root,
      overrides: [
        ...overrides,
        runningRowsProvider.overrideWith((ref) => Stream.value([row])),
        documents,
      ],
    );
    await tester.tap(find.byKey(const ValueKey('phone.tab.running')));
    await pumpTransitions(tester);
    expect(app.shell.tab, PhoneTab.running);
    expect(app.shell.pages.single.kind, PhonePageKind.running);
    expect(card(1), findsOneWidget);
    await tester.tap(find.byKey(ValueKey('runCard.body.${uuid(0x701)}')));
    await pumpTransitions(tester);
    expect(app.location, '/running/${uuid(0x701)}?t=${threadId(1).value}');
    expect(app.shell.tab, PhoneTab.running);
    expect(app.shell.pages.map((p) => p.kind), [
      PhonePageKind.running,
      PhonePageKind.runGraph,
    ]);
    expect(find.byKey(const ValueKey('runGraph.screen')), findsOneWidget);
    // The phone frame carries back; the screen has no back button of its own.
    expect(find.byKey(const ValueKey('runGraph.back')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('phone.back')));
    await pumpTransitions(tester);
    expect(app.location, '/running');
    expect(app.shell.pages.single.kind, PhonePageKind.running);
  });

  testWidgets('M11: cards in one column at full width, the switch Cards and '
      'List, Stop only with the capability and at full width', (tester) async {
    final host = await pumpPhoneCards(tester);
    expect(find.byKey(const ValueKey('running.form.cards')), findsOneWidget);
    expect(find.byKey(const ValueKey('running.form.list')), findsOneWidget);
    final first = tester.getRect(card(1));
    final second = tester.getRect(card(2));
    expect(second.left, first.left, reason: 'one column');
    expect(second.top, greaterThan(first.bottom));
    expect(first.width, closeTo(390 - 2 * DsSpace.s16, 1));
    expect(cardStop(1), findsOneWidget, reason: 'own run with trigger');
    expect(cardStop(2), findsNothing, reason: 'foreign run without manage');
    expect(cardStop(3), findsNothing, reason: 'no trigger in the channel');
    expect(cardStop(4), findsOneWidget, reason: 'manage in the channel');
    expect(
      tester.getSize(cardStop(1)).width,
      greaterThan(first.width - 2 * DsSpace.s16 - DsBorderWidth.spine),
      reason: 'Stop at full width',
    );
    final targetMin = tester.element(cardStop(1)).dsDensity.targetMin;
    for (final n in [1, 4]) {
      expect(
        tester.getSize(cardStop(n)).height,
        greaterThanOrEqualTo(targetMin),
      );
    }
    await tester.tap(find.byKey(const ValueKey('running.form.list')));
    await host.settle(tester);
    expect(card(1), findsNothing);
    expect(find.byKey(ValueKey('running.row.${uuid(0x701)}')), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('M11: the filters open as a sheet and apply at once', (
    tester,
  ) async {
    final host = await pumpPhoneCards(tester);
    // The desktop menus are not in the header of the phone.
    expect(find.byKey(const ValueKey('running.filter.space')), findsNothing);
    expect(find.byKey(const ValueKey('running.filter.agent')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('running.filters')));
    await pumpTransitions(tester);
    final sheet = find.byKey(const ValueKey('running.filterSheet'));
    expect(sheet, findsOneWidget);
    await tester.tap(
      find.descendant(
        of: sheet,
        matching: find.byKey(const ValueKey('running.filter.mine')),
      ),
    );
    await host.settle(tester);
    await tester.tap(find.byKey(const ValueKey('running.filtersDone')));
    await pumpTransitions(tester);
    await host.settle(tester);
    expect(sheet, findsNothing);
    // Mine only: Alex's runs 1 and 3 stay, Sam's runs 2 and 4 leave.
    expect(card(1), findsOneWidget);
    expect(card(3), findsOneWidget);
    expect(card(2), findsNothing);
    expect(card(4), findsNothing);
    await host.dispose(tester);
  });

  testWidgets('the focus of the visible cards is released when the app goes '
      'to the background and requested again when it returns', (tester) async {
    final lifecycle = SwitchLifecycle();
    final host = await pumpPhoneCards(tester, lifecycle: lifecycle);
    final live = host.engine.focus.liveBackgroundRuns;
    expect(live, isNotEmpty);
    expect(
      find.descendant(of: card(1), matching: find.text(en.runningLivePaused)),
      findsNothing,
    );

    // A phone in the background draws no frames: the places go without one.
    lifecycle.set(AppActivity.background);
    expect(host.engine.focus.liveBackgroundRuns, isEmpty);
    await host.settle(tester);
    for (var n = 1; n <= 4; n++) {
      expect(
        find.descendant(of: card(n), matching: find.text(en.runningLivePaused)),
        findsOneWidget,
        reason: 'card $n shows its summary',
      );
    }

    lifecycle.set(AppActivity.foreground);
    expect(host.engine.focus.liveBackgroundRuns, live);
    await host.settle(tester);
    expect(
      find.descendant(of: card(1), matching: find.text(en.runningLivePaused)),
      findsNothing,
    );
    await host.dispose(tester);
  });

  testWidgets('the desktop keeps its cards live in the background', (
    tester,
  ) async {
    final lifecycle = SwitchLifecycle();
    final host = await pumpPhoneCards(
      tester,
      platform: 'macos',
      lifecycle: lifecycle,
    );
    final live = host.engine.focus.liveBackgroundRuns;
    expect(live, isNotEmpty);
    lifecycle.set(AppActivity.background);
    expect(host.engine.focus.liveBackgroundRuns, live);
    await host.dispose(tester);
  });

  testWidgets('M12: touch rows toggle on a tap, Stop at full width at the '
      'bottom, no back button of its own', (tester) async {
    final (host, _) = await pumpGraph(tester, 1, platform: 'ios');
    expect(find.byKey(const ValueKey('runGraph.back')), findsNothing);
    final stop = find.byKey(const ValueKey('runGraph.stop'));
    expect(stop, findsOneWidget);
    final screen = tester.getRect(
      find.byKey(const ValueKey('runGraph.screen')),
    );
    final button = tester.getRect(stop);
    expect(button.width, greaterThan(screen.width - 2 * DsSpace.s16));
    expect(button.bottom, closeTo(screen.bottom, DsSpace.s16));
    // Portrait: the bar below the label.
    expect(
      tester.getRect(find.byKey(const ValueKey('runGraph.bar.s2'))).top,
      greaterThanOrEqualTo(
        tester.getRect(find.byKey(const ValueKey('runGraph.label.s2'))).bottom,
      ),
    );
    expect(find.byKey(const ValueKey('runGraph.row.s3')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('runGraph.label.s2')));
    await host.settle(tester);
    expect(find.byKey(const ValueKey('runGraph.row.s3')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('runGraph.label.s2')));
    await host.settle(tester);
    expect(find.byKey(const ValueKey('runGraph.row.s3')), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('M12 without run.cancel has no Stop', (tester) async {
    final (host, _) = await pumpGraph(tester, 2, platform: 'ios');
    expect(find.byKey(const ValueKey('runGraph.stop')), findsNothing);
    await host.dispose(tester);
  });
}
