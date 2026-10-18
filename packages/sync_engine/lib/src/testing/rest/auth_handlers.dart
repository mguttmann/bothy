// SPDX-License-Identifier: Apache-2.0

/// Handlers of discovery, login, pairing, device approval, refresh, logout, `getMe` and
/// `getInstance` (brief M4-17 Design note 3; 08 5, 08 7, 08 14.1 to 14.3 in a reduced form).
library;

import 'dart:convert';

import '../dpop_verifier.dart';
import '../rest_fake.dart';
import '../world.dart';
import 'self_handlers.dart' show requireStepUp;

/// Registers the auth and instance handlers on [rest]; access tokens live [accessTokenTtl]
/// (`auth.access_token_ttl`).
///
/// Simplifications (README): a password login of a person without
/// `newDevicesPending` gets an active device; the TOTP code is a fixed value of the person.
/// `approveDevice` and `approvePairing` need a fresh step-up of the session (08 5.2, 5.3).
void registerAuthHandlers(RestFake rest, {required Duration accessTokenTtl}) {
  final world = rest.world;
  final tokens = _Tokens(world, accessTokenTtl);

  rest
    ..handle('getDiscovery', (r) {
      final gateway = r.baseUri.replace(
        scheme: r.baseUri.scheme == 'https' ? 'wss' : 'ws',
        path: '/api/v1/gateway',
      );
      return FakeResponse(200, {
        'instance_id': world.instanceId,
        'setup_required': false,
        'api_versions': ['v1'],
        'spec_version': '1',
        'features': <Object?>[],
        'gateway': {
          'url': gateway.toString(),
          'protocol_versions': [1],
        },
        'branding': {'product_name': world.productName},
        'auth_providers': <Object?>[],
        'local_password_enabled': true,
        'jwks_uri': '/api/v1/auth/jwks',
        'webhook_keys_uri': '/api/v1/webhooks/keys',
      });
    })
    ..handle('loginWithPassword', (r) {
      final body = r.json;
      final person = world.people.values
          .where(
            (p) => p.loginName != null && p.loginName == body['login_name'],
          )
          .firstOrNull;
      if (person == null ||
          person.password != body['password'] ||
          person.totpCode != body['totp_code']) {
        throw const FakeProblem(401, 'unauthenticated');
      }
      final registration = body['device']! as JsonMap;
      final device = _register(
        world,
        r,
        person.id,
        registration,
        state: person.newDevicesPending ? 'pending' : 'active',
      );
      final session = tokens.open(person.id, device, 'password_totp');
      return FakeResponse(201, tokens.response(session, device));
    })
    ..handle('beginPairing', (r) {
      final registration = r.json['device']! as JsonMap;
      final jkt = _checkKey(r, registration);
      final id = world.nextId();
      final pairing = FakePairing(
        id: id,
        code: _pairingCode(id),
        registration: registration,
        jkt: jkt,
        expiresAt: world.stamp(world.now().add(const Duration(minutes: 5))),
      );
      world.pairings[id] = pairing;
      return FakeResponse(201, {
        'pairing_id': id,
        'pairing_code': pairing.code,
        'expires_at': pairing.expiresAt,
      });
    })
    ..handle('approvePairing', (r) {
      requireStepUp(r);
      final code = r.json['pairing_code'];
      final pairing = world.pairings.values
          .where((p) => p.code == code && p.state == 'requested')
          .firstOrNull;
      if (pairing == null) {
        // A code without an open pairing ends the step-up of the session (08 5.3).
        r.session!.stepUpAt = null;
        throw const FakeProblem(404, 'not_found');
      }
      pairing
        ..state = 'approved'
        ..approvedBy = r.person.id;
      final device = _register(
        world,
        null,
        r.person.id,
        pairing.registration,
        state: 'active',
        jkt: pairing.jkt,
      );
      return FakeResponse(201, device.toJson());
    })
    ..handle('completePairing', (r) {
      final pairing = world.pairings[r.json['pairing_id']];
      if (pairing == null) throw const FakeProblem(404, 'not_found');
      if (pairing.state == 'expired' || pairing.state == 'completed') {
        throw const FakeProblem(410, 'gone');
      }
      if (r.proofJkt != pairing.jkt) {
        // A proof with a different key at pickup rejects the pairing (08 5.3).
        pairing.state = 'expired';
        throw const FakeProblem(401, 'invalid_dpop_proof');
      }
      if (pairing.state == 'requested') {
        return FakeResponse(202, {
          'pairing_id': pairing.id,
          'status': 'requested',
          'expires_at': pairing.expiresAt,
        });
      }
      pairing.state = 'completed';
      final device = world.devices[(pairing.registration['device_id'])]!;
      final session = tokens.open(
        pairing.approvedBy!,
        device,
        'device_pairing',
      );
      return FakeResponse(201, tokens.response(session, device));
    })
    ..handle('refreshToken', (r) {
      final token = r.json['refresh_token'];
      final session = world.sessions.values
          .where((s) => s.refreshToken == token)
          .firstOrNull;
      if (session == null || session.revoked) {
        throw const FakeProblem(401, 'session_revoked');
      }
      if (r.proofJkt != session.jkt) {
        // A refresh with another key than the family's (08 13).
        throw const FakeProblem(401, 'invalid_dpop_proof');
      }
      tokens.rotate(session);
      return FakeResponse(
        200,
        tokens.response(session, world.devices[session.deviceId]!),
      );
    })
    ..handle('logout', (r) {
      world.revokeSession(r.session!.id, reason: 'logout');
      return const FakeResponse.noContent();
    })
    ..handle('getDevice', (r) {
      final device = world.devices[r.param('device_id')];
      if (device == null || device.userId != r.person.id) {
        throw const FakeProblem(404, 'not_found');
      }
      return FakeResponse(
        200,
        device.toJson(self: device.id == r.session!.deviceId),
      );
    })
    ..handle('approveDevice', (r) {
      final device = world.devices[r.param('device_id')];
      if (device == null ||
          (device.userId != r.person.id && !r.person.isOwner)) {
        throw const FakeProblem(404, 'not_found');
      }
      requireStepUp(r);
      if (device.state != 'pending') throw const FakeProblem(409, 'conflict');
      if (r.json['verification_code'] == device.verificationCode) {
        world.approveDevice(device.id);
      } else {
        // A wrong code revokes the waiting device; the request took effect (08 5.2).
        world.rejectPendingDevice(device.id);
      }
      return FakeResponse(200, device.toJson());
    })
    ..handle('getMe', (r) {
      final person = r.person;
      return FakeResponse(200, {
        'principal': person.ref,
        'is_owner': person.isOwner,
        'locale': ?person.locale,
        'time_zone': ?person.timeZone,
        'permission_version': world.permissionVersion,
        'session_id': r.session!.id,
        'device_id': r.session!.deviceId,
      });
    })
    ..handle(
      'getInstance',
      (r) => FakeResponse(200, {
        'instance_id': world.instanceId,
        'branding': {'product_name': world.productName, 'version': 1},
        'locale_default': 'en',
        'spec_version': '1',
        'features': <Object?>[],
        'limits': {
          'message_max_content_bytes': 65536,
          'message_max_attachments': 10,
          'file_max_upload_bytes': 104857600,
          'thread_title_max_chars': 120,
          'voice_max_duration_ms': 600000,
        },
      }),
    );
}

