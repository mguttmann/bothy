// SPDX-License-Identifier: Apache-2.0

/// The administration of people, owners, groups and waiting devices over REST
/// (docs/spec/01-product.md 5.2 A1; 07 2.1, 2.7, SR5, SR8; 08 3.2, 4, 5.2, 11; brief M4-26
/// Design note 1).
///
/// No local database and no permission logic: what a screen shows follows the answers of the
/// server (15 1 principle 1); the console passes its own client (M6-04).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';

import 'self_repository.dart';

/// The state of a person (`UserStatus`).
enum AdminPersonStatus {
  /// Created, not signed in yet.
  pending,

  /// Signed in at least once and not disabled.
  active,

  /// Disabled: no sessions, no devices, no permissions (07 SR7, 08 11).
  disabled,

  /// Deleted.
  deleted,
}

/// One person as the administration shows it (`User`).
final class AdminPerson {
  /// Creates a person.
  const AdminPerson({
    required this.id,
    required this.handle,
    required this.displayName,
    required this.status,
    required this.isOwner,
    required this.createdAt,
  });

  /// The person ID.
  final String id;

  /// The handle, for example `sam`.
  final String handle;

  /// The display name.
  final String displayName;

  /// The state.
  final AdminPersonStatus status;

  /// Whether the person is an owner (07 2.7).
  final bool isOwner;

  /// When the person was created.
  final DateTime createdAt;
}

/// One page of a list: [items] and the opaque cursor of the next page (null on the last).
final class AdminPage<T> {
  /// Creates a page.
  const AdminPage(this.items, {this.nextCursor});

  /// The entries of this page.
  final List<T> items;

  /// The cursor of the next page, or null.
  final String? nextCursor;
}

/// Why an enrollment code exists (08 3.2).
enum EnrollmentPurpose {
  /// The first login of a person with a local identity.
  firstLogin,

  /// Recovery of a person with a local identity and a device.
  recovery,

  /// Linking an external identity.
  identityLink,
}

/// An enrollment code, a one-time secret: shown once, never stored by the client (08 3.2).
final class EnrollmentCodeInfo {
  /// Creates the code.
  const EnrollmentCodeInfo({
    required this.code,
    required this.purpose,
    required this.expiresAt,
  });

  /// The code, `en_` and groups of Crockford Base32.
  final String code;

  /// The purpose.
  final EnrollmentPurpose purpose;

  /// The end of its validity.
  final DateTime expiresAt;
}

/// The answer of [AdminRepository.createPerson]: the person and, for a local account, the
/// enrollment code of the first login.
final class PersonCreated {
  /// Creates the answer.
  const PersonCreated(this.person, {this.enrollment});

  /// The new person.
  final AdminPerson person;

  /// The code of the first login, or null without a local account.
  final EnrollmentCodeInfo? enrollment;
}

/// A linked external identity (`Identity`).
final class AdminIdentity {
  /// Creates the identity.
  const AdminIdentity({
    required this.id,
    required this.providerKind,
    required this.subject,
    required this.linkedAt,
    this.issuer,
  });

  /// The identity ID.
  final String id;

  /// `oidc`, `trusted_header` or `local`.
  final String providerKind;

  /// The subject at the provider.
  final String subject;

  /// The issuer, if any.
  final String? issuer;

  /// When it was linked.
  final DateTime linkedAt;
}

/// A passkey (`Passkey`).
final class AdminPasskey {
  /// Creates the passkey.
  const AdminPasskey({
    required this.id,
    required this.name,
    required this.createdAt,
  });

  /// The passkey ID.
  final String id;

  /// Its name.
  final String name;

  /// When it was registered.
  final DateTime createdAt;
}

/// The credentials of a person (`CredentialOverview`).
final class AdminCredentials {
  /// Creates the overview.
  const AdminCredentials({
    required this.identities,
    required this.passkeys,
    required this.localPasswordSet,
    required this.totpEnrolled,
  });

  /// The linked identities.
  final List<AdminIdentity> identities;

  /// The passkeys.
  final List<AdminPasskey> passkeys;

  /// Whether a local password is set.
  final bool localPasswordSet;

