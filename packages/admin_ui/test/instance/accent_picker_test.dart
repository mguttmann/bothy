// SPDX-License-Identifier: Apache-2.0

// The accent picker (PLAN row of M6-09; R-PR-08; brief M6-09 Design note 2): no input of the
// branding screen accepts a color value, the picker offers exactly the seven palette keys, and
// a selection sends only a palette key to `updateBranding`; the preview renders the accent
// roles of the chosen key in light and dark.

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository r;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    r = instanceRepository(navigator)..stepUpFresh = true;
  });

  testWidgets('exactly the seven palette keys and no color input', (
    tester,
  ) async {
    await pumpInstance(tester, BrandingSection(repository: r), navigator);
    final picker = find.byKey(const ValueKey('accent.picker'));
    expect(picker, findsOneWidget);
    expect(
      find.descendant(of: picker, matching: find.byType(Radio<String>)),
      findsNWidgets(7),
    );
    for (final key in accentPaletteKeys) {
      expect(find.byKey(ValueKey('accent.$key')), findsOneWidget);
    }
    // The only text inputs of the screen are the product name; nothing takes a color.
    expect(find.byType(EditableText), findsOneWidget);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('branding.name')),
        matching: find.byType(EditableText),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(of: picker, matching: find.byType(EditableText)),
      findsNothing,
    );
  });

  testWidgets('a selection sends only the palette key', (tester) async {
    await pumpInstance(tester, BrandingSection(repository: r), navigator);
    await tester.tap(find.byKey(const ValueKey('accent.loch')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('branding.save')));
    await tester.pumpAndSettle();
    await confirmChange(tester);

    expect(r.calls.last, 'updateBranding Team Chat loch');
    expect(r.accentColor, 'loch');
    expect(accentPaletteKeys, contains(r.accentColor));
  });

  testWidgets('the preview uses the accent roles in light and dark', (
    tester,
  ) async {
    await pumpInstance(
      tester,
      const AccentPreview(accent: 'heather', productName: 'Team Chat'),
      navigator,
    );
    for (final (mode, brightness) in [
      ('light', Brightness.light),
      ('dark', Brightness.dark),
    ]) {
      final card = find.byKey(ValueKey('accent.preview.$mode'));
      expect(card, findsOneWidget);
      final theme = tester.widget<Theme>(card).data;
      expect(theme.brightness, brightness);
      expect(theme.extension<DsAccent>(), DsAccent.of('heather', brightness));
      final button = find.descendant(
        of: card,
        matching: find.byType(FilledButton),
      );
      expect(
        Theme.of(tester.element(button)).colorScheme.primary,
        DsAccent.of('heather', brightness).accentBg,
      );
    }
    expect(find.text('Team Chat'), findsNWidgets(2));
  });

  testWidgets('a stored accent that is no palette key selects nothing', (
    tester,
  ) async {
    r.accentColor = 'granite';
    await pumpInstance(tester, BrandingSection(repository: r), navigator);
    final radios = tester.widgetList<RadioGroup<String>>(
      find.byType(RadioGroup<String>),
    );
    expect(radios.single.groupValue, 'granite');
    expect(
      tester
          .widgetList<Radio<String>>(find.byType(Radio<String>))
          .map((w) => w.value),
      isNot(contains('granite')),
    );
  });
}
