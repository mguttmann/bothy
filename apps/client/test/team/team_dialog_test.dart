// SPDX-License-Identifier: Apache-2.0

// The team dialog over a fake API (09 3.9.1, 3.9.7; design cross-project-handoff.md MVP item
// 6): the switch, the member list, adding with agent, home channel and handle (the agents of
// the server, filtered as typed, the free home channels and a suggested handle), removing, stop
// all, and the server's reason of a refusal, in English and German.

import 'package:client/team/team_controller.dart';
import 'package:client/team/team_dialog.dart';
import 'package:client/team/team_port.dart';
import 'package:design_system/design_system.dart'
    show DsButton, DsDropdownField;
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/structure.dart';
import '../support/team_fakes.dart';
import '../support/thread_fixtures.dart';
import '../support/widgets.dart';

final Map<PrincipalId, Principal> principals = {
  for (final p in people) p.id: p,
  nova: Principal(
    id: nova,
    kind: PrincipalKind.agent,
    handle: 'nova',
    displayName: 'Nova',
  ),
};

final List<Channel> channels = [
  channel(1, name: 'team'),
  channel(2, name: 'website', kind: ChannelKind.forum, operator: robin),
  channel(3, name: 'docs', operator: nova),
  channel(4, name: 'api', kind: ChannelKind.forum, operator: nova),
];

Future<(TeamController, AppLocalizations)> open(
  WidgetTester tester,
  FakeTeamPort port, {
  Locale locale = const Locale('en'),
  String? handle,
}) async {
  desktopWindow(tester);
  var keys = 0;
  final controller = TeamController(
    channel: channelId(1),
    port: port,
    newKey: () => 'key-${++keys}',
  );
  addTearDown(controller.dispose);
  await tester.pumpWidget(
    hostScreen(
      Scaffold(
        body: TeamDialog(
          controller: controller,
          principals: principals,
          channels: channels,
          initialHandle: handle,
        ),
      ),
      locale: locale,
    ),
  );
  await controller.load();
  await tester.pumpAndSettle();
  return (controller, await AppLocalizations.delegate.load(locale));
}

/// Chooses [value] in the dropdown field [field], as a pick from its open list would; the
/// field's list itself is tested in the design system.
Future<void> choose<T>(WidgetTester tester, String field, T value) async {
  final widget = tester.widget<DsDropdownField<T>>(find.byKey(ValueKey(field)));
  expect(widget.entries.map((e) => e.value), contains(value));
  widget.onSelected(value);
  await tester.pumpAndSettle();
}

/// Whether the button [key] can be pressed.
bool canPress(WidgetTester tester, String key) =>
    tester.widget<DsButton>(find.byKey(ValueKey(key))).onPressed != null;

/// The text of [label] without the isolation marks of `dsIsolate`.
String plain(String label) => label.replaceAll(RegExp('[\u2068\u2069]'), '');

