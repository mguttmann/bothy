// SPDX-License-Identifier: Apache-2.0

// A9 in the browser (brief M6-09 Tests): the owner changes an instance setting with step-up and
// sees it after a reload, sets the accent and sees the theme of the console change, and switches
// a feature flag that getInstance then reports.

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

/// The accent of the theme the instance screen is drawn with.
DsAccent _themeAccent(PatrolIntegrationTester $) =>
    Theme.of($.tester.element(find.byType(InstanceView)))
        .extension<DsAccent>()!;

void main() {
  patrolTest('A9 change a setting with step-up and see it after reload', (
    $,
  ) async {
    await openConsole($, '/instance');
    await tapKey($, 'setting.locale.default.edit');
    await tapKey($, 'setting.input');
    await tapOn($, find.text('en'));
    await tapKey($, 'setting.save');
    await tapKey($, 'confirm.submit');
    await waitFor($, find.text('locale.default is now en.'));
    expectStepUp();

    await openConsole($, '/instance');
    await waitFor($, byKey('setting.locale.default.value'));
    expect(
      $.tester.widget<Text>(byKey('setting.locale.default.value')).data,
      'en',
    );
  });

  patrolTest('A9 set the accent and see the console theme change', ($) async {
    await openConsole($, '/instance');
    await tapKey($, 'instance.tab.branding');
    await waitFor($, byKey('accent.picker'));
    final brightness = Theme.of($.tester.element(find.byType(InstanceView)))
        .brightness;
    expect(_themeAccent($), isNot(DsAccent.of('heather', brightness)));
    await tapKey($, 'accent.heather');
    await tapKey($, 'branding.save');
    await tapKey($, 'confirm.submit');
    await waitFor($, find.text('The branding is saved.'));
    expectStepUp();
    // The console reads discovery again and draws itself with the new accent.
    final end = DateTime.now().add(const Duration(seconds: 30));
    while (_themeAccent($) != DsAccent.of('heather', brightness) &&
        DateTime.now().isBefore(end)) {
      await $.tester.pump(const Duration(milliseconds: 200));
    }
    expect(_themeAccent($), DsAccent.of('heather', brightness));
    expect(controllerOf($).discovery!.branding.accentColor, 'heather');
  });

  patrolTest('A9 switch a flag and see getInstance report it', ($) async {
    final before = await consoleApi('GET', '/instance');
    final features = ((before.json! as Map)['features']! as List)
        .cast<Map<String, Object?>>();
    expect(
      features.firstWhere((f) => f['key'] == 'dm.enabled')['enabled'],
      isFalse,
    );
    await openConsole($, '/instance');
    await tapKey($, 'instance.tab.flags');
    await tapKey($, 'flag.dm.enabled.switch');
    await typeKey($, 'flag.reason', 'Direct messages for the pilot');
    await tapKey($, 'flag.submit');
    await waitFor($, find.text('dm.enabled is switched on.'));
    expectStepUp();

    final after = await consoleApi('GET', '/instance');
    expect(after.status, 200);
    final now = ((after.json! as Map)['features']! as List)
        .cast<Map<String, Object?>>();
    expect(now.firstWhere((f) => f['key'] == 'dm.enabled')['enabled'], isTrue);
  });
}