  /// Whether a one-time code app is enrolled.
  final bool totpEnrolled;
}

/// One owner (`Owner`).
final class AdminOwner {
  /// Creates the owner.
  const AdminOwner({
    required this.userId,
    required this.displayName,
    required this.grantedAt,
    required this.reason,
  });

  /// The person ID.
  final String userId;

  /// The display name.
  final String displayName;

  /// Since when.
  final DateTime grantedAt;

  /// Why.
  final String reason;
}

/// A group (`Group`).
final class AdminGroup {
  /// Creates the group.
  const AdminGroup({
    required this.id,
    required this.name,
    required this.description,
    required this.memberCount,
    this.fromProvider = false,
  });

  /// The group ID.
  final String id;

  /// The name.
  final String name;

  /// The description.
  final String description;

  /// The number of members.
  final int memberCount;

  /// Whether the identity provider manages the members (`source = oidc_claim`, 08 3.4).
  final bool fromProvider;
}

/// A member of a group (`GroupMember`).
final class AdminGroupMember {
  /// Creates the member.
  const AdminGroupMember({
    required this.userId,
    required this.displayName,
    required this.addedAt,
    this.expiresAt,
  });

  /// The person ID.
  final String userId;

  /// The display name.
  final String displayName;

  /// When the person was added.
  final DateTime addedAt;

  /// When the membership ends, or null (07 2.1).
  final DateTime? expiresAt;
}

/// A device that waits for its approval, with its person and the origin of its login
/// (01 5.2 A1; 08 5.2).
final class WaitingDevice {
  /// Creates the entry.
  const WaitingDevice({
    required this.person,
    required this.device,
    this.origin,
  });

  /// The person of the device.
  final AdminPerson person;

  /// The device (`state = pending`).
  final SelfDevice device;

  /// How the login of the device started (`Session.auth_method`), or null without a session.
  final SelfAuthMethod? origin;
}

/// The administration's calls; every changing one carries the [purpose] text that the step-up
/// dialog shows when the server answers `401 step_up_required` (05 2.4, 08 6.2). The
/// repository never asks for a step-up on its own (fixed requirement 10).
///
/// Errors: [SelfActionFailed] with the server's problem (null without an answer),
/// [StepUpCancelled] when the person cancels the step-up.
abstract interface class AdminRepository {
  /// One page of people; [query] and [status] are request parameters of `listUsers`.
  Future<AdminPage<AdminPerson>> people({
    String? query,
    AdminPersonStatus? status,
    String? cursor,
  });

  /// One person (`getUser`).
  Future<AdminPerson> person(String userId);

  /// Creates a person (`createUser`); with [localAccount] the answer carries the enrollment
  /// code of the first login, shown once.
  Future<PersonCreated> createPerson({
    required String handle,
    required String displayName,
    required bool localAccount,
    required String purpose,
  });

  /// Disables [userId] with [reason] (`disableUser`, 08 11): every session and device ends.
  Future<AdminPerson> disablePerson(
    String userId, {
    required String reason,
    required String purpose,
  });

  /// Enables [userId] with [reason] (`enableUser`); may fail with `409 i1_violation`.
  Future<AdminPerson> enablePerson(
    String userId, {
    required String reason,
    required String purpose,
  });

  /// Issues a new enrollment code for [userId] (`createEnrollmentCode`), shown once.
  Future<EnrollmentCodeInfo> newEnrollmentCode(
    String userId, {
    required String purpose,
  });

  /// The credentials of [userId] (`getUserCredentials`).
  Future<AdminCredentials> credentials(String userId);

  /// Unlinks the identity [identityId] of [userId] (`unlinkIdentity`).
  Future<void> unlinkIdentity(
    String userId,
    String identityId, {
    required String purpose,
  });

  /// Revokes the passkey [passkeyId] of [userId] (`revokePasskey`).
  Future<void> revokePasskey(
    String userId,
    String passkeyId, {
    required String purpose,
  });

  /// Every device of [userId] (`listUserDevices`, all pages).
  Future<List<SelfDevice>> devicesOf(String userId);

