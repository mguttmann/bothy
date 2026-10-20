// SPDX-License-Identifier: Apache-2.0

// The check results of a host (brief M4-37 Tests, Design note 2; 24 2.2 rule for names,
// TM-T21, TM-T24): every entry as the server returns it; a named session user with the person
// of its valid assignment from `session_users`, a former one with the person of its ended
// assignment from the audit of the host (one step-up of `audit.read`), a name without any
// known assignment alone with a note; also while the host is disconnected.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = hostRepository(navigator);
    repository.hostsById['build'] = fakeHost(
      'build',
      name: 'Build host',
      connected: false,
      reported: const [HostReportedName('term-sam')],
      sessionUsers: [
        HostSessionUserInfo(
          userId: 'sam',
          osUser: 'term-sam',
          createdAt: DateTime.utc(2026, 10, 2),
        ),
      ],
      doctor: const [
        HostDoctorEntry(check: 'hd1_user', ok: true),
        HostDoctorEntry(
          check: 'hd6_llm_login',
          ok: false,
          code: 'subscription_login_detected',
          sessionUser: 'term-sam',
        ),
        HostDoctorEntry(
          check: 'hd9_foreign_homes',
          ok: false,
          code: 'foreign_home_listable',
          sessionUser: 'term-sam',
        ),
        HostDoctorEntry(
          check: 'hd9_foreign_homes',
          ok: false,
          code: 'foreign_home_listable',
          sessionUser: 'term-robin',
        ),
        HostDoctorEntry(
          check: 'hd9_foreign_homes',
          ok: false,
          code: 'foreign_home_listable',
        ),
        HostDoctorEntry(
          check: 'hd10_helper_integrity',
          ok: false,
          code: 'wrong_peer',
          sessionUser: 'term-sam',
        ),
        HostDoctorEntry(
          check: 'hd11_shared_locations',
          ok: false,
          code: 'shared_location_writable',
          sessionUser: 'term-gone',
        ),
      ],
    );
    // Robin's assignment to term-robin ended; only the audit of the host knows it.
    repository.hostAudit['build'] = {'term-robin': 'robin'};
  });

  Future<void> page(WidgetTester tester) => pumpInApp(
    tester,
    HostDetail(repository: repository, hostId: 'build', clock: testClock),
    navigatorKey: navigator,
  );

  testWidgets(
    'HD9 names a current and a former session user with their people',
    (tester) async {
      await page(tester);
      // One step-up for the audit of the host, asked because term-robin has no valid
      // assignment.
      await confirmStepUp(tester);
      expect(repository.calls, contains('assignmentHistory build'));

      final current = textOf(tester, 'doctor.hd9_foreign_homes.term-sam');
      expect(current, contains('HD9'));
      expect(current, contains('foreign_home_listable'));
      expect(current, contains('term-sam'));
      expect(current, contains('Sam'));

      final former = textOf(tester, 'doctor.hd9_foreign_homes.term-robin');
      expect(former, contains('term-robin'));
      expect(former, contains('formerly assigned to Robin'));

      expect(textOf(tester, 'doctor.hd9_foreign_homes.-'), contains('HD9'));
    },
  );

  testWidgets('HD6, HD10 and HD11 entries with names, on a disconnected host', (
    tester,
  ) async {
    await page(tester);
    await confirmStepUp(tester);
    expect(textOf(tester, 'host.connection'), contains('disconnected'));
    expect(textOf(tester, 'doctor.hd1_user.-'), contains('HD1 ok'));
    expect(
      textOf(tester, 'doctor.hd6_llm_login.term-sam'),
      allOf(
        contains('HD6'),
        contains('subscription_login_detected'),
        contains('Sam'),
      ),
    );
    expect(
      textOf(tester, 'doctor.hd10_helper_integrity.term-sam'),
      allOf(contains('HD10'), contains('wrong_peer'), contains('Sam')),
    );
    final gone = textOf(tester, 'doctor.hd11_shared_locations.term-gone');
    expect(gone, contains('HD11'));
    expect(gone, contains('term-gone'));
    expect(gone, contains('no assignment known'));
  });

  testWidgets('without a name lacking an assignment the audit is not read', (
    tester,
  ) async {
    repository.hostsById['build'] = changedHost(
      repository.hostsById['build']!,
      doctor: const [
        HostDoctorEntry(
          check: 'hd9_foreign_homes',
          ok: false,
          code: 'foreign_home_listable',
          sessionUser: 'term-sam',
        ),
      ],
    );
    await page(tester);
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsNothing);
    expect(repository.calls, isNot(contains('assignmentHistory build')));
    expect(
      textOf(tester, 'doctor.hd9_foreign_homes.term-sam'),
      contains('Sam'),
    );
  });
}
