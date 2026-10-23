// SPDX-License-Identifier: Apache-2.0

// The place "Instance and branding" (brief M6-09 Scope): its path, its entry in the shell for
// owners, and the four parts of the screen.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  test('the place has a path and no detail', () {
    expect(
      AdminLocation.parse(['instance']),
      const AdminLocation(AdminSection.instance),
    );
    expect(AdminLocation.parse(['instance', 'x']), isNull);
    expect(const AdminLocation(AdminSection.instance).path, '/instance');
    expect(AdminSection.listed, contains(AdminSection.instance));
  });

  testWidgets('the tabs show settings, branding, providers and flags', (
    tester,
  ) async {
    final navigator = GlobalKey<NavigatorState>();
    final r = instanceRepository(navigator);
    await pumpInstance(
      tester,
      AdminBody(
        repository: r,
        location: const AdminLocation(AdminSection.instance),
        onNavigate: (_) {},
        maxWidth: null,
      ),
      navigator,
    );
    expect(find.text('Instance and branding'), findsOneWidget);
    expect(find.byKey(const ValueKey('settings.list')), findsOneWidget);
    for (final (tab, key) in [
      ('branding', 'branding.name'),
      ('providers', 'providers.list'),
      ('flags', 'flags.list'),
      ('settings', 'settings.list'),
    ]) {
      await tester.tap(find.byKey(ValueKey('instance.tab.$tab')));
      await tester.pumpAndSettle();
      expect(find.byKey(ValueKey(key)), findsOneWidget, reason: tab);
    }
  });

  testWidgets('the shell lists the instance for owners only', (tester) async {
    final navigator = GlobalKey<NavigatorState>();
    final r = instanceRepository(navigator);
    await pumpInstance(
      tester,
      SizedBox(
        height: 900,
        child: AdminShell(
          repository: r,
          location: AdminLocation.start,
          onNavigate: (_) {},
        ),
      ),
      navigator,
    );
    expect(find.text('Instance and branding'), findsOneWidget);

    r.callerOwner = false;
    await pumpInstance(
      tester,
      SizedBox(
        height: 900,
        child: AdminShell(
          key: UniqueKey(),
          repository: r,
          location: AdminLocation.start,
          onNavigate: (_) {},
        ),
      ),
      navigator,
    );
    expect(find.text('Instance and branding'), findsNothing);
  });
}
