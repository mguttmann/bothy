// SPDX-License-Identifier: Apache-2.0

// The administration of the app against the fake server, headless (brief M4-26 Design note 3;
// the same enumeration as integration_test/admin_step_up_test.dart): every changing operation
// the screens call asks the step-up prompt exactly when its action has `step_up: true` in
// docs/spec/actions.yaml, which the fake reads; never for `false`. Also the paths of `/admin`.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:auth/auth.dart';
import 'package:client/admin/admin_route.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';

/// Answers every step-up with yes and records its purpose.
class RecordingPrompt implements StepUpPrompt {
  /// The purposes asked.
  final List<String> asked = [];

  @override
  Future<bool> ask(String purposeText) async {
    asked.add(purposeText);
    return true;
  }

  /// These tests confirm with the step-up key; a password prompt cancels.
  @override
  Future<StepUpCredentials?> askPassword(
    String purposeText, {
    bool rejected = false,
  }) async => null;
}

/// A pending device of [userId] that logged in with [method], as the server stores it.
String addWaiting(FakeWorld world, String userId, String name, String method) {
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
    authMethod: method,
    createdAt: world.stamp(),
    expiresAt: world.stamp(world.now().add(const Duration(days: 1))),
    jkt: 'jkt-$id',
  );
  return id;
}

