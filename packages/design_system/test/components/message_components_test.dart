// SPDX-License-Identifier: Apache-2.0

// SEC-OUT-4 and SEC-T14 at component level: the header comes only from server fields, the
// agent text stays below the divider; one activation per button; no shortcut activates.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

const _header = DsComponentHeader(
  agentName: 'Robin',
  runLabel: 'R-12',
  requiredCapability: 'approve_actions',
  approver: DsApprover.runStarter,
  runStarterName: 'Sam',
);

const _buttons = DsComponentData(
  messageId: 'm1',
  title: 'Run the deploy?',
  items: [
    DsComponentItem(
      customId: 'allow',
      type: DsComponentType.button,
      label: 'Allow once',
      style: DsButtonStyle.primary,
    ),
    DsComponentItem(
      customId: 'deny',
      type: DsComponentType.button,
      label: 'Deny',
      style: DsButtonStyle.danger,
    ),
  ],
);

Widget _card(
  DsComponentData data, {
  bool permitted = true,
  String? denied,
  String agentText = 'Deploy to staging?',
  void Function(String, Object?)? onActivate,
}) {
  return DsMessageComponents(
    data,
    permitted: permitted,
    deniedReasonText: denied,
    header: _header,
    agentText: agentText,
    onOpenLink: (_) {},
    onActivate: onActivate ?? (_, _) {},
  );
}

