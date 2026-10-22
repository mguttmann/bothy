// SPDX-License-Identifier: Apache-2.0

// The REST source of "Permission hints" and "Expiring soon" (brief M6-07 Design notes 3 and 4;
// 07 9.4, 9.5): `listPermissionHints` and `listExpiringAccess` map the wire shapes of
// docs/api/openapi.yaml to the types of admin_ui unchanged, send the filters and the cursor as
// query parameters and turn a problem answer into SelfActionFailed. A routing adapter answers
// by method and path; no socket is opened.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter_test/flutter_test.dart';

import '../access/api_access_repository_test.dart' show RoutingAdapter;

const String _alex = '0192f4a0-7b1c-7f89-8d6b-6960453241bc';
const String _sam = '0192f4a0-7b1c-733c-835c-6d67630008cd';
const String _robin = '0192f4a0-7b1c-7b9a-8c1d-2e3f4a5b6c7d';
const String _texter = '0192f4a0-7b1c-7d07-84b0-358556a08a99';
const String _app = '0192f4a0-7b1c-7e01-8f02-0a0b0c0d0e0f';
const String _space = '0192f4a0-7b1c-7fd5-831e-d4370e087874';
const String _intern = '0192f4a0-7b1c-72d4-8bae-7661d2b2f203';
const String _grant = '0192f4a0-7b1c-727f-86bd-90fe7bdb3024';
const String _group = '0192f4a0-7b1c-7aaa-8bbb-6960453241bc';
const String _membership = '0192f4a0-7b1c-7b22-8d3e-4f5a6b7c8d9e';
const String _token = '0192f4a0-7b1c-7c44-8e5f-6a7b8c9d0e1f';
const String _appToken = '0192f4a0-7b1c-7c55-8e6f-7a8b9c0d1e2f';
const String _at = '2026-10-10T09:30:00.000Z';

Map<String, Object?> _ref(String id, String name, {String kind = 'user'}) => {
  'id': id,
  'kind': kind,
  'handle': name.toLowerCase(),
  'display_name': name,
};

Map<String, Object?> _grantJson({String level = 'trigger'}) => {
  'id': _grant,
  'subject_kind': 'user',
  'subject_id': _robin,
  'target_kind': 'channel',
  'target_id': _intern,
  'level': level,
  'capabilities': ['upload'],
  'expires_at': '2026-10-31T23:00:00.000Z',
  'reason': 'Campaign work',
  'granted_by': _alex,
  'granted_at': _at,
};

Map<String, Object?> _labels() => {
  'principals': [
    _ref(_texter, 'Texter', kind: 'agent'),
    _ref(_sam, 'Sam'),
    _ref(_robin, 'Robin'),
    _ref(_app, 'Board', kind: 'app'),
  ],
  'groups': [
    {'id': _group, 'name': 'Support'},
  ],
  'spaces': [
    {'id': _space, 'name': 'Marketing'},
  ],
  'channels': [
    {'id': _intern, 'space_id': _space, 'name': 'Intern'},
  ],
};

Map<String, (int, Object)> _routes() => {
  'GET /api/v1/access/hints': (
    200,
    {
      'items': [
        {
          'kind': 'h1_read_horizon',
          'agent_id': _texter,
          'person_id': _sam,
          'space_id': _space,
          'channel_id': _intern,
        },
        {'kind': 'grant_to_disabled', 'grant': _grantJson(level: 'read')},
        {
          'kind': 'trigger_blocked',
          'grant': _grantJson(),
          'rules': ['sr2', 'sr3'],
        },
        {
          'kind': 'agent_personal',
          'agent_id': _texter,
          'credential_holder_id': _sam,
          'trigger_set': [_sam, _robin],
        },
      ],
      'labels': _labels(),
      'next_cursor': 'page-2',
    },
  ),
  'GET /api/v1/access/expiring': (
    200,
    {
      'items': [
        {
          'kind': 'grant',
          'expires_at': '2026-10-31T23:00:00.000Z',
          'grant': _grantJson(),
        },
        {
          'kind': 'membership',
          'expires_at': '2026-11-01T23:00:00.000Z',
          'membership_id': _membership,
          'group_id': _group,
          'user_id': _sam,
        },
        {
          'kind': 'token',
          'expires_at': '2026-11-02T08:00:00.000Z',
          'token_id': _token,
          'principal_id': _texter,
        },
        {
          'kind': 'token',
          'expires_at': '2026-11-03T08:00:00.000Z',
          'token_id': _appToken,
          'principal_id': _app,
        },
      ],
      'labels': _labels(),
    },
  ),
};

