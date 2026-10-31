// SPDX-License-Identifier: Apache-2.0

/// Handlers of the administration of people, owners, groups and the devices and sessions of
/// another person (brief M4-26 Scope; 07 2.1, 2.7, SR5, SR8; 08 3.2, 4, 5.2, 11).
///
/// Every handler asks for the step-up through [requireCatalogStepUp], which reads the `step_up`
/// value of the operation's `x-authz-action` from `docs/spec/actions.yaml` (Design note 3), so
/// the fake never holds its own copy of 05. Since M4-28 `addGroupMember` and `addOwner` run
/// through [FakeI1Guard]: `resolve_i1`, the I1 check after the change and the audit rows
/// (10 5.1, 11).
library;

import 'dart:math';

import '../action_catalog.dart';
import '../audit_log.dart';
import '../rest_fake.dart';
import '../schema_validator.dart';
import '../world.dart';
import 'agent_handlers.dart' show FakeI1Guard;
import 'self_handlers.dart' show requireStepUp;

/// Throws `401 step_up_required` for an operation whose action has `step_up: true` or `fresh`
/// in [catalog], unless the session confirmed recently, for `fresh` with the device key
/// ([requireStepUp]); `false` and `conditional` ask nothing here.
void requireCatalogStepUp(
  FakeRequest r,
  ContractSchemas contract,
  ActionCatalog catalog,
) {
  final action = contract.operation(r.operationId).authzAction;
  if (action == null) return;
  final value = catalog.stepUp(action);
  if (value == 'true' || value == 'fresh') {
    requireStepUp(r, fresh: value == 'fresh');
  }
}

