// SPDX-License-Identifier: Apache-2.0

/// Handlers of the own devices, sessions and profile and of the step-up with the device key or,
/// on a device without a step-up key, with password and TOTP (brief M4-25 Scope; 08 5.4, 6.2,
/// 10; 05 `device.approve`, `device.revoke` with step-up, `session.revoke` without).
library;

import '../dpop_verifier.dart';
import '../rest_fake.dart';
import '../world.dart';

/// How long one confirmation of a session counts (`auth.step_up_max_age`).
const Duration stepUpMaxAge = Duration(minutes: 5);

/// How long one confirmation counts for the terminal's fresh step-up
/// (`terminal.step_up_max_age`, 24 section 17, 08 6.3).
const Duration terminalStepUpMaxAge = Duration(seconds: 60);

/// Throws `401 step_up_required` unless the session of [r] confirmed within [stepUpMaxAge];
/// with [fresh] (the terminal, 08 6.3) only a confirmation with the device key within
/// [terminalStepUpMaxAge] counts, so a missing or older one or a step-up with password and TOTP
/// answers `401 step_up_required` with reason `terminal_step_up_not_fresh` (07 section 4.2 step 8).
void requireStepUp(FakeRequest r, {bool fresh = false}) {
  final session = r.session!;
  final at = session.stepUpAt;
  final age = at == null ? null : r.world.now().difference(at);
  if (fresh &&
      (age == null ||
          session.stepUpMethod != 'device_key' ||
          age > terminalStepUpMaxAge)) {
    throw const FakeProblem(
      401,
      'step_up_required',
      reason: 'terminal_step_up_not_fresh',
    );
  }
  if (age == null || age > stepUpMaxAge) {
    throw const FakeProblem(401, 'step_up_required');
  }
}

/// Registers the self-service and step-up handlers on [rest].
void registerSelfHandlers(RestFake rest) {
  final world = rest.world;

  /// The open challenge [id] of the session of [r], removed (each answer ends it); `401
  /// step_up_required` for an unknown one or one of another session.
  void takeChallenge(FakeRequest r, Object? id) {
    final open = world.stepUpChallenges.remove(id);
    if (open == null || open.sessionId != r.session!.id) {
      throw const FakeProblem(401, 'step_up_required');
    }
  }

  /// Sets the step-up of the session of [r] with [method] and answers `StepUpDone`.
  FakeResponse confirmed(FakeRequest r, String method) {
    final now = world.now();
    r.session!
      ..stepUpAt = now
      ..stepUpMethod = method;
    return FakeResponse(200, {
      'step_up_at': world.stamp(now),
      'valid_until': world.stamp(now.add(stepUpMaxAge)),
    });
  }

  rest
    ..handle('createStepUpChallenge', (r) {
      final session = r.session!;
      // A cli device has no step-up key and no user verification (08 6.4).
      if (world.devices[session.deviceId]?.platform == 'cli') {
        throw const FakeProblem(
          403,
          'forbidden',
          reason: 'step_up_not_possible',
        );
      }
      final id = world.nextId();
      final challenge = 'c${id.replaceAll('-', '')}';
      world.stepUpChallenges[id] = (
        challenge: challenge,
        sessionId: session.id,
      );
      return FakeResponse(200, {
        'challenge_id': id,
        'challenge': challenge,
        'expires_at': world.stamp(world.now().add(const Duration(minutes: 2))),
      });
    })
    ..handle('completeStepUpWithDeviceKey', (r) {
      final session = r.session!;
      final jwk = world.devices[session.deviceId]!.stepUpJwk;
      if (jwk == null) {
        throw const FakeProblem(
          403,
          'forbidden',
          reason: 'step_up_not_possible',
        );
      }
      final open = world.stepUpChallenges.remove(r.json['challenge_id']);
      if (open == null || open.sessionId != session.id) {
        throw const FakeProblem(401, 'step_up_required');
      }
      final payload = DpopVerifier.verifyCompactJws(
        jwk,
        r.json['signature']! as String,
      );
      if (payload == null ||
          payload['challenge'] != open.challenge ||
          payload['sid'] != session.id) {
        throw const FakeProblem(401, 'step_up_required');
      }
      return confirmed(r, 'device_key');
    })
    // Password with TOTP (08 6.2): an app session on a device without a step-up key, of a
    // person with a local account; a device with a step-up key and a cli device keep their
    // refusal. The fake has no console sessions.
    ..handle('stepUpWithPassword', (r) {
      final session = r.session!;
      final person = r.person;
      final device = world.devices[session.deviceId]!;
      if (session.clientComponent != null ||
          device.platform == 'cli' ||
          device.stepUpJwk != null ||
          person.password == null) {
        throw const FakeProblem(
          403,
          'forbidden',
          reason: 'step_up_not_possible',
        );
      }
      takeChallenge(r, r.json['challenge_id']);
      if (r.json['password'] != person.password ||
          r.json['totp_code'] != person.totpCode) {
        throw const FakeProblem(401, 'step_up_required');
      }
      return confirmed(r, 'password_totp');
    })
    ..handle(
      'listMyDevices',
      (r) => FakeResponse(200, {
        'items': [
          for (final d in world.devices.values)
            if (d.userId == r.person.id)
              d.toJson(self: d.id == r.session!.deviceId),
        ],
      }),
    )
    ..handle('revokeDevice', (r) {
      final device = world.devices[r.param('device_id')];
      if (device == null ||
          (device.userId != r.person.id && !r.person.isOwner)) {
        throw const FakeProblem(404, 'not_found');
      }
      requireStepUp(r);
      if (device.state != 'revoked') {
        world.revokeDevice(
          device.id,
          reason: device.userId == r.person.id
              ? 'revoked_by_self'
              : 'revoked_by_owner',
        );
      }
      return FakeResponse(200, device.toJson());
    })
    ..handle(
      'listMySessions',
      (r) => FakeResponse(200, {
        'items': [
          for (final s in world.sessions.values)
            if (s.userId == r.person.id && !s.revoked)
              s.toJson(current: s.id == r.session!.id),
        ],
      }),
    )
    ..handle('revokeSession', (r) {
      final session = world.sessions[r.param('session_id')];
      if (session == null ||
          (session.userId != r.person.id && !r.person.isOwner)) {
        throw const FakeProblem(404, 'not_found');
      }
      // `session.revoke`: self or owner (05), never a step-up.
      world.revokeSession(
        session.id,
        reason: session.userId == r.person.id ? 'logout' : 'admin',
      );
      return const FakeResponse.noContent();
    })
    ..handle('updateMyProfile', (r) {
      final person = r.person;
      final body = r.json;
      person
        ..locale = body['locale'] as String? ?? person.locale
        ..timeZone = body['time_zone'] as String? ?? person.timeZone;
      return FakeResponse(200, _user(world, person));
    });
}

JsonMap _user(FakeWorld world, FakePerson person) => {
  'id': person.id,
  'handle': person.handle,
  'display_name': person.displayName,
  'status': 'active',
  'is_owner': person.isOwner,
  'locale': ?person.locale,
  'time_zone': ?person.timeZone,
  'created_at': world.stamp(DateTime.utc(2026, 10)),
};
