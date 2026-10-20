// SPDX-License-Identifier: Apache-2.0

// The REST source of screen A8 (brief M4-37): every call maps the wire shapes of
// docs/api/openapi.yaml to the types of admin_ui, sends a write with the step-up purpose, an
// idempotency key where the operation takes one and the version as If-Match, follows the cursor
// of a list and turns a problem answer into SelfActionFailed or FingerprintMismatch. A handler
// adapter answers each request; no socket is opened.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

typedef Sent = ({RequestOptions options, Object? json});

/// Answers each request with the status and JSON that [answer] returns; records every request.
class HandlerAdapter implements HttpClientAdapter {
  HandlerAdapter(this.answer);

  (int, Object?) Function(RequestOptions options) answer;
  final List<Sent> sent = [];

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
    sent.add((options: options, json: body.isEmpty ? null : jsonDecode(body)));
    final (status, json) = answer(options);
    return ResponseBody.fromString(
      jsonEncode(json),
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

const String _host = '0192f4a0-7b1c-7a00-8b00-000000000001';
const String _other = '0192f4a0-7b1c-7a00-8b00-000000000002';
const String _sam = '0192f4a0-7b1c-733c-835c-6d67630008cd';
const String _robin = '0192f4a0-7b1c-7c2a-8e1d-41a6b2c3d4e5';
const String _device = '0192f4a0-7b1c-7d07-84b0-358556a08a99';
const String _at = '2026-10-10T09:30:00.000Z';

Map<String, Object?> _hostJson(String id, {String state = 'active'}) => {
  'id': id,
  'name': 'Build host',
  'state': state,
  'disabled_reason': 'owner',
  'platform': 'linux',
  'assigned_user_id': _sam,
  'llm_credentials': 'organization',
  'declared_server_host': false,
  'reported_is_server_host': false,
  'key_fingerprint': '7KQ2-9XTA-AAAA-BBBB',
  'key_generation': 2,
  'profiles': [
    {'name': 'shell', 'display_name': 'Shell', 'kind': 'shell'},
  ],
  'doctor': [
    {'check': 'hd1_user', 'ok': true},
    {
      'check': 'hd6_llm_login',
      'ok': false,
      'code': 'subscription_login_detected',
      'session_user': 'term-sam',
    },
  ],
  'session_users': [
    {'user_id': _sam, 'os_user': 'term-sam', 'created_at': _at},
  ],
  'reported_session_users': [
    {
      'os_user': 'term-sam',
      'workdirs': ['Project A'],
    },
    {'os_user': 'term-robin', 'workdirs': <Object?>[]},
  ],
  'connected': true,
  'host_version': '1.0.0',
  'os_version': 'Ubuntu 24.04',
  'last_seen_at': _at,
  'created_at': _at,
  'version': 4,
};

const Map<String, Object?> _code = {
  'host_id': _host,
  'pairing_code': 'ABCD-EFGH',
  'expires_at': _at,
};

Map<String, Object?> _audit(int seq, Map<String, Object?> details) => {
  'seq': seq,
  'id': '0192f4a0-7b1c-7e00-8f00-00000000000$seq',
  'occurred_at': _at,
  'action': 'host.manage',
  'decision': 'allow',
  'resource_id': _host,
  'details': details,
  'prev_hash': 'a' * 64,
  'hash': 'b' * 64,
};

Map<String, Object?> _session(String id, String state) => {
  'id': id,
  'host_id': _host,
  'user_id': _sam,
  'device_id': _device,
  'profile': 'shell',
  'state': state,
  'opened_at': _at,
  'bytes_in': 0,
  'bytes_out': 0,
  'version': 1,
};

Map<String, Object?> _problem(int status, String code, [Object? extra]) => {
  'type': 'about:blank',
  'title': 'Problem',
  'status': status,
  'code': code,
  'attempts_remaining': ?extra,
};

(int, Object?) _answer(RequestOptions o) {
  final cursor = o.uri.queryParameters['cursor'];
  return switch ('${o.method} ${o.uri.path}') {
    'GET /api/v1/me' => (
      200,
      {
        'principal': {
          'id': _sam,
          'kind': 'user',
          'handle': 'sam',
          'display_name': 'Sam',
        },
        'is_owner': true,
        'permission_version': 3,
      },
    ),
    'GET /api/v1/hosts' when cursor == null => (
      200,
      {
        'items': [_hostJson(_host)],
        'next_cursor': 'c2',
      },
    ),
    'GET /api/v1/hosts' => (
      200,
      {
        'items': [_hostJson(_other, state: 'pairing')],
      },
    ),
    'GET /api/v1/hosts/$_host' => (200, _hostJson(_host)),
    'POST /api/v1/hosts' => (201, _code),
    'POST /api/v1/hosts/$_host/pairing-code' => (201, _code),
    'POST /api/v1/hosts/$_host/confirm' ||
    'PATCH /api/v1/hosts/$_host' ||
    'PUT /api/v1/hosts/$_host/llm-credentials' ||
    'POST /api/v1/hosts/$_host/state' => (200, _hostJson(_host)),
    'GET /api/v1/audit/events' when cursor == null => (
      200,
      {
        'items': [
          _audit(3, {
            'operation': 'session_user',
            'os_user': 'term-sam',
            'user_id': _sam,
          }),
          _audit(1, {
            'operation': 'session_user',
            'os_user': 'term-sam',
            'user_id': _robin,
          }),
        ],
        'next_cursor': 'a2',
      },
    ),
    'GET /api/v1/audit/events' => (
      200,
      {
        'items': [
          _audit(2, {'operation': 'state', 'state': 'disabled'}),
          _audit(4, {
            'operation': 'session_user',
            'os_user': 'term-robin',
            'user_id': _robin,
          }),
        ],
      },
    ),
    'GET /api/v1/terminal/sessions' when cursor == null => (
      200,
      {
        'items': [
          _session('0192f4a0-7b1c-7e00-8f00-0000000000a1', 'active'),
          _session('0192f4a0-7b1c-7e00-8f00-0000000000a2', 'closed'),
        ],
        'next_cursor': 's2',
      },
    ),
    'GET /api/v1/terminal/sessions' => (
      200,
      {
        'items': [_session('0192f4a0-7b1c-7e00-8f00-0000000000a3', 'detached')],
      },
    ),
    final other => throw StateError('no answer for $other'),
  };
}

void main() {
  late HandlerAdapter adapter;
  late ApiHostsRepository repository;
  var keys = 0;

  setUp(() {
    adapter = HandlerAdapter(_answer);
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
    repository = ApiHostsRepository(
      client,
      newIdempotencyKey: () => '0192f4a0-0000-7000-8000-00000000000${keys++}',
    );
  });

  Sent last(String method, String path) => adapter.sent.lastWhere(
    (s) => s.options.method == method && s.options.uri.path == path,
  );

  void expectWrite(
    String method,
    String path,
    String purpose, {
    bool idempotent = true,
  }) {
    final s = last(method, path);
    expect(s.options.extra[stepUpPurposeExtra], purpose);
    expect(
      s.options.headers['If-Match'] ?? s.options.headers['if-match'],
      '"4"',
    );
    expect(
      s.options.headers['Idempotency-Key'],
      idempotent ? isNotNull : isNull,
    );
  }

  test('the origin of the server and the owner flag', () async {
    expect(repository.serverOrigin, 'https://chat.example.com');
    expect(await repository.callerIsOwner(), isTrue);
  });

  test('hosts follow the cursor and map every field', () async {
    final list = await repository.hosts();
    expect([for (final h in list) h.id], [_host, _other]);
    expect(list.last.state, HostLifecycle.pairing);
    final h = list.first;
    expect(h.name, 'Build host');
    expect(h.state, HostLifecycle.active);
    expect(h.ownerDisabled, isTrue);
    expect(h.platform, 'linux');
    expect(h.assignedUserId, _sam);
    expect(h.llmLogin, HostLlmLogin.organization);
    expect(h.declaredServerHost, isFalse);
    expect(h.reportedIsServerHost, isFalse);
    expect(h.keyFingerprint, '7KQ2-9XTA-AAAA-BBBB');
    expect(h.keyGeneration, 2);
    expect(h.profiles.single.displayName, 'Shell');
    expect([for (final d in h.doctor) d.check], ['hd1_user', 'hd6_llm_login']);
    expect(h.doctor.last.ok, isFalse);
    expect(h.doctor.last.code, 'subscription_login_detected');
    expect(h.doctor.last.sessionUser, 'term-sam');
    expect(h.sessionUsers.single.osUser, 'term-sam');
    expect(h.sessionUsers.single.userId, _sam);
    expect(
      [for (final r in h.reportedSessionUsers!) r.osUser],
      ['term-sam', 'term-robin'],
    );
    expect(h.reportedSessionUsers!.first.workdirs, ['Project A']);
    expect(h.connected, isTrue);
    expect(h.hostVersion, '1.0.0');
    expect(h.osVersion, 'Ubuntu 24.04');
    expect(h.lastSeenAt, DateTime.utc(2026, 10, 10, 9, 30));
    expect(h.version, 4);
    expect((await repository.host(_host)).id, _host);
  });

  test('create and a new pairing code carry reason, key and purpose', () async {
    final created = await repository.createHost(
      name: 'Build host',
      assigneeId: _sam,
      reason: 'Builds',
      purpose: 'Create',
    );
    expect(created.hostId, _host);
    expect(created.code, 'ABCD-EFGH');
    final post = last('POST', '/api/v1/hosts');
    expect(post.json, {
      'name': 'Build host',
      'assigned_user_id': _sam,
      'reason': 'Builds',
    });
    expect(post.options.extra[stepUpPurposeExtra], 'Create');
    expect(post.options.headers['Idempotency-Key'], isNotNull);
    final next = await repository.newPairingCode(
      _host,
      reason: 'Again',
      purpose: 'Code',
    );
    expect(next.expiresAt, DateTime.utc(2026, 10, 10, 9, 30));
    final code = last('POST', '/api/v1/hosts/$_host/pairing-code');
    expect(code.json, {'reason': 'Again'});
    expect(code.options.extra[stepUpPurposeExtra], 'Code');
  });

  test('confirm, update, LLM login and state send the version', () async {
    final host = await repository.host(_host);
    await repository.confirmHost(
      host,
      const HostConfirmRequest(
        fingerprintPrefix: '7kq2 9xta',
        llmLogin: HostLlmLogin.personal,
        credentialHolderId: _sam,
        declaredServerHost: false,
        reason: 'Pair',
        sessionUsers: [HostAssignment(userId: _sam, osUser: 'term-sam')],
      ),
      purpose: 'Confirm',
    );
    expectWrite('POST', '/api/v1/hosts/$_host/confirm', 'Confirm');
    expect(last('POST', '/api/v1/hosts/$_host/confirm').json, {
      'fingerprint_prefix': '7KQ29XTA',
      'llm_credentials': 'personal',
      'credential_holder_id': _sam,
      'declared_server_host': false,
      'session_users': [
        {'user_id': _sam, 'os_user': 'term-sam'},
      ],
      'reason': 'Pair',
    });

    await repository.updateHost(
      host,
      clearAssignee: true,
      add: const [HostAssignment(userId: _robin, osUser: 'term-robin')],
      revoke: const [_sam],
      reason: 'Rotate',
      purpose: 'Update',
    );
    expectWrite('PATCH', '/api/v1/hosts/$_host', 'Update');
    expect(last('PATCH', '/api/v1/hosts/$_host').json, {
      'clear_assigned_user': true,
      'add_session_users': [
        {'user_id': _robin, 'os_user': 'term-robin'},
      ],
      'revoke_session_users': [_sam],
      'reason': 'Rotate',
    });

    await repository.setLlmLogin(
      host,
      HostLlmLogin.none,
      holderId: _sam,
      reason: 'No login',
      purpose: 'Login',
    );
    expectWrite(
      'PUT',
      '/api/v1/hosts/$_host/llm-credentials',
      'Login',
      idempotent: false,
    );
    expect(last('PUT', '/api/v1/hosts/$_host/llm-credentials').json, {
      'llm_credentials': 'none',
      'reason': 'No login',
    });

    for (final (state, wire) in [
      (HostStateRequest.active, 'active'),
      (HostStateRequest.disabled, 'disabled'),
      (HostStateRequest.retired, 'retired'),
    ]) {
      await repository.setHostState(
        host,
        state,
        reason: 'Maintenance',
        purpose: 'State',
      );
      expectWrite('POST', '/api/v1/hosts/$_host/state', 'State');
      expect(last('POST', '/api/v1/hosts/$_host/state').json, {
        'state': wire,
        'reason': 'Maintenance',
      });
    }
  });

  test('the assignment history keeps the latest person per name', () async {
    final history = await repository.assignmentHistory(_host, purpose: 'Audit');
    expect(history, {'term-sam': _sam, 'term-robin': _robin});
    final query = last('GET', '/api/v1/audit/events').options.uri;
    expect(query.queryParameters['action'], 'host.manage');
    expect(query.queryParameters['resource_id'], _host);
    expect(query.queryParameters['cursor'], 'a2');
  });

  test('open sessions count every session that is not closed', () async {
    expect(await repository.openSessions(_host), 2);
    final query = last('GET', '/api/v1/terminal/sessions').options.uri;
    expect(query.queryParameters['host_id'], _host);
  });

  test('a fingerprint mismatch names the attempts left', () async {
    final host = await repository.host(_host);
    adapter.answer = (o) => (409, _problem(409, 'fingerprint_mismatch', 2));
    await expectLater(
      repository.confirmHost(
        host,
        const HostConfirmRequest(
          fingerprintPrefix: 'ZZZZ',
          llmLogin: HostLlmLogin.organization,
          declaredServerHost: false,
          reason: 'Pair',
        ),
        purpose: 'Confirm',
      ),
      throwsA(
        isA<FingerprintMismatch>()
            .having((e) => e.attemptsRemaining, 'attempts', 2)
            .having((e) => e.problem.code, 'code', 'fingerprint_mismatch'),
      ),
    );
  });

  test('another problem answer becomes SelfActionFailed', () async {
    adapter.answer = (o) => (404, _problem(404, 'not_found'));
    await expectLater(
      repository.host(_host),
      throwsA(
        isA<SelfActionFailed>().having(
          (e) => e.problem?.code,
          'code',
          'not_found',
        ),
      ),
    );
  });
}
