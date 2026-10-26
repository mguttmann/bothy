// SPDX-License-Identifier: Apache-2.0

// The emergency actions on the phone against the fake server (brief M5-09 Tests; R-PR-07; 08 6.2,
// 14.3): Alex, an owner, opens Emergency under Me and Sam. Revoking Sam's device first meets
// `401 step_up_required`: a cancelled prompt sends no second request and changes nothing; a
// confirmed one runs the device key step-up (the software key of the test ports stands for the
// biometric match) and the request repeats with the same `Idempotency-Key`. Then a grant of Sam
// is revoked and Sam is disabled, each through the step-up that 05 requires; disabling Alex, the
// last owner, shows the server's SR8 answer; a run stops from Running now without any prompt.

import 'package:client/phone/phone_shell.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import '../../test/support/widgets.dart' show en;
import 'support/phone_flow.dart';

/// An active device of [userId] named [name], as the server stores it.
String addDevice(FakeWorld world, String userId, String name) {
  final id = world.nextId();
  world.devices[id] = FakeDevice(
    id: id,
    userId: userId,
    name: name,
    platform: 'android',
    state: 'active',
    jkt: 'jkt-$id',
    createdAt: world.stamp(),
  );
  return id;
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('revoke a device, revoke a grant, disable, last owner, stop', (
    tester,
  ) async {
    await asPhone(() async {
      final app = await startPhone(tester);
      final server = app.fake.server;
      final world = server.world;
      final seed = app.fake.seed;
      final sam = seed.sam.id;
      final pixel = addDevice(world, sam, 'Pixel');
      final grant = world.nextId();
      world.grants[grant] = {
        'id': grant,
        'subject_kind': 'user',
        'subject_id': sam,
        'target_kind': 'space',
        'target_id': seed.space,
        'level': 'write',
        'capabilities': <Object?>[],
        'reason': 'Support rota',
        'granted_by': seed.alex.id,
        'granted_at': world.stamp(),
      };

      /// Forgets every confirmation, so the next change meets `401 step_up_required`.
      void forgetStepUp() {
        for (final s in world.sessions.values) {
          s.stepUpAt = null;
        }
      }

      Future<void> tap(String key) async {
        await app.waitFor(find.byKey(ValueKey(key)));
        await tester.ensureVisible(find.byKey(ValueKey(key)).first);
        await tester.pumpAndSettle();
        await app.tap(key);
      }

      Future<void> confirmStepUp() async {
        await app.waitFor(find.byKey(const ValueKey('stepUp.dialog')));
        await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
        await tester.pumpAndSettle();
      }

      try {
        await signIn(app);
        await tester.tap(find.byKey(const ValueKey('phone.tab.me')));
        await tester.pumpAndSettle();
        await tap('phone.me.emergency');
        await tap('emergency.person.$sam');
        await app.waitFor(find.byKey(ValueKey('device.$pixel.revoke')));

        // Revoke Sam's device: the step-up prompt, cancelled.
        forgetStepUp();
        await tap('device.$pixel.revoke');
        expect(
          find.text(en.emergencyRevokeDeviceBody('Pixel', 'Sam')),
          findsOneWidget,
        );
        await tap('emergency.sheet.confirm');
        await app.waitFor(find.byKey(const ValueKey('stepUp.dialog')));
        await tester.tap(find.byKey(const ValueKey('stepUp.cancel')));
        await app.waitFor(find.text(en.emergencyCancelled));
        expect(
          [for (final r in server.requestsOf('revokeDevice')) r.status],
          [401],
        );
        expect(world.devices[pixel]!.state, 'active');

        // Again, confirmed: the request repeats with the same key and succeeds.
        await tap('device.$pixel.revoke');
        await tap('emergency.sheet.confirm');
        await confirmStepUp();
        await until(
          tester,
          'the device revoked on the server',
          () => world.devices[pixel]!.state == 'revoked',
        );
        final attempts = server.requestsOf('revokeDevice').skip(1).toList();
        expect([for (final r in attempts) r.status], [401, 200]);
        expect(
          attempts.last.headers['idempotency-key'],
          attempts.first.headers['idempotency-key'],
        );
        expect(attempts.first.headers['idempotency-key'], isNotEmpty);
        await app.waitFor(find.text(en.devicesRevoked('Pixel')));

        // Revoke Sam's grant on the space, with its required reason.
        forgetStepUp();
        await tap('emergency.grant.$grant.revoke');
        await tester.enterText(
          find.byKey(const ValueKey('emergency.sheet.reason')),
          'Left the rota',
        );
        await tester.pump();
        await tap('emergency.sheet.confirm');
        await confirmStepUp();
        await until(
          tester,
          'the grant revoked on the server',
          () => world.grants[grant]!['revoked_at'] != null,
        );
        await app.waitFor(find.text(en.adminGrantRevoked('Sam')));

        // Disable Sam.
        forgetStepUp();
        await tap('emergency.disable');
        expect(find.text(en.emergencyDisableBody('Sam')), findsOneWidget);
        await tap('emergency.sheet.confirm');
        await confirmStepUp();
        await until(
          tester,
          'Sam disabled on the server',
          () => world.people[sam]!.status == 'disabled',
        );
        await app.waitFor(find.text(en.adminPersonDisabled('Sam')));

        // Disable Alex, the last owner: the server refuses with SR8 and the view explains.
        await goBack(tester);
        await tap('emergency.person.${seed.alex.id}');
        forgetStepUp();
        await tap('emergency.disable');
        await tap('emergency.sheet.confirm');
        await confirmStepUp();
        await app.waitFor(
          find.text('${en.problemLastOwner}. ${en.emergencyLastOwner}'),
        );
        expect(server.requestsOf('disableUser').last.status, 409);
        expect(world.people[seed.alex.id]!.status, 'active');

        // Stop a run from Running now: `run.cancel` has no step-up and asks nothing.
        final trigger = world.createMessage(seed.alex.id, seed.releaseNotes, {
          'client_msg_id': world.nextId(),
          'content_md': '@robin summarize the notes',
          'mention_ids': [seed.robin.id],
        }).message;
        final run = world.createRun(
          agentId: seed.robin.id,
          triggeringMessageId: trigger['id']! as String,
          triggeredBy: seed.alex.id,
        );
        final runId = run['id']! as String;
        world.setRunState(runId, 'running', triggerState: 'working');
        forgetStepUp();
        GoRouter.of(tester.element(find.byType(PhoneShell))).go('/running');
        // A running run animates, so nothing settles here: wait for the button and tap it.
        // On the phone Running now shows cards (M11, M5-14), so Stop is the card's button.
        final stop = find.byKey(ValueKey('runCard.stop.$runId'));
        await app.waitFor(stop);
        await tester.tap(stop);
        await tester.pump();
        expect(find.byKey(const ValueKey('stepUp.dialog')), findsNothing);
        expect(find.byKey(const ValueKey('emergency.sheet')), findsNothing);
        await until(
          tester,
          'the run cancelled on the server',
          () => world.runs[runId]!['state'] == 'cancelled',
        );
        expect(server.violations, isEmpty);
      } finally {
        await app.stop();
      }
    });
  });
}
