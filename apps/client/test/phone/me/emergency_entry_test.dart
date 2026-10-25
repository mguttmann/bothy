// SPDX-License-Identifier: Apache-2.0

// The emergency entry under Me on the phone (brief M5-09 Tests; R-PR-07; 01 4, 5.3 M13): who
// the entry is for (the owner flag and a reported `manage` on a space, never a grant), the
// closed list of entries of the Me tab, the stack Me, Emergency, person, and the area's views
// for an owner and for a manager; a link into the area without the entry shows "Not found or
// no access", and every other administration path stays closed on the phone.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:client/admin/admin_route.dart' show adminRepositoryProvider;
import 'package:client/app/router.dart';
import 'package:client/phone/me/emergency_entry.dart';
import 'package:client/phone/phone_navigation.dart';
import 'package:client/phone/phone_pages.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart' as store;

import '../support/phone_app.dart';

/// The calls of the emergency area against people Alex (owner) and Sam, Sam with one device
/// and one grant; every other call of the administration fails the test.
class _Admin implements AdminRepository {
  final List<String> calls = [];

  static AdminPerson _p(String id, String name, {bool owner = false}) =>
      AdminPerson(
        id: id,
        handle: id,
        displayName: name,
        status: AdminPersonStatus.active,
        isOwner: owner,
        createdAt: DateTime.utc(2026, 10),
      );

  final Map<String, AdminPerson> _people = {
    'alex': _p('alex', 'Alex', owner: true),
    'sam': _p('sam', 'Sam'),
  };

  @override
  Future<AdminPage<AdminPerson>> people({
    String? query,
    AdminPersonStatus? status,
    String? cursor,
  }) async {
    calls.add('people');
    return AdminPage(_people.values.toList());
  }

  @override
  Future<AdminPerson> person(String userId) async {
    calls.add('person $userId');
    return _people[userId]!;
  }

  @override
  Future<List<SelfDevice>> devicesOf(String userId) async {
    calls.add('devicesOf $userId');
    return [
      SelfDevice(
        id: 'pixel',
        name: 'Pixel',
        platform: 'android',
        state: SelfDeviceState.active,
        createdAt: DateTime.utc(2026, 10, 2),
      ),
    ];
  }

  @override
  Future<List<AdminGrant>> grantsOf(
    AdminSubjectKind kind,
    String subjectId,
  ) async {
    calls.add('grantsOf $subjectId');
    return [
      AdminGrant(
        id: 'g1',
        subjectKind: kind,
        subjectId: subjectId,
        targetKind: AdminTargetKind.space,
        targetId: 'support',
        level: AccessLevel.write,
        capabilities: const [],
        reason: 'Support rota',
        grantedAt: DateTime.utc(2026, 10, 3),
      ),
    ];
  }

  @override
  Future<List<AdminTarget>> targets() async => const [
    AdminTarget(AdminTargetKind.space, id: 'support', name: 'Support'),
  ];

  @override
  Future<List<AdminHost>> hosts() async => const [];

  @override
  dynamic noSuchMethod(Invocation invocation) =>
      throw StateError('unexpected call ${invocation.memberName}');
}

const store.AccessView _manage = store.AccessView(
  level: store.AccessLevel.write,
  capabilities: [Capability.manage],
);

const store.AccessView _write = store.AccessView(
  level: store.AccessLevel.write,
  capabilities: [Capability.upload],
);