  /// The live sessions of [userId] with their device names (`listUserSessions`, all pages).
  Future<List<SelfSession>> sessionsOf(String userId);

  /// Ends the session [sessionId] (`revokeSession`, no step-up in 05).
  Future<void> endSession(String sessionId);

  /// Approves the waiting device [deviceId] with the [code] it shows (`approveDevice`); a
  /// wrong code revokes it (08 5.2).
  Future<SelfDevice> approveDevice(
    String deviceId,
    String code, {
    required String purpose,
  });

  /// Revokes the device [deviceId] (`revokeDevice`); rejecting a waiting device is this call.
  Future<SelfDevice> revokeDevice(
    String deviceId, {
    required String reason,
    required String purpose,
  });

  /// The devices that wait for their approval, assembled from `listUsers`,
  /// `listUserDevices` (state `pending`) and `listUserSessions` (the origin), because the API
  /// has no instance-wide list of them (brief M4-26 requirement 15).
  Future<List<WaitingDevice>> waitingDevices();

  /// The owners (`listOwners`).
  Future<List<AdminOwner>> owners();

  /// Makes [userId] an owner (`addOwner`).
  Future<void> addOwner(
    String userId, {
    required String reason,
    required String purpose,
  });

  /// Revokes the owner [userId] (`revokeOwner`); the last owner gives `409 last_owner`.
  Future<void> revokeOwner(
    String userId, {
    required String reason,
    required String purpose,
  });

  /// One page of groups (`listGroups`).
  Future<AdminPage<AdminGroup>> groups({String? cursor});

  /// One group (`getGroup`).
  Future<AdminGroup> group(String groupId);

  /// Creates a group (`createGroup`).
  Future<AdminGroup> createGroup(
    String name, {
    required String description,
    required String purpose,
  });

  /// Renames [groupId] (`updateGroup`).
  Future<AdminGroup> renameGroup(
    String groupId,
    String name, {
    required String purpose,
  });

  /// Deletes [groupId] (`deleteGroup`).
  Future<void> deleteGroup(String groupId, {required String purpose});

  /// The members of [groupId] (`listGroupMembers`, all pages).
  Future<List<AdminGroupMember>> members(String groupId);

  /// Adds [userId] to [groupId], until [expiresAt] when given (`addGroupMember`).
  Future<void> addMember(
    String groupId,
    String userId, {
    DateTime? expiresAt,
    required String purpose,
  });

  /// Removes [userId] from [groupId] (`removeGroupMember`).
  Future<void> removeMember(
    String groupId,
    String userId, {
    required String purpose,
  });
}

/// [AdminRepository] over the generated client of `api_client`.
///
/// The `api` argument is the client of the signed-in session (`AuthSession.api`), so DPoP,
/// refresh and the step-up interceptor apply; [newIdempotencyKey] returns a fresh UUIDv7 per
/// action, which the interceptor repeats after a step-up.
///
/// Example:
/// ```dart
/// final admin = ApiAdminRepository(session.api, newIdempotencyKey: ids.next);
/// final first = await admin.people(query: 'sam');
/// ```
final class ApiAdminRepository implements AdminRepository {
  /// Creates the repository.
  ApiAdminRepository(this._api, {required String Function() newIdempotencyKey})
    : _newKey = newIdempotencyKey;

  final ApiClient _api;
  final String Function() _newKey;

  UsersApi get _users => _api.getUsersApi();
  GroupsApi get _groups => _api.getGroupsApi();
  DevicesApi get _devices => _api.getDevicesApi();

  Map<String, Object?> _purpose(String text) => {stepUpPurposeExtra: text};

  @override
  Future<AdminPage<AdminPerson>> people({
    String? query,
    AdminPersonStatus? status,
    String? cursor,
  }) => _call(() async {
    final page = (await _users.listUsers(
      cursor: cursor,
      q: query == null || query.isEmpty ? null : query,
      status: switch (status) {
        null => null,
        AdminPersonStatus.pending => UserStatus.pending,
        AdminPersonStatus.active => UserStatus.active,
        AdminPersonStatus.disabled => UserStatus.disabled,
        AdminPersonStatus.deleted => UserStatus.deleted,
      },
    )).data!;
    return AdminPage([
      for (final u in page.items) _person(u),
    ], nextCursor: page.nextCursor);
  });