void main() {
  group('the paths of the administration', () {
    test('map to locations and back', () {
      expect(adminLocationOf(['admin']), AdminLocation.start);
      expect(
        adminLocationOf(['admin', 'people', 'x']),
        const AdminLocation(AdminSection.people, id: 'x'),
      );
      expect(
        adminLocationOf(['admin', 'grants', 'channel.x']),
        const AdminLocation(AdminSection.grants, id: 'channel.x'),
      );
      expect(
        adminLocationOf(['admin', 'hosts', 'h']),
        const AdminLocation(AdminSection.hosts, id: 'h'),
      );
      expect(
        adminLocationOf(['admin', 'agents', 'a']),
        const AdminLocation(AdminSection.agents, id: 'a'),
      );
      expect(
        adminLocationOf(['admin', 'apps']),
        const AdminLocation(AdminSection.apps),
      );
      expect(
        adminLocationOf(['admin', 'apps', 'a']),
        const AdminLocation(AdminSection.apps, id: 'a'),
      );
      expect(
        adminLocationOf(['admin', 'incoming-webhooks']),
        const AdminLocation(AdminSection.webhooks),
      );
      expect(adminLocationOf(['admin', 'incoming-webhooks', 'x']), isNull);
      expect(
        adminLocationOf(['admin', 'statistics']),
        const AdminLocation(AdminSection.statistics),
      );
      expect(adminLocationOf(['admin', 'statistics', 'x']), isNull);
      // The lists of 07 9.4 and 9.5 (M6-07) have no detail.
      expect(
        adminLocationOf(['admin', 'hints']),
        const AdminLocation(AdminSection.hints),
      );
      expect(
        adminLocationOf(['admin', 'expiring']),
        const AdminLocation(AdminSection.expiring),
      );
      expect(adminLocationOf(['admin', 'hints', 'x']), isNull);
      expect(
        adminPathOf(const AdminLocation(AdminSection.hints)),
        '/admin/hints',
      );
      expect(
        adminPathOf(const AdminLocation(AdminSection.expiring)),
        '/admin/expiring',
      );
      expect(adminLocationOf(['admin', 'devices', 'x']), isNull);
      expect(
        adminPathOf(const AdminLocation(AdminSection.groups, id: 'g')),
        '/admin/groups/g',
      );
    });
  });

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

  test('every change asks for step-up exactly as actions.yaml says', () async {
    final control = await connected(c, fake);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    expect(c.read(onboardingControllerProvider).step, OnboardingStep.signedIn);
    final admin = c.read(adminRepositoryProvider)!;
    final world = fake.server.world;
    final sam = fake.seed.sam.id;
    final pending = addWaiting(world, sam, 'Pixel', 'oidc');
    final other = addWaiting(world, sam, 'Old tablet', 'password_totp');
    final passkey = world.addPasskey(sam, name: 'Sam laptop');
    final identity = world.addIdentity(sam, subject: 'sam');

    final waiting = await admin.waitingDevices();
    expect(waiting.map((w) => w.device.name), ['Pixel', 'Old tablet']);
    expect(waiting.first.origin, SelfAuthMethod.oidc);
    expect(waiting.first.person.displayName, 'Sam');

    late AdminGroup group;
    late AdminGrant grant;
    late AdminHost host;
    late AgentTokenCreated token;
    final alex = fake.seed.alex.id;
    final helper = world.addAgent(handle: 'helper', displayName: 'Helper');
    final general = AdminTarget(
      AdminTargetKind.channel,
      id: fake.seed.general,
      name: 'general',
    );
    final code = world.devices[pending]!.verificationCode;
    final samSession = world.sessions.values
        .firstWhere((s) => s.deviceId == other)
        .id;
    final steps = <String, Future<void> Function()>{
      'createUser': () => admin.createPerson(
        handle: 'robin.k',
        displayName: 'Robin K',
        localAccount: true,
        purpose: 'createUser',
      ),
      'createEnrollmentCode': () =>
          admin.newEnrollmentCode(sam, purpose: 'createEnrollmentCode'),
      'unlinkIdentity': () =>
          admin.unlinkIdentity(sam, identity, purpose: 'unlinkIdentity'),
      'revokePasskey': () =>
          admin.revokePasskey(sam, passkey, purpose: 'revokePasskey'),
      'approveDevice': () =>
          admin.approveDevice(pending, code, purpose: 'approveDevice'),
      'revokeSession': () => admin.endSession(samSession),
      'revokeDevice': () =>
          admin.revokeDevice(other, reason: 'Lost', purpose: 'revokeDevice'),
      'addOwner': () =>
          admin.addOwner(sam, reason: 'Deputy', purpose: 'addOwner'),
      'revokeOwner': () =>
          admin.revokeOwner(sam, reason: 'Back', purpose: 'revokeOwner'),
      'createGroup': () async => group = await admin.createGroup(
        'Support',
        description: '',
        purpose: 'createGroup',
      ),
      'updateGroup': () =>
          admin.renameGroup(group.id, 'Help desk', purpose: 'updateGroup'),
      'addGroupMember': () => admin.addMember(
        group.id,
        sam,
        expiresAt: DateTime.utc(2026, 12, 31),
        purpose: 'addGroupMember',
      ),
      'removeGroupMember': () =>
          admin.removeMember(group.id, sam, purpose: 'removeGroupMember'),
      'deleteGroup': () => admin.deleteGroup(group.id, purpose: 'deleteGroup'),
      'createGrant': () async => grant = await admin.createGrant(
        GrantRequest(
          subjectKind: AdminSubjectKind.user,
          subjectId: sam,
          target: general,
          level: AccessLevel.trigger,
          reason: 'Release',
        ),
        purpose: 'createGrant',
      ),
      'revokeGrant': () =>
          admin.revokeGrant(grant.id, reason: 'Done', purpose: 'revokeGrant'),
      'viewAsUser': () => admin.viewAs(sam, purpose: 'viewAsUser'),
      // Hosts (M4-37): host.manage and audit.read have step-up.
      'createHost': () async {
        final code = await admin.createHost(
          name: 'Build host',
          reason: 'Builds',
          purpose: 'createHost',
        );
        world.pairHost(code.hostId, ['term-sam']);
        host = await admin.host(code.hostId);
      },
      'confirmHost': () async => host = await admin.confirmHost(
        host,
        HostConfirmRequest(
          fingerprintPrefix: '7KQ29XTA',
          llmLogin: HostLlmLogin.organization,
          declaredServerHost: false,
          sessionUsers: [HostAssignment(userId: sam, osUser: 'term-sam')],
          reason: 'Builds',
        ),
        purpose: 'confirmHost',
      ),
      'updateHost': () async => host = await admin.updateHost(
        host,
        revoke: [sam],
        reason: 'Rotate',
        purpose: 'updateHost',
      ),
      'setHostLlmCredentials': () async => host = await admin.setLlmLogin(
        host,
        HostLlmLogin.none,
        reason: 'No login',
        purpose: 'setHostLlmCredentials',
      ),
      'listAuditEvents': () =>
          admin.assignmentHistory(host.id, purpose: 'listAuditEvents'),
      'setHostState': () async => host = await admin.setHostState(
        host,
        HostStateRequest.disabled,
        reason: 'Maintenance',
        purpose: 'setHostState',
      ),
      'issueHostPairingCode': () => admin.newPairingCode(
        host.id,
        reason: 'New disk',
        purpose: 'issueHostPairingCode',
      ),
      'disableUser': () =>
          admin.disablePerson(sam, reason: 'Left', purpose: 'disableUser'),
      'enableUser': () =>
          admin.enablePerson(sam, reason: 'Back', purpose: 'enableUser'),
      // Agents (M4-28): agent.billing_mode.set, agent.billing_mode.set_all,
      // agent.credential_holder.set and token.create have step-up, token.revoke has none. Alex
      // is the only owner by now and the agent operates nothing, so Alex alone may trigger it
      // and holding it keeps I1 (10 5.4).
      'setAgentBillingMode': () async => admin.setMode(
        await admin.agentRecord(helper.id),
        AgentBillingMode.personal,
        holderId: alex,
        reason: 'Alex pays',
        purpose: 'setAgentBillingMode',
      ),
      'setCredentialHolder': () async => admin.setHolder(
        await admin.agentRecord(helper.id),
        alex,
        reason: 'Still Alex',
        purpose: 'setCredentialHolder',
      ),
      'setAllAgentsBillingMode': () => admin.setAllModes(
        AgentBillingMode.organization,
        reason: 'Company key',
        purpose: 'setAllAgentsBillingMode',
      ),
      'createToken': () async => token = await admin.createToken(
        helper.id,
        name: 'Adapter',
        expiresAt: DateTime.utc(2027, 10, 1),
        dpopJkt: 'NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs',
        purpose: 'createToken',
      ),
      'revokeToken': () => admin.revokeToken(token.token.id),
    };

    final seen = <String>[];
    for (final MapEntry(key: op, value: run) in steps.entries) {
      for (final s in world.sessions.values) {
        s.stepUpAt = null;
      }
      prompt.asked.clear();
      await run();
      final stepUp = fake.server.stepUpOf(op);
      expect(
        stepUp,
        anyOf('true', 'false'),
        reason: '$op: fresh and conditional are not used by these screens',
      );
      expect(
        prompt.asked,
        stepUp == 'true' ? [op] : isEmpty,
        reason: '$op (${fake.server.actionOf(op)}, step_up $stepUp)',
      );
      final last = fake.server.requestsOf(op).last.status;
      expect(last, inInclusiveRange(200, 299), reason: op);
      seen.add('$op ${fake.server.actionOf(op)} $stepUp');
    }
    // The table the report names (brief M4-26 Report item 4).
    // ignore: avoid_print
    print(seen.join('\n'));
    expect(fake.server.violations, isEmpty);
  });
}