/// The thumbprint of the registered key, which must be the key of the request's proof.
String _checkKey(FakeRequest r, JsonMap registration) {
  final jwk = (registration['public_jwk']! as JsonMap).cast<String, Object?>();
  final jkt = DpopVerifier.thumbprint(jwk);
  if (r.proofJkt != jkt) throw const FakeProblem(401, 'invalid_dpop_proof');
  return jkt;
}

/// Registers the device of [registration] for [userId], or returns the known one with the
/// same ID and key.
FakeDevice _register(
  FakeWorld world,
  FakeRequest? r,
  String userId,
  JsonMap registration, {
  required String state,
  String? jkt,
}) {
  final key = jkt ?? _checkKey(r!, registration);
  final id = registration['device_id']! as String;
  final known = world.devices[id];
  if (known != null) {
    if (known.jkt != key || known.userId != userId) {
      throw const FakeProblem(409, 'conflict');
    }
    return known;
  }
  final device = FakeDevice(
    id: id,
    userId: userId,
    name: registration['name']! as String,
    platform: registration['platform']! as String,
    appVersion: registration['app_version'] as String?,
    state: state,
    jkt: key,
    createdAt: world.stamp(),
  );
  if (state == 'active') device.approvedAt = device.createdAt;
  final stepUp = registration['step_up_key'];
  if (stepUp is JsonMap && stepUp['public_jwk'] is JsonMap) {
    device.stepUpJwk = (stepUp['public_jwk']! as JsonMap)
        .cast<String, Object?>();
  }
  world.devices[id] = device;
  return device;
}

/// An 8-character Crockford Base32 code derived from the pairing ID (08 18).
String _pairingCode(String id) {
  const alphabet = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';
  final digits = id.replaceAll('-', '');
  var n = BigInt.parse(digits.substring(digits.length - 10), radix: 16);
  final out = StringBuffer();
  for (var i = 0; i < 8; i++) {
    out.write(alphabet[(n % BigInt.from(32)).toInt()]);
    n = n ~/ BigInt.from(32);
  }
  return out.toString();
}

/// Issues the tokens of the fake: JWS-shaped access tokens whose payload carries `sub`,
/// `sid`, `did`, `exp` and `cnf.jkt` (unsigned: the fake recognizes its own tokens by value),
/// and opaque refresh tokens.
final class _Tokens {
  _Tokens(this._world, this._ttl);

  final FakeWorld _world;
  final Duration _ttl;

  FakeSession open(String userId, FakeDevice device, String method) {
    final now = _world.now();
    final session = FakeSession(
      id: _world.nextId(),
      userId: userId,
      deviceId: device.id,
      authMethod: method,
      createdAt: _world.stamp(now),
      expiresAt: _world.stamp(now.add(const Duration(days: 30))),
      jkt: device.jkt,
    );
    _world.sessions[session.id] = session;
    rotate(session);
    return session;
  }

  void rotate(FakeSession session) {
    if (session.accessToken.isNotEmpty) {
      session.olderAccessTokens.add(session.accessToken);
    }
    final exp = _world.now().add(_ttl).millisecondsSinceEpoch ~/ 1000;
    String enc(JsonMap m) =>
        base64Url.encode(utf8.encode(jsonEncode(m))).replaceAll('=', '');
    session
      ..accessToken =
          '${enc({'alg': 'none', 'typ': 'at+jwt'})}.'
          '${enc({
            'sub': session.userId,
            'sid': session.id,
            'did': session.deviceId,
            'jti': _world.nextId(),
            'exp': exp,
            'cnf': {'jkt': session.jkt},
          })}.'
          'ZmFrZQ'
      ..refreshToken = 'rt-${_world.nextId()}';
  }

  JsonMap response(FakeSession session, FakeDevice device) => {
    'access_token': session.accessToken,
    'token_type': 'DPoP',
    'expires_in': _ttl.inSeconds,
    'refresh_token': session.refreshToken,
    'refresh_expires_at': session.expiresAt,
    'session': session.toJson(),
    'device': device.toJson(self: true),
  };
}
