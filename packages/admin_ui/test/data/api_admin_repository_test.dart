// SPDX-License-Identifier: Apache-2.0

// The REST source of the administration (brief M4-26 Design note 1): every call maps the wire
// shapes of docs/api/openapi.yaml to the types of admin_ui, sends a write with an idempotency
// key and the step-up purpose, follows the cursor of a list and turns a problem answer into
// SelfActionFailed. A routing adapter answers by method and path; no socket is opened.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

typedef Seen = ({RequestOptions options, Object? json});

/// Answers each request with the JSON of [routes] by `METHOD path?query` or `METHOD path`
/// (status 200 unless the route gives one); records every request.
class RoutingAdapter implements HttpClientAdapter {
  RoutingAdapter(this.routes);

  final Map<String, (int, Object?)> routes;
  final List<Seen> seen = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final bytes = <int>[];
    if (requestStream != null) {
      await for (final chunk in requestStream) {
        bytes.addAll(chunk);
      }
    }
    final body = utf8.decode(bytes);
    seen.add((options: options, json: body.isEmpty ? null : jsonDecode(body)));
    final path = '${options.method} ${options.uri.path}';
    final withQuery = options.uri.query.isEmpty
        ? path
        : '$path?${options.uri.query}';
    final (status, answer) =
        routes[withQuery] ??
        routes[path] ??
        (throw StateError('no route for $withQuery'));
    if (answer == null) return ResponseBody.fromString('', status);
    return ResponseBody.fromString(
      jsonEncode(answer),
      status,
      headers: {
        Headers.contentTypeHeader: [
          status >= 400 ? 'application/problem+json' : 'application/json',
        ],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

const String _alex = '0192f4a0-7b1c-7f89-8d6b-6960453241bc';
const String _sam = '0192f4a0-7b1c-733c-835c-6d67630008cd';
const String _robin = '0192f4a0-7b1c-7c2a-8e1d-41a6b2c3d4e5';
const String _group = '0192f4a0-7b1c-7aaa-8bbb-6960453241bc';
const String _laptop = '0192f4a0-7b1c-7d07-84b0-358556a08a99';
const String _tablet = '0192f4a0-7b1c-7fd5-831e-d4370e087874';
const String _session = '0192f4a0-7b1c-72d4-8bae-7661d2b2f203';
const String _identity = '0192f4a0-7b1c-727f-86bd-90fe7bdb3024';
const String _passkey = '0192f4a0-7b1c-7e01-8f02-000000000001';
const String _provider = '0192f4a0-7b1c-7e01-8f02-000000000002';
const String _at = '2026-10-10T09:30:00.000Z';
const String _later = '2026-10-10T10:00:00.000Z';

Map<String, Object?> _user(
  String id,
  String name, {
  String status = 'active',
}) => {
  'id': id,
  'handle': name.toLowerCase(),
  'display_name': name,
  'status': status,
  'is_owner': id == _alex,
  'created_at': _at,
};

Map<String, Object?> _ref(String id, String name) => {
  'id': id,
  'kind': 'user',
  'handle': name.toLowerCase(),
  'display_name': name,
};

Map<String, Object?> _device(String id, String name, String state, String at) =>
    {
      'id': id,
      'user_id': _sam,
      'name': name,
      'platform': 'macos',
      'state': state,
      'key_thumbprint': 'thumb-$id',
      'created_at': at,
    };

Map<String, Object?> _groupJson({String name = 'Support'}) => {
  'id': _group,
  'name': name,
  'description': 'First line',
  'source': 'local',
  'created_at': _at,
  'member_count': 1,
};

Map<String, Object?> _code = {
  'code': 'en_ABCD-EFGH-JKMN',
  'purpose': 'first_login',
  'expires_at': _later,
};

Map<String, (int, Object?)> _routes() => {
  'GET /api/v1/users?q=sam&status=active': (
    200,
    {
      'items': [_user(_sam, 'Sam')],
      'next_cursor': 'c2',
    },
  ),
  'GET /api/v1/users?cursor=c2': (
    200,
    {
      'items': [_user(_robin, 'Robin', status: 'disabled')],
    },
  ),
  'GET /api/v1/users': (
    200,
    {
      'items': [
        _user(_alex, 'Alex'),
        _user(_sam, 'Sam'),
        _user(_robin, 'Robin', status: 'disabled'),
      ],
    },
  ),
  'GET /api/v1/users/$_sam': (200, _user(_sam, 'Sam')),
  'POST /api/v1/users': (
    201,
    {'user': _user(_sam, 'Sam', status: 'pending'), 'enrollment': _code},
  ),
  'POST /api/v1/users/$_sam/disable': (
    200,
    _user(_sam, 'Sam', status: 'disabled'),
  ),
  'POST /api/v1/users/$_sam/enable': (200, _user(_sam, 'Sam')),
  'POST /api/v1/users/$_sam/enrollment-codes': (201, _code),
  'GET /api/v1/users/$_sam/credentials': (
    200,
    {
      'identities': [
        {
          'id': _identity,
          'provider_id': _provider,
          'provider_kind': 'oidc',
          'subject': 'sam@example.com',
          'issuer': 'https://login.example.com',
          'linked_at': _at,
        },
      ],
      'passkeys': [
        {
          'id': _passkey,
          'name': 'Laptop',
          'backup_eligible': true,
          'created_at': _at,
        },
      ],
      'local_password_set': true,
      'totp_enrolled': false,
    },
  ),
  'DELETE /api/v1/users/$_sam/identities/$_identity': (204, null),
  'DELETE /api/v1/users/$_sam/passkeys/$_passkey': (204, null),
  'GET /api/v1/users/$_sam/devices': (
    200,
    {
      'items': [_device(_laptop, 'Laptop', 'active', _at)],
      'next_cursor': 'd2',
    },
  ),
  'GET /api/v1/users/$_sam/devices?cursor=d2': (
    200,
    {
      'items': [_device(_tablet, 'Tablet', 'pending', _later)],
    },
  ),
  'GET /api/v1/users/$_sam/sessions': (
    200,
    {
      'items': [
        {
          'id': _session,
          'user_id': _sam,
          'device_id': _tablet,
          'auth_method': 'password_totp',
          'created_at': _later,
          'expires_at': '2026-11-10T10:00:00.000Z',
        },
      ],
    },
  ),
  'GET /api/v1/users/$_alex/devices': (200, {'items': <Object?>[]}),
  'POST /api/v1/sessions/$_session/revoke': (204, null),
  'POST /api/v1/devices/$_tablet/approve': (
    200,
    _device(_tablet, 'Tablet', 'active', _later),
  ),
  'POST /api/v1/devices/$_tablet/revoke': (
    200,
    _device(_tablet, 'Tablet', 'revoked', _later),
  ),
  'GET /api/v1/owners': (
    200,
    {
      'items': [
        {'user': _ref(_alex, 'Alex'), 'granted_at': _at, 'reason': 'Founder'},
      ],
    },
  ),
  'POST /api/v1/owners': (
    201,
    {
      'user': _ref(_sam, 'Sam'),
      'granted_by': _alex,
      'granted_at': _later,
      'reason': 'Deputy',
    },
  ),
  'POST /api/v1/owners/$_sam/revoke': (204, null),
  'GET /api/v1/groups': (
    200,
    {
      'items': [_groupJson()],
    },
  ),
  'GET /api/v1/groups/$_group': (200, _groupJson()),
  'POST /api/v1/groups': (201, _groupJson()),
  'PATCH /api/v1/groups/$_group': (200, _groupJson(name: 'Second line')),
  'DELETE /api/v1/groups/$_group': (204, null),
  'GET /api/v1/groups/$_group/members': (
    200,
    {
      'items': [
        {
          'user': _ref(_sam, 'Sam'),
          'added_by': _alex,
          'added_at': _at,
          'expires_at': _later,
        },
      ],
    },
  ),
  'POST /api/v1/groups/$_group/members': (
    201,
    {'user': _ref(_robin, 'Robin'), 'added_by': _alex, 'added_at': _at},
  ),
  'DELETE /api/v1/groups/$_group/members/$_robin': (204, null),
};

void main() {
  late RoutingAdapter adapter;
  late AdminRepository repository;
  var keys = 0;

  setUp(() {
    adapter = RoutingAdapter(_routes());
    keys = 0;
    final client = createApiClient(
      baseUrl: Uri.parse('https://chat.example.com'),
      traceContext: () => TraceContext(
        traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
        sampled: true,
      ),
      readRetry: ReadRetryPolicy.fromBackoff(
        base: const Duration(milliseconds: 1),
        cap: const Duration(milliseconds: 1),
        sleep: (_) async {},
      ),
      adapter: adapter,
    );
    repository = ApiAdminRepository(
      client,
      newIdempotencyKey: () => '0192f4a0-0000-7000-8000-00000000000${keys++}',
    );
  });

  Seen last(String method, String path) => adapter.seen.lastWhere(
    (s) => s.options.method == method && s.options.uri.path == path,
  );

  void expectWrite(String method, String path, String purpose) {
    final s = last(method, path);
    expect(s.options.extra[stepUpPurposeExtra], purpose);
  }

  test('people: query, status, the next page and one person', () async {
    final page = await repository.people(
      query: 'sam',
      status: AdminPersonStatus.active,
    );
    expect(page.items.single.displayName, 'Sam');
    expect(page.nextCursor, 'c2');
    final next = await repository.people(cursor: 'c2');
    expect(next.items.single.status, AdminPersonStatus.disabled);
    final sam = await repository.person(_sam);
    expect(sam.handle, 'sam');
    expect(sam.isOwner, isFalse);
    expect((await allPeople(repository)).map((p) => p.id), [
      _alex,
      _sam,
      _robin,
    ]);
  });

  test('a new person with its code, disable, enable and a new code', () async {
    final created = await repository.createPerson(
      handle: 'sam',
      displayName: 'Sam',
      localAccount: true,
      purpose: 'Add Sam',
    );
    expect(created.person.status, AdminPersonStatus.pending);
    expect(created.enrollment!.code, 'en_ABCD-EFGH-JKMN');
    final sent = last('POST', '/api/v1/users');
    expect(sent.json, {
      'handle': 'sam',
      'display_name': 'Sam',
      'local_account': true,
    });
    expect(sent.options.headers['Idempotency-Key'], isNotNull);
    expectWrite('POST', '/api/v1/users', 'Add Sam');

    final disabled = await repository.disablePerson(
      _sam,
      reason: 'Left',
      purpose: 'Disable Sam',
    );
    expect(disabled.status, AdminPersonStatus.disabled);
    expect(last('POST', '/api/v1/users/$_sam/disable').json, {
      'reason': 'Left',
    });
    final enabled = await repository.enablePerson(
      _sam,
      reason: 'Back',
      purpose: 'Enable Sam',
    );
    expect(enabled.status, AdminPersonStatus.active);
    final code = await repository.newEnrollmentCode(_sam, purpose: 'Code');
    expect(code.code, 'en_ABCD-EFGH-JKMN');
    expectWrite('POST', '/api/v1/users/$_sam/enrollment-codes', 'Code');
  });

  test('credentials, unlink an identity and revoke a passkey', () async {
    final c = await repository.credentials(_sam);
    expect(c.identities.single.providerKind, 'oidc');
    expect(c.identities.single.subject, 'sam@example.com');
    expect(c.passkeys.single.name, 'Laptop');
    expect(c.localPasswordSet, isTrue);
    expect(c.totpEnrolled, isFalse);
    await repository.unlinkIdentity(_sam, _identity, purpose: 'Unlink');
    expectWrite(
      'DELETE',
      '/api/v1/users/$_sam/identities/$_identity',
      'Unlink',
    );
    await repository.revokePasskey(_sam, _passkey, purpose: 'Passkey');
    expectWrite('DELETE', '/api/v1/users/$_sam/passkeys/$_passkey', 'Passkey');
  });

  test('devices over every page, sessions with device names, end, approve and revoke', () async {
    final devices = await repository.devicesOf(_sam);
    expect(devices.map((d) => d.name), ['Laptop', 'Tablet']);
    final sessions = await repository.sessionsOf(_sam);
    expect(sessions.single.deviceName, 'Tablet');
    expect(sessions.single.authMethod, SelfAuthMethod.passwordTotp);
    await repository.endSession(_session);
    expect(
      last(
        'POST',
        '/api/v1/sessions/$_session/revoke',
      ).options.headers['Idempotency-Key'],
      isNotNull,
    );
    final approved = await repository.approveDevice(
      _tablet,
      '123456',
      purpose: 'Approve',
    );
    expect(approved.state, SelfDeviceState.active);
    expect(last('POST', '/api/v1/devices/$_tablet/approve').json, {
      'verification_code': '123456',
    });
    final revoked = await repository.revokeDevice(
      _tablet,
      reason: 'Lost',
      purpose: 'Revoke',
    );
    expect(revoked.state, SelfDeviceState.revoked);
  });

  test(
    'the waiting devices of active people with the origin of the login',
    () async {
      final waiting = await repository.waitingDevices();
      expect(waiting.single.person.id, _sam);
      expect(waiting.single.device.id, _tablet);
      expect(waiting.single.origin, SelfAuthMethod.passwordTotp);
    },
  );

  test('owners: list, add and revoke', () async {
    final owners = await repository.owners();
    expect(owners.single.displayName, 'Alex');
    expect(owners.single.reason, 'Founder');
    await repository.addOwner(_sam, reason: 'Deputy', purpose: 'Owner');
    expect(last('POST', '/api/v1/owners').json, {
      'user_id': _sam,
      'reason': 'Deputy',
    });
    await repository.revokeOwner(_sam, reason: 'Done', purpose: 'Revoke');
    expectWrite('POST', '/api/v1/owners/$_sam/revoke', 'Revoke');
  });

  test('groups: list, read, create, rename, delete and members', () async {
    final groups = await repository.groups();
    expect(groups.items.single.name, 'Support');
    expect((await repository.group(_group)).memberCount, 1);
    final created = await repository.createGroup(
      'Support',
      description: '',
      purpose: 'Group',
    );
    expect(created.id, _group);
    expect(last('POST', '/api/v1/groups').json, {'name': 'Support'});
    final renamed = await repository.renameGroup(
      _group,
      'Second line',
      purpose: 'Rename',
    );
    expect(renamed.name, 'Second line');
    await repository.deleteGroup(_group, purpose: 'Delete');
    expectWrite('DELETE', '/api/v1/groups/$_group', 'Delete');
    final members = await repository.members(_group);
    expect(members.single.displayName, 'Sam');
    expect(members.single.expiresAt, DateTime.parse(_later));
    await repository.addMember(
      _group,
      _robin,
      expiresAt: DateTime.utc(2026, 12, 31),
      purpose: 'Member',
    );
    expect(last('POST', '/api/v1/groups/$_group/members').json, {
      'user_id': _robin,
      'expires_at': '2026-12-31T00:00:00.000Z',
    });
    await repository.removeMember(_group, _robin, purpose: 'Remove');
    expectWrite('DELETE', '/api/v1/groups/$_group/members/$_robin', 'Remove');
  });

  test('a problem answer becomes SelfActionFailed', () async {
    adapter.routes['GET /api/v1/users/$_sam'] = (
      404,
      {
        'type': 'about:blank',
        'title': 'Not Found',
        'status': 404,
        'code': 'not_found',
      },
    );
    await expectLater(
      repository.person(_sam),
      throwsA(isA<SelfActionFailed>()),
    );
  });
}
