// SPDX-License-Identifier: Apache-2.0

// The agent administration of A5 headless, through the app's REST source against the fake
// server (brief M4-28 Tests; the same story as integration_test/guided_flow_test.dart): the
// preview reports the I1 conflict with `organization_ready`, the plain grant is refused with
// the conflicts, the guided grant switches the agent and grants in one request (BR-07), the
// agent card then carries the server's mismatch warning until the heartbeat changes, the
// proposal of "All to Personal" comes from the audit, BR-08 changes nothing, and a token's
// secret comes once. Every request and response matches docs/api/.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:auth/auth.dart';
import 'package:client/admin/admin_route.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';

/// Answers every step-up with yes.
class _Yes implements StepUpPrompt {
  @override
  Future<bool> ask(String purposeText) async => true;
}

void main() {
  HttpOverrides? saved;
  late FakeInstance fake;
  late ProviderContainer c;
  late AdminRepository admin;
  late FakeWorld world;
  late String alex;
  late String sam;
  late String support;
  late FakeAgent agent;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    fake = await FakeInstance.start(stepUpPrompt: _Yes());
    c = appContainer(fake);
    final control = await connected(c, fake);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    world = fake.server.world;
    alex = fake.seed.alex.id;
    sam = fake.seed.sam.id;
    support = world.addSpace('Support');
    world.addChannel(support, 'Queue', {alex: 'trigger'});
    agent = world.addAgent(
      handle: 'helper',
      displayName: 'Helper',
      billingMode: 'personal',
      credentialHolderId: alex,
      reportedBillingMode: 'personal',
    );
    world.setSpaceOperator(support, agent.id);
    admin = c.read(adminRepositoryProvider)!;
  });

  tearDown(() async {
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  GrantRequest samTrigger(String target) => GrantRequest(
    subjectKind: AdminSubjectKind.user,
    subjectId: sam,
    target: AdminTarget(AdminTargetKind.space, id: target, name: 'Support'),
    level: AccessLevel.trigger,
    reason: 'Support rota',
  );

  test('the guided grant switches and grants in one request', () async {
    final preview = await admin.previewGrant(samTrigger(support));
    expect(preview.allowed, isFalse);
    final finding = preview.violations.single;
    expect(finding.code, 'i1_violation');
    expect(finding.agentId, agent.id);
    expect(finding.holderId, alex);
    expect(finding.principalIds, [sam]);
    expect(finding.organizationReady, isTrue);

    // Without resolve_i1 the server refuses and names the conflicts (10 5.4).
    await expectLater(
      admin.createGrant(samTrigger(support), purpose: 'grant'),
      throwsA(
        isA<SelfActionFailed>()
            .having((e) => e.problem?.code, 'code', 'i1_violation')
            .having((e) => e.conflicts.single.principalIds, 'principals', [
              sam,
            ]),
      ),
    );
    expect(world.agents[agent.id]!.billingMode, 'personal');

    await admin.createGrant(
      samTrigger(support),
      purpose: 'grant',
      resolveI1: I1Resolution([agent.id], reason: 'Team uses it'),
    );
    final record = await admin.agentRecord(agent.id);
    expect(record.mode, AgentBillingMode.organization);
    expect(record.holderId, isNull);
    expect(record.runtime.reportedMode, AgentBillingMode.personal);
    expect(record.runtime.warnings, ['billing_mode_mismatch']);
    // The step-up answer (401) comes first; the refused and the guided request follow.
    expect(fake.server.requestsOf('createGrant').map((r) => r.status), [
      401,
      409,
      201,
    ]);

    world.agentHeartbeat(agent.id, reportedBillingMode: 'organization');
    expect((await admin.agentRecord(agent.id)).runtime.warnings, isEmpty);
    expect(fake.server.violations, isEmpty);
  });

  test('a group member and an owner take the same way', () async {
    final group = await admin.createGroup(
      'Rota',
      description: '',
      purpose: 'group',
    );
    await admin.createGrant(
      GrantRequest(
        subjectKind: AdminSubjectKind.group,
        subjectId: group.id,
        target: AdminTarget(
          AdminTargetKind.space,
          id: support,
          name: 'Support',
        ),
        level: AccessLevel.trigger,
        reason: 'Rota',
      ),
      purpose: 'grant',
    );
    await expectLater(
      admin.addMember(group.id, sam, purpose: 'member'),
      throwsA(isA<SelfActionFailed>()),
    );
    await admin.addMember(
      group.id,
      sam,
      purpose: 'member',
      resolveI1: I1Resolution([agent.id], reason: 'Rota'),
    );
    expect(world.agents[agent.id]!.billingMode, 'organization');

    // Back to personal for Alex alone is refused while Sam may trigger (no guided way back).
    await expectLater(
      admin.setMode(
        await admin.agentRecord(agent.id),
        AgentBillingMode.personal,
        holderId: alex,
        reason: 'Back',
        purpose: 'switch',
      ),
      throwsA(
        isA<SelfActionFailed>().having(
          (e) => e.conflicts.single.principalIds,
          'principals',
          [sam],
        ),
      ),
    );

    final other = world.addAgent(
      handle: 'desk',
      displayName: 'Desk',
      billingMode: 'personal',
      credentialHolderId: alex,
    );
    await expectLater(
      admin.addOwner(sam, reason: 'Deputy', purpose: 'owner'),
      throwsA(isA<SelfActionFailed>()),
    );
    await admin.addOwner(
      sam,
      reason: 'Deputy',
      purpose: 'owner',
      resolveI1: I1Resolution([other.id], reason: 'Deputy'),
    );
    expect(world.agents[other.id]!.billingMode, 'organization');
    expect(fake.server.violations, isEmpty);
  });

  test('all to personal proposes the last global switch; BR-08', () async {
    await admin.setAllModes(
      AgentBillingMode.organization,
      reason: 'Doubts',
      purpose: 'all',
    );
    expect(world.agents[agent.id]!.billingMode, 'organization');
    final proposal = await admin.lastGlobalSwitch(purpose: 'audit');
    expect(proposal?.agentIds, [agent.id]);
    expect(proposal?.holderId, alex);

    // Sam may trigger the agent now, so one violating agent changes nothing (BR-08).
    await admin.createGrant(samTrigger(support), purpose: 'grant');
    final idle = world.addAgent(handle: 'idle', displayName: 'Idle');
    await expectLater(
      admin.setAllModes(
        AgentBillingMode.personal,
        holderId: alex,
        agentIds: [agent.id, idle.id],
        reason: 'Back',
        purpose: 'all',
      ),
      throwsA(
        isA<SelfActionFailed>().having(
          (e) => e.conflicts.map((f) => f.agentId),
          'agents',
          [agent.id],
        ),
      ),
    );
    expect(world.agents[agent.id]!.billingMode, 'organization');
    expect(world.agents[idle.id]!.billingMode, 'organization');
    expect(fake.server.violations, isEmpty);
  });

  test('a token secret comes once and the list shows the prefix', () async {
    final created = await admin.createToken(
      agent.id,
      name: 'Adapter',
      expiresAt: DateTime.utc(2027, 10, 1),
      purpose: 'token',
    );
    expect(created.secret, startsWith(created.token.prefix));
    final listed = await admin.tokens(agent.id);
    expect(listed.single.prefix, created.token.prefix);
    await admin.revokeToken(created.token.id);
    expect((await admin.tokens(agent.id)).single.revokedAt, isNotNull);
    expect(fake.server.violations, isEmpty);
  });
}
