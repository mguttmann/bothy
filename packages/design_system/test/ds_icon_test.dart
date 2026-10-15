// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter_test/flutter_test.dart';

// Test strings only; interface texts come from packages/l10n (M4-06).
const _label = 'Working';

DsContext _ctx(DsMotionMode motion) => DsContext(
  theme: DsThemeMode.light,
  density: DsDensityMode.comfortable,
  motion: motion,
  accent: dsDefaultAccent,
);

Future<void> _pump(
  WidgetTester tester,
  Widget icon, {
  DsMotionMode motion = DsMotionMode.full,
}) async {
  await tester.pumpWidget(
    Directionality(
      textDirection: TextDirection.ltr,
      child: DsThemeScope(
        dsContext: _ctx(motion),
        child: Center(child: icon),
      ),
    ),
  );
}

Finder _rotation() => find.descendant(
  of: find.byType(DsIcon),
  matching: find.byType(RotationTransition),
);

void main() {
  test('an icon without label and not decorative throws', () {
    expect(() => DsIcon(DsIcons.triggerWorking), throwsArgumentError);
    expect(
      () => DsIcon(DsIcons.triggerWorking, label: ''),
      throwsArgumentError,
    );
  });

  testWidgets('a decorative icon is excluded from semantics', (tester) async {
    final handle = tester.ensureSemantics();
    await _pump(tester, DsIcon(DsIcons.actionAttach, decorative: true));
    expect(
      tester.getSemantics(find.byType(DsIcon)),
      isNot(isSemantics(isImage: true)),
    );
    expect(find.bySemanticsLabel(_label), findsNothing);
    handle.dispose();
  });

  testWidgets('a labeled icon has the label as an image', (tester) async {
    final handle = tester.ensureSemantics();
    await _pump(tester, DsIcon(DsIcons.triggerWorking, label: _label));
    expect(
      tester.getSemantics(find.bySemanticsLabel(_label)),
      isSemantics(label: _label, isImage: true),
    );
    handle.dispose();
  });

  testWidgets('trigger.working spins in full motion', (tester) async {
    await _pump(tester, DsIcon(DsIcons.triggerWorking, label: _label));
    final first = tester.widget<RotationTransition>(_rotation()).turns.value;
    await tester.pump(DsMotion.full.durationPulse ~/ 4);
    final second = tester.widget<RotationTransition>(_rotation()).turns.value;
    expect(second, isNot(first));
    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('the three spinning glyphs spin, and only they', (tester) async {
    for (final MapEntry(key: name, value: glyph) in dsSemanticIcons.entries) {
      await _pump(tester, DsIcon(glyph, label: _label));
      expect(
        _rotation(),
        glyph.spins ? findsOneWidget : findsNothing,
        reason: name,
      );
    }
    expect(
      {
        for (final e in dsSemanticIcons.entries)
          if (e.value.spins) e.key,
      },
      {'trigger.working', 'run.running', 'state.reconnecting'},
    );
    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('trigger.working stands still in reduced motion', (tester) async {
    await _pump(
      tester,
      DsIcon(DsIcons.triggerWorking, label: _label),
      motion: DsMotionMode.reduced,
    );
    expect(_rotation(), findsNothing);
    expect(SchedulerBinding.instance.transientCallbackCount, 0);
  });

  testWidgets('switching to reduced motion stops the spin', (tester) async {
    await _pump(tester, DsIcon(DsIcons.runRunning, label: _label));
    expect(_rotation(), findsOneWidget);
    await _pump(
      tester,
      DsIcon(DsIcons.runRunning, label: _label),
      motion: DsMotionMode.reduced,
    );
    await tester.pump();
    expect(_rotation(), findsNothing);
    expect(SchedulerBinding.instance.transientCallbackCount, 0);
  });

  testWidgets('sizes sm, md and lg equal icon.size.*', (tester) async {
    for (final (role, value) in [
      (DsIconSizeRole.sm, DsIconSize.sizeSm),
      (DsIconSizeRole.md, DsIconSize.sizeMd),
      (DsIconSizeRole.lg, DsIconSize.sizeLg),
    ]) {
      expect(role.size, value);
      await _pump(
        tester,
        DsIcon(DsIcons.actionSend, size: role, label: _label),
      );
      expect(tester.getSize(find.byType(DsIcon)), Size(value, value));
    }
  });

  testWidgets('the color defaults to color.text.primary', (tester) async {
    await _pump(tester, DsIcon(DsIcons.actionSend, label: _label));
    expect(
      tester.state<DsIconState>(find.byType(DsIcon)).effectiveColor,
      DsColors.light.textPrimary,
    );
  });

  testWidgets('filled selects the fill asset', (tester) async {
    await _pump(
      tester,
      DsIcon(DsIcons.actionAttach, label: _label, filled: true),
    );
    expect(
      tester.state<DsIconState>(find.byType(DsIcon)).asset,
      'assets/icons/fill/${DsIcons.actionAttach.name}.vec',
    );
  });
}
