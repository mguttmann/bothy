// SPDX-License-Identifier: Apache-2.0

/// Handlers of the hosts of the terminal, their pairing and their session users, and the
/// audit of the assignments (brief M4-37 Scope; 24 2.2, 3, 7.3, 10.1; 05 `host.*`,
/// `audit.read`).
///
/// A small model of the server: `createHost` and `issueHostPairingCode` give a code, the tests
/// play `bothy-host pair` with [FakeWorld.pairHost], `confirmHost` compares the typed prefix
/// with `terminal.fingerprint_max_attempts` failures (the reference default 3 of 24 17), I6 is
/// checked against the grants on the host, names come only from the reported list and stay
/// bound to their person after the assignment ends. The grant preview on a host lives in
/// `access_handlers.dart`.
library;

import 'dart:math';

import '../action_catalog.dart';
import '../fake_host.dart';
import '../rest_fake.dart';
import '../world.dart';
import 'admin_handlers.dart' show requireCatalogStepUp;

/// `terminal.fingerprint_confirm_chars` and `terminal.fingerprint_max_attempts` of the fake
/// (the reference defaults of 24 17).
const int _confirmChars = 8;
const int _maxAttempts = 3;

/// Registers the host handlers on [rest], with the step-up of [catalog].
void registerHostHandlers(RestFake rest, ActionCatalog catalog) {
  final world = rest.world;
  final random = Random(37);
  // 06 7: the pairing code is a one-time secret.
  rest.oneTimeSecrets.addAll(const {'createHost', 'issueHostPairingCode'});

  void owner(FakeRequest r) {
    if (!r.person.isOwner) {
      throw const FakeProblem(403, 'forbidden', reason: 'owner_only');
    }
  }

  void stepUp(FakeRequest r) => requireCatalogStepUp(r, rest.contract, catalog);

  FakeHost hostOf(FakeRequest r) =>
      world.hosts[r.param('host_id')] ??
      (throw const FakeProblem(404, 'not_found'));

  /// `If-Match` against the version of [h] (06: `428` without, `412` on a mismatch).
  void ifMatch(FakeRequest r, FakeHost h) {
    final match = RegExp(r'^"?(\d+)"?$')
        .firstMatch(r.headers['if-match'] ?? '');
    if (match == null) throw const FakeProblem(428, 'precondition_required');
    if (int.parse(match[1]!) != h.version) {
      throw FakeProblem(412, 'version_conflict', current: h.json);
    }
  }

  JsonMap code(FakeHost h) {
    const alphabet =
        '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';
    String part(int n) =>
        [for (var i = 0; i < n; i++) alphabet[random.nextInt(alphabet.length)]]
            .join();
    final value = 'bothy_hpc_${part(43)}_${part(6)}';
    h.pairingCode = value;
    return {
      'host_id': h.id,
      'pairing_code': value,
      'expires_at': world.stamp(world.now().add(const Duration(minutes: 15))),
    };
  }

  void audit(FakeRequest r, FakeHost h, JsonMap details) {
    world.auditEvents.add({
      'seq': world.auditEvents.length + 1,
      'id': world.nextId(),
      'occurred_at': world.stamp(),
      'actor_principal_id': r.person.id,
      'action': 'host.manage',
      'decision': 'allow',
      'resource_kind': 'host',
      'resource_id': h.id,
      'details': details,
      'prev_hash': '00',
      'hash': '00',
    });
  }

  /// I6 (24 7.3): with a personal login only the holder may hold a valid grant on [h].
  void i6(FakeHost h, String credentials, String? holder) {
    if (credentials != 'personal') return;
    final others = world.grants.values.any(
      (g) =>
          g['revoked_at'] == null &&
          g['target_kind'] == 'host' &&
          g['target_id'] == h.id &&
          (g['subject_kind'] != 'user' || g['subject_id'] != holder),
    );
    if (others) {
      throw const FakeProblem(
        409,
        'i6_violation',
        extra: {
          'hint': 'Revoke the other grants on the host or declare organization first.',
        },
      );
    }
  }

  /// The checks of new assignments (24 2.2, T12, TM-T28): reported names only, a name of
  /// another person or a second valid assignment is `session_user_taken`.
  List<(String, String)> assignments(FakeHost h, Object? list, String pointer) {
    final out = <(String, String)>[];
    final items = list is List ? list : const <Object?>[];
    for (var i = 0; i < items.length; i++) {
      final a = items[i]! as JsonMap;
      final user = a['user_id']! as String;
      final name = a['os_user']! as String;
      if (!(h.reported ?? const []).contains(name)) {
        throw FakeProblem(
          400,
          'validation_failed',
          errors: [
            {
              'pointer': '$pointer/$i/os_user',
              'code': 'session_user_not_on_host',
            },
          ],
        );
      }
      final bound = h.taken[name];
      if ((bound != null && bound != user) ||
          h.sessionUsers.containsKey(user)) {
        throw const FakeProblem(409, 'conflict', reason: 'session_user_taken');
      }
      out.add((user, name));
    }
    return out;
  }

  void assign(
    FakeRequest r,
    FakeHost h,
    List<(String, String)> add,
    String reason,
  ) {
    for (final (user, name) in add) {
      h.sessionUsers[user] = (name, world.stamp());
      h.taken[name] = user;
      audit(r, h, {
        'operation': 'session_user',
        'change': 'assigned',
        'user_id': user,
        'os_user': name,
        'reason': reason,
      });
    }
  }

  rest
    ..handle('listHosts', (r) {
      final me = r.person;
      final hosts = [
        for (final h in world.hosts.values)
          if (me.isOwner ||
              h.assignedUserId == me.id ||
              world.grants.values.any(
                (g) =>
                    g['revoked_at'] == null &&
                    g['target_kind'] == 'host' &&
                    g['target_id'] == h.id &&
                    g['subject_id'] == me.id,
              ))
            h,
      ]..sort((a, b) => a.name.compareTo(b.name));
      return FakeResponse(200, {
        'items': [for (final h in hosts) h.json],
      });
    })
    ..handle('getHost', (r) => FakeResponse(200, hostOf(r).json))
    ..handle('createHost', (r) {
      owner(r);
      stepUp(r);
      final body = r.json;
      final h = FakeHost(
        id: world.nextId(),
        name: body['name']! as String,
        assignedUserId: body['assigned_user_id'] as String?,
        createdAt: world.stamp(),
      );
      world.hosts[h.id] = h;
      return FakeResponse(201, code(h));
    })
    ..handle('issueHostPairingCode', (r) {
      owner(r);
      final h = hostOf(r);
      stepUp(r);
      if (h.state == 'retired') {
        throw const FakeProblem(409, 'invalid_state_transition');
      }
      h
        ..state = 'pairing'
        ..disabledReason = null
        ..fingerprint = null
        ..failures = 0
        ..connected = false
        ..version += 1;
      world.permissionVersion++;
      return FakeResponse(201, code(h));
    })
    ..handle('confirmHost', (r) {
      owner(r);
      final h = hostOf(r);
      stepUp(r);
      ifMatch(r, h);
      if (h.state != 'confirming') {
        throw const FakeProblem(409, 'invalid_state_transition');
      }
      final body = r.json;
      final prefix = (body['fingerprint_prefix']! as String)
          .replaceAll(RegExp(r'[\s-]'), '')
          .toUpperCase();
      if (prefix.length != _confirmChars) {
        throw const FakeProblem(
          400,
          'validation_failed',
          errors: [
            {'pointer': '/fingerprint_prefix', 'code': 'invalid_format'},
          ],
        );
      }
      if (!(h.fingerprint ?? '').replaceAll('-', '').startsWith(prefix)) {
        h.failures += 1;
        final left = _maxAttempts - h.failures;
        if (left <= 0) {
          h
            ..state = 'pairing'
            ..fingerprint = null
            ..failures = 0
            ..connected = false
            ..version += 1;
        }
        throw FakeProblem(
          409,
          'fingerprint_mismatch',
          extra: {'attempts_remaining': left < 0 ? 0 : left},
        );
      }
      final credentials = body['llm_credentials']! as String;
      final holder = body['credential_holder_id'] as String?;
      i6(h, credentials, holder);
      final reason = body['reason']! as String;
      final add = assignments(h, body['session_users'], '/session_users');
      assign(r, h, add, reason);
      h
        ..state = 'active'
        ..failures = 0
        ..llmCredentials = credentials
        ..credentialHolderId = holder
        ..declaredServerHost = body['declared_server_host']! as bool
        ..version += 1;
      world.permissionVersion++;
      return FakeResponse(200, h.json);
    })
    ..handle('updateHost', (r) {
      owner(r);
      final h = hostOf(r);
      stepUp(r);
      ifMatch(r, h);
      final body = r.json;
      final reason = body['reason']! as String;
      final add = assignments(
        h,
        body['add_session_users'],
        '/add_session_users',
      );
      for (final user in (body['revoke_session_users'] as List? ?? const [])) {
        final ended = h.sessionUsers.remove(user);
        if (ended != null) {
          audit(r, h, {
            'operation': 'session_user',
            'change': 'revoked',
            'user_id': user,
            'os_user': ended.$1,
            'reason': reason,
          });
        }
      }
      assign(r, h, add, reason);
      if (body['name'] case final String name) h.name = name;
      if (body['assigned_user_id'] case final String user) {
        h.assignedUserId = user;
      }
      if (body['clear_assigned_user'] == true) h.assignedUserId = null;
      h.version += 1;
      world.permissionVersion++;
      return FakeResponse(200, h.json);
    })
    ..handle('setHostLlmCredentials', (r) {
      owner(r);
      final h = hostOf(r);
      stepUp(r);
      ifMatch(r, h);
      final body = r.json;
      final credentials = body['llm_credentials']! as String;
      final holder = body['credential_holder_id'] as String?;
      i6(h, credentials, holder);
      h
        ..llmCredentials = credentials
        ..credentialHolderId = holder
        ..version += 1;
      world.permissionVersion++;
      return FakeResponse(200, h.json);
    })
    ..handle('setHostState', (r) {
      owner(r);
      final h = hostOf(r);
      stepUp(r);
      ifMatch(r, h);
      final to = r.json['state']! as String;
      final allowed = switch (to) {
        'active' => h.state == 'disabled' && h.disabledReason == 'owner',
        'disabled' => h.state == 'active',
        _ => h.state == 'active' || h.state == 'disabled',
      };
      if (!allowed) throw const FakeProblem(409, 'invalid_state_transition');
      h
        ..state = to
        ..disabledReason = to == 'disabled' ? 'owner' : null
        ..version += 1;
      world.permissionVersion++;
      return FakeResponse(200, h.json);
    })
    ..handle(
      'listTerminalSessions',
      (r) => const FakeResponse(200, {'items': <Object?>[]}),
    )
    ..handle('listAuditEvents', (r) {
      owner(r);
      stepUp(r);
      final action = r.query('action');
      final resource = r.query('resource_id');
      return FakeResponse(200, {
        'items': [
          for (final e in world.auditEvents.reversed)
            if ((action == null || e['action'] == action) &&
                (resource == null || e['resource_id'] == resource))
              e,
        ],
      });
    });
}