  @override
  Future<AdminPerson> person(String userId) =>
      _call(() async => _person((await _users.getUser(userId: userId)).data!));

  @override
  Future<PersonCreated> createPerson({
    required String handle,
    required String displayName,
    required bool localAccount,
    required String purpose,
  }) => _call(() async {
    final created = (await _users.createUser(
      userCreate: UserCreate(
        (b) => b
          ..handle = handle
          ..displayName = displayName
          ..localAccount = localAccount,
      ),
      idempotencyKey: _newKey(),
      extra: _purpose(purpose),
    )).data!;
    final code = created.enrollment;
    return PersonCreated(
      _person(created.user),
      enrollment: code == null ? null : _code(code),
    );
  });

  @override
  Future<AdminPerson> disablePerson(
    String userId, {
    required String reason,
    required String purpose,
  }) => _call(
    () async => _person(
      (await _users.disableUser(
        userId: userId,
        reasonBody: ReasonBody((b) => b..reason = reason),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<AdminPerson> enablePerson(
    String userId, {
    required String reason,
    required String purpose,
  }) => _call(
    () async => _person(
      (await _users.enableUser(
        userId: userId,
        reasonBody: ReasonBody((b) => b..reason = reason),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<EnrollmentCodeInfo> newEnrollmentCode(
    String userId, {
    required String purpose,
  }) => _call(
    () async => _code(
      (await _users.createEnrollmentCode(
        userId: userId,
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<AdminCredentials> credentials(String userId) => _call(() async {
    final c = (await _users.getUserCredentials(userId: userId)).data!;
    return AdminCredentials(
      identities: [
        for (final i in c.identities)
          AdminIdentity(
            id: i.id,
            providerKind: i.providerKind.name,
            subject: i.subject,
            issuer: i.issuer,
            linkedAt: i.linkedAt,
          ),
      ],
      passkeys: [
        for (final p in c.passkeys)
          AdminPasskey(id: p.id, name: p.name, createdAt: p.createdAt),
      ],
      localPasswordSet: c.localPasswordSet,
      totpEnrolled: c.totpEnrolled,
    );
  });

  @override
  Future<void> unlinkIdentity(
    String userId,
    String identityId, {
    required String purpose,
  }) => _call(
    () => _users.unlinkIdentity(
      userId: userId,
      identityId: identityId,
      extra: _purpose(purpose),
    ),
  );

  @override
  Future<void> revokePasskey(
    String userId,
    String passkeyId, {
    required String purpose,
  }) => _call(
    () => _users.revokePasskey(
      userId: userId,
      passkeyId: passkeyId,
      extra: _purpose(purpose),
    ),
  );

  @override
  Future<List<SelfDevice>> devicesOf(String userId) => _call(() async {
    final items = await _all<Device>((cursor) async {
      final page = (await _devices.listUserDevices(
        userId: userId,
        cursor: cursor,
      )).data!;
      return (page.items, page.nextCursor);
    });
    return [for (final d in items) _device(d)];
  });

  @override
  Future<List<SelfSession>> sessionsOf(String userId) => _call(() async {
    final names = {for (final d in await devicesOf(userId)) d.id: d.name};
    final items = await _sessions(userId);
    return [
      for (final s in items)
        SelfSession(
          id: s.id,
          deviceId: s.deviceId,
          deviceName: names[s.deviceId],
          authMethod: _method(s.authMethod),
          lastUsedAt: s.lastSeenAt ?? s.createdAt,
        ),
    ];
  });

  @override
  Future<void> endSession(String sessionId) => _call(
    () =>
        _devices.revokeSession(sessionId: sessionId, idempotencyKey: _newKey()),
  );

  @override
  Future<SelfDevice> approveDevice(
    String deviceId,
    String code, {
    required String purpose,
  }) => _call(
    () async => _device(
      (await _devices.approveDevice(
        deviceId: deviceId,
        deviceApprove: DeviceApprove((b) => b..verificationCode = code),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<SelfDevice> revokeDevice(
    String deviceId, {
    required String reason,
    required String purpose,
  }) => _call(
    () async => _device(
      (await _devices.revokeDevice(
        deviceId: deviceId,
        reasonBody: ReasonBody((b) => b..reason = reason),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<List<WaitingDevice>> waitingDevices() => _call(() async {
    final people = <AdminPerson>[];
    String? cursor;
    do {
      final page = await this.people(cursor: cursor);
      people.addAll(page.items);
      cursor = page.nextCursor;
    } while (cursor != null);
    final out = <WaitingDevice>[];
    for (final person in people) {
      if (person.status == AdminPersonStatus.disabled ||
          person.status == AdminPersonStatus.deleted) {
        continue;
      }
      final waiting = [
        for (final d in await devicesOf(person.id))
          if (d.state == SelfDeviceState.pending) d,
      ];
      if (waiting.isEmpty) continue;
      final sessions = await _sessions(person.id);
      for (final device in waiting) {
        final session = sessions
            .where((s) => s.deviceId == device.id)
            .firstOrNull;
        out.add(
          WaitingDevice(
            person: person,
            device: device,
            origin: session == null ? null : _method(session.authMethod),
          ),
        );
      }
    }
    out.sort((a, b) => a.device.createdAt.compareTo(b.device.createdAt));
    return out;
  });

  @override
  Future<List<AdminOwner>> owners() => _call(() async {
    final list = (await _users.listOwners()).data!;
    return [
      for (final o in list.items)
        AdminOwner(
          userId: o.user.id,
          displayName: o.user.displayName,
          grantedAt: o.grantedAt,
          reason: o.reason,
        ),
    ];
  });

  @override
  Future<void> addOwner(
    String userId, {
    required String reason,
    required String purpose,
  }) => _call(
    () => _users.addOwner(
      ownerAdd: OwnerAdd(
        (b) => b
          ..userId = userId
          ..reason = reason,
      ),
      idempotencyKey: _newKey(),
      extra: _purpose(purpose),
    ),
  );

  @override
  Future<void> revokeOwner(
    String userId, {
    required String reason,
    required String purpose,
  }) => _call(
    () => _users.revokeOwner(
      userId: userId,
      reasonBody: ReasonBody((b) => b..reason = reason),
      idempotencyKey: _newKey(),
      extra: _purpose(purpose),
    ),
  );

  @override
  Future<AdminPage<AdminGroup>> groups({String? cursor}) => _call(() async {
    final page = (await _groups.listGroups(cursor: cursor)).data!;
    return AdminPage([
      for (final g in page.items) _group(g),
    ], nextCursor: page.nextCursor);
  });

  @override
  Future<AdminGroup> group(String groupId) => _call(
    () async => _group((await _groups.getGroup(groupId: groupId)).data!),
  );

  @override
  Future<AdminGroup> createGroup(
    String name, {
    required String description,
    required String purpose,
  }) => _call(
    () async => _group(
      (await _groups.createGroup(
        groupCreate: GroupCreate(
          (b) => b
            ..name = name
            ..description = description.isEmpty ? null : description,
        ),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<AdminGroup> renameGroup(
    String groupId,
    String name, {
    required String purpose,
  }) => _call(
    () async => _group(
      (await _groups.updateGroup(
        groupId: groupId,
        groupUpdate: GroupUpdate((b) => b..name = name),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<void> deleteGroup(String groupId, {required String purpose}) => _call(
    () => _groups.deleteGroup(groupId: groupId, extra: _purpose(purpose)),
  );

  @override
  Future<List<AdminGroupMember>> members(String groupId) => _call(() async {
    final items = await _all<GroupMember>((cursor) async {
      final page = (await _groups.listGroupMembers(
        groupId: groupId,
        cursor: cursor,
      )).data!;
      return (page.items, page.nextCursor);
    });
    return [
      for (final m in items)
        AdminGroupMember(
          userId: m.user.id,
          displayName: m.user.displayName,
          addedAt: m.addedAt,
          expiresAt: m.expiresAt,
        ),
    ];
  });

  @override
  Future<void> addMember(
    String groupId,
    String userId, {
    DateTime? expiresAt,
    required String purpose,
  }) => _call(
    () => _groups.addGroupMember(
      groupId: groupId,
      groupMemberAdd: GroupMemberAdd(
        (b) => b
          ..userId = userId
          ..expiresAt = expiresAt?.toUtc(),
      ),
      idempotencyKey: _newKey(),
      extra: _purpose(purpose),
    ),
  );

  @override
  Future<void> removeMember(
    String groupId,
    String userId, {
    required String purpose,
  }) => _call(
    () => _groups.removeGroupMember(
      groupId: groupId,
      userId: userId,
      extra: _purpose(purpose),
    ),
  );

  Future<List<Session>> _sessions(String userId) =>
      _all<Session>((cursor) async {
        final page = (await _devices.listUserSessions(
          userId: userId,
          cursor: cursor,
        )).data!;
        return (page.items, page.nextCursor);
      });

  /// Reads every page that [page] returns, following the cursor.
  static Future<List<T>> _all<T>(
    Future<(Iterable<T>, String?)> Function(String? cursor) page,
  ) async {
    final items = <T>[];
    String? cursor;
    do {
      final (batch, next) = await page(cursor);
      items.addAll(batch);
      cursor = next;
    } while (cursor != null);
    return items;
  }

  static AdminPerson _person(User u) => AdminPerson(
    id: u.id,
    handle: u.handle,
    displayName: u.displayName,
    status: switch (u.status) {
      UserStatus.pending => AdminPersonStatus.pending,
      UserStatus.disabled => AdminPersonStatus.disabled,
      UserStatus.deleted => AdminPersonStatus.deleted,
      _ => AdminPersonStatus.active,
    },
    isOwner: u.isOwner,
    createdAt: u.createdAt,
  );

  static EnrollmentCodeInfo _code(EnrollmentCode c) => EnrollmentCodeInfo(
    code: c.code,
    purpose: switch (c.purpose) {
      EnrollmentCodePurposeEnum.recovery => EnrollmentPurpose.recovery,
      EnrollmentCodePurposeEnum.identityLink => EnrollmentPurpose.identityLink,
      _ => EnrollmentPurpose.firstLogin,
    },
    expiresAt: c.expiresAt,
  );

  static AdminGroup _group(Group g) => AdminGroup(
    id: g.id,
    name: g.name,
    description: g.description,
    memberCount: g.memberCount,
    fromProvider: g.source_ == GroupSource.oidcClaim,
  );

  static SelfDevice _device(Device d) => SelfDevice(
    id: d.id,
    name: d.name,
    platform: d.platform.name,
    state: switch (d.state) {
      DeviceStateEnum.pending => SelfDeviceState.pending,
      DeviceStateEnum.active => SelfDeviceState.active,
      _ => SelfDeviceState.revoked,
    },
    createdAt: d.createdAt,
    appVersion: d.appVersion,
    lastSeenAt: d.lastSeenAt,
  );

  static SelfAuthMethod _method(SessionAuthMethodEnum m) => switch (m) {
    SessionAuthMethodEnum.oidc => SelfAuthMethod.oidc,
    SessionAuthMethodEnum.trustedHeader => SelfAuthMethod.trustedHeader,
    SessionAuthMethodEnum.passkey => SelfAuthMethod.passkey,
    SessionAuthMethodEnum.passwordTotp => SelfAuthMethod.passwordTotp,
    SessionAuthMethodEnum.enrollmentCode => SelfAuthMethod.enrollmentCode,
    SessionAuthMethodEnum.devicePairing => SelfAuthMethod.devicePairing,
    _ => SelfAuthMethod.other,
  };

  /// Runs [call] and turns its errors into [SelfActionFailed]; an error of the login layer
  /// ([StepUpCancelled], a lost session) passes as it is.
  static Future<T> _call<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on DioException catch (e) {
      final cause = e.error;
      if (cause is AuthException) throw cause;
      final response = e.response;
      throw SelfActionFailed(
        ApiProblem.of(e) ??
            (response == null ? null : ApiProblem.fromResponse(response)),
      );
    }
  }
}
