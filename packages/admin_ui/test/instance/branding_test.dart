// SPDX-License-Identifier: Apache-2.0

// The branding screen (brief M6-09 Design note 2, Tests): the server's 400 for a too long or a
// bidi product name stands at the field (the screen counts nothing itself), a change states
// what changes and goes through the step-up, a 412 shows the newer branding next to the input
// and asks again, and the logo is uploaded, shown from the server's preview once it is checked
// and removed.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository r;
  late List<PickedFile?> picks;
  late int changed;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    r = instanceRepository(navigator);
    picks = [];
    changed = 0;
  });

  Future<void> pump(WidgetTester tester) => pumpInstance(
    tester,
    BrandingSection(
      repository: r,
      pickLogo: () async => picks.removeAt(0),
      onBrandingChanged: () => changed++,
    ),
    navigator,
  );

  Future<void> saveName(WidgetTester tester, String name) async {
    await tester.enterText(find.byKey(const ValueKey('branding.name')), name);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('branding.save')));
    await tester.pumpAndSettle();
    await confirmChange(tester);
  }

  String? fieldError(WidgetTester tester) => tester
      .widget<TextField>(find.byKey(const ValueKey('branding.name')))
      .decoration!
      .errorText;

  testWidgets('a change states what changes and asks for step-up', (
    tester,
  ) async {
    await pump(tester);
    expect(
      tester
          .widget<FilledButton>(find.byKey(const ValueKey('branding.save')))
          .onPressed,
      isNull,
    );
    await tester.enterText(
      find.byKey(const ValueKey('branding.name')),
      'Forum',
    );
    await tester.tap(find.byKey(const ValueKey('accent.moss')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('branding.save')));
    await tester.pumpAndSettle();
    expect(
      tester.widget<Text>(find.byKey(const ValueKey('confirm.body'))).data,
      'Product name: Team Chat becomes Forum\n'
      'Accent: Spruce becomes Moss\n'
      'Everyone sees the change immediately.',
    );
    await confirmChange(tester);
    await confirmStepUp(tester, 'Change the branding of the instance');

    expect(r.calls.last, 'updateBranding Forum moss');
    expect(find.text('The branding is saved.'), findsOneWidget);
    expect(changed, 1);
  });

  testWidgets("the server's 400 for a too long name stands at the field", (
    tester,
  ) async {
    r.stepUpFresh = true;
    await pump(tester);
    await saveName(tester, 'A' * (fakeProductNameMax + 1));
    expect(fieldError(tester), 'This is too long for the server.');
    expect(r.productName, 'Team Chat');
    expect(changed, 0);
  });

  testWidgets('a name with a bidi character is refused at the field', (
    tester,
  ) async {
    r.stepUpFresh = true;
    await pump(tester);
    await saveName(tester, 'Team\u202EChat');
    expect(
      fieldError(tester),
      'Contains a character that is not allowed, for example a control or '
      'text direction character.',
    );
    expect(r.productName, 'Team Chat');
  });

  testWidgets('a 412 shows the newer branding and the next save wins', (
    tester,
  ) async {
    r.stepUpFresh = true;
    await pump(tester);
    r.concurrent['updateBranding'] = () => r.otherOwnerBrands('Forum', 'rowan');
    await saveName(tester, 'Help Desk');

    final conflict = find.byKey(const ValueKey('branding.conflict'));
    expect(conflict, findsOneWidget);
    expect(
      find.descendant(
        of: conflict,
        matching: find.textContaining('now Forum with the accent Rowan'),
      ),
      findsOneWidget,
    );
    // The input stays.
    expect(
      tester
          .widget<TextField>(find.byKey(const ValueKey('branding.name')))
          .controller!
          .text,
      'Help Desk',
    );
    await tester.tap(find.byKey(const ValueKey('branding.save')));
    await tester.pumpAndSettle();
    await confirmChange(tester);
    expect(r.productName, 'Help Desk');
    expect(find.byKey(const ValueKey('branding.conflict')), findsNothing);
  });

  testWidgets('upload a logo, see it once checked, and remove it', (
    tester,
  ) async {
    r.logoReady = false;
    picks.add(PickedFile('logo.png', fakeLogoBytes));
    await pump(tester);
    expect(find.byKey(const ValueKey('branding.logo.none')), findsOneWidget);
    expect(find.byKey(const ValueKey('branding.logo.remove')), findsNothing);

    await tester.tap(find.byKey(const ValueKey('branding.logo.upload')));
    await tester.pumpAndSettle();
    expect(
      tester.widget<Text>(find.byKey(const ValueKey('confirm.body'))).data,
      contains('logo.png replaces the current logo for everyone'),
    );
    await confirmChange(tester);
    await confirmStepUp(tester, 'Upload the logo of the instance');
    expect(r.logoUploads, ['logo.png']);
    expect(find.byKey(const ValueKey('branding.logo.pending')), findsOneWidget);
    expect(changed, 1);

    // The scan passed: "Check again" shows the server's preview.
    r.logoReady = true;
    await tester.tap(find.byKey(const ValueKey('branding.logo.refresh')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('branding.logo.preview')), findsOneWidget);

    r.stepUpFresh = false;
    await tester.tap(find.byKey(const ValueKey('branding.logo.remove')));
    await tester.pumpAndSettle();
    await confirmChange(tester);
    await confirmStepUp(tester, 'Remove the logo of the instance');
    expect(r.logoFileId, isNull);
    expect(find.byKey(const ValueKey('branding.logo.none')), findsOneWidget);
    expect(changed, 2);
  });

  testWidgets('a refused logo type shows the reason of the server', (
    tester,
  ) async {
    r.stepUpFresh = true;
    picks.add(PickedFile('logo.gif', fakeLogoBytes));
    await pump(tester);
    await tester.tap(find.byKey(const ValueKey('branding.logo.upload')));
    await tester.pumpAndSettle();
    await confirmChange(tester);
    expect(r.logoFileId, isNull);
    expect(find.byKey(const ValueKey('self.notice')), findsOneWidget);
    expect(changed, 0);
  });

  testWidgets('without a file dialog there is no upload', (tester) async {
    await pumpInstance(tester, BrandingSection(repository: r), navigator);
    expect(
      tester
          .widget<OutlinedButton>(
            find.byKey(const ValueKey('branding.logo.upload')),
          )
          .onPressed,
      isNull,
    );
  });
}
