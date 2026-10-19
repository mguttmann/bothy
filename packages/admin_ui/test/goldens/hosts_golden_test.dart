// SPDX-License-Identifier: Apache-2.0

// Goldens of screen A8 (brief M4-37 Tests): the host page, the pairing steps with the code and
// with the confirmation, and the check results with current, former and unknown session users;
// light and dark, two densities, text scaling 1 and 2, German and English. One file per view
// keeps each file below repo.max_file_kb.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../admin/support.dart';

Widget _frame(Widget child) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 760,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

const List<HostDoctorEntry> _doctor = [
  HostDoctorEntry(check: 'hd1_user', ok: true),
  HostDoctorEntry(check: 'hd7_profiles', ok: true),
  HostDoctorEntry(
    check: 'hd9_foreign_homes',
    ok: false,
    code: 'foreign_home_listable',
    sessionUser: 'term-robin',
  ),
  HostDoctorEntry(
    check: 'hd11_shared_locations',
    ok: false,
    code: 'shared_location_writable',
    sessionUser: 'term-alex',
  ),
  HostDoctorEntry(
    check: 'hd10_helper_integrity',
    ok: false,
    code: 'wrong_peer',
    sessionUser: 'term-gone',
  ),
];

FakeAdminRepository _repository() {
  final r = FakeAdminRepository(prompt: StepUpDialogPrompt(() => null))
    ..stepUpFresh = true;
  r.hostsById['build'] = fakeHost(
    'build',
    name: 'Build host',
    assignedUserId: 'robin',
    keyGeneration: 2,
    doctor: _doctor,
    reported: const [
      HostReportedName('term-alex', workdirs: ['server', 'apps']),
      HostReportedName('term-sam'),
    ],
    sessionUsers: [
      HostSessionUserInfo(
        userId: 'alex',
        osUser: 'term-alex',
        createdAt: DateTime.utc(2026, 10, 2),
      ),
    ],
  );
  r.hostsById['pairing'] = fakeHost(
    'pairing',
    name: 'Build host',
    state: HostLifecycle.pairing,
    keyFingerprint: null,
  );
  r.hostsById['confirming'] = fakeHost(
    'confirming',
    name: 'Build host',
    state: HostLifecycle.confirming,
    reported: const [
      HostReportedName('term-alex'),
      HostReportedName('term-sam'),
    ],
  );
  r.hostAudit['build'] = {'term-robin': 'robin'};
  r.sessionCounts['build'] = 2;
  r.grantList.add(
    AdminGrant(
      id: 'g-sam',
      subjectKind: AdminSubjectKind.user,
      subjectId: 'sam',
      targetKind: AdminTargetKind.host,
      targetId: 'build',
      capabilities: const [AccessCapability.terminal],
      reason: 'Builds',
      grantedAt: DateTime.utc(2026, 10, 10),
    ),
  );
  r.previewAnswer = (request) => GrantPreviewResult(
    allowed: true,
    effects: const [],
    violations: const [],
    hints: [
      GrantHint(code: 'terminal_not_effective', principalId: request.subjectId),
    ],
    text: 'Not effective: Sam is not the assignee of this host (SR10).',
    textLocale: 'en',
  );
  return r;
}

final Map<String, Widget Function()> _views = {
  'host_page': () => _frame(
    HostDetail(repository: _repository(), hostId: 'build', clock: testClock),
  ),
  'pairing_code': () {
    final r = _repository();
    return _frame(
      PairingWizard(
        repository: r,
        host: r.hostsById['pairing'],
        code: HostPairingCodeInfo(
          hostId: 'pairing',
          code: 'bothy_hpc_EXAMPLE000000000000000000000000000000000000_EXAMPL',
          expiresAt: DateTime.utc(2026, 10, 17, 9, 14, 12),
        ),
        clock: testClock,
      ),
    );
  },
  'pairing_confirm': () {
    final r = _repository();
    return _frame(
      PairingWizard(
        repository: r,
        host: r.hostsById['confirming'],
        clock: testClock,
      ),
    );
  },
  'doctor_results': () {
    final r = _repository();
    return _frame(
      DoctorResults(
        host: r.hostsById['build']!,
        people: r.peopleById.values.toList(),
        former: const {'term-robin': 'robin'},
      ),
    );
  },
};

void main() {
  for (final MapEntry(key: name, value: view) in _views.entries) {
    goldenTest(
      '$name in light and dark, two densities, text scale 1 and 2, de and en',
      fileName: 'hosts_$name',
      pumpBeforeTest: dsPumpIcons,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        densities: const [DsDensityMode.comfortable, DsDensityMode.compact],
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
