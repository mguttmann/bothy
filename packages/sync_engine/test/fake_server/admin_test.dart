// SPDX-License-Identifier: Apache-2.0

// The administration handlers of the fake (brief M4-26 Scope "Fake server handlers"): people,
// owners, groups and the devices and sessions of another person. The step-up follows
// `docs/spec/actions.yaml` through the `x-authz-action` of each operation (Design note 3);
// enrollment codes are one-time secrets (06 7), the last owner stays (07 SR8), disabling ends
// sessions and devices (08 11), enabling may meet I1 (10 5.4).

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;

  setUp(() async => h = await FakeWorldHarness.start());
  tearDown(() => h.stop());

  List<String> ops() => [
    for (final r in h.server.requests)
      if (r.operationId != 'loginWithPassword') '${r.operationId} ${r.status}',
  ];

  Future<ApiProblem?> problemOf(Future<Object?> call) async {
    try {
      await call;
      return null;
    } on DioException catch (e) {
      return ApiProblem.of(e);
    }
  }

  String key(int n) =>
      '0193f2a7-0000-7000-8000-${n.toRadixString(16).padLeft(12, '0')}';

  test('the catalogue gives the step-up of each operation', () {
    expect(h.server.stepUpOf('createUser'), 'true');
    expect(h.server.stepUpOf('listUsers'), 'false');
    expect(h.server.stepUpOf('revokeSession'), 'false');
    expect(h.server.stepUpOf('addGroupMember'), 'true');
    expect(h.server.actionOf('revokeOwner'), 'owner.remove');
  });

  test('creating a person asks for step-up and shows the code once', () async {
    final alex = h.device();
    await signedIn(h, alex);
    h.server.requests.clear();
    final users = alex.api.getUsersApi();
    final body = UserCreate(
      (b) => b
        ..handle = 'robin.k'
        ..displayName = 'Robin K'
        ..localAccount = true,
    );

    final created = (await users.createUser(
      userCreate: body,
      idempotencyKey: key(1),
    )).data!;

    expect(created.user.handle, 'robin.k');
    expect(created.enrollment!.code, startsWith('en_'));
    expect(created.enrollment!.purpose, EnrollmentCodePurposeEnum.firstLogin);
    expect(ops(), [
      'createUser 401',
      'createStepUpChallenge 200',
      'completeStepUpWithDeviceKey 200',
      'createUser 201',
    ]);

    final replay = await problemOf(
      users.createUser(userCreate: body, idempotencyKey: key(1)),
    );
    expect(replay?.status, 409);
    expect(replay?.code, 'conflict');
    expect(replay?.reason, 'one_time_secret');

    final fresh = (await users.createEnrollmentCode(
      userId: created.user.id,
      idempotencyKey: key(2),
    )).data!;
    expect(fresh.code, isNot(created.enrollment!.code));
    final again = await problemOf(
      users.createEnrollmentCode(
        userId: created.user.id,
        idempotencyKey: key(2),
      ),
    );
    expect(again?.reason, 'one_time_secret');
    expect(h.server.violations, isEmpty);
  });

  test('the list searches and filters by status', () async {
    final alex = h.device();
    await signedIn(h, alex);
    h.server.world.disablePerson(h.seed.sam.id);
    final users = alex.api.getUsersApi();

    final all = (await users.listUsers()).data!.items;
    expect(all.map((u) => u.handle), containsAll(['alex', 'sam']));
    expect(all.map((u) => u.handle), isNot(contains('robin')));
    final found = (await users.listUsers(q: 'SA')).data!.items;
    expect(found.map((u) => u.handle), ['sam']);
    final disabled = (await users.listUsers(status: UserStatus.disabled))
        .data!
        .items;
    expect(disabled.map((u) => u.handle), ['sam']);
    expect(h.server.violations, isEmpty);
  });

  test('disabling ends sessions and devices; enabling may meet I1', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final sam = h.device(login: WorldSeed.samLogin, keySeed: 3);
    final samSession = await signedIn(h, sam);
    final users = alex.api.getUsersApi();
    final reason = ReasonBody((b) => b..reason = 'Left the team');

    final disabled = (await users.disableUser(
      userId: h.seed.sam.id,
      reasonBody: reason,
      idempotencyKey: key(3),
    )).data!;
    expect(disabled.status, UserStatus.disabled);
    final world = h.server.world;
    expect(world.devices[samSession.deviceId]!.state, 'revoked');
    expect(world.devices[samSession.deviceId]!.revokeReason, 'user_disabled');
    expect(
      world.sessions.values
          .where((s) => s.userId == h.seed.sam.id)
          .every((s) => s.revoked),
      isTrue,
    );

    world.i1OnEnable.add(h.seed.sam.id);
    final blocked = await problemOf(
      users.enableUser(
        userId: h.seed.sam.id,
        reasonBody: reason,
        idempotencyKey: key(4),
      ),
    );
    expect(blocked?.status, 409);
    expect(blocked?.code, 'i1_violation');
    world.i1OnEnable.clear();
    final enabled = (await users.enableUser(
      userId: h.seed.sam.id,
      reasonBody: reason,
      idempotencyKey: key(5),
    )).data!;
    expect(enabled.status, UserStatus.active);
    expect(h.server.violations, isEmpty);
  });

  test('the last owner can be neither revoked nor disabled', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final users = alex.api.getUsersApi();
    final reason = ReasonBody((b) => b..reason = 'Handover');

    final owners = (await users.listOwners()).data!.items;
    expect(owners.map((o) => o.user.handle), ['alex']);
    final revoke = await problemOf(
      users.revokeOwner(
        userId: h.seed.alex.id,
        reasonBody: reason,
        idempotencyKey: key(6),
      ),
    );
    expect(revoke?.status, 409);
    expect(revoke?.code, 'last_owner');
    final disable = await problemOf(
      users.disableUser(
        userId: h.seed.alex.id,
        reasonBody: reason,
        idempotencyKey: key(7),
      ),
    );
    expect(disable?.code, 'last_owner');

    final added = (await users.addOwner(
      ownerAdd: OwnerAdd(
        (b) => b
          ..userId = h.seed.sam.id
          ..reason = 'Deputy',
      ),
      idempotencyKey: key(8),
    )).data!;
    expect(added.user.handle, 'sam');
    expect(h.seed.sam.isOwner, isTrue);
    await users.revokeOwner(
      userId: h.seed.sam.id,
      reasonBody: reason,
      idempotencyKey: key(9),
    );
    expect(h.seed.sam.isOwner, isFalse);
    expect(h.server.violations, isEmpty);
  });

  test('credentials, devices and sessions of another person', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final sam = h.device(login: WorldSeed.samLogin, keySeed: 3);
    final samSession = await signedIn(h, sam);
    final world = h.server.world;
    final passkey = world.addPasskey(h.seed.sam.id, name: 'Sam laptop');
    final identity = world.addIdentity(h.seed.sam.id, subject: 'sam');
    final users = alex.api.getUsersApi();
    final devices = alex.api.getDevicesApi();

    final credentials = (await users.getUserCredentials(userId: h.seed.sam.id))
        .data!;
    expect(credentials.localPasswordSet, isTrue);
    expect(credentials.totpEnrolled, isTrue);
    expect(credentials.passkeys.single.name, 'Sam laptop');
    expect(credentials.identities.single.subject, 'sam');
    await users.revokePasskey(userId: h.seed.sam.id, passkeyId: passkey);
    await users.unlinkIdentity(userId: h.seed.sam.id, identityId: identity);
    final after = (await users.getUserCredentials(userId: h.seed.sam.id)).data!;
    expect(after.passkeys, isEmpty);
    expect(after.identities, isEmpty);

    final samDevices = (await devices.listUserDevices(userId: h.seed.sam.id))
        .data!
        .items;
    expect(samDevices.single.id, samSession.deviceId);
    final samSessions = (await devices.listUserSessions(userId: h.seed.sam.id))
        .data!
        .items;
    expect(samSessions.single.authMethod, SessionAuthMethodEnum.passwordTotp);
    await devices.revokeSession(sessionId: samSessions.single.id);
    expect(world.sessions[samSessions.single.id]!.revoked, isTrue);
    expect(h.server.violations, isEmpty);
  });

  test('groups with members that expire', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final groups = alex.api.getGroupsApi();

    final group = (await groups.createGroup(
      groupCreate: GroupCreate(
        (b) => b
          ..name = 'Support'
          ..description = 'Support team',
      ),
      idempotencyKey: key(10),
    )).data!;
    final renamed = (await groups.updateGroup(
      groupId: group.id,
      groupUpdate: GroupUpdate((b) => b..name = 'Help desk'),
      idempotencyKey: key(11),
    )).data!;
    expect(renamed.name, 'Help desk');
    final until = DateTime.utc(2026, 12, 31);
    final member = (await groups.addGroupMember(
      groupId: group.id,
      groupMemberAdd: GroupMemberAdd(
        (b) => b
          ..userId = h.seed.sam.id
          ..expiresAt = until,
      ),
      idempotencyKey: key(12),
    )).data!;
    expect(member.expiresAt, until);
    final listed = (await groups.listGroupMembers(groupId: group.id))
        .data!
        .items;
    expect(listed.single.user.handle, 'sam');
    expect((await groups.getGroup(groupId: group.id)).data!.memberCount, 1);
    final samGroups = (await groups.listGroups(memberId: h.seed.sam.id))
        .data!
        .items;
    expect(samGroups.single.id, group.id);
    await groups.removeGroupMember(groupId: group.id, userId: h.seed.sam.id);
    expect(
      (await groups.listGroupMembers(groupId: group.id)).data!.items,
      isEmpty,
    );
    await groups.deleteGroup(groupId: group.id);
    expect((await groups.listGroups()).data!.items, isEmpty);
    expect(h.server.violations, isEmpty);
  });

  test('a person without the owner flag is refused the changes', () async {
    final sam = h.device(login: WorldSeed.samLogin, keySeed: 3);
    await signedIn(h, sam);
    final refused = await problemOf(
      sam.api.getGroupsApi().createGroup(
        groupCreate: GroupCreate((b) => b..name = 'Mine'),
        idempotencyKey: key(13),
      ),
    );
    expect(refused?.status, 403);
    expect(refused?.reason, 'owner_only');
    final devices = await problemOf(
      sam.api.getDevicesApi().listUserDevices(userId: h.seed.alex.id),
    );
    expect(devices?.status, 403);
    expect(h.server.violations, isEmpty);
  });

  test('an owner approves the waiting device of another person', () async {
    final alex = h.device();
    await signedIn(h, alex);
    h.seed.sam.newDevicesPending = true;
    final sam = h.device(login: WorldSeed.samLogin, keySeed: 3);
    final waiting = (await sam.signIn() as LoginPending).device;

    final pending = (await alex.api.getDevicesApi().listUserDevices(
      userId: h.seed.sam.id,
    )).data!.items;
    expect(pending.single.state, DeviceStateEnum.pending);
    // 08 5.2: the code appears only for the waiting device itself.
    expect(pending.single.verificationCode, isNull);
    final approved = (await alex.api.getDevicesApi().approveDevice(
      deviceId: waiting.deviceId.value,
      deviceApprove: DeviceApprove(
        (b) => b..verificationCode = waiting.verificationCode,
      ),
      idempotencyKey: key(14),
    )).data!;
    expect(approved.state, DeviceStateEnum.active);
    expect(h.server.violations, isEmpty);
  });
}
