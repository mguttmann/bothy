// SPDX-License-Identifier: Apache-2.0

// The feature flags (brief M6-09 Design note 4, Tests): key, description, the state evaluated
// for the signed-in person and the stored value; the switch needs a reason, states the change,
// goes through the step-up and keeps the rollout; a 412 shows the newer state and changes
// nothing; an orphaned flag cannot be switched; `instance.updated` loads the list again.

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
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
        FlagsSection(
          repository: r,
          timeZone: 'Europe/Berlin',
          clock: testClock,
          changes: changes,
        ),
        navigator,
      );

  Future<void> openSwitch(WidgetTester tester, String key) async {
    final s = find.byKey(ValueKey('flag.$key.switch'));
    await tester.ensureVisible(s);
    await tester.tap(s);
    await tester.pumpAndSettle();
  }

  testWidgets('each flag shows its evaluated and its stored state', (
    tester,
  ) async {
    await pump(tester);
    expect(find.text('Direct messages between people'), findsOneWidget);
    expect(textOf(tester, 'flag.dm.enabled.evaluated'), 'For you: off');
    expect(
      textOf(tester, 'flag.dm.enabled.stored'),
      'Stored: off · Default: off',
    );
    expect(textOf(tester, 'flag.dm.enabled.meta'), 'Default, never changed');
    // A rollout naming Alex inverts the global value for Alex.
    expect(
      textOf(tester, 'flag.tokens.personal_enabled.evaluated'),
      'For you: on',
    );
    expect(
      textOf(tester, 'flag.tokens.personal_enabled.stored'),
      'Stored: off · Default: off · Inverted for 1 person or group',
    );
  });

  testWidgets('a switch needs a reason, states the change and asks for '
      'step-up', (tester) async {
    await pump(tester);
    await openSwitch(tester, 'dm.enabled');
    expect(find.text('Switch on dm.enabled?'), findsOneWidget);
    expect(
      tester.widget<Text>(find.byKey(const ValueKey('flag.body'))).data,
      'From: off\nTo: on\nThe global value changes for everyone immediately.',
    );
    final submit = find.byKey(const ValueKey('flag.submit'));
    expect(tester.widget<FilledButton>(submit).onPressed, isNull);
    await tester.enterText(
      find.byKey(const ValueKey('flag.reason')),
      'Pilot with the support team',
    );
    await tester.pumpAndSettle();
    await tester.tap(submit);
    await confirmStepUp(tester, 'Switch on the feature flag dm.enabled');

    expect(
      r.calls.last,
      'putFeatureFlag dm.enabled true Pilot with the support team',
    );
    expect(r.flagsByKey['dm.enabled']!.enabled, isTrue);
    expect(find.text('dm.enabled is switched on.'), findsOneWidget);
    expect(textOf(tester, 'flag.dm.enabled.evaluated'), 'For you: on');
    expect(
      tester
          .widget<Switch>(find.byKey(const ValueKey('flag.dm.enabled.switch')))
          .value,
      isTrue,
    );
  });

  testWidgets('switching keeps the rollout', (tester) async {
    r.stepUpFresh = true;
    await pump(tester);
    await openSwitch(tester, 'tokens.personal_enabled');
    expect(
      find.textContaining('The rollout to 1 person or group stays.'),
      findsOneWidget,
    );
    await tester.enterText(find.byKey(const ValueKey('flag.reason')), 'Go');
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('flag.submit')));
    await tester.pumpAndSettle();
    expect(r.flagsByKey['tokens.personal_enabled']!.rolloutUserIds, ['alex']);
    // Global on, inverted for Alex.
    expect(
      textOf(tester, 'flag.tokens.personal_enabled.evaluated'),
      'For you: off',
    );
  });

  testWidgets('a 412 shows the newer state and changes nothing', (
    tester,
  ) async {
    r.stepUpFresh = true;
    await pump(tester);
    r.concurrent['putFeatureFlag'] = () =>
        r.otherOwnerFlags('dm.enabled', true);
    await openSwitch(tester, 'dm.enabled');
    await tester.enterText(find.byKey(const ValueKey('flag.reason')), 'Pilot');
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('flag.submit')));
    await tester.pumpAndSettle();

    final conflict = find.byKey(const ValueKey('flags.conflict'));
    expect(conflict, findsOneWidget);
    expect(
      find.descendant(
        of: conflict,
        matching: find.textContaining('it is now on. Nothing was changed'),
      ),
      findsOneWidget,
    );
    expect(textOf(tester, 'flag.dm.enabled.stored'), startsWith('Stored: on'));
    expect(r.flagsByKey['dm.enabled']!.updatedBy, 'robin');
    expect(
      textOf(tester, 'flag.dm.enabled.meta'),
      startsWith('Version 1, changed by Robin, '),
    );
  });

  testWidgets('an orphaned flag cannot be switched', (tester) async {
    r.flagsByKey['old.flag'] = const FeatureFlagEntry(
      key: 'old.flag',
      kind: FeatureFlagKind.unknown,
      orphaned: true,
      enabled: false,
      version: 3,
    );
    await pump(tester);
    final s = tester.widget<Switch>(
      find.byKey(const ValueKey('flag.old.flag.switch')),
    );
    expect(s.onChanged, isNull);
    expect(find.textContaining('No longer registered'), findsOneWidget);
  });

  testWidgets('instance.updated loads the flags again', (tester) async {
    final changes = StreamController<void>.broadcast();
    addTearDown(changes.close);
    await pump(tester, changes: () => changes.stream);
    r.otherOwnerFlags('voice.enabled', true);
    changes.add(null);
    await tester.pumpAndSettle();
    expect(textOf(tester, 'flag.voice.enabled.evaluated'), 'For you: on');
  });
}
