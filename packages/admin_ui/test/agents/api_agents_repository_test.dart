// SPDX-License-Identifier: Apache-2.0

// The REST source of screen A5 (brief M4-28 Design notes 1 to 4): every call maps the wire
// shapes of docs/api/openapi.yaml to the types of admin_ui unchanged (the warnings of 10 8 are
// the server's codes, passed through, never derived), sends If-Match, the step-up purpose, the
// reason and the holder only with personal (10 9.1, BR-07), turns a `409 i1_violation` into
// SelfActionFailed with its conflicts (10 5.4), reads the proposal of "All to Personal" from the
// audit (10 9.1) and sends `resolve_i1` of the guided flow (10 11). A routing adapter answers
// by method, path and cursor; no socket is opened.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' hide AgentPage;
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// One request as the adapter saw it.
typedef Seen = ({RequestOptions options, Object? json});

/// Answers each request with the JSON of [routes] by `METHOD path`, or by
/// `METHOD path?cursor=<c>` for a later page (status 200 unless the route gives one; a null
/// answer is an empty body); records every request.
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
    final key = '${options.method} ${options.uri.path}';
    final cursor = options.uri.queryParameters['cursor'];
    final (status, answer) =
        (cursor == null ? null : routes['$key?cursor=$cursor']) ??
        routes[key] ??
        (throw StateError('no route for $key cursor $cursor'));
    return ResponseBody.fromString(
      answer == null ? '' : jsonEncode(answer),
      status,
      headers: {
        if (answer != null)
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
const String _robin = '0192f4a0-7b1c-7e11-8f22-6960453241bc';
const String _support = '0192f4a0-7b1c-7d07-84b0-358556a08a99';
const String _texter = '0192f4a0-7b1c-7d07-84b0-358556a08a98';
const String _ops = '0192f4a0-7b1c-7d07-84b0-358556a08a97';
const String _idle = '0192f4a0-7b1c-7d07-84b0-358556a08a96';
const String _probe = '0192f4a0-7b1c-7d07-84b0-358556a08a95';
const String _space = '0192f4a0-7b1c-7fd5-831e-d4370e087874';
const String _space2 = '0192f4a0-7b1c-7fd5-831e-d4370e087875';
const String _channel = '0192f4a0-7b1c-72d4-8bae-7661d2b2f203';
const String _channel2 = '0192f4a0-7b1c-72d4-8bae-7661d2b2f204';
const String _channel3 = '0192f4a0-7b1c-72d4-8bae-7661d2b2f205';
const String _token = '0192f4a0-7b1c-7a01-8b02-1234567890ab';
const String _token2 = '0192f4a0-7b1c-7a01-8b02-1234567890ac';
const String _grant = '0192f4a0-7b1c-727f-86bd-90fe7bdb3024';
const String _group = '0192f4a0-7b1c-7aaa-8bbb-6960453241bc';
const String _at = '2026-10-10T09:30:00.000Z';

Map<String, Object?> _ref(String id, String name) => {
  'id': id,
  'kind': 'user',
  'handle': name.toLowerCase(),
  'display_name': name,
};

Map<String, Object?> _agentJson(
  String id,
  String handle, {
  String status = 'active',
  String mode = 'organization',
  String? holder,
  String? changedAt,
  String toolClass = 'chat',
  Map<String, Object?> presence = const {'state': 'unknown'},
  int version = 1,
}) => {
  'id': id,
  'handle': handle,
  'display_name': handle.toUpperCase(),
  'status': status,
  'adapter_kind': 'hermes',
  'description': 'Answers $handle tickets',
  'billing_mode': mode,
  'credential_holder_id': ?holder,
  'billing_mode_changed_at': ?changedAt,
  'tool_class': toolClass,
  'presence': presence,
  'operator_config': <String, Object?>{},
  'version': version,
};

/// Support: personal, held by Alex, reports personal, two warnings of which one is unknown.
Map<String, Object?> _supportJson({int version = 7}) => _agentJson(
  _support,
  'support',
  mode: 'personal',
  holder: _alex,
  changedAt: _at,
  toolClass: 'scoped',
  version: version,
  presence: {
    'state': 'online',
    'reported_billing_mode': 'personal',
    'runtime_version': '2.3.1',
    'last_heartbeat_at': '2026-10-10T09:31:00.000Z',
    'billing_modes_supported': ['personal', 'organization'],
    'billing_mode_matches': true,
    'billing_evidence': {
      'provider': 'anthropic',
      'key_type': 'oauth',
      'method': 'resolved_credentials',
    },
    'warnings': ['billing_mode_mismatch', 'future_warning_code'],
  },
);

/// Texter: organization, follows the configured mode per run (no reported mode).
Map<String, Object?> _texterJson() => _agentJson(
  _texter,
  'texter',
  presence: {
    'state': 'degraded',
    'billing_modes_supported': ['organization'],
    'billing_mode_matches': false,
    'billing_evidence': {'key_type': 'api_key'},
    'warnings': ['billing_evidence_unknown'],
  },
);

Map<String, Object?> _problem(
  int status,
  String code, {
  List<Object?>? conflicts,
}) => {
  'type': '/api/v1/problems/$code',
  'title': code,
  'status': status,
  'code': code,
  'conflicts': ?conflicts,
};

Map<String, Object?> _spaceJson(String id, String name, {String? operator}) => {
  'id': id,
  'name': name,
  'sort_key': 'a0',
  'default_operator_agent_id': ?operator,
  'version': 1,
  'created_at': _at,
};

Map<String, Object?> _channelJson(
  String id,
  String spaceId,
  String name, {
  String? effective,
}) => {
  'channel': {
    'id': id,
    'space_id': spaceId,
    'kind': 'text',
    'name': name,
    'topic': '',
    'sort_key': 'a1',
    'read_only': false,
    'operator': {'mode': 'inherit', 'effective_agent_id': ?effective},
    'version': 1,
    'settings_version': 1,
    'created_at': _at,
  },
  'level': 'manage',
  'capabilities': <Object?>[],
};

Map<String, Object?> _tokenJson(
  String id,
  String name, {
  String? dpop,
  String? revokedAt,
}) => {
  'id': id,
  'principal_id': _support,
  'name': name,
  'token_prefix': 'bothy_agt_EXAMPL',
  'scopes': <Object?>[],
  'dpop_jkt': ?dpop,
  'expires_at': '2026-12-31T23:00:00.000Z',
  'created_at': _at,
  'last_used_at': ?(revokedAt == null ? '2026-10-11T08:00:00.000Z' : null),
  'revoked_at': ?revokedAt,
};

Map<String, Object?> _audit(int seq, Map<String, Object?> details) => {
  'seq': seq,
  'id': '0192f4a0-7b1c-7b00-8c00-00000000000$seq',
  'occurred_at': _at,
  'actor_principal_id': _alex,
  'action': 'agent.billing_mode.set_all',
  'decision': 'allow',
  'details': details,
  'prev_hash': 'aa',
  'hash': 'bb',
};

Map<String, Object?> _grantJson() => {
  'id': _grant,
  'subject_kind': 'user',
  'subject_id': _sam,
  'target_kind': 'channel',
  'target_id': _channel,
  'level': 'trigger',
  'capabilities': <Object?>[],
  'reason': 'Campaign work',
  'granted_by': _alex,
  'granted_at': _at,
};

Map<String, (int, Object?)> _routes() => {
  'GET /api/v1/agents': (
    200,
    {
      'items': [_supportJson(), _texterJson()],
      'next_cursor': 'agents-2',
    },
  ),
  'GET /api/v1/agents?cursor=agents-2': (
    200,
    {
      'items': [
        _agentJson(
          _ops,
          'ops',
          status: 'disabled',
          toolClass: 'host',
          presence: {
            'state': 'offline',
            'reported_billing_mode': 'organization',
            'billing_evidence': {'key_type': 'cloud'},
          },
        ),
        _agentJson(
          _idle,
          'idle',
          presence: {
            'state': 'unknown',
            'billing_evidence': {'key_type': 'none'},
          },
        ),
        _agentJson(
          _probe,
          'probe',
          presence: {
            'state': 'online',
            'billing_modes_supported': <Object?>[],
            'billing_evidence': {'key_type': 'unknown'},
          },
        ),
      ],
    },
  ),
  'GET /api/v1/agents/$_support': (200, _supportJson()),
  'GET /api/v1/agents/$_texter': (200, _texterJson()),
  'PUT /api/v1/agents/$_support/billing-mode': (200, _supportJson(version: 8)),
  'PUT /api/v1/agents/$_texter/billing-mode': (
    409,
    _problem(
      409,
      'i1_violation',
      conflicts: [
        {
          'code': 'i1_violation',
          'agent_id': _texter,
          'credential_holder_id': _alex,
          'principal_ids': [_sam, _robin],
          'organization_ready': true,
        },
        {'code': 'i1_violation', 'principal_ids': <Object?>[]},
        {'agent_id': _texter, 'principal_ids': <Object?>[]},
        'not an object',
      ],
    ),
  ),
  'PUT /api/v1/agents/$_support/credential-holder': (
    200,
    _supportJson(version: 9),
  ),
  'PUT /api/v1/instance/agent-billing-mode': (
    200,
    {
      'changed_agent_ids': [_support, _ops],
    },
  ),
  'GET /api/v1/spaces': (
    200,
    {
      'items': [
        {
          'space': _spaceJson(_space, 'Marketing', operator: _support),
          'level': 'manage',
          'capabilities': <Object?>[],
        },
        {
          'space': _spaceJson(_space2, 'Support'),
          'level': 'manage',
          'capabilities': <Object?>[],
        },
      ],
    },
  ),
  'GET /api/v1/spaces/$_space/channels': (
    200,
    {
      'items': [
        _channelJson(_channel, _space, 'Kampagnen', effective: _support),
        _channelJson(_channel2, _space, 'Presse', effective: _texter),
      ],
    },
  ),
  'GET /api/v1/spaces/$_space2/channels': (
    200,
    {
      'items': [
        _channelJson(_channel3, _space2, 'Tickets', effective: _support),
        _channelJson(
          '0192f4a0-7b1c-72d4-8bae-7661d2b2f206',
          _space2,
          'Ohne Operator',
        ),
      ],
    },
  ),
  'GET /api/v1/audit/events': (
    200,
    {
      'items': [
        _audit(4, {
          'to_billing_mode': 'personal',
          'agent_ids': [_support],
          'from_billing_modes': ['organization'],
          'from_credential_holder_ids': <Object?>[],
        }),
        _audit(3, {
          'to_billing_mode': 'organization',
          'agent_ids': [_texter],
          'from_billing_modes': ['organization'],
        }),
      ],
      'next_cursor': 'audit-2',
    },
  ),
  'GET /api/v1/audit/events?cursor=audit-2': (
    200,
    {
      'items': [
        _audit(2, {
          'to_billing_mode': 'organization',
          'agent_ids': [_support, _texter, _ops],
          'from_billing_modes': ['personal', 'organization', 'personal'],
          'held_agent_ids': [_support, _ops],
          'from_credential_holder_ids': [_alex, _alex],
        }),
        _audit(1, {
          'to_billing_mode': 'organization',
          'agent_ids': [_idle],
          'from_billing_modes': ['personal'],
          'from_credential_holder_ids': [_sam],
        }),
      ],
    },
  ),
  'GET /api/v1/principals/$_support/tokens': (
    200,
    {
      'items': [_tokenJson(_token, 'Adapter production', dpop: 'jkt')],
      'next_cursor': 'tokens-2',
    },
  ),
  'GET /api/v1/principals/$_support/tokens?cursor=tokens-2': (
    200,
    {
      'items': [_tokenJson(_token2, 'Old adapter', revokedAt: _at)],
    },
  ),
  'POST /api/v1/principals/$_support/tokens': (
    201,
    {
      'token': _tokenJson(_token, 'Adapter production', dpop: 'jkt'),
      'secret': 'bothy_agt_EXAMPLEOnlyNotARealTokenForTheChecksumTests_3NymP3',
    },
  ),
  'DELETE /api/v1/tokens/$_token': (204, null),
  'POST /api/v1/grants': (201, _grantJson()),
  'POST /api/v1/owners': (
    201,
    {'user': _ref(_sam, 'Sam'), 'granted_at': _at, 'reason': 'Deputy'},
  ),
  'POST /api/v1/groups/$_group/members': (
    201,
    {'user': _ref(_sam, 'Sam'), 'added_at': _at},
  ),
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

  Future<AgentRecord> support() => repository.agentRecord(_support);

  group('agent records', () {
    test('listAgents reads every page and maps every field', () async {
      final agents = await repository.agentRecords();
      expect(adapter.seen.map((s) => s.options.uri.queryParameters['cursor']), [
        null,
        'agents-2',
      ]);
      expect(agents.map((a) => a.id), [_support, _texter, _ops, _idle, _probe]);

      final s = agents[0];
      expect(s.handle, 'support');
      expect(s.displayName, 'SUPPORT');
      expect(s.active, isTrue);
      expect(s.adapterKind, 'hermes');
      expect(s.description, 'Answers support tickets');
      expect(s.mode, AgentBillingMode.personal);
      expect(s.holderId, _alex);
      expect(s.modeChangedAt, DateTime.parse(_at));
      expect(s.toolClass, 'scoped');
      expect(s.version, 7);
      expect(s.runtime.state, AgentLiveStateKind.online);
      expect(s.runtime.reportedMode, AgentBillingMode.personal);
      expect(s.runtime.runtimeVersion, '2.3.1');
      expect(s.runtime.lastHeartbeatAt, DateTime.utc(2026, 10, 10, 9, 31));
      expect(s.runtime.modesSupported, [
        AgentBillingMode.personal,
        AgentBillingMode.organization,
      ]);
      expect(s.runtime.modeMatches, isTrue);
      expect(s.runtime.evidence?.keyType, 'oauth');
      expect(s.runtime.evidence?.provider, 'anthropic');
      expect(s.runtime.evidence?.method, 'resolved_credentials');
      // 10 8: the warnings are the server's codes, an unknown one included, in its order.
      expect(s.runtime.warnings, [
        'billing_mode_mismatch',
        'future_warning_code',
      ]);

      final t = agents[1];
      expect(t.mode, AgentBillingMode.organization);
      // The holder exists only with personal (I1); organization carries none.
      expect(t.holderId, isNull);
      expect(t.modeChangedAt, isNull);
      expect(t.toolClass, 'chat');
      expect(t.runtime.state, AgentLiveStateKind.degraded);
      // An adapter that follows the configured mode per run reports none (10 8).
      expect(t.runtime.reportedMode, isNull);
      expect(t.runtime.modeMatches, isFalse);
      expect(t.runtime.evidence?.keyType, 'api_key');
      expect(t.runtime.evidence?.provider, isNull);
      expect(t.runtime.evidence?.method, isNull);
      expect(t.runtime.warnings, ['billing_evidence_unknown']);

      final o = agents[2];
      expect(o.active, isFalse);
      expect(o.toolClass, 'host');
      expect(o.runtime.state, AgentLiveStateKind.offline);
      expect(o.runtime.reportedMode, AgentBillingMode.organization);
      expect(o.runtime.modesSupported, isNull);
      expect(o.runtime.modeMatches, isNull);
      expect(o.runtime.evidence?.keyType, 'cloud');
      expect(o.runtime.warnings, isEmpty);

      final i = agents[3];
      expect(i.runtime.state, AgentLiveStateKind.unknown);
      expect(i.runtime.evidence?.keyType, 'none');
      expect(i.runtime.lastHeartbeatAt, isNull);
      expect(i.runtime.runtimeVersion, isNull);

      final p = agents[4];
      expect(p.runtime.modesSupported, isEmpty);
      expect(p.runtime.evidence?.keyType, 'unknown');
    });

    test('an agent without evidence has none', () async {
      adapter.routes['GET /api/v1/agents/$_idle'] = (
        200,
        _agentJson(_idle, 'idle'),
      );
      final idle = await repository.agentRecord(_idle);
      expect(idle.runtime.evidence, isNull);
      expect(idle.runtime.warnings, isEmpty);
    });

    test('getAgent reads one agent', () async {
      final s = await support();
      expect(adapter.seen.single.options.method, 'GET');
      expect(adapter.seen.single.options.uri.path, '/api/v1/agents/$_support');
      expect(s.id, _support);
      expect(s.version, 7);
      expect(s.runtime.warnings, contains('future_warning_code'));
    });

    test('AgentBillingMode.of maps the wire and keeps unknown values', () {
      expect(AgentBillingMode.of('personal'), AgentBillingMode.personal);
      expect(
        AgentBillingMode.of('organization'),
        AgentBillingMode.organization,
      );
      expect(AgentBillingMode.of('team'), AgentBillingMode.unknown);
      expect(AgentBillingMode.of(''), AgentBillingMode.unknown);
      expect(AgentBillingMode.of(null), AgentBillingMode.unknown);
    });
  });

  test('operated targets: the spaces whose default operator is the agent and the channels '
      'it effectively operates (10 5.2)', () async {
    final targets = await repository.operatedTargets(_support);
    expect(adapter.seen.map((s) => s.options.uri.path), [
      '/api/v1/spaces',
      '/api/v1/spaces/$_space/channels',
      '/api/v1/spaces/$_space2/channels',
    ]);
    expect(targets.map((t) => t.target.key), [
      'space.$_space',
      'channel.$_channel',
      'channel.$_channel3',
    ]);
    expect(targets.map((t) => t.byDefault), [true, false, false]);
    expect(targets[0].target.name, 'Marketing');
    expect(targets[1].target.path, 'Marketing/Kampagnen');
    expect(targets[2].target.spaceName, 'Support');
    expect(targets[2].target.kind, AdminTargetKind.channel);

    final texter = await repository.operatedTargets(_texter);
    expect(texter.single.target.key, 'channel.$_channel2');
    expect(texter.single.byDefault, isFalse);
    expect(await repository.operatedTargets(_idle), isEmpty);
  });

  group('switch (10 9.1, BR-07)', () {
    test(
      'to personal sends If-Match, the purpose, the mode, the holder and the '
      'reason',
      () async {
        final agent = await support();
        final changed = await repository.setMode(
          agent,
          AgentBillingMode.personal,
          holderId: _sam,
          reason: 'Sam runs it now',
          purpose: 'Switch support to Personal',
        );
        final sent = adapter.seen.last;
        expect(sent.options.method, 'PUT');
        expect(sent.options.uri.path, '/api/v1/agents/$_support/billing-mode');
        expect(sent.options.headers['If-Match'], '"7"');
        expect(
          sent.options.extra[stepUpPurposeExtra],
          'Switch support to Personal',
        );
        expect(sent.json, {
          'billing_mode': 'personal',
          'credential_holder_id': _sam,
          'reason': 'Sam runs it now',
        });
        expect(changed.version, 8);
      },
    );

    test(
      'to organization never sends a holder, even when one is given',
      () async {
        final agent = await support();
        await repository.setMode(
          agent,
          AgentBillingMode.organization,
          holderId: _sam,
          reason: 'Team uses it',
          purpose: 'Switch support to Organization',
        );
        expect(adapter.seen.last.json, {
          'billing_mode': 'organization',
          'reason': 'Team uses it',
        });
      },
    );

    test('a 409 i1_violation becomes SelfActionFailed with its conflicts '
        '(10 5.4)', () async {
      final texter = await repository.agentRecord(_texter);
      await expectLater(
        repository.setMode(
          texter,
          AgentBillingMode.personal,
          holderId: _alex,
          reason: 'Alex pays',
          purpose: 'Switch texter to Personal',
        ),
        throwsA(
          isA<SelfActionFailed>()
              .having((e) => e.problem?.status, 'status', 409)
              .having((e) => e.problem?.code, 'code', 'i1_violation')
              .having((e) => e.conflicts.length, 'conflicts', 2)
              .having((e) => e.conflicts.first.code, 'code', 'i1_violation')
              .having((e) => e.conflicts.first.agentId, 'agentId', _texter)
              .having((e) => e.conflicts.first.holderId, 'holderId', _alex)
              .having((e) => e.conflicts.first.principalIds, 'principals', [
                _sam,
                _robin,
              ])
              .having(
                (e) => e.conflicts.first.organizationReady,
                'organizationReady',
                isTrue,
              )
              .having((e) => e.conflicts.last.agentId, 'agentId', isNull)
              .having((e) => e.conflicts.last.holderId, 'holderId', isNull)
              .having(
                (e) => e.conflicts.last.organizationReady,
                'organizationReady',
                isNull,
              ),
        ),
      );
    });

    test('a problem without conflicts carries none', () async {
      adapter.routes['PUT /api/v1/agents/$_support/billing-mode'] = (
        412,
        _problem(412, 'precondition_failed'),
      );
      await expectLater(
        repository.setMode(
          await support(),
          AgentBillingMode.organization,
          reason: 'r',
          purpose: 'p',
        ),
        throwsA(
          isA<SelfActionFailed>()
              .having((e) => e.problem?.code, 'code', 'precondition_failed')
              .having((e) => e.conflicts, 'conflicts', isEmpty),
        ),
      );
    });

    test('the holder change sends If-Match, the purpose, the holder and the '
        'reason', () async {
      final agent = await support();
      final changed = await repository.setHolder(
        agent,
        _sam,
        reason: 'Handover to deputy',
        purpose: 'Change the holder of support',
      );
      final sent = adapter.seen.last;
      expect(sent.options.method, 'PUT');
      expect(
        sent.options.uri.path,
        '/api/v1/agents/$_support/credential-holder',
      );
      expect(sent.options.headers['If-Match'], '"7"');
      expect(
        sent.options.extra[stepUpPurposeExtra],
        'Change the holder of support',
      );
      expect(sent.json, {
        'credential_holder_id': _sam,
        'reason': 'Handover to deputy',
      });
      expect(changed.version, 9);
    });
  });

  group('switch all (10 9.1, BR-08)', () {
    test('to organization without agent_ids switches every agent', () async {
      final result = await repository.setAllModes(
        AgentBillingMode.organization,
        holderId: _alex,
        reason: 'Company key',
        purpose: 'Switch all to Organization',
      );
      final sent = adapter.seen.single;
      expect(sent.options.method, 'PUT');
      expect(sent.options.uri.path, '/api/v1/instance/agent-billing-mode');
      expect(
        sent.options.extra[stepUpPurposeExtra],
        'Switch all to Organization',
      );
      expect(sent.json, {
        'billing_mode': 'organization',
        'reason': 'Company key',
      });
      expect(result.changedAgentIds, [_support, _ops]);
    });

    test('to personal sends the holder and the agent_ids', () async {
      await repository.setAllModes(
        AgentBillingMode.personal,
        holderId: _alex,
        agentIds: [_support, _ops],
        reason: 'Back to Alex',
        purpose: 'Switch all to Personal',
      );
      expect(adapter.seen.single.json, {
        'billing_mode': 'personal',
        'credential_holder_id': _alex,
        'agent_ids': [_support, _ops],
        'reason': 'Back to Alex',
      });
    });
  });

  group('the proposal of All to Personal (10 9.1)', () {
    // The server lists the audit newest first (`ORDER BY seq DESC`), so the first entry that
    // proposes something is the newest global switch.
    test(
      'lastGlobalSwitch pages the audit, skips other directions and '
      'proposes the agents that were personal with their single holder',
      () async {
        final proposal = await repository.lastGlobalSwitch(
          purpose: 'Read audit',
        );
        expect(adapter.seen, hasLength(2));
        for (final s in adapter.seen) {
          expect(s.options.uri.path, '/api/v1/audit/events');
          expect(
            s.options.uri.queryParameters['action'],
            'agent.billing_mode.set_all',
          );
          expect(s.options.extra[stepUpPurposeExtra], 'Read audit');
        }
        expect(
          adapter.seen.last.options.uri.queryParameters['cursor'],
          'audit-2',
        );
        expect(proposal?.agentIds, [_support, _ops]);
        expect(proposal?.holderId, _alex);
      },
    );

    test('lastGlobalSwitch is null when no entry proposes anything', () async {
      adapter.routes.remove('GET /api/v1/audit/events?cursor=audit-2');
      adapter.routes['GET /api/v1/audit/events'] = (
        200,
        {
          'items': [
            _audit(1, {
              'to_billing_mode': 'organization',
              'agent_ids': <Object?>[],
            }),
          ],
        },
      );
      expect(await repository.lastGlobalSwitch(purpose: 'p'), isNull);
      expect(adapter.seen, hasLength(1));
    });

    test('switchAllProposalOf edge cases', () {
      expect(
        switchAllProposalOf({
          'to_billing_mode': 'personal',
          'agent_ids': ['a'],
        }),
        isNull,
      );
      expect(
        switchAllProposalOf({
          'agent_ids': ['a'],
        }),
        isNull,
      );
      expect(switchAllProposalOf({'to_billing_mode': 'organization'}), isNull);
      expect(
        switchAllProposalOf({
          'to_billing_mode': 'organization',
          'agent_ids': 'a',
        }),
        isNull,
      );
      expect(
        switchAllProposalOf({
          'to_billing_mode': 'organization',
          'agent_ids': <Object?>[],
        }),
        isNull,
      );
      // Every agent was organization already: nothing to propose.
      expect(
        switchAllProposalOf({
          'to_billing_mode': 'organization',
          'agent_ids': ['a', 'b'],
          'from_billing_modes': ['organization', 'organization'],
        }),
        isNull,
      );

      // Without from_billing_modes every agent is proposed; no holders, no holder.
      final all = switchAllProposalOf({
        'to_billing_mode': 'organization',
        'agent_ids': ['a', 'b'],
      })!;
      expect(all.agentIds, ['a', 'b']);
      expect(all.holderId, isNull);

      // A shorter from list proposes the agents past its end; a non-string ID is skipped.
      final short = switchAllProposalOf({
        'to_billing_mode': 'organization',
        'agent_ids': ['a', 42, 'b', 'c'],
        'from_billing_modes': ['organization', 'personal'],
        'from_credential_holder_ids': [_alex, 7, _alex],
      })!;
      expect(short.agentIds, ['b', 'c']);
      expect(short.holderId, _alex);

      // Two different former holders: no single holder to propose.
      final two = switchAllProposalOf({
        'to_billing_mode': 'organization',
        'agent_ids': ['a', 'b'],
        'from_billing_modes': ['personal', 'personal'],
        'from_credential_holder_ids': [_alex, _sam],
      })!;
      expect(two.agentIds, ['a', 'b']);
      expect(two.holderId, isNull);

      // A from list that is not a list counts as absent; a holder that is no list is none.
      final odd = switchAllProposalOf({
        'to_billing_mode': 'organization',
        'agent_ids': ['a'],
        'from_billing_modes': 'personal',
        'from_credential_holder_ids': _alex,
      })!;
      expect(odd.agentIds, ['a']);
      expect(odd.holderId, isNull);
    });
  });

  group('tokens (08 12)', () {
    test('listTokens reads every page and keeps only the prefix', () async {
      final tokens = await repository.tokens(_support);
      expect(adapter.seen.map((s) => s.options.uri.queryParameters['cursor']), [
        null,
        'tokens-2',
      ]);
      expect(tokens.map((t) => t.id), [_token, _token2]);
      final first = tokens.first;
      expect(first.name, 'Adapter production');
      expect(first.prefix, 'bothy_agt_EXAMPL');
      expect(first.createdAt, DateTime.parse(_at));
      expect(first.expiresAt, DateTime.utc(2026, 12, 31, 23));
      expect(first.lastUsedAt, DateTime.utc(2026, 10, 11, 8));
      expect(first.revokedAt, isNull);
      expect(first.dpopBound, isTrue);
      final old = tokens.last;
      expect(old.dpopBound, isFalse);
      expect(old.revokedAt, DateTime.parse(_at));
      expect(old.lastUsedAt, isNull);
    });

    test('createToken sends name, expiry, key binding, an idempotency key and '
        'the purpose, and returns the secret once', () async {
      final created = await repository.createToken(
        _support,
        name: 'Adapter production',
        expiresAt: DateTime.utc(2026, 12, 31, 23),
        dpopJkt: 'jkt',
        purpose: 'Create a token for support',
      );
      final sent = adapter.seen.single;
      expect(sent.options.method, 'POST');
      expect(sent.options.uri.path, '/api/v1/principals/$_support/tokens');
      expect(
        sent.options.headers['Idempotency-Key'],
        '0192f4a0-0000-7000-8000-000000000000',
      );
      expect(
        sent.options.extra[stepUpPurposeExtra],
        'Create a token for support',
      );
      expect(sent.json, {
        'name': 'Adapter production',
        'dpop_jkt': 'jkt',
        'expires_at': '2026-12-31T23:00:00.000Z',
      });
      expect(
        created.secret,
        'bothy_agt_EXAMPLEOnlyNotARealTokenForTheChecksumTests_3NymP3',
      );
      expect(created.token.id, _token);
      expect(created.token.dpopBound, isTrue);
    });

    test('createToken without a key binding sends no dpop_jkt and a local '
        'expiry in UTC', () async {
      final local = DateTime(2026, 12, 31, 23);
      await repository.createToken(
        _support,
        name: 'n',
        expiresAt: local,
        purpose: 'p',
      );
      final json = adapter.seen.single.json! as Map;
      expect(json.containsKey('dpop_jkt'), isFalse);
      expect(DateTime.parse(json['expires_at']! as String), local.toUtc());
    });

    test('revokeToken deletes the token without a purpose', () async {
      await repository.revokeToken(_token);
      final sent = adapter.seen.single;
      expect(sent.options.method, 'DELETE');
      expect(sent.options.uri.path, '/api/v1/tokens/$_token');
      expect(sent.options.extra.containsKey(stepUpPurposeExtra), isFalse);
    });
  });

  group('the guided flow sends resolve_i1 (10 11)', () {
    const resolution = I1Resolution([_support, _ops], reason: 'Sam joins');
    const resolveJson = {
      'switch_to_organization': [_support, _ops],
      'reason': 'Sam joins',
    };

    test('I1Resolution.wire carries the agents and the reason', () {
      final wire = resolution.wire;
      expect(wire.switchToOrganization.toList(), [_support, _ops]);
      expect(wire.reason, 'Sam joins');
    });

    test('createGrant', () async {
      const request = GrantRequest(
        subjectKind: AdminSubjectKind.user,
        subjectId: _sam,
        target: AdminTarget(
          AdminTargetKind.channel,
          id: _channel,
          name: 'Kampagnen',
        ),
        level: AccessLevel.trigger,
        reason: 'Campaign work',
      );
      await repository.createGrant(
        request,
        purpose: 'Grant',
        resolveI1: resolution,
      );
      expect((adapter.seen.last.json! as Map)['resolve_i1'], resolveJson);
      await repository.createGrant(request, purpose: 'Grant');
      expect(
        (adapter.seen.last.json! as Map).containsKey('resolve_i1'),
        isFalse,
      );
    });

    test('addOwner', () async {
      await repository.addOwner(
        _sam,
        reason: 'Deputy',
        purpose: 'Add owner',
        resolveI1: resolution,
      );
      final sent = adapter.seen.last;
      expect(sent.options.uri.path, '/api/v1/owners');
      expect(sent.options.headers['Idempotency-Key'], isNotNull);
      expect(sent.options.extra[stepUpPurposeExtra], 'Add owner');
      expect(sent.json, {
        'user_id': _sam,
        'reason': 'Deputy',
        'resolve_i1': resolveJson,
      });
      await repository.addOwner(_sam, reason: 'Deputy', purpose: 'Add owner');
      expect(adapter.seen.last.json, {'user_id': _sam, 'reason': 'Deputy'});
    });

    test('addMember', () async {
      await repository.addMember(
        _group,
        _sam,
        expiresAt: DateTime.utc(2026, 11, 30, 23),
        purpose: 'Add member',
        resolveI1: resolution,
      );
      final sent = adapter.seen.last;
      expect(sent.options.uri.path, '/api/v1/groups/$_group/members');
      expect(sent.options.extra[stepUpPurposeExtra], 'Add member');
      expect(sent.json, {
        'user_id': _sam,
        'expires_at': '2026-11-30T23:00:00.000Z',
        'resolve_i1': resolveJson,
      });
      await repository.addMember(_group, _sam, purpose: 'Add member');
      expect(adapter.seen.last.json, {'user_id': _sam});
    });

    test('a 409 i1_violation of addOwner carries the conflicts', () async {
      adapter.routes['POST /api/v1/owners'] = (
        409,
        _problem(
          409,
          'i1_violation',
          conflicts: [
            {
              'code': 'i1_violation',
              'agent_id': _support,
              'credential_holder_id': _alex,
              'principal_ids': [_sam],
              'organization_ready': false,
            },
          ],
        ),
      );
      await expectLater(
        repository.addOwner(_sam, reason: 'Deputy', purpose: 'Add owner'),
        throwsA(
          isA<SelfActionFailed>()
              .having((e) => e.problem?.code, 'code', 'i1_violation')
              .having((e) => e.conflicts.single.agentId, 'agentId', _support)
              .having(
                (e) => e.conflicts.single.organizationReady,
                'organizationReady',
                isFalse,
              ),
        ),
      );
    });
  });

  group('conflictsOf', () {
    test('is empty for a body that is not a problem with a conflicts list', () {
      expect(conflictsOf(null), isEmpty);
      expect(conflictsOf('text'), isEmpty);
      expect(conflictsOf(<String, Object?>{'code': 'conflict'}), isEmpty);
      expect(conflictsOf({'conflicts': 'none'}), isEmpty);
    });

    test('ignores fields of the wrong type', () {
      final c = conflictsOf({
        'conflicts': [
          {
            'code': 'i1_violation',
            'agent_id': 1,
            'credential_holder_id': true,
            'principal_ids': [_sam, 3, null],
            'organization_ready': 'yes',
          },
          {'code': 'i1_violation', 'principal_ids': 'x'},
        ],
      });
      expect(c, hasLength(2));
      expect(c.first.agentId, isNull);
      expect(c.first.holderId, isNull);
      expect(c.first.principalIds, [_sam]);
      expect(c.first.organizationReady, isNull);
      expect(c.last.principalIds, isEmpty);
    });
  });

  test(
    'a call without an answer is SelfActionFailed without a problem',
    () async {
      final failing = ApiAgentsRepository(
        createApiClient(
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
          adapter: _OfflineAdapter(),
        ),
        newIdempotencyKey: () => 'k',
      );
      await expectLater(
        failing.revokeToken(_token),
        throwsA(
          isA<SelfActionFailed>()
              .having((e) => e.problem, 'problem', isNull)
              .having((e) => e.conflicts, 'conflicts', isEmpty),
        ),
      );
    },
  );
}

/// Fails every request like a lost network.
class _OfflineAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) => throw DioException.connectionError(
    requestOptions: options,
    reason: 'offline',
  );

  @override
  void close({bool force = false}) {}
}
