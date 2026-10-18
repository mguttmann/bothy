// SPDX-License-Identifier: Apache-2.0

// The REST source of the grant screens (brief M4-27 Design notes 1 and 2): every call maps
// the wire shapes of docs/api/openapi.yaml to the types of admin_ui unchanged, sends the full
// `GrantCreate` with an idempotency key and the step-up purpose, and turns a problem answer
// into SelfActionFailed. A routing adapter answers by method and path; no socket is opened.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// One request as the adapter saw it.
typedef Seen = ({RequestOptions options, Object? json});

/// Answers each request with the JSON of [routes] by `METHOD path` (status 200 unless the
/// route gives one); records every request.
class RoutingAdapter implements HttpClientAdapter {
  RoutingAdapter(this.routes);

  final Map<String, (int, Object)> routes;
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
    final (status, answer) =
        routes[key] ?? (throw StateError('no route for $key'));
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
const String _agent = '0192f4a0-7b1c-7d07-84b0-358556a08a99';
const String _space = '0192f4a0-7b1c-7fd5-831e-d4370e087874';
const String _channel = '0192f4a0-7b1c-72d4-8bae-7661d2b2f203';
const String _other = '0192f4a0-7b1c-7c2a-8e1d-41a6b2c3d4e5';
const String _grant = '0192f4a0-7b1c-727f-86bd-90fe7bdb3024';
const String _group = '0192f4a0-7b1c-7aaa-8bbb-6960453241bc';
const String _at = '2026-10-10T09:30:00.000Z';

Map<String, Object?> _ref(String id, String name, {String kind = 'user'}) => {
  'id': id,
  'kind': kind,
  'handle': name.toLowerCase(),
  'display_name': name,
};

Map<String, Object?> _spaceJson() => {
  'id': _space,
  'name': 'Marketing',
  'sort_key': 'a0',
  'version': 1,
  'created_at': _at,
};

Map<String, Object?> _channelJson(String id, String name) => {
  'id': id,
  'space_id': _space,
  'kind': 'text',
  'name': name,
  'topic': '',
  'sort_key': 'a1',
  'read_only': false,
  'operator': {'mode': 'inherit'},
  'version': 1,
  'settings_version': 1,
  'created_at': _at,
};

Map<String, Object?> _grantJson({String? revokedAt}) => {
  'id': _grant,
  'subject_kind': 'user',
  'subject_id': _sam,
  'target_kind': 'channel',
  'target_id': _channel,
  'level': 'trigger',
  'capabilities': ['upload', 'export'],
  'expires_at': '2026-12-31T23:00:00.000Z',
  'reason': 'Campaign work',
  'granted_by': _alex,
  'granted_at': _at,
  'revoked_at': ?revokedAt,
};

Map<String, (int, Object)> _routes() => {
  'GET /api/v1/spaces': (
    200,
    {
      'items': [
        {
          'space': _spaceJson(),
          'level': 'trigger',
          'capabilities': <Object?>[],
        },
      ],
    },
  ),
  'GET /api/v1/spaces/$_space/channels': (
    200,
    {
      'items': [
        {
          'channel': _channelJson(_channel, 'Kampagnen'),
          'level': 'trigger',
          'capabilities': <Object?>[],
        },
      ],
    },
  ),
  'GET /api/v1/agents': (
    200,
    {
      'items': [
        {
          'id': _agent,
          'handle': 'texter',
          'display_name': 'Texter',
          'status': 'active',
          'adapter_kind': 'scripted',
          'description': '',
          'billing_mode': 'organization',
          'tool_class': 'chat',
          'presence': {'state': 'unknown'},
          'operator_config': <String, Object?>{},
          'version': 1,
        },
      ],
    },
  ),
  'POST /api/v1/grants/preview': (
    200,
    {
      'allowed': false,
      'denial_reason': 'delegation_exceeds_own_level',
      'effects': [
        {
          'principal': _ref(_sam, 'Sam'),
          'target_kind': 'channel',
          'target_id': _channel,
          'before_level': 'none',
          'after_level': 'trigger',
          'capabilities_added': ['approve_actions', 'manage'],
        },
      ],
      'invariant_violations': [
        {
          'code': 'i1_violation',
          'agent_id': _agent,
          'principal_ids': [_sam],
          'organization_ready': true,
        },
      ],
      'hints': [
        for (final code in [
          'read_horizon',
          'trigger_blocked_tool_class',
          'group_reach',
          'expiry_missing',
          'terminal_not_effective',
        ])
          {
            'code': code,
            'agent_id': _agent,
            'principal_id': _sam,
            'channel_id': _other,
            'count': 4,
          },
      ],
      'text': 'Sam gets the level "Trigger".',
      'text_locale': 'en',
    },
  ),
  'POST /api/v1/grants': (201, _grantJson()),
  'GET /api/v1/grants': (
    200,
    {
      'items': [_grantJson()],
    },
  ),
  'POST /api/v1/grants/$_grant/revoke': (200, _grantJson(revokedAt: _at)),
  'GET /api/v1/channels/$_channel/access': (
    200,
    {
      'target_kind': 'channel',
      'target_id': _channel,
      'entries': [
        {
          'principal': _ref(_sam, 'Sam'),
          'level': 'trigger',
          'inactive': true,
          'capabilities': ['upload'],
          'sources': [
            {'kind': 'owner'},
            {
              'kind': 'direct',
              'grant_id': _grant,
              'expires_at': '2026-12-31T23:00:00.000Z',
              'reason': 'Stand-in',
            },
            {
              'kind': 'group',
              'group_id': _group,
              'membership_expires_at': '2026-11-30T23:00:00.000Z',
            },
            {
              'kind': 'inherited',
              'from_target_kind': 'space',
              'from_target_id': _space,
            },
            {'kind': 'dm_participant'},
            {'kind': 'operator', 'agent_id': _agent},
            {'kind': 'via_agent', 'agent_id': _agent},
            {'kind': 'guard_blocked', 'agent_id': _agent, 'rule': 'sr3'},
            {'kind': 'guard_blocked', 'agent_id': _agent, 'rule': 'sr2'},
          ],
        },
        {
          'principal': _ref(_agent, 'Texter', kind: 'agent'),
          'level': 'none',
          'capabilities': <Object?>[],
          'sources': <Object?>[],
        },
      ],
    },
  ),
  'GET /api/v1/spaces/$_space/access': (
    200,
    {
      'target_kind': 'space',
      'target_id': _space,
      'entries': [
        {
          'principal': _ref(_alex, 'Alex'),
          'level': 'read',
          'capabilities': <Object?>[],
          'sources': [
            {'kind': 'owner'},
          ],
        },
      ],
    },
  ),
  'GET /api/v1/users/$_sam/view-as': (
    200,
    {
      'user': _ref(_sam, 'Sam'),
      'expires_at': '2026-10-10T09:45:00.000Z',
      'spaces': [
        {'space': _spaceJson(), 'level': 'write', 'capabilities': <Object?>[]},
      ],
      'channels': [
        {
          'channel': _channelJson(_channel, 'Kampagnen'),
          'level': 'write',
          'capabilities': ['upload'],
        },
      ],
    },
  ),
  'GET /api/v1/users/$_alex/view-as': (
    403,
    {
      'type': 'about:blank',
      'title': 'Forbidden',
      'status': 403,
      'code': 'forbidden',
      'reason': 'owner_only',
    },
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

  const marketing = AdminTarget(
    AdminTargetKind.space,
    id: _space,
    name: 'Marketing',
  );
  const channel = AdminTarget(
    AdminTargetKind.channel,
    id: _channel,
    name: 'Kampagnen',
    spaceName: 'Marketing',
  );

  test('targets are the spaces, each followed by its channels', () async {
    final targets = await repository.targets();
    expect(targets.map((t) => t.key), ['space.$_space', 'channel.$_channel']);
    expect(targets.last.path, 'Marketing/Kampagnen');
    expect(targets.first.path, 'Marketing');
    expect(targets.first, marketing);
    expect(targets.first.hashCode, marketing.hashCode);
  });

  test('agents come from listAgents', () async {
    final agents = await repository.agents();
    expect(agents.single.displayName, 'Texter');
    expect(agents.single.id, _agent);
  });

  test('the preview sends the full request and maps every field', () async {
    final preview = await repository.previewGrant(
      GrantRequest(
        subjectKind: AdminSubjectKind.user,
        subjectId: _sam,
        target: channel,
        level: AccessLevel.trigger,
        capabilities: {AccessCapability.upload, AccessCapability.export},
        expiresAt: DateTime.utc(2026, 12, 31, 23),
        reason: 'Campaign work',
      ),
    );
    final sent = adapter.seen.single;
    expect(sent.json, {
      'subject_kind': 'user',
      'subject_id': _sam,
      'target_kind': 'channel',
      'target_id': _channel,
      'level': 'trigger',
      'capabilities': ['upload', 'export'],
      'expires_at': '2026-12-31T23:00:00.000Z',
      'reason': 'Campaign work',
    });
    expect(sent.options.headers.containsKey('Idempotency-Key'), isFalse);
    expect(preview.allowed, isFalse);
    expect(preview.denialReason, 'delegation_exceeds_own_level');
    expect(preview.text, 'Sam gets the level "Trigger".');
    expect(preview.textLocale, 'en');
    final effect = preview.effects.single;
    expect(effect.principal.displayName, 'Sam');
    expect(effect.before, AccessLevel.none);
    expect(effect.after, AccessLevel.trigger);
    expect(effect.capabilitiesAdded, [
      AccessCapability.approveActions,
      AccessCapability.manage,
    ]);
    expect(effect.targetKind, AdminTargetKind.channel);
    expect(effect.targetId, _channel);
    final violation = preview.violations.single;
    expect(violation.code, 'i1_violation');
    expect(violation.agentId, _agent);
    expect(violation.principalIds, [_sam]);
    expect(violation.organizationReady, isTrue);
    expect(preview.hints.map((h) => h.code), [
      'read_horizon',
      'trigger_blocked_tool_class',
      'group_reach',
      'expiry_missing',
      'terminal_not_effective',
    ]);
    expect(preview.hints.first.channelId, _other);
    expect(preview.hints.first.count, 4);
  });

  test(
    'every subject kind and target kind is sent with its wire name',
    () async {
      for (final (kind, wire) in [
        (AdminSubjectKind.group, 'group'),
        (AdminSubjectKind.agent, 'agent'),
        (AdminSubjectKind.app, 'app'),
      ]) {
        await repository.previewGrant(
          GrantRequest(
            subjectKind: kind,
            subjectId: _group,
            target: AdminTarget.instance,
            level: AccessLevel.read,
            reason: 'r',
          ),
        );
        final json = adapter.seen.last.json! as Map;
        expect(json['subject_kind'], wire);
        expect(json['target_kind'], 'instance');
        expect(json.containsKey('target_id'), isFalse);
        expect(json['level'], 'read');
      }
      await repository.previewGrant(
        const GrantRequest(
          subjectKind: AdminSubjectKind.user,
          subjectId: _sam,
          target: marketing,
          level: AccessLevel.write,
          reason: 'r',
        ),
      );
      final json = adapter.seen.last.json! as Map;
      expect(json['target_kind'], 'space');
      expect(json['level'], 'write');
    },
  );

  test('create carries an idempotency key and the step-up purpose', () async {
    final grant = await repository.createGrant(
      const GrantRequest(
        subjectKind: AdminSubjectKind.user,
        subjectId: _sam,
        target: channel,
        level: AccessLevel.trigger,
        capabilities: {AccessCapability.terminal},
        reason: 'Campaign work',
      ),
      purpose: 'Create the access grant for Sam',
    );
    final sent = adapter.seen.single.options;
    expect(sent.headers['Idempotency-Key'], isNotNull);
    expect(sent.extra[stepUpPurposeExtra], 'Create the access grant for Sam');
    expect(grant.id, _grant);
    expect(grant.subjectKind, AdminSubjectKind.user);
    expect(grant.targetKind, AdminTargetKind.channel);
    expect(grant.level, AccessLevel.trigger);
    expect(grant.capabilities, [
      AccessCapability.upload,
      AccessCapability.export,
    ]);
    expect(grant.expiresAt, DateTime.utc(2026, 12, 31, 23));
    expect(grant.revokedAt, isNull);
  });

  test('the grants of a target and their revocation', () async {
    final grants = await repository.grantsOn(channel);
    expect(grants.single.reason, 'Campaign work');
    final query = adapter.seen.single.options.uri.queryParameters;
    expect(query['target_kind'], 'channel');
    expect(query['target_id'], _channel);

    final revoked = await repository.revokeGrant(
      _grant,
      reason: 'Campaign over',
      purpose: 'Revoke',
    );
    expect(adapter.seen.last.json, {'reason': 'Campaign over'});
    expect(adapter.seen.last.options.extra[stepUpPurposeExtra], 'Revoke');
    expect(revoked.revokedAt, DateTime.parse(_at));
  });

  test('Who has access? maps every source kind of 07 9.1', () async {
    final explained = await repository.explain(channel);
    final sam = explained.entries.first;
    expect(sam.inactive, isTrue);
    expect(sam.capabilities, [AccessCapability.upload]);
    expect(sam.sources.map((s) => s.kind), [
      AccessSourceKind.owner,
      AccessSourceKind.direct,
      AccessSourceKind.group,
      AccessSourceKind.inherited,
      AccessSourceKind.dmParticipant,
      AccessSourceKind.operator,
      AccessSourceKind.viaAgent,
      AccessSourceKind.guardBlocked,
      AccessSourceKind.guardBlocked,
    ]);
    expect(sam.sources[1].grantId, _grant);
    expect(sam.sources[1].reason, 'Stand-in');
    expect(sam.sources[2].groupId, _group);
    expect(sam.sources[2].membershipExpiresAt, DateTime.utc(2026, 11, 30, 23));
    expect(sam.sources[3].fromTargetKind, AdminTargetKind.space);
    expect(sam.sources[3].fromTargetId, _space);
    expect(sam.sources[7].rule, 'sr3');
    expect(sam.sources[8].rule, 'sr2');
    final agent = explained.entries.last;
    expect(agent.principal.kind, 'agent');
    expect(agent.level, AccessLevel.none);
    expect(agent.inactive, isFalse);

    final space = await repository.explain(marketing);
    expect(space.entries.single.principal.displayName, 'Alex');
    expect(space.entries.single.level, AccessLevel.read);
  });

  test(
    'View as person maps the snapshot; a refusal is SelfActionFailed',
    () async {
      final view = await repository.viewAs(_sam, purpose: 'View as Sam');
      expect(
        adapter.seen.single.options.extra[stepUpPurposeExtra],
        'View as Sam',
      );
      expect(view.user.displayName, 'Sam');
      expect(view.expiresAt, DateTime.utc(2026, 10, 10, 9, 45));
      expect(view.spaces.single.name, 'Marketing');
      expect(view.spaces.single.level, AccessLevel.write);
      expect(view.channels.single.spaceId, _space);
      expect(view.channels.single.capabilities, [AccessCapability.upload]);

      await expectLater(
        repository.viewAs(_alex, purpose: 'View as Alex'),
        throwsA(
          isA<SelfActionFailed>()
              .having((e) => e.problem?.status, 'status', 403)
              .having((e) => e.problem?.reason, 'reason', 'owner_only'),
        ),
      );
    },
  );
}
