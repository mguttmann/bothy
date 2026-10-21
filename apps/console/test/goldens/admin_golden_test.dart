// SPDX-License-Identifier: Apache-2.0

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:console/console.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

/// The matrix of the brief at a browser width: light and dark, text scaling 1 and 2, German
/// and English (brief M6-04 Tests).
GoldenTestGroup _matrix(String name, Widget Function() builder) =>
    dsGoldenMatrix(
      name: name,
      builder: () => SizedBox(width: 1280, height: 720, child: builder()),
      textScales: const [1.0, 2.0],
      locales: const [Locale('de'), Locale('en')],
    );

/// A repository with Alex and Sam and nothing waiting; every other call is unexpected.
class _People implements AdminRepository {
  @override
  Future<AdminPage<AdminPerson>> people({
    String? query,
    AdminPersonStatus? status,
    String? cursor,
  }) async => AdminPage([
    for (final (id, handle, name, owner) in [
      ('a', 'alex', 'Alex', true),
      ('s', 'sam', 'Sam', false),
    ])
      AdminPerson(
        id: id,
        handle: handle,
        displayName: name,
        status: AdminPersonStatus.active,
        isOwner: owner,
        createdAt: DateTime.utc(2026, 10, 10, 9, 30),
      ),
  ]);

  @override
  Future<AdminPage<AdminGroup>> groups({String? cursor}) async =>
      const AdminPage([]);

  @override
  dynamic noSuchMethod(Invocation invocation) =>
      throw UnsupportedError('${invocation.memberName}');
}

final _endsAt = DateTime.utc(2026, 10, 10, 9, 45);

final _viewAs = ViewAsResult(
  user: const AccessPrincipal(
    id: 's',
    kind: 'user',
    handle: 'sam',
    displayName: 'Sam',
  ),
  expiresAt: _endsAt,
  spaces: const [
    ViewAsItem(
      id: 'support',
      name: 'Support',
      level: AccessLevel.write,
      capabilities: [],
    ),
  ],
  channels: const [
    ViewAsItem(
      id: 'general',
      name: 'general',
      level: AccessLevel.write,
      capabilities: [],
      spaceId: 'support',
    ),
    ViewAsItem(
      id: 'requests',
      name: 'requests',
      level: AccessLevel.read,
      capabilities: [],
      spaceId: 'support',
    ),
  ],
);

/// The shell inside a Material, as the router of the app puts every page into a Scaffold.
Widget _shell(Widget body) => Material(child: _bare(body));

Widget _bare(Widget body) => ConsoleShell(
  productName: 'Bothy',
  displayName: 'Alex',
  page: ConsolePage.people,
  onOpen: (_) {},
  onLogout: () {},
  banner: ViewAsBanner(
    name: 'Sam',
    endsAt: _endsAt,
    now: _endsAt.subtract(const Duration(minutes: 9, seconds: 30)),
    onEnd: () {},
  ),
  body: body,
);

void main() {
  goldenTest(
    'People of A1 in the console with the banner of a stored view',
    fileName: 'console_admin_people',
    builder: () => _matrix(
      'people',
      () => _shell(
        AdminBody(
          repository: _People(),
          location: AdminLocation.start,
          maxWidth: null,
          onNavigate: (_) {},
        ),
      ),
    ),
  );

  goldenTest(
    'the structure of View as person in the console',
    fileName: 'console_admin_view_as',
    builder: () => _matrix(
      'view as',
      () => _shell(
        SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: ViewAsStructure(
            name: 'Sam',
            snapshot: _viewAs,
            notVisible: const ['Engineering', 'Marketing'],
          ),
        ),
      ),
    ),
  );

  goldenTest(
    'the step-up of an action in the console',
    fileName: 'console_admin_step_up',
    builder: () => _matrix(
      'step-up',
      () => const Center(
        child: ConsoleStepUpDialog(
          purpose: 'Create the group Support Team',
          ways: ConsoleStepUpWays(passkey: false, password: true, oidc: []),
        ),
      ),
    ),
  );
}
