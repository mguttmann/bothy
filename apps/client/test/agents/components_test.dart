// SPDX-License-Identifier: Apache-2.0

// Components on messages (brief M4-21 Tests; 01 6.5, D4; 09 11.2, 11.3; components.md 8;
// R-PR-04): one interaction per activation and none twice for a double click, disabled with
// the reason for those not allowed, the used and expired states, and no keyboard shortcut
// reaches a button: only Enter or Space on the focused button.

import 'package:client/agents/components_mapping.dart';
import 'package:client/sync/agent_ops.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/agent_fakes.dart';
import '../support/card_host.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

List<MessageComponent> approval({
  ComponentState state = ComponentState.active,
}) => [
  button(1, cardId, customId: 'allow', label: 'Allow once', state: state),
  button(
    2,
    cardId,
    customId: 'deny',
    label: 'Deny',
    style: 'danger',
    state: state,
  ),
];

Finder buttonOf(String customId) =>
    find.byKey(ValueKey('ds-component-$customId'));

/// Puts the keyboard focus on the button [customId] (the focus node of its ink well).
void focusButton(WidgetTester tester, String customId) {
  final inside = find.descendant(
    of: find.descendant(of: buttonOf(customId), matching: find.byType(InkWell)),
    matching: find.byType(ConstrainedBox),
  );
  Focus.of(tester.element(inside.first)).requestFocus();
}

void main() {
  testWidgets('a double click creates one interaction with the values of the '
      'button', (tester) async {
    final host = await openCard(
      tester,
      text: 'Run the tests?',
      components: approval(),
    );
    await tester.tap(buttonOf('allow'));
    await tester.tap(buttonOf('allow'));
    await tester.tap(buttonOf('deny'));
    await host.settle(tester);
    final entries = await entriesOf(tester, host, AgentOps.createInteraction);
    expect(entries, hasLength(1));
    expect(entries.single.payload, contains('"custom_id":"allow"'));
    expect(entries.single.payload, contains(componentId(1).value));
    expect(entries.single.payload, contains('"values":{}'));
    // Sent and waiting: the pressed button loads, every other one is locked.
    final allow = tester.widget<DsButton>(buttonOf('allow'));
    expect(allow.loading, isTrue);
    expect(tester.widget<DsButton>(buttonOf('deny')).onPressed, isNull);
    await host.dispose(tester);
  });

  testWidgets(
    'without the level trigger the card is disabled with the reason',
    (tester) async {
      final host = await openCard(
        tester,
        text: 'Run the tests?',
        components: approval(),
        level: AccessLevel.write,
      );
      expect(find.text(en.componentsDeniedLevel), findsOneWidget);
      expect(tester.widget<DsButton>(buttonOf('allow')).onPressed, isNull);
      await tester.tap(buttonOf('allow'));
      await host.settle(tester);
      expect(
        await entriesOf(tester, host, AgentOps.createInteraction),
        isEmpty,
      );
      await host.dispose(tester);
    },
  );

  testWidgets('run_starter: another person sees the lock note with the '
      "starter's name", (tester) async {
    final host = await openCard(
      tester,
      text: 'Deploy?',
      starter: sam,
      components: [
        button(
          1,
          cardId,
          customId: 'allow',
          label: 'Allow once',
          restrictedTo: ComponentRestriction.runStarter,
        ),
      ],
    );
    expect(
      find.text(en.componentsApproverRunStarter(dsIsolate('Sam'))),
      findsWidgets,
    );
    expect(tester.widget<DsButton>(buttonOf('allow')).onPressed, isNull);
    await host.dispose(tester);
  });

  testWidgets('used shows who chose what and when; expired shows "Expired"', (
    tester,
  ) async {
    final host = await openCard(
      tester,
      text: 'Run the tests?',
      components: [
        button(
          1,
          cardId,
          customId: 'allow',
          label: 'Allow once',
          state: ComponentState.used,
          usedBy: sam,
          usedAt: t0,
        ),
        button(
          2,
          cardId,
          customId: 'deny',
          label: 'Deny',
          state: ComponentState.disabled,
        ),
      ],
    );
    expect(
      find.textContaining("chose 'Allow once'", findRichText: true),
      findsOneWidget,
    );
    expect(buttonOf('allow'), findsNothing);
    await host.dispose(tester);

    final expired = await openCard(
      tester,
      text: 'Run the tests?',
      components: approval(state: ComponentState.expired),
    );
    expect(find.text(en.componentsExpired), findsOneWidget);
    expect(buttonOf('allow'), findsNothing);
    await expired.dispose(tester);
  });

  testWidgets('no keyboard shortcut activates a button; Enter and Space on the '
      'focused button do', (tester) async {
    final host = await openCard(
      tester,
      text: 'Run the tests?',
      components: approval(),
    );
    // Keys without focus on a button reach no component (R-PR-04).
    for (final key in [
      LogicalKeyboardKey.enter,
      LogicalKeyboardKey.space,
      LogicalKeyboardKey.keyA,
      LogicalKeyboardKey.digit1,
    ]) {
      await tester.sendKeyEvent(key);
    }
    await host.settle(tester);
    expect(await entriesOf(tester, host, AgentOps.createInteraction), isEmpty);

    // Focus the button "Allow once", then Enter.
    focusButton(tester, 'allow');
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await host.settle(tester);
    final entries = await entriesOf(tester, host, AgentOps.createInteraction);
    expect(entries, hasLength(1));
    expect(entries.single.payload, contains('"custom_id":"allow"'));
    await host.dispose(tester);
  });

  testWidgets('Space on the focused button activates it', (tester) async {
    final host = await openCard(
      tester,
      text: 'Run the tests?',
      components: approval(),
    );
    focusButton(tester, 'deny');
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.space);
    await host.settle(tester);
    final entries = await entriesOf(tester, host, AgentOps.createInteraction);
    expect(entries.single.payload, contains('"custom_id":"deny"'));
    await host.dispose(tester);
  });

  test('the values of a select and a form follow InteractionCreate', () {
    final select = button(
      1,
      cardId,
      customId: 's',
      label: 'x',
    ).copyWith(type: ComponentType.select);
    expect(interactionValues(select, ['model-large']), {
      'selected': ['model-large'],
    });
    final form = select.copyWith(type: ComponentType.form);
    expect(interactionValues(form, {'port': 55432, 'note': null}), {
      'port': 55432,
    });
    expect(
      interactionValues(button(2, cardId, customId: 'b', label: 'B'), null),
      isEmpty,
    );
  });
}
