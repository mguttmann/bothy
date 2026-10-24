// SPDX-License-Identifier: Apache-2.0

// The mobile form of the card components (01 5.3 M3 and M4; brief M5-02 Design note 5; R-PR-04):
// buttons stacked at full width, a form in a sheet over the keyboard, a select in the system
// picker (CupertinoPicker on iOS, a list elsewhere), and no swipe, drag or fling activates; only
// a press on the button does (a long press released on it is a press, as on every iOS button,
// docs/decisions.d/M5-02.md D-M5-02-8).

import 'package:design_system/design_system.dart';
import 'package:flutter/cupertino.dart' show CupertinoPicker;
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

const _header = DsComponentHeader(agentName: 'Robin', runLabel: 'R-12');

const _buttons = DsComponentData(
  messageId: 'm1',
  items: [
    DsComponentItem(
      customId: 'once',
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

const _select = DsComponentData(
  messageId: 'm2',
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
);

const _multi = DsComponentData(
  messageId: 'm3',
  items: [
    DsComponentItem(
      customId: 'targets',
      type: DsComponentType.select,
      maxValues: 2,
      options: [
        DsSelectOption(label: 'Staging', value: 'st'),
        DsSelectOption(label: 'Production', value: 'pr'),
      ],
    ),
  ],
);

const _form = DsComponentData(
  messageId: 'm4',
  items: [
    DsComponentItem(
      customId: 'f',
      type: DsComponentType.form,
      title: 'Details',
      fields: [
        DsFormField(name: 'reason', label: 'Reason', required: true),
        DsFormField(name: 'urgent', label: 'Urgent', type: DsFieldType.boolean),
      ],
    ),
  ],
);

Widget _card(DsComponentData data, void Function(String, Object?) onActivate) =>
    DsMessageComponents(
      data,
      permitted: true,
      deniedReasonText: null,
      header: _header,
      agentText: 'Choose.',
      onOpenLink: (_) {},
      onActivate: onActivate,
      stacked: true,
      sheets: true,
    );

void main() {
  testWidgets('stacked buttons take the full width, one below the other', (
    tester,
  ) async {
    await pumpDs(tester, _card(_buttons, (_, _) {}), width: 390);
    final once = tester.getRect(
      find.byKey(const ValueKey('ds-component-once')),
    );
    final deny = tester.getRect(
      find.byKey(const ValueKey('ds-component-deny')),
    );
    final card = tester.getRect(find.byType(DsMessageComponents));
    expect(once.width, card.width);
    expect(deny.width, card.width);
    expect(deny.top, greaterThan(once.bottom - 1));
  });

  testWidgets('no swipe, drag or fling activates an approval', (tester) async {
    final calls = <String>[];
    await pumpDs(tester, _card(_buttons, (id, _) => calls.add(id)), width: 390);
    final once = find.byKey(const ValueKey('ds-component-once'));
    final card = find.byType(DsMessageComponents);
    await tester.drag(card, const Offset(300, 0));
    await tester.drag(card, const Offset(-300, 0));
    await tester.drag(once, const Offset(200, 0));
    await tester.drag(once, const Offset(-200, 0));
    await tester.fling(once, const Offset(0, -300), 1000);
    await tester.pumpAndSettle();
    expect(calls, isEmpty);
    await tester.tap(once);
    await tester.pump();
    expect(calls, ['once']);
  });

  testWidgets('a form opens as a sheet and closes after a valid submit', (
    tester,
  ) async {
    final l = await en();
    final calls = <Object?>[];
    await pumpDs(tester, _card(_form, (_, v) => calls.add(v)), width: 390);
    // Inline there is only the button that opens the form.
    expect(find.text('Reason'), findsNothing);
    await tester.tap(find.byKey(const ValueKey('ds-component-form-f')));
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('ds-component-form-sheet-f')),
      findsOneWidget,
    );
    await tester.tap(find.byKey(const ValueKey('ds-component-submit-f')));
    await tester.pumpAndSettle();
    expect(find.text(l.componentsFieldRequired), findsOneWidget);
    expect(calls, isEmpty);
    await tester.enterText(find.byKey(const ValueKey('ds-field-reason')), 'ok');
    await tester.tap(find.byType(Switch));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('ds-component-submit-f')));
    await tester.pumpAndSettle();
    expect(calls, [
      {'reason': 'ok', 'urgent': true},
    ]);
    expect(
      find.byKey(const ValueKey('ds-component-form-sheet-f')),
      findsNothing,
    );
  });

  testWidgets('a select on iOS opens the system picker', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    try {
      final calls = <(String, Object?)>[];
      await pumpDs(
        tester,
        _card(_select, (id, v) => calls.add((id, v))),
        width: 390,
      );
      await tester.tap(find.byKey(const ValueKey('ds-component-select-model')));
      await tester.pumpAndSettle();
      expect(find.byType(CupertinoPicker), findsOneWidget);
      expect(calls, isEmpty, reason: 'turning the wheel chooses nothing yet');
      await tester.tap(find.byKey(const ValueKey('ds-component-submit-model')));
      await tester.pumpAndSettle();
      expect(calls.single.$1, 'model');
      expect(calls.single.$2, ['s']);
      expect(find.byType(CupertinoPicker), findsNothing);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('a select elsewhere opens a list; a tap on an option sends it', (
    tester,
  ) async {
    final calls = <(String, Object?)>[];
    await pumpDs(
      tester,
      _card(_select, (id, v) => calls.add((id, v))),
      width: 390,
    );
    await tester.tap(find.byKey(const ValueKey('ds-component-select-model')));
    await tester.pumpAndSettle();
    expect(find.byType(CupertinoPicker), findsNothing);
    await tester.tap(find.byKey(const ValueKey('ds-component-option-l')));
    await tester.pumpAndSettle();
    expect(calls.single.$1, 'model');
    expect(calls.single.$2, ['l']);
  });

  testWidgets('a multiple select in the sheet sends the checked values', (
    tester,
  ) async {
    final calls = <Object?>[];
    await pumpDs(tester, _card(_multi, (_, v) => calls.add(v)), width: 390);
    await tester.tap(find.byKey(const ValueKey('ds-component-select-targets')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('ds-component-option-st')));
    await tester.tap(find.byKey(const ValueKey('ds-component-option-pr')));
    await tester.pumpAndSettle();
    expect(calls, isEmpty);
    await tester.tap(find.byKey(const ValueKey('ds-component-submit-targets')));
    await tester.pumpAndSettle();
    expect(calls, [
      ['st', 'pr'],
    ]);
  });
}
