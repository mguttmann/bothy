// SPDX-License-Identifier: Apache-2.0

// The step-up enumeration of the administration through its screens (brief M4-26 Design note
// 3): the app runs against FakeServer, which reads docs/spec/actions.yaml and answers
// `401 step_up_required` for an operation whose action has `step_up: true` until the client
// confirmed. For every changing operation of these screens the test forgets the confirmation,
// performs the action in the UI and asserts that the step-up dialog appears exactly when the
// table says `true` (and never for `false`). test/admin/admin_step_up_test.dart runs the same
// enumeration headless.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('every change goes through step-up exactly as 05 says', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      final server = app.fake.server;
      final world = server.world;
      final sam = app.fake.seed.sam.id;
      final pending = _waiting(world, sam, 'Pixel');
      final other = _waiting(world, sam, 'Old tablet');
      final passkey = world.addPasskey(sam, name: 'Sam laptop');
      final identity = world.addIdentity(sam, subject: 'sam');
      final checked = <String>[];

      Future<void> go(String path) async {
        GoRouter.of(tester.element(find.byType(Navigator).first)).go(path);
        await tester.pumpAndSettle();
      }

      Future<void> tap(String key) async {
        await app.waitFor(find.byKey(ValueKey(key)));
        await tester.ensureVisible(find.byKey(ValueKey(key)).first);
        await tester.tap(find.byKey(ValueKey(key)).first);
        await tester.pumpAndSettle();
      }

      /// Forgets the confirmation, runs [ui] up to the submitting tap and checks the dialog.
      Future<void> check(String op, Future<void> Function() ui) async {
        for (final s in world.sessions.values) {
          s.stepUpAt = null;
        }
        final before = server.requestsOf(op).length;
        await ui();
        if (server.stepUpOf(op) == 'true') {
          await app.waitFor(find.byKey(const ValueKey('stepUp.dialog')));
          await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
        } else {
          expect(server.stepUpOf(op), 'false', reason: op);
        }
        await eventually(
          '$op answered',
          () => server
              .requestsOf(op)
              .skip(before)
              .any((r) => r.status >= 200 && r.status < 300),
        );
        await tester.pumpAndSettle();
        expect(
          find.byKey(const ValueKey('stepUp.dialog')),
          findsNothing,
          reason: op,
        );
        checked.add(op);
      }

      // Waiting devices: approve with the code, reject.
      await go('/admin/devices');
      await check('approveDevice', () async {
        await tap('waiting.$pending.approve');
        await tester.enterText(
          find.byKey(const ValueKey('approveDevice.code')),
          world.devices[pending]!.verificationCode,
        );
        await tester.pump();
        await tap('approveDevice.submit');
      });
      await check('revokeDevice', () async {
        await tap('waiting.$other.reject');
        await tap('confirm.submit');
      });

      // People: create, new code, credentials, sessions, disable, enable, owners.
      await go('/admin/people');
      await check('createUser', () async {
        await tap('people.add');
        await tester.enterText(
          find.byKey(const ValueKey('createPerson.name')),
          'Robin K',
        );
        await tester.enterText(
          find.byKey(const ValueKey('createPerson.handle')),
          'robin.k',
        );
        await tester.pump();
        await tap('confirm.submit');
      });
      await tap('code.close');
      await check('addOwner', () async {
        await tap('owners.add');
        await tap('owners.pick');
        await tester.tap(find.text('Sam').last);
        await tester.pumpAndSettle();
        await tap('confirm.submit');
      });
      await check('revokeOwner', () async {
        await tap('owner.$sam.revoke');
        await tap('confirm.submit');
      });

      await go('/admin/people/$sam');
      await check('createEnrollmentCode', () async {
        await tap('person.newCode');
        await tap('confirm.submit');
      });
      await tap('code.close');
      await check('unlinkIdentity', () async {
        await tap('identity.$identity.unlink');
        await tap('confirm.submit');
      });
      await check('revokePasskey', () async {
        await tap('passkey.$passkey.revoke');
        await tap('confirm.submit');
      });
      final session = world.sessions.values
          .firstWhere((s) => s.deviceId == pending && !s.revoked)
          .id;
      await check('revokeSession', () async {
        await tap('session.$session.end');
        await tap('confirm.submit');
      });
      await check('disableUser', () async {
        await tap('person.disable');
        await tap('confirm.submit');
      });
      await check('enableUser', () async {
        await tap('person.enable');
        await tap('confirm.submit');
      });

      // Groups: create, rename, members, delete.
      await go('/admin/groups');
      await check('createGroup', () async {
        await tap('groups.create');
        await tester.enterText(
          find.byKey(const ValueKey('groupName.name')),
          'Support',
        );
        await tester.pump();
        await tap('confirm.submit');
      });
      await check('updateGroup', () async {
        await tap('group.rename');
        await tester.enterText(
          find.byKey(const ValueKey('groupName.name')),
          'Help desk',
        );
        await tester.pump();
        await tap('confirm.submit');
      });
      await check('addGroupMember', () async {
        await tap('group.addMember');
        await tap('member.person');
        await tester.tap(find.text('Sam').last);
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(const ValueKey('member.until')),
          '2026-12-31',
        );
        await tester.pump();
        await tap('confirm.submit');
      });
      await check('removeGroupMember', () async {
        await tap('member.$sam.remove');
        await tap('confirm.submit');
      });
      await check('deleteGroup', () async {
        await tap('group.delete');
        await tap('confirm.submit');
      });

      expect(checked, hasLength(16));
      expect(server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });
}

/// A waiting device of [userId] with a session that logged in with OIDC.
String _waiting(FakeWorld world, String userId, String name) {
  final id = world.nextId();
  world.devices[id] = FakeDevice(
    id: id,
    userId: userId,
    name: name,
    platform: 'android',
    state: 'pending',
    jkt: 'jkt-$id',
    createdAt: world.stamp(),
  );
  final sid = world.nextId();
  world.sessions[sid] = FakeSession(
    id: sid,
    userId: userId,
    deviceId: id,
    authMethod: 'oidc',
    createdAt: world.stamp(),
    expiresAt: world.stamp(world.now().add(const Duration(days: 1))),
    jkt: 'jkt-$id',
  );
  return id;
}