void main() {
  testWidgets(
    'a channel without team mode shows only the switch; on enables it',
    (tester) async {
      final port = FakeTeamPort();
      final (_, l) = await open(tester, port);
      expect(find.text(l.teamSwitch), findsOneWidget);
      expect(find.byKey(const ValueKey('team.add.handle')), findsNothing);
      expect(find.byKey(const ValueKey('team.stopAll')), findsNothing);
      await tester.tap(find.byKey(const ValueKey('team.switch')));
      await tester.pumpAndSettle();
      expect(port.calls, ['read', 'enable']);
      expect(find.byKey(const ValueKey('team.empty')), findsOneWidget);
      expect(find.byKey(const ValueKey('team.stopAll')), findsOneWidget);
    },
  );

  testWidgets(
    'lists the members with agent and home channel; remove removes one',
    (tester) async {
      final port = FakeTeamPort(
        team(
          members: [
            member(1, 'coding'),
            member(2, 'qa', agent: nova, homeN: 3, threadN: 3),
          ],
        ),
      );
      final (_, l) = await open(tester, port);
      expect(find.text('@coding'), findsOneWidget);
      expect(
        find.byWidgetPredicate(
          (w) => w is Text && plain(w.data ?? '') == 'Robin in website',
        ),
        findsOneWidget,
      );
      expect(find.byTooltip(l.teamRemove('qa')), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('team.remove.qa')));
      await tester.pumpAndSettle();
      expect(port.calls.last, 'remove ${member(2, 'qa').id}');
      expect(find.text('@qa'), findsNothing);
    },
  );

  testWidgets('adds an agent with its only home channel and a handle', (
    tester,
  ) async {
    final port = FakeTeamPort(team());
    await open(tester, port);
    final submit = find.byKey(const ValueKey('team.add.submit'));
    expect(canPress(tester, 'team.add.submit'), isFalse);
    await choose(tester, 'team.add.agent.0', robin);
    await tester.enterText(
      find.byKey(const ValueKey('team.add.handle')),
      'coding',
    );
    await tester.pumpAndSettle();
    await tester.tap(submit);
    await tester.pumpAndSettle();
    expect(port.calls.last, 'add coding key-1');
    expect(port.current!.members.single.homeChannelId, channelId(2));
    expect(find.text('@coding'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('team.add.agent.1')),
      findsOneWidget,
      reason: 'the agent field starts empty again',
    );
    expect(
      tester
          .widget<TextField>(find.byKey(const ValueKey('team.add.handle')))
          .controller!
          .text,
      isEmpty,
    );
  });

  testWidgets('an agent with two home channels waits for the choice', (
    tester,
  ) async {
    final port = FakeTeamPort(team());
    await open(tester, port, handle: 'docs');
    await choose(tester, 'team.add.agent.0', nova);
    expect(canPress(tester, 'team.add.submit'), isFalse);
    await choose(tester, 'team.add.home.${nova.value}', channelId(4));
    expect(canPress(tester, 'team.add.submit'), isTrue);
    await tester.tap(find.byKey(const ValueKey('team.add.submit')));
    await tester.pumpAndSettle();
    expect(port.calls.last, 'add docs key-1');
    expect(port.current!.members.single.homeChannelId, channelId(4));
  });

  testWidgets('an agent that operates no channel says so', (tester) async {
    final port = FakeTeamPort(team());
    final silent = PrincipalId.parse(uuid(0xa9));
    desktopWindow(tester);
    final controller = TeamController(
      channel: channelId(1),
      port: port,
      newKey: () => 'k',
    );
    addTearDown(controller.dispose);
    await tester.pumpWidget(
      hostScreen(
        Scaffold(
          body: TeamDialog(
            controller: controller,
            principals: {
              silent: Principal(
                id: silent,
                kind: PrincipalKind.agent,
                handle: 'idle',
                displayName: 'Idle',
              ),
            },
            channels: channels,
          ),
        ),
      ),
    );
    await controller.load();
    await tester.pumpAndSettle();
    await choose(tester, 'team.add.agent.0', silent);
    expect(find.byKey(const ValueKey('team.add.noHome')), findsOneWidget);
  });

  testWidgets('an invalid handle shows the rule and cannot be sent', (
    tester,
  ) async {
    final (_, l) = await open(tester, FakeTeamPort(team()), handle: 'Coding');
    await choose(tester, 'team.add.agent.0', robin);
    expect(find.text(l.teamAddHandleInvalid), findsOneWidget);
    expect(canPress(tester, 'team.add.submit'), isFalse);
  });

  testWidgets('a refusal shows the server reason in German', (tester) async {
    final port = FakeTeamPort(team())
      ..failures.add(
        const TeamFailure.problem('conflict', reason: 'team_handle_taken'),
      );
    final (_, l) = await open(tester, port, locale: const Locale('de'));
    await choose(tester, 'team.add.agent.0', robin);
    await tester.enterText(find.byKey(const ValueKey('team.add.handle')), 'qa');
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('team.add.submit')));
    await tester.pumpAndSettle();
    expect(find.text(l.teamRefused(l.teamReasonHandleTaken)), findsOneWidget);
    expect(l.teamReasonHandleTaken, 'ein anderes Mitglied hat dieses Handle');
    expect(find.text('@qa'), findsNothing);
  });

  testWidgets('the agents come from the server and filter as typed', (
    tester,
  ) async {
    final quill = PrincipalId.parse(uuid(0xaa));
    final port = FakeTeamPort(team())
      ..agentList = [
        TeamAgent(id: nova, handle: 'nova', name: 'Nova'),
        TeamAgent(id: quill, handle: 'quill', name: 'Quill'),
        TeamAgent(id: robin, handle: 'robin', name: 'Robin'),
      ];
    await open(tester, port);
    final field = tester.widget<DsDropdownField<PrincipalId>>(
      find.byKey(const ValueKey('team.add.agent.0')),
    );
    expect(field.entries.map((e) => e.value), [
      nova,
      quill,
      robin,
    ], reason: 'Quill is no stored principal, the server names it');
    expect(field.enableFilter, isTrue);
    await tester.enterText(
      find.descendant(
        of: find.byKey(const ValueKey('team.add.agent.0')),
        matching: find.byType(TextField),
      ),
      'q',
    );
    await tester.pumpAndSettle();
    final shown = [
      for (final t in tester.widgetList<Text>(
        find
            .descendant(
              of: find.byType(MenuItemButton),
              matching: find.byType(Text),
            )
            .hitTestable(),
      ))
        plain(t.data ?? ''),
    ];
    expect(shown, ['Quill']);
  });

  testWidgets(
    'the handle is suggested from the home channel and stays editable',
    (tester) async {
      final port = FakeTeamPort(team(members: [member(1, 'api', homeN: 3)]));
      await open(tester, port);
      final handle = find.byKey(const ValueKey('team.add.handle'));
      String text() => tester.widget<TextField>(handle).controller!.text;
      await choose(tester, 'team.add.agent.0', robin);
      expect(text(), 'website', reason: 'Robin\'s only free home is website');
      await choose(tester, 'team.add.agent.0', nova);
      expect(text(), isEmpty, reason: 'Nova has two homes and none is chosen');
      await choose(tester, 'team.add.home.${nova.value}', channelId(4));
      expect(text(), 'api-2', reason: 'api is taken by a member');
      await tester.enterText(handle, 'backend');
      await tester.pumpAndSettle();
      await choose(tester, 'team.add.home.${nova.value}', channelId(3));
      expect(text(), 'backend', reason: 'an edited handle is never replaced');
      await tester.tap(find.byKey(const ValueKey('team.add.submit')));
      await tester.pumpAndSettle();
      expect(port.calls.last, 'add backend key-1');
    },
  );

  testWidgets('the handle of /team add stays', (tester) async {
    final port = FakeTeamPort(team());
    await open(tester, port, handle: 'coding');
    await choose(tester, 'team.add.agent.0', robin);
    expect(
      tester
          .widget<TextField>(find.byKey(const ValueKey('team.add.handle')))
          .controller!
          .text,
      'coding',
    );
  });

  testWidgets('an agent and home channel that are a member are not offered', (
    tester,
  ) async {
    final port = FakeTeamPort(
      team(
        members: [
          member(1, 'coding'),
          member(2, 'docs', agent: nova, homeN: 3, threadN: 3),
        ],
      ),
    );
    await open(tester, port);
    final agents = tester
        .widget<DsDropdownField<PrincipalId>>(
          find.byKey(const ValueKey('team.add.agent.0')),
        )
        .entries
        .map((e) => e.value);
    expect(agents, isNot(contains(robin)), reason: 'website is taken');
    expect(agents, contains(nova));
    await choose(tester, 'team.add.agent.0', nova);
    final homes = tester
        .widget<DsDropdownField<ChannelId>>(
          find.byKey(ValueKey('team.add.home.${nova.value}')),
        )
        .entries
        .map((e) => e.value);
    expect(homes, [channelId(4)]);
    expect(
      canPress(tester, 'team.add.submit'),
      isTrue,
      reason: 'api is chosen at once',
    );
  });

  testWidgets('stop all stops the members', (tester) async {
    final port = FakeTeamPort(team(members: [member(1, 'coding')]));
    final (controller, _) = await open(tester, port);
    await tester.tap(find.byKey(const ValueKey('team.stopAll')));
    await tester.pumpAndSettle();
    expect(port.calls.last, 'stop key-1');
    expect(controller.team!.pause, TeamPause.stopped);
  });

  testWidgets('switching off asks the server and hides the members', (
    tester,
  ) async {
    final port = FakeTeamPort(team(members: [member(1, 'coding')]));
    await open(tester, port);
    await tester.tap(find.byKey(const ValueKey('team.switch')));
    await tester.pumpAndSettle();
    expect(port.calls.last, 'disable');
    expect(find.text('@coding'), findsNothing);
  });
}
