// SPDX-License-Identifier: Apache-2.0

// The body of the administration without its navigation (brief M6-04 Scope: layouts at full
// browser width): the screen of each place, 880 logical pixels wide by default and the full
// width without a maximum, and the view-as hook threaded to the screens that start a view.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
  });

  Future<void> pump(
    WidgetTester tester,
    AdminLocation location, {
    double? maxWidth = 880,
    AdminViewAs? onViewAs,
    ValueChanged<AdminLocation>? onNavigate,
    VoidCallback? onChanged,
  }) => pumpInApp(
    tester,
    SizedBox(
      height: 900,
      child: AdminBody(
        repository: repository,
        location: location,
        onNavigate: onNavigate ?? (_) {},
        onChanged: onChanged,
        timeZone: 'Europe/Berlin',
        clock: testClock,
        maxWidth: maxWidth,
        onViewAs: onViewAs,
      ),
    ),
    navigatorKey: navigator,
  );

  double bodyWidth(WidgetTester tester, AdminLocation at) =>
      tester.getSize(find.byKey(ValueKey('admin.body.${at.path}'))).width;

  testWidgets('the content is 880 wide by default', (tester) async {
    const at = AdminLocation(AdminSection.groups);
    await pumpInApp(
      tester,
      SizedBox(
        height: 900,
        child: AdminBody(
          repository: repository,
          location: at,
          onNavigate: (_) {},
        ),
      ),
      navigatorKey: navigator,
    );
    await tester.pumpAndSettle();

    expect(find.byType(GroupsList), findsOneWidget);
    expect(bodyWidth(tester, at), 880);
  });

  testWidgets('without a maximum the content fills the width', (tester) async {
    const at = AdminLocation(AdminSection.groups);
    await pump(tester, at, maxWidth: null);
    await tester.pumpAndSettle();

    // The test app is 1200 wide with a padding of 24, the body pads by 24 again.
    expect(bodyWidth(tester, at), 1200 - 2 * 24 - 2 * 24);
  });

  testWidgets('a wider maximum than the room still fits the room', (
    tester,
  ) async {
    const at = AdminLocation.start;
    await pump(tester, at, maxWidth: 4000);
    await tester.pumpAndSettle();

    expect(bodyWidth(tester, at), lessThanOrEqualTo(1200 - 4 * 24));
  });

  final screens = <AdminLocation, Type>{
    AdminLocation.start: PeopleList,
    const AdminLocation(AdminSection.people, id: 'sam'): PersonDetail,
    const AdminLocation(AdminSection.groups): GroupsList,
    const AdminLocation(AdminSection.groups, id: 'support'): GroupDetail,
    const AdminLocation(AdminSection.grants): GrantsList,
    const AdminLocation(AdminSection.devices): WaitingDevicesView,
    const AdminLocation(AdminSection.agents): AgentsList,
    const AdminLocation(AdminSection.agents, id: 'support'): AgentPage,
    const AdminLocation(AdminSection.audit): AuditLogView,
    const AdminLocation(AdminSection.hosts): HostsList,
    const AdminLocation(AdminSection.hosts, id: 'h1'): HostDetail,
  };
  for (final MapEntry(key: at, value: screen) in screens.entries) {
    testWidgets('${at.path} shows $screen', (tester) async {
      await pump(tester, at, maxWidth: null);
      await tester.pump();

      expect(find.byType(screen), findsOneWidget);
      expect(find.byKey(ValueKey('admin.body.${at.path}')), findsOneWidget);
      // The body never brings the navigation of the shell.
      expect(find.byKey(const ValueKey('admin.nav.people')), findsNothing);
    });
  }

  testWidgets('the audit list gets the height of the body', (tester) async {
    const at = AdminLocation(AdminSection.audit);
    await pump(tester, at);
    await tester.pump();

    // The virtualized list scrolls itself, so no scroll view of the body wraps it.
    expect(
      find.ancestor(
        of: find.byType(AuditLogView),
        matching: find.byType(SingleChildScrollView),
      ),
      findsOneWidget, // only the one of the test app
    );
    expect(bodyWidth(tester, at), 1200 - 4 * 24);
  });

  testWidgets('a person detail opens the list again', (tester) async {
    final places = <AdminLocation>[];
    await pump(
      tester,
      const AdminLocation(AdminSection.people, id: 'sam'),
      onNavigate: places.add,
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('person.back')));
    expect(places, [AdminLocation.start]);
  });

  testWidgets('a person detail offers the view with the hook', (tester) async {
    final viewed = <String>[];
    await pump(
      tester,
      const AdminLocation(AdminSection.people, id: 'sam'),
      onViewAs: (p) => viewed.add(p.id),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('person.viewAs')));
    expect(viewed, ['sam']);
  });

  testWidgets('a person detail without the hook offers no view', (
    tester,
  ) async {
    await pump(tester, const AdminLocation(AdminSection.people, id: 'sam'));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('person.viewAs')), findsNothing);
  });

  testWidgets('"Who has access?" of a target hands the person to the hook', (
    tester,
  ) async {
    final viewed = <String>[];
    await pump(
      tester,
      AdminLocation(AdminSection.grants, id: campaigns.key),
      onViewAs: (p) => viewed.add(p.id),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('grants.access')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('access.viewAs')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('access.viewAs.sam')).last);
    await tester.pumpAndSettle();

    expect(viewed, ['sam']);
    expect(find.byType(ViewAsPerson), findsNothing);
    expect(repository.calls, isNot(contains('viewAs sam')));
  });
}