void main() {
  late RoutingAdapter adapter;
  late AdminRepository repository;

  setUp(() {
    adapter = RoutingAdapter(_routes());
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
      newIdempotencyKey: () => '0192f4a0-0000-7000-8000-000000000001',
    );
  });

  group('permissionHints', () {
    test('maps every kind and the labels of the page', () async {
      final page = await repository.permissionHints();

      final query = adapter.seen.single.options.uri.queryParameters;
      expect(query, isEmpty, reason: 'the instance: no space, no kind');
      expect(page.nextCursor, 'page-2');
      expect(page.items.map((h) => h.kind), [
        AccessHintKind.h1ReadHorizon,
        AccessHintKind.grantToDisabled,
        AccessHintKind.triggerBlocked,
        AccessHintKind.agentPersonal,
      ]);
      final h1 = page.items[0];
      expect(
        (h1.agentId, h1.personId, h1.spaceId, h1.channelId),
        (_texter, _sam, _space, _intern),
      );
      expect(h1.grant, isNull);
      final disabled = page.items[1].grant!;
      expect(disabled.id, _grant);
      expect(disabled.subjectId, _robin);
      expect(disabled.level, AccessLevel.read);
      expect(disabled.targetKind, AdminTargetKind.channel);
      expect(disabled.capabilities, [AccessCapability.upload]);
      expect(page.items[2].rules, ['sr2', 'sr3']);
      final personal = page.items[3];
      expect(personal.agentId, _texter);
      expect(personal.credentialHolderId, _sam);
      expect(personal.triggerSet, [_sam, _robin]);

      final labels = page.labels;
      expect(labels.principal(_texter), 'Texter');
      expect(labels.principals[_texter]!.kind, 'agent');
      expect(labels.principals[_app]!.kind, 'app');
      expect(labels.group(_group), 'Support');
      expect(labels.space(_space), 'Marketing');
      expect(labels.channelPath(_intern), 'Marketing/Intern');
      final target = labels.target(AdminTargetKind.channel, _intern);
      expect(target, const AdminTarget(AdminTargetKind.channel, id: _intern));
      expect(target.path, 'Marketing/Intern');
      expect(labels.target(AdminTargetKind.space, _space).name, 'Marketing');
      expect(labels.target(null, null), AdminTarget.instance);
      expect(labels.principal('unknown'), 'unknown');
    });

    test(
      'sends the space, the kind and the cursor as query parameters',
      () async {
        await repository.permissionHints(
          spaceId: _space,
          kind: AccessHintKind.triggerBlocked,
          cursor: 'page-2',
        );

        expect(adapter.seen.single.options.uri.queryParameters, {
          'space_id': _space,
          'kind': 'trigger_blocked',
          'cursor': 'page-2',
        });
      },
    );

    test('a refused list is a SelfActionFailed with the reason', () async {
      adapter.routes['GET /api/v1/access/hints'] = (
        403,
        {
          'type': 'about:blank',
          'title': 'Forbidden',
          'status': 403,
          'code': 'forbidden',
          'reason': 'owner_only',
        },
      );

      await expectLater(
        repository.permissionHints(),
        throwsA(
          isA<SelfActionFailed>()
              .having((e) => e.problem?.status, 'status', 403)
              .having((e) => e.problem?.reason, 'reason', 'owner_only'),
        ),
      );
    });
  });

  group('expiringAccess', () {
    test('maps grants, memberships and tokens with their labels', () async {
      final page = await repository.expiringAccess();

      expect(adapter.seen.single.options.uri.queryParameters, isEmpty);
      expect(page.nextCursor, isNull);
      expect(page.items.map((e) => e.kind), [
        ExpiringKind.grant,
        ExpiringKind.membership,
        ExpiringKind.token,
        ExpiringKind.token,
      ]);
      final grant = page.items[0];
      expect(grant.expiresAt, DateTime.utc(2026, 10, 31, 23));
      expect(grant.grant!.id, _grant);
      final membership = page.items[1];
      expect(
        (membership.membershipId, membership.groupId, membership.userId),
        (_membership, _group, _sam),
      );
      final token = page.items[2];
      expect((token.tokenId, token.principalId), (_token, _texter));
      expect(token.grant, isNull);
      expect(page.labels.principals[_app]!.kind, 'app');
    });

    test('sends the space and the cursor as query parameters', () async {
      await repository.expiringAccess(spaceId: _space, cursor: 'next');

      expect(adapter.seen.single.options.uri.queryParameters, {
        'space_id': _space,
        'cursor': 'next',
      });
    });

    test('a refused list is a SelfActionFailed with the reason', () async {
      adapter.routes['GET /api/v1/access/expiring'] = (
        404,
        {
          'type': 'about:blank',
          'title': 'Not found',
          'status': 404,
          'code': 'not_found',
        },
      );

      await expectLater(
        repository.expiringAccess(spaceId: _space),
        throwsA(
          isA<SelfActionFailed>().having(
            (e) => e.problem?.status,
            'status',
            404,
          ),
        ),
      );
    });
  });

  test('labels of a later page join the labels of the first', () {
    const first = AccessPageLabels(
      principals: {
        _sam: AccessPrincipal(
          id: _sam,
          kind: 'user',
          handle: 'sam',
          displayName: 'Sam',
        ),
      },
      spaces: {_space: 'Marketing'},
    );
    const next = AccessPageLabels(
      groups: {_group: 'Support'},
      channels: {_intern: (spaceId: _space, name: 'Intern')},
    );

    final all = first.merge(next);

    expect(all.principal(_sam), 'Sam');
    expect(all.group(_group), 'Support');
    expect(all.channelPath(_intern), 'Marketing/Intern');
  });
}