void main() {
  late Directory root;
  late _Admin admin;

  setUp(() {
    root = Directory.systemTemp.createTempSync('emergency');
    admin = _Admin();
  });
  tearDown(() => root.deleteSync(recursive: true));

  List<Override> as(EmergencyRole role) => [
    emergencyRoleProvider.overrideWith((ref) => Stream.value(role)),
    adminRepositoryProvider.overrideWithValue(admin),
  ];

  group('who the entry is for', () {
    test('an owner, whatever the spaces say', () {
      expect(
        emergencyRoleOf(owner: true, spaceAccess: const []),
        EmergencyRole.owner,
      );
    });

    test('a manager of at least one space, by the reported capability', () {
      expect(
        emergencyRoleOf(
          owner: false,
          spaceAccess: const [null, _write, _manage],
        ),
        EmergencyRole.manager,
      );
    });

    test('nobody else', () {
      expect(
        emergencyRoleOf(owner: false, spaceAccess: const [null, _write]),
        EmergencyRole.none,
      );
    });
  });

  group('the stack under Me', () {
    test('the area and a person belong to Me', () {
      final at = parseLocation(Uri.parse('/admin/emergency/sam'));
      expect(tabOf(at, PhoneTab.spaces), PhoneTab.me);
      expect(phoneStack(PhoneTab.me, Uri.parse('/admin/emergency')), const [
        PhonePage(PhonePageKind.me, '/settings'),
        PhonePage(PhonePageKind.emergency, '/admin/emergency'),
      ]);
      expect(phoneStack(PhoneTab.me, Uri.parse('/admin/emergency/sam')), const [
        PhonePage(PhonePageKind.me, '/settings'),
        PhonePage(PhonePageKind.emergency, '/admin/emergency'),
        PhonePage(PhonePageKind.emergencyPerson, '/admin/emergency/sam'),
      ]);
    });

    test('every other administration path is not on the phone', () {
      for (final path in const [
        '/admin',
        '/admin/people',
        '/admin/grants',
        '/admin/audit',
        '/admin/statistics',
        '/admin/emergency/sam/devices',
      ]) {
        expect(
          phoneStack(PhoneTab.me, Uri.parse(path)).last.kind,
          PhonePageKind.notFound,
          reason: path,
        );
      }
    });
  });

  /// The keys of the entries of the Me tab, top to bottom.
  List<String> meEntries(WidgetTester tester) => [
    for (final e
        in find
            .descendant(
              of: find.byKey(const ValueKey('phone.me')),
              matching: find.byWidgetPredicate(
                (w) =>
                    w.key is ValueKey<String> &&
                    ((w.key! as ValueKey<String>).value.startsWith(
                          'phone.me.',
                        ) ||
                        (w.key! as ValueKey<String>).value == 'shell.logout'),
              ),
            )
            .evaluate())
      (e.widget.key! as ValueKey<String>).value,
  ];

  for (final role in const [EmergencyRole.owner, EmergencyRole.manager]) {
    testWidgets('Me offers exactly the settings, Emergency and Log out '
        '(${role.name})', (tester) async {
      await PhoneApp.start(tester, root: root, overrides: as(role));
      await tester.tap(find.byKey(const ValueKey('phone.tab.me')));
      await tester.pumpAndSettle();
      expect(meEntries(tester), [
        'phone.me.name',
        for (final s in phoneSettingsSections) 'phone.me.${s.segment}',
        'phone.me.emergency',
        'shell.logout',
      ]);
    });
  }

  testWidgets('no entry without the owner flag and without manage', (
    tester,
  ) async {
    await PhoneApp.start(tester, root: root, overrides: as(EmergencyRole.none));
    await tester.tap(find.byKey(const ValueKey('phone.tab.me')));
    await tester.pumpAndSettle();
    expect(meEntries(tester), [
      'phone.me.name',
      for (final s in phoneSettingsSections) 'phone.me.${s.segment}',
      'shell.logout',
    ]);
  });

  testWidgets('a link into the area without the entry is not found', (
    tester,
  ) async {
    final app = await PhoneApp.start(
      tester,
      root: root,
      overrides: as(EmergencyRole.none),
    );
    await app.go('/admin/emergency/sam');
    expect(find.byKey(const ValueKey('target.notFound')), findsOneWidget);
    expect(admin.calls, isEmpty);
  });

  testWidgets('an owner opens Emergency, a person, and sees all actions', (
    tester,
  ) async {
    final app = await PhoneApp.start(
      tester,
      root: root,
      overrides: as(EmergencyRole.owner),
    );
    await app.go('/settings');
    await tester.tap(find.byKey(const ValueKey('phone.me.emergency')));
    await tester.pumpAndSettle();
    expect(app.location, '/admin/emergency');
    expect(find.byKey(const ValueKey('emergency.home')), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('emergency.person.sam')));
    await tester.pumpAndSettle();
    expect(app.location, '/admin/emergency/sam');
    expect(app.shell.pages.map((p) => p.kind), [
      PhonePageKind.me,
      PhonePageKind.emergency,
      PhonePageKind.emergencyPerson,
    ]);
    expect(find.byKey(const ValueKey('emergency.disable')), findsOneWidget);
    expect(find.byKey(const ValueKey('device.pixel.revoke')), findsOneWidget);
    expect(
      find.byKey(const ValueKey('emergency.grant.g1.revoke')),
      findsOneWidget,
    );
  });

  testWidgets('a manager sees only the grants of the person', (tester) async {
    final app = await PhoneApp.start(
      tester,
      root: root,
      overrides: as(EmergencyRole.manager),
    );
    await app.go('/admin/emergency/sam');
    expect(find.byKey(const ValueKey('emergency.disable')), findsNothing);
    expect(find.byKey(const ValueKey('device.pixel.revoke')), findsNothing);
    expect(admin.calls, isNot(contains('devicesOf sam')));
    expect(
      find.byKey(const ValueKey('emergency.grant.g1.revoke')),
      findsOneWidget,
    );
  });
}
