// SPDX-License-Identifier: Apache-2.0

// The places "Permission hints" and "Expiring soon" of the administration (brief M6-07 Scope:
// the desktop administration navigation; 07 9.4, 9.5): their sections, paths and labels, the
// entries of the shell's navigation, and the body that shows their screens.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../admin/support.dart';

void main() {
  test('the sections follow the access grants and parse without an ID', () {
    final listed = AdminSection.listed;
    final grants = listed.indexOf(AdminSection.grants);
    expect(listed.sublist(grants, grants + 3), [
      AdminSection.grants,
      AdminSection.hints,
      AdminSection.expiring,
    ]);
    expect(
      AdminLocation.parse(['hints']),
      const AdminLocation(AdminSection.hints),
    );
    expect(
      AdminLocation.parse(['expiring']),
      const AdminLocation(AdminSection.expiring),
    );
    expect(const AdminLocation(AdminSection.hints).path, '/hints');
    expect(const AdminLocation(AdminSection.expiring).path, '/expiring');
    // The filters live in the screen, not in the path.
    expect(AdminLocation.parse(['hints', 'x']), isNull);
    expect(AdminLocation.parse(['expiring', 'x']), isNull);
  });

  test('the labels come from l10n in both languages', () async {
    final en = await AppLocalizations.delegate.load(const Locale('en'));
    final de = await AppLocalizations.delegate.load(const Locale('de'));
    expect(adminSectionLabel(en, AdminSection.hints), 'Permission hints');
    expect(adminSectionLabel(en, AdminSection.expiring), 'Expiring soon');
    expect(adminSectionLabel(de, AdminSection.hints), de.adminNavHints);
    expect(adminSectionLabel(de, AdminSection.expiring), de.adminNavExpiring);
  });

  testWidgets('the shell lists both entries and opens their places', (
    tester,
  ) async {
    final navigator = GlobalKey<NavigatorState>();
    final repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    final opened = <AdminLocation>[];
    await pumpInApp(
      tester,
      SizedBox(
        height: 900,
        child: AdminShell(
          repository: repository,
          location: AdminLocation.start,
          onNavigate: opened.add,
          clock: testClock,
        ),
      ),
      navigatorKey: navigator,
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('admin.nav.hints')));
    await tester.tap(find.byKey(const ValueKey('admin.nav.expiring')));

    expect(find.text('Permission hints'), findsOneWidget);
    expect(find.text('Expiring soon'), findsOneWidget);
    expect(opened, const [
      AdminLocation(AdminSection.hints),
      AdminLocation(AdminSection.expiring),
    ]);
  });
}