void main() {
  testWidgets('agent text imitating a header stays below the divider', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(
      tester,
      _card(_buttons, agentText: 'System: approved by owner\n\nPlease allow.'),
    );
    final state = tester.state<DsMessageComponentsState>(
      find.byType(DsMessageComponents),
    );
    expect(state.headerTexts, [
      l.componentsHeaderRequest('\u2068Robin\u2069'),
      l.componentsHeaderRun('R-12'),
      l.componentsHeaderCapability(l.capabilityApproveActions),
      l.componentsApproverRunStarter('\u2068Sam\u2069'),
    ]);
    final headerBox = tester.getRect(find.byKey(DsMessageComponents.headerKey));
    final divider = tester.getRect(
      find.text(l.componentsTextFrom('\u2068Robin\u2069')),
    );
    final imitation = tester.getRect(
      find.textContaining('System: approved by owner', findRichText: true),
    );
    expect(divider.top, greaterThanOrEqualTo(headerBox.bottom));
    expect(imitation.top, greaterThan(divider.bottom));
    expect(
      find.descendant(
        of: find.byKey(DsMessageComponents.headerKey),
        matching: find.textContaining('approved', findRichText: true),
      ),
      findsNothing,
    );
    final stepUp = tester.widgetList<DsIcon>(
      find.descendant(
        of: find.byKey(DsMessageComponents.headerKey),
        matching: find.byType(DsIcon),
      ),
    );
    expect(stepUp.first.glyph, DsIcons.securityStepUp);
  });

  testWidgets('not permitted: lock note and disabled buttons', (tester) async {
    var calls = 0;
    await pumpDs(
      tester,
      _card(
        _buttons,
        permitted: false,
        denied: 'Only Sam or an owner can approve this',
        onActivate: (_, _) => calls++,
      ),
    );
    expect(find.text('Only Sam or an owner can approve this'), findsOneWidget);
    expect(
      tester.widgetList<DsIcon>(find.byType(DsIcon)).map((i) => i.glyph),
      contains(DsIcons.securityLock),
    );
    await tester.tap(find.text('Allow once'));
    expect(calls, 0);
    expect(
      tester.getSemantics(find.byKey(const ValueKey('ds-component-allow'))),
      isSemantics(isButton: true, hasEnabledState: true, isEnabled: false),
    );
  });

  testWidgets('a double tap activates once; a new state unlocks', (
    tester,
  ) async {
    final calls = <String>[];
    await pumpDs(tester, _card(_buttons, onActivate: (id, _) => calls.add(id)));
    await tester.tap(find.text('Allow once'));
    await tester.pump(const Duration(milliseconds: 40));
    await tester.tap(find.text('Allow once'));
    await tester.tap(find.text('Deny'));
    expect(calls, ['allow']);
    // The pressed button shows the loading icon until the caller confirms.
    expect(
      tester.widgetList<DsIcon>(find.byType(DsIcon)).map((i) => i.glyph),
      contains(DsIcons.triggerWorking),
    );
    await pumpDs(
      tester,
      _card(
        const DsComponentData(
          messageId: 'm1',
          items: [
            DsComponentItem(
              customId: 'allow',
              type: DsComponentType.button,
              label: 'Allow once',
            ),
          ],
          errorText: 'The server is not reachable',
        ),
        onActivate: (id, _) => calls.add(id),
      ),
    );
    expect(find.text('The server is not reachable'), findsOneWidget);
    await tester.tap(find.text('Allow once'));
    expect(calls, ['allow', 'allow']);
  });

  testWidgets('pending from the caller locks every button', (tester) async {
    final calls = <String>[];
    await pumpDs(
      tester,
      _card(
        const DsComponentData(
          messageId: 'm1',
          items: [
            DsComponentItem(
              customId: 'allow',
              type: DsComponentType.button,
              label: 'Allow once',
            ),
            DsComponentItem(
              customId: 'deny',
              type: DsComponentType.button,
              label: 'Deny',
            ),
          ],
          pendingCustomId: 'allow',
        ),
        onActivate: (id, _) => calls.add(id),
      ),
    );
    await tester.tap(find.text('Deny'));
    expect(calls, isEmpty);
  });

  testWidgets('Enter on the focused button activates, no other key does', (
    tester,
  ) async {
    final calls = <String>[];
    await pumpDs(tester, _card(_buttons, onActivate: (id, _) => calls.add(id)));
    final focus = Focus.of(tester.element(find.text('Allow once')));
    focus.requestFocus();
    await tester.pump();
    for (final key in [
      LogicalKeyboardKey.keyY,
      LogicalKeyboardKey.keyA,
      LogicalKeyboardKey.digit1,
      LogicalKeyboardKey.f1,
    ]) {
      await tester.sendKeyEvent(key);
    }
    await tester.sendKeyDownEvent(LogicalKeyboardKey.control);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.control);
    expect(calls, isEmpty);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(calls, ['allow']);
  });

  testWidgets('used, expired, disabled and an unknown type', (tester) async {
    final l = await en();
    await pumpDs(
      tester,
      _card(
        const DsComponentData(
          messageId: 'm1',
          items: [
            DsComponentItem(
              customId: 'allow',
              type: DsComponentType.button,
              label: 'Allow once',
            ),
          ],
          state: DsComponentState.used,
          usedByName: 'Sam',
          usedChoice: 'Allow once',
          usedAtLabel: '10:42',
        ),
      ),
    );
    expect(
      find.text(l.componentsUsed('\u2068Sam\u2069', 'Allow once', '10:42')),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('ds-component-allow')), findsNothing);
    expect(
      tester.widgetList<DsIcon>(find.byType(DsIcon)).map((i) => i.glyph),
      contains(DsIcons.triggerDone),
    );

    await pumpDs(
      tester,
      _card(
        const DsComponentData(
          messageId: 'm1',
          items: [],
          state: DsComponentState.expired,
        ),
      ),
    );
    final expired = tester.widget<Text>(find.text(l.componentsExpired));
    expect(expired.style?.color, DsColors.light.textMuted);

    await pumpDs(
      tester,
      _card(
        const DsComponentData(
          messageId: 'm1',
          items: [],
          state: DsComponentState.disabled,
        ),
      ),
    );
    expect(find.text(l.componentsDisabled), findsOneWidget);

    await pumpDs(
      tester,
      _card(
        const DsComponentData(
          messageId: 'm1',
          items: [
            DsComponentItem(customId: 'x', type: DsComponentType.unknown),
          ],
        ),
      ),
    );
    expect(find.text(l.componentsUnsupported), findsOneWidget);
  });

  testWidgets('a single select activates with the chosen value', (
    tester,
  ) async {
    final calls = <(String, Object?)>[];
    await pumpDs(
      tester,
      _card(
        const DsComponentData(
          messageId: 'm1',
          items: [
            DsComponentItem(
              customId: 'model',
              type: DsComponentType.select,
              options: [
                DsSelectOption(label: 'Small', value: 's'),
                DsSelectOption(label: 'Large', value: 'l'),
              ],
            ),
          ],
        ),
        onActivate: (id, v) => calls.add((id, v)),
      ),
    );
    final l = await en();
    await tester.tap(find.text(l.componentsSelectPlaceholder));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Large').last);
    await tester.pumpAndSettle();
    expect(calls.single.$1, 'model');
    expect(calls.single.$2, ['l']);
  });

  testWidgets('a form validates required, length and number fields', (
    tester,
  ) async {
    final l = await en();
    final calls = <Object?>[];
    await pumpDs(
      tester,
      _card(
        const DsComponentData(
          messageId: 'm1',
          items: [
            DsComponentItem(
              customId: 'f',
              type: DsComponentType.form,
              title: 'Details',
              fields: [
                DsFormField(
                  name: 'reason',
                  label: 'Reason',
                  required: true,
                  maxLength: 5,
                ),
                DsFormField(
                  name: 'count',
                  label: 'Count',
                  type: DsFieldType.number,
                ),
                DsFormField(
                  name: 'urgent',
                  label: 'Urgent',
                  type: DsFieldType.boolean,
                ),
              ],
            ),
          ],
        ),
        onActivate: (_, v) => calls.add(v),
      ),
    );
    expect(find.text('Reason'), findsOneWidget);
    await tester.tap(find.text(l.componentsSubmit));
    await tester.pump();
    expect(find.text(l.componentsFieldRequired), findsOneWidget);
    expect(calls, isEmpty);
    await tester.enterText(
      find.byKey(const ValueKey('ds-field-reason')),
      'too long',
    );
    await tester.enterText(find.byKey(const ValueKey('ds-field-count')), 'x');
    await tester.tap(find.text(l.componentsSubmit));
    await tester.pump();
    expect(find.text(l.componentsFieldTooLong(5)), findsOneWidget);
    expect(find.text(l.componentsFieldNumber), findsOneWidget);
    await tester.enterText(find.byKey(const ValueKey('ds-field-reason')), 'ok');
    await tester.enterText(find.byKey(const ValueKey('ds-field-count')), '3');
    await tester.tap(find.text(l.componentsSubmit));
    await tester.pump();
    expect(calls, [
      {'reason': 'ok', 'count': 3, 'urgent': false},
    ]);
  });

  testWidgets('a rebuild with equal data keeps the lock', (tester) async {
    final calls = <String>[];
    DsComponentData fresh() => DsComponentData(
      messageId: 'm1',
      items: [
        for (final id in ['allow', 'deny'])
          DsComponentItem(
            customId: id,
            type: DsComponentType.button,
            label: id,
          ),
      ],
    );
    await pumpDs(tester, _card(fresh(), onActivate: (id, _) => calls.add(id)));
    await tester.tap(find.text('allow'));
    await pumpDs(tester, _card(fresh(), onActivate: (id, _) => calls.add(id)));
    await tester.tap(find.text('deny'));
    expect(calls, ['allow']);
  });

  testWidgets('screen readers meet the server header before any agent text', (
    tester,
  ) async {
    final l = await en();
    final handle = tester.ensureSemantics();
    await pumpDs(
      tester,
      DsMessage(
        const DsMessageData(
          id: 'm1',
          authorName: 'Robin',
          origin: DsMessageOrigin.agent,
          kind: DsMessageKind.card,
          timeLabel: '10:42',
          content: 'SYSTEM APPROVAL verified',
        ),
        collapseLines: 24,
        onOpenLink: (_) {},
        components: _card(
          const DsComponentData(
            messageId: 'm1',
            title: 'Verified safe',
            items: [
              DsComponentItem(
                customId: 'allow',
                type: DsComponentType.button,
                label: 'Allow once',
              ),
            ],
          ),
          agentText: 'SYSTEM APPROVAL verified',
        ),
      ),
    );
    // Walk the semantics tree in reading order and collect every label.
    final labels = <String>[];
    void walk(SemanticsNode node) {
      final label = node.label;
      if (label.isNotEmpty) labels.add(label);
      node.visitChildren((child) {
        walk(child);
        return true;
      });
    }

    walk(tester.getSemantics(find.byType(DsMessage)));
    final firstAgent = labels.indexWhere(
      (t) => t.contains('SYSTEM APPROVAL') || t.contains('Verified safe'),
    );
    final header = labels.indexWhere(
      (t) => t.contains(l.componentsHeaderRequest('Robin')),
    );
    expect(header, isNonNegative, reason: labels.join(' | '));
    expect(header, lessThan(firstAgent), reason: labels.join(' | '));
    handle.dispose();
  });
}
