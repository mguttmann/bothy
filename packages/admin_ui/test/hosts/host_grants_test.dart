// SPDX-License-Identifier: Apache-2.0

// Grants on a host (brief M4-37 Tests, Design note 4; 07 2.12, SR10, SR12, 9.3): the wizard
// on a host target offers no level and exactly the capability `terminal`, sends no level, and
// shows the server's preview text with a "Not effective" badge for each hint
// `terminal_not_effective`; the client adds no cause of its own. The host page lists the
// grants on the host with the same badge.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

/// The server's texts of the fake (in a real answer they come from templates in `authz`).
const String _notAssignee =
    'Sam gets the capability "Terminal" on the host Build host.\n'
    'Not effective: Sam is not the assignee of this host (SR10).';
const String _noSessionUser =
    'Robin gets the capability "Terminal" on the host Build host.\n'
    'Not effective: no session user (SR12).';
const String _effective =
    'Robin gets the capability "Terminal" on the host Build host.';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = hostRepository(navigator);
    repository.hostsById['build'] = fakeHost(
      'build',
      name: 'Build host',
      assignedUserId: 'robin',
    );
    repository.previewAnswer = (r) {
      final robinReady = repository.hostsById['build']!.sessionUsers.any(
        (s) => s.userId == 'robin',
      );
      final notEffective =
          r.subjectId == 'sam' || (r.subjectId == 'robin' && !robinReady);
      return GrantPreviewResult(
        allowed: true,
        effects: const [],
        violations: const [],
        hints: [
          if (notEffective)
            GrantHint(code: 'terminal_not_effective', principalId: r.subjectId),
        ],
        text: r.subjectId == 'sam'
            ? _notAssignee
            : robinReady
            ? _effective
            : _noSessionUser,
        textLocale: 'en',
      );
    };
  });

  Future<void> wizardOnHost(WidgetTester tester) => pumpInApp(
    tester,
    GrantWizard(
      repository: repository,
      target: repository.hostsById['build']!.target,
      clock: testClock,
      debounce: Duration.zero,
    ),
    navigatorKey: navigator,
  );

  testWidgets(
    'a non-assignee: the server says not effective (SR10), with the badge',
    (tester) async {
      await wizardOnHost(tester);
      await tapKey(tester, 'subject.sam');
      await tapKey(tester, 'wizard.next');
      expect(find.byKey(const ValueKey('target.host.build')), findsOneWidget);
      await tapKey(tester, 'wizard.next');

      // No level on a host; the capability is exactly `terminal` and cannot be turned off.
      expect(find.byKey(const ValueKey('level.read')), findsNothing);
      expect(find.byKey(const ValueKey('capability.upload')), findsNothing);
      final terminal = tester.widget<FilterChip>(
        find.byKey(const ValueKey('capability.terminal')),
      );
      expect(terminal.selected, isTrue);
      expect(terminal.onSelected, isNull);

      final request = repository.previews.last;
      expect(request.target.kind, AdminTargetKind.host);
      expect(request.level, AccessLevel.none);
      expect(request.capabilities, {AccessCapability.terminal});

      expect(
        tester.widget<Text>(find.byKey(const ValueKey('preview.text'))).data,
        _notAssignee,
      );
      expect(
        find.byKey(const ValueKey('preview.notEffective')),
        findsOneWidget,
      );
      // The cause is the server's sentence alone: no text of the client names SR10.
      expect(find.textContaining('not the assignee'), findsOneWidget);
    },
  );

  testWidgets(
    'a person without a session user: not effective: no session user (SR12)',
    (tester) async {
      await wizardOnHost(tester);
      await tapKey(tester, 'subject.robin');
      await tapKey(tester, 'wizard.next');
      await tapKey(tester, 'wizard.next');

      expect(
        tester.widget<Text>(find.byKey(const ValueKey('preview.text'))).data,
        _noSessionUser,
      );
      expect(
        find.byKey(const ValueKey('preview.notEffective')),
        findsOneWidget,
      );
      expect(find.textContaining('no session user'), findsOneWidget);
    },
  );

  testWidgets('the host page lists its grants with the preview state', (
    tester,
  ) async {
    for (final (id, who) in [('g-sam', 'sam'), ('g-robin', 'robin')]) {
      repository.grantList.add(
        AdminGrant(
          id: id,
          subjectKind: AdminSubjectKind.user,
          subjectId: who,
          targetKind: AdminTargetKind.host,
          targetId: 'build',
          capabilities: const [AccessCapability.terminal],
          reason: 'Builds',
          grantedAt: DateTime.utc(2026, 10, 10),
        ),
      );
    }
    repository.hostsById['build'] = changedHost(
      repository.hostsById['build']!,
      reported: const [HostReportedName('term-robin')],
      sessionUsers: [
        HostSessionUserInfo(
          userId: 'robin',
          osUser: 'term-robin',
          createdAt: DateTime.utc(2026, 10, 2),
        ),
      ],
    );
    await pumpInApp(
      tester,
      HostDetail(repository: repository, hostId: 'build', clock: testClock),
      navigatorKey: navigator,
    );
    expect(textOf(tester, 'hostGrant.g-sam'), contains('Sam'));
    expect(
      find.byKey(const ValueKey('hostGrant.g-sam.notEffective')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('hostGrant.g-robin.notEffective')),
      findsNothing,
    );
    expect(textOf(tester, 'hostGrant.g-sam'), contains('not the assignee'));
    // "Grant terminal" opens the wizard on this host.
    await tapKey(tester, 'hostGrants.add');
    expect(find.byKey(const ValueKey('wizard')), findsOneWidget);
  });
}