/// Registers the administration handlers on [rest], with the step-up of [catalog]; the audit
/// rows of `addGroupMember` and `addOwner` go to [audit].
void registerAdminHandlers(
  RestFake rest,
  ActionCatalog catalog,
  FakeAuditLog audit,
) {
  final world = rest.world;
  final guard = FakeI1Guard(world, audit);
  // 06 7: the enrollment code is a one-time secret; it comes from the secure generator like
  // the server's, and no test depends on its letters.
  final random = Random.secure();
  rest.oneTimeSecrets.addAll(const {'createUser', 'createEnrollmentCode'});

  void owner(FakeRequest r) {
    if (!r.person.isOwner) {
      throw const FakeProblem(403, 'forbidden', reason: 'owner_only');
    }
  }

  void stepUp(FakeRequest r) => requireCatalogStepUp(r, rest.contract, catalog);

  FakePerson person(FakeRequest r) {
    final p = world.people[r.param('user_id')];
    if (p == null || p.kind != 'user') {
      throw const FakeProblem(404, 'not_found');
    }
    return p;
  }

  /// Self or owner (`identity.read`, `device.list`, `session.list`, ...).
  FakePerson selfOrOwner(FakeRequest r) {
    final p = person(r);
    if (p.id != r.person.id) owner(r);
    return p;
  }

  int activeOwners() => world.people.values
      .where((p) => p.isOwner && p.status == 'active')
      .length;

  JsonMap enrollment(FakePerson p, String purpose) {
    const alphabet = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';
    String group() =>
        [for (var i = 0; i < 4; i++) alphabet[random.nextInt(alphabet.length)]]
            .join();
    final code = 'en_${group()}-${group()}-${group()}';
    world.enrollmentCodes[p.id] = code;
    return {
      'code': code,
      'purpose': purpose,
      'expires_at': world.stamp(world.now().add(const Duration(days: 7))),
    };
  }

  JsonMap group(JsonMap g) => {
    ...g,
    'member_count': world.groupMembers[g['id']]?.length ?? 0,
  };

  JsonMap groupOf(FakeRequest r) =>
      world.groups[r.param('group_id')] ??
      (throw const FakeProblem(404, 'not_found'));

  rest
    ..handle('listUsers', (r) {
      final q = r.query('q')?.toLowerCase();
      final status = r.query('status');
      return FakeResponse(200, {
        'items': [
          for (final p in world.people.values)
            if (p.kind == 'user' &&
                (status == null || p.status == status) &&
                (q == null ||
                    p.handle.toLowerCase().contains(q) ||
                    p.displayName.toLowerCase().contains(q)))
              p.user,
        ],
      });
    })
    ..handle('createUser', (r) {
      owner(r);
      stepUp(r);
      final body = r.json;
      final handle = body['handle']! as String;
      if (world.people.values.any((p) => p.handle == handle)) {
        throw const FakeProblem(409, 'conflict', reason: 'handle_taken');
      }
      final p =
          world.addPerson(
              handle: handle,
              displayName: body['display_name']! as String,
              loginName: body['local_account'] == true ? handle : null,
            )
            ..status = 'pending'
            ..email = body['email'] as String?
            ..locale = body['locale'] as String?
            ..timeZone = body['time_zone'] as String?
            ..createdAt = world.stamp();
      return FakeResponse(201, {
        'user': p.user,
        if (body['local_account'] == true)
          'enrollment': enrollment(p, 'first_login'),
      });
    })
    ..handle('getUser', (r) => FakeResponse(200, person(r).user))
    ..handle('updateUser', (r) {
      owner(r);
      final p = person(r);
      stepUp(r);
      p
        ..email = r.json['email'] as String? ?? p.email
        ..locale = r.json['locale'] as String? ?? p.locale
        ..timeZone = r.json['time_zone'] as String? ?? p.timeZone;
      return FakeResponse(200, p.user);
    })
    ..handle('disableUser', (r) {
      owner(r);
      final p = person(r);
      stepUp(r);
      if (p.status == 'deleted') {
        throw const FakeProblem(409, 'invalid_state_transition');
      }
      // 08 11 step 1: the last active owner stays (07 SR8).
      if (p.isOwner && p.status == 'active' && activeOwners() == 1) {
        throw const FakeProblem(409, 'last_owner');
      }
      if (p.status != 'disabled') world.disablePerson(p.id);
      return FakeResponse(200, p.user);
    })
    ..handle('enableUser', (r) {
      owner(r);
      final p = person(r);
      stepUp(r);
      if (p.status == 'deleted') {
        throw const FakeProblem(409, 'invalid_state_transition');
      }
      if (world.i1OnEnable.contains(p.id)) {
        throw const FakeProblem(409, 'i1_violation');
      }
      p.status = 'active';
      return FakeResponse(200, p.user);
    })
    ..handle('createEnrollmentCode', (r) {
      owner(r);
      final p = person(r);
      stepUp(r);
      if (p.status == 'disabled' || p.status == 'deleted') {
        throw const FakeProblem(409, 'invalid_state_transition');
      }
      final hasDevice = world.devices.values.any(
        (d) => d.userId == p.id && d.state == 'active',
      );
      final purpose = p.loginName == null
          ? 'identity_link'
          : hasDevice
          ? 'recovery'
          : 'first_login';
      return FakeResponse(201, enrollment(p, purpose));
    })
    ..handle('getUserCredentials', (r) {
      final p = selfOrOwner(r);
      return FakeResponse(200, {
        'identities': p.identities.values.toList(),
        'passkeys': p.passkeys.values.toList(),
        'local_password_set': p.password != null,
        'totp_enrolled': p.totpCode != null,
      });
    })
    ..handle('unlinkIdentity', (r) {
      final p = selfOrOwner(r);
      stepUp(r);
      if (p.identities.remove(r.param('identity_id')) == null) {
        throw const FakeProblem(404, 'not_found');
      }
      return const FakeResponse.noContent();
    })
    ..handle('revokePasskey', (r) {
      final p = selfOrOwner(r);
      stepUp(r);
      if (p.passkeys.remove(r.param('passkey_id')) == null) {
        throw const FakeProblem(404, 'not_found');
      }
      return const FakeResponse.noContent();
    })
    ..handle('listUserDevices', (r) {
      final p = selfOrOwner(r);
      return FakeResponse(200, {
        'items': [
          for (final d in world.devices.values)
            if (d.userId == p.id) d.toJson(),
        ],
      });
    })
    ..handle('listUserSessions', (r) {
      final p = selfOrOwner(r);
      return FakeResponse(200, {
        'items': [
          for (final s in world.sessions.values)
            if (s.userId == p.id && !s.revoked) s.toJson(),
        ],
      });
    })
    ..handle('listOwners', (r) {
      return FakeResponse(200, {
        'items': [
          for (final p in world.people.values)
            if (p.isOwner)
              {
                'user': p.ref,
                'granted_at': p.ownerGrantedAt,
                'reason': p.ownerReason,
              },
        ],
      });
    })
    ..handle('addOwner', (r) {
      owner(r);
      stepUp(r);
      final p = world.people[r.json['user_id']];
      if (p == null || p.kind != 'user') {
        throw const FakeProblem(404, 'not_found');
      }
      if (p.isOwner) throw const FakeProblem(409, 'conflict');
      // An owner triggers every agent, so I1 covers all personal agents (10 5.1).
      return guard.run(
        r,
        action: 'owner.add',
        resourceKind: 'user',
        apply: () {
          p
            ..isOwner = true
            ..ownerGrantedAt = world.stamp()
            ..ownerReason = r.json['reason']! as String;
          world.permissionVersion++;
          return (
            response: FakeResponse(201, {
              'user': p.ref,
              'granted_by': r.person.id,
              'granted_at': p.ownerGrantedAt,
              'reason': p.ownerReason,
            }),
            resourceId: p.id,
            details: {'reason': p.ownerReason},
          );
        },
      );
    })
    ..handle('revokeOwner', (r) {
      owner(r);
      final p = person(r);
      stepUp(r);
      if (!p.isOwner) throw const FakeProblem(404, 'not_found');
      if (p.status == 'active' && activeOwners() == 1) {
        throw const FakeProblem(409, 'last_owner');
      }
      p.isOwner = false;
      world.permissionVersion++;
      return const FakeResponse.noContent();
    })
    ..handle('listGroups', (r) {
      final member = r.query('member_id');
      if (member != r.person.id) owner(r);
      return FakeResponse(200, {
        'items': [
          for (final g in world.groups.values)
            if (member == null ||
                (world.groupMembers[g['id']]?.containsKey(member) ?? false))
              group(g),
        ],
      });
    })
    ..handle('createGroup', (r) {
      owner(r);
      stepUp(r);
      final id = world.nextId();
      final g = <String, Object?>{
        'id': id,
        'name': r.json['name']! as String,
        'description': r.json['description'] as String? ?? '',
        'source': r.json['source'] as String? ?? 'local',
        'created_at': world.stamp(),
      };
      world.groups[id] = g;
      world.groupMembers[id] = {};
      return FakeResponse(201, group(g));
    })
    ..handle('getGroup', (r) {
      owner(r);
      return FakeResponse(200, group(groupOf(r)));
    })
    ..handle('updateGroup', (r) {
      owner(r);
      final g = groupOf(r);
      stepUp(r);
      if (r.json['name'] case final String name) g['name'] = name;
      if (r.json['description'] case final String text) {
        g['description'] = text;
      }
      return FakeResponse(200, group(g));
    })
    ..handle('deleteGroup', (r) {
      owner(r);
      final g = groupOf(r);
      stepUp(r);
      world.groups.remove(g['id']);
      world.groupMembers.remove(g['id']);
      world.permissionVersion++;
      return const FakeResponse.noContent();
    })
    ..handle('listGroupMembers', (r) {
      owner(r);
      final g = groupOf(r);
      return FakeResponse(200, {
        'items': world.groupMembers[g['id']]!.values.toList(),
      });
    })
    ..handle('addGroupMember', (r) {
      owner(r);
      final g = groupOf(r);
      stepUp(r);
      final p = world.people[r.json['user_id']];
      if (p == null || p.kind != 'user') {
        throw const FakeProblem(404, 'not_found');
      }
      // A new member gains the group's grants, so I1 covers all personal agents (10 5.1).
      return guard.run(
        r,
        action: 'group.member.add',
        resourceKind: 'group',
        apply: () {
          final member = <String, Object?>{
            'user': p.ref,
            'added_by': r.person.id,
            'added_at': world.stamp(),
            'expires_at': ?r.json['expires_at'],
          };
          world.groupMembers[g['id']]![p.id] = member;
          world.permissionVersion++;
          return (
            response: FakeResponse(201, member),
            resourceId: g['id']! as String,
            details: {'user_id': p.id, 'expires_at': ?r.json['expires_at']},
          );
        },
      );
    })
    ..handle('removeGroupMember', (r) {
      owner(r);
      final g = groupOf(r);
      stepUp(r);
      if (world.groupMembers[g['id']]!.remove(r.param('user_id')) == null) {
        throw const FakeProblem(404, 'not_found');
      }
      world.permissionVersion++;
      return const FakeResponse.noContent();
    });
}
