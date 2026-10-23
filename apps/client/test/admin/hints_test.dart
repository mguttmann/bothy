// SPDX-License-Identifier: Apache-2.0

// "Permission hints" and "Expiring soon" of the app against the fake server of M4-17, headless
// (brief M6-07 Tests: integration with the fake server): the REST source of admin_ui reads both
// lists through the client of the signed-in session, every answer validates against the
// contract, and "Extend" (a new grant, then the revocation of the old one, 07 9.4) asks for the
// step-up once, because one confirmation covers both requests within
// `auth.step_up_max_age` (the fake's `stepUpMaxAge`, counted on its world clock,
// which follows the system clock here; the two requests run back to back).

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:client/admin/admin_route.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import 'admin_step_up_test.dart' show RecordingPrompt;

void main() {
  HttpOverrides? saved;
  late FakeInstance fake;
  late ProviderContainer c;
  late RecordingPrompt prompt;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    prompt = RecordingPrompt();
    fake = await FakeInstance.start(stepUpPrompt: prompt);
    c = appContainer(fake);
  });

  tearDown(() async {
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  Future<AdminRepository> signedIn() async {
    final control = await connected(c, fake);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    expect(c.read(onboardingControllerProvider).step, OnboardingStep.signedIn);
    // Read into a local first: as the operand of `return` in an async function the type
    // argument of `read` would be inferred from `FutureOr<AdminRepository>`, and Riverpod
    // finds no element for that type.
    final admin = c.read(adminRepositoryProvider)!;
    return admin;
  }

  test('the hints of the example of 07 9.3 with their names', () async {
    final admin = await signedIn();
    final world = fake.server.world;
    final sam = fake.seed.sam.id;
    // Texter operates Marketing and reads Kampagnen and Intern; Sam may trigger on Kampagnen
    // only, so Sam can learn content of Intern through Texter (H1, 07 6).
    final marketing = world.addSpace('Marketing');
    final texter = world.addAgent(handle: 'texter', displayName: 'Texter');
    final campaigns = world.addChannel(marketing, 'Kampagnen', {
      texter.id: 'write',
      sam: 'trigger',
    });
    final internal = world.addChannel(marketing, 'Intern', {texter.id: 'read'});
    world.setSpaceOperator(marketing, texter.id);

    final page = await admin.permissionHints(
      kind: AccessHintKind.h1ReadHorizon,
    );

    final h1 = page.items.where(
      (h) => h.agentId == texter.id && h.personId == sam,
    );
    expect(h1.map((h) => h.channelId), [internal]);
    expect(h1.single.spaceId, marketing);
    expect(
      page.items.every((h) => h.kind == AccessHintKind.h1ReadHorizon),
      isTrue,
    );
    expect(page.labels.principal(texter.id), 'Texter');
    expect(page.labels.principal(sam), 'Sam');
    expect(page.labels.channelPath(internal), 'Marketing/Intern');

    final inSpace = await admin.permissionHints(spaceId: marketing);
    expect(
      inSpace.items
          .where(
            (h) =>
                h.kind == AccessHintKind.h1ReadHorizon &&
                h.agentId == texter.id &&
                h.personId == sam,
          )
          .map((h) => h.channelId),
      [internal],
    );
    expect(
      inSpace.items.where((h) => h.channelId == campaigns),
      isEmpty,
      reason: 'Sam reads Kampagnen',
    );
    expect(fake.server.violations, isEmpty);
    expect(prompt.asked, isEmpty, reason: 'reading the lists has no step-up');
  });

  test('"Extend" creates the new grant, then revokes the old one, with one step-up', () async {
    final admin = await signedIn();
    final world = fake.server.world;
    final sam = fake.seed.sam.id;
    final general = AdminTarget(
      AdminTargetKind.channel,
      id: fake.seed.general,
      name: 'general',
    );
    // A grant that ends tomorrow on the fake's clock lies within the lead of
    // authz.expiry_reminder_lead.
    final old = await admin.createGrant(
      GrantRequest(
        subjectKind: AdminSubjectKind.user,
        subjectId: sam,
        target: general,
        level: AccessLevel.trigger,
        expiresAt: world.now().add(const Duration(days: 1)),
        reason: 'Release',
      ),
      purpose: 'create the first grant',
    );
    final before = await admin.expiringAccess();
    final listed = before.items.where((e) => e.grant?.id == old.id).single;
    expect(listed.kind, ExpiringKind.grant);
    expect(before.labels.principal(sam), 'Sam');
    expect(before.labels.channelPath(fake.seed.general), 'Team/general');

    // A fresh confirmation for the extension, as after a while without one.
    for (final s in world.sessions.values) {
      s.stepUpAt = null;
    }
    prompt.asked.clear();
    final renewed = await admin.createGrant(
      GrantRequest(
        subjectKind: AdminSubjectKind.user,
        subjectId: sam,
        target: general,
        level: AccessLevel.trigger,
        expiresAt: world.now().add(const Duration(days: 90)),
        reason: 'Next quarter',
      ),
      purpose: 'extend: create',
    );
    await admin.revokeGrant(
      old.id,
      reason: renewed.reason,
      purpose: 'extend: revoke',
    );

    expect(prompt.asked, ['extend: create'], reason: 'one step-up for both');
    final creates = fake.server.requestsOf('createGrant');
    final revokes = fake.server.requestsOf('revokeGrant');
    expect(creates.last.status, 201);
    expect(revokes.last.status, 200);
    final after = await admin.expiringAccess();
    expect(after.items.where((e) => e.grant?.id == old.id), isEmpty);
    expect(fake.server.violations, isEmpty);
  });
}
