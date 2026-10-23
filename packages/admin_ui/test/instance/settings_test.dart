// SPDX-License-Identifier: Apache-2.0

// The settings list and its typed editor (brief M6-09 Design note 1, Tests): each kind of the
// configuration reference section 6 with its value, version and who changed it, a key the
// table does not know shown read-only, branding keys pointing to the branding screen, a change
// that states "from, to" before the confirmation and goes through the step-up, the server's
// 400 at the input, and a 412 that shows the newer value next to the input and asks again.

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository r;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    r = instanceRepository(navigator);
  });

  Future<void> pump(WidgetTester tester, {Stream<void> Function()? changes}) =>
      pumpInstance(
        tester,
        InstanceSettingsSection(
          repository: r,
          timeZone: 'Europe/Berlin',
          clock: testClock,
          changes: changes,
        ),
        navigator,
      );

  testWidgets('every kind shows its value, its type and its history', (
    tester,
  ) async {
    r.otherOwnerSets('locale.default', 'en');
    r.settingsByKey['push.future_setting'] = const InstanceSettingEntry(
      key: 'push.future_setting',
      value: {'mode': 'quiet'},
      version: 4,
    );
    await pump(tester);

    expect(textOf(tester, 'setting.locale.default.value'), 'en');
    expect(find.text('Language: de, en'), findsOneWidget);
    expect(
      textOf(tester, 'setting.locale.default.meta'),
      startsWith('Version 1, changed by Robin, '),
    );
    expect(
      find.text(
        'One of the palette colors: '
        'spruce, loch, heather, rowan, amber, gorse, moss',
      ),
      findsOneWidget,
    );
    expect(find.text('Text'), findsOneWidget);
    expect(
      textOf(tester, 'setting.branding.product_name.meta'),
      'Default, never changed',
    );
    // Branding keys are changed on the branding screen only (DECISIONS M1-46/1).
    expect(
      find.byKey(const ValueKey('setting.branding.product_name.edit')),
      findsNothing,
    );
    expect(
      textOf(tester, 'setting.branding.accent_color.readOnly'),
      'Changed under Branding.',
    );
    // A key of a newer server: its value read-only, with the note to use the API.
    expect(
      textOf(tester, 'setting.push.future_setting.value'),
      '{"mode":"quiet"}',
    );
    expect(
      find.byKey(const ValueKey('setting.push.future_setting.edit')),
      findsNothing,
    );
    expect(
      textOf(tester, 'setting.push.future_setting.readOnly'),
      contains('through the API'),
    );
  });

  testWidgets('a change states from and to, asks for step-up and saves', (
    tester,
  ) async {
    await pump(tester);
    await tester.tap(find.byKey(const ValueKey('setting.locale.default.edit')));
    await tester.pumpAndSettle();
    final save = find.byKey(const ValueKey('setting.save'));
    expect(tester.widget<FilledButton>(save).onPressed, isNull);

    await pickOption(tester, 'setting.input', 'en');
    await tester.tap(save);
    await tester.pumpAndSettle();
    expect(
      tester.widget<Text>(find.byKey(const ValueKey('confirm.body'))).data,
      'From: de\nTo: en',
    );
    await confirmChange(tester);
    await confirmStepUp(tester, 'Change the instance setting locale.default');

    expect(find.byKey(const ValueKey('setting.editor')), findsNothing);
    expect(r.calls.last, 'putInstanceSetting locale.default en');
    expect(r.stepUps, ['Change the instance setting locale.default']);
    expect(textOf(tester, 'setting.locale.default.value'), 'en');
    expect(find.text('locale.default is now en.'), findsOneWidget);
    expect(
      textOf(tester, 'setting.locale.default.meta'),
      startsWith('Version 1, changed by Alex, '),
    );
  });

  testWidgets('a cancelled step-up changes nothing and says nothing', (
    tester,
  ) async {
    await pump(tester);
    await tester.tap(find.byKey(const ValueKey('setting.locale.default.edit')));
    await tester.pumpAndSettle();
    await pickOption(tester, 'setting.input', 'en');
    await tester.tap(find.byKey(const ValueKey('setting.save')));
    await tester.pumpAndSettle();
    await confirmChange(tester);
    expect(find.byType(StepUpDialog), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('stepUp.cancel')));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('setting.editor')), findsOneWidget);
    expect(find.byKey(const ValueKey('setting.error')), findsNothing);
    expect(r.settingsByKey['locale.default']!.value, 'de');
  });

  testWidgets("the server's 400 stands at the input", (tester) async {
    r.refuse['putInstanceSetting'] = const ApiProblem(
      status: 400,
      code: 'validation_failed',
      errors: [ProblemFieldError(pointer: '/value', code: 'invalid_enum')],
    );
    r.stepUpFresh = true;
    await pump(tester);
    await tester.tap(find.byKey(const ValueKey('setting.locale.default.edit')));
    await tester.pumpAndSettle();
    await pickOption(tester, 'setting.input', 'en');
    await tester.tap(find.byKey(const ValueKey('setting.save')));
    await tester.pumpAndSettle();
    await confirmChange(tester);

    expect(
      find.text('This value is not one of the allowed values.'),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('setting.error')), findsNothing);
    expect(find.byKey(const ValueKey('setting.editor')), findsOneWidget);
  });

  testWidgets('a 412 shows the newer value next to the input and asks again', (
    tester,
  ) async {
    r.stepUpFresh = true;
    await pump(tester);
    await tester.tap(find.byKey(const ValueKey('setting.locale.default.edit')));
    await tester.pumpAndSettle();
    await pickOption(tester, 'setting.input', 'en');
    // Robin writes the same key while Alex confirms.
    r.concurrent['putInstanceSetting'] = () =>
        r.otherOwnerSets('locale.default', 'de');
    await tester.tap(find.byKey(const ValueKey('setting.save')));
    await tester.pumpAndSettle();
    await confirmChange(tester);

    final conflict = find.byKey(const ValueKey('setting.conflict'));
    expect(conflict, findsOneWidget);
    expect(
      find.descendant(
        of: conflict,
        matching: find.textContaining('Now stored: de. Your input: en.'),
      ),
      findsOneWidget,
    );
    expect(r.settingsByKey['locale.default']!.version, 1);

    // Asked again: the next save names the newer version and wins.
    await tester.tap(find.byKey(const ValueKey('setting.save')));
    await tester.pumpAndSettle();
    await confirmChange(tester);
    expect(find.byKey(const ValueKey('setting.editor')), findsNothing);
    expect(r.settingsByKey['locale.default']!.value, 'en');
    expect(r.settingsByKey['locale.default']!.version, 2);
  });

  testWidgets('instance.updated loads the list again', (tester) async {
    final changes = StreamController<void>.broadcast();
    addTearDown(changes.close);
    await pump(tester, changes: () => changes.stream);
    expect(textOf(tester, 'setting.locale.default.value'), 'de');

    r.otherOwnerSets('locale.default', 'en');
    changes.add(null);
    await tester.pumpAndSettle();
    expect(textOf(tester, 'setting.locale.default.value'), 'en');
  });

  testWidgets('a refused list shows the reason of the server', (tester) async {
    r.refuse['instanceSettings'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'owner_only',
    );
    await pump(tester);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('setting.locale.default')), findsNothing);
  });
}
