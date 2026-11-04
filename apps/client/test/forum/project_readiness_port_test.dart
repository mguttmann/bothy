// SPDX-License-Identifier: Apache-2.0

// The server side of the project banner (F-PROJEKT-BEREIT): the readiness read from the
// binding, the effective operator and the only runner, and the two actions with their
// preconditions, over a scripted HTTP adapter.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:client/forum/project_readiness.dart';
import 'package:client/new_project/new_project_flow.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart' as d;
import 'package:flutter_test/flutter_test.dart';

const String _at = '2026-10-10T09:30:00.000Z';
const String _forumId = '0192f4a0-7b1c-72d4-8bae-000000000002';
const String _runnerId = '0192f4a0-7b1c-72d4-8bae-000000000003';
const String _hermesId = '0192f4a0-7b1c-72d4-8bae-000000000004';
const String _spaceId = '0192f4a0-7b1c-72d4-8bae-000000000006';

final d.ChannelId _forum = d.ChannelId.parse(_forumId);

Map<String, Object?> _agent(
  String id,
  String name,
  String kind, {
  String status = 'active',
  String presence = 'online',
}) => {
  'id': id,
  'handle': name.toLowerCase().replaceAll(' ', '-'),
  'display_name': name,
  'status': status,
  'adapter_kind': kind,
  'description': '',
  'billing_mode': 'organization',
  'tool_class': 'chat',
  'presence': {'agent_id': id, 'state': presence},
  'operator_config': <String, Object?>{},
  'version': 1,
};

Map<String, Object?> _behavior({Map<String, Object?>? binding}) => {
  'require_mention': false,
  'thread_require_mention': false,
  'free_response': true,
  'auto_thread': false,
  'allow_mentioned_agents': true,
  'history_backfill_limit': 50,
  'prompt': '',
  'skills': <Object?>[],
  'project_binding': ?binding,
  'settings_version': 3,
};

Map<String, Object?> _binding({required bool ready, String? reason}) => {
  'workspace_ref': 'shop',
  'repository_url': 'https://git.example.com/shop.git',
  'default_branch': 'main',
  'runner_config': {'model': 'opus'},
  'ready': ready,
  'ready_reason': ?reason,
  'version': 7,
};

Map<String, Object?> _channel({String? effective}) => {
  'channel': {
    'id': _forumId,
    'space_id': _spaceId,
    'kind': 'forum',
    'name': 'Shop',
    'topic': '',
    'sort_key': 'a1',
    'read_only': false,
    'project_bound': true,
    'operator': {'mode': 'agent', 'effective_agent_id': ?effective},
    'version': 4,
    'settings_version': 3,
    'created_at': _at,
  },
  'level': 'manage',
  'capabilities': <Object?>[],
};

/// Answers every request from [routes] (`METHOD path` to status and body) and records it.
final class _Adapter implements HttpClientAdapter {
  _Adapter(this.routes);

  final Map<String, (int, Object?)> routes;
  final List<(String, Object?, Map<String, dynamic>)> seen = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final query = options.uri.query;
    final key =
        '${options.method} ${options.uri.path}${query.isEmpty ? '' : '?$query'}';
    final bytes = requestStream == null
        ? <int>[]
        : [for (final chunk in await requestStream.toList()) ...chunk];
    seen.add((
      key,
      bytes.isEmpty ? null : jsonDecode(utf8.decode(bytes)),
      options.headers,
    ));
    final (status, body) = routes[key] ?? (404, {'code': 'not_found'});
    return ResponseBody.fromString(
      body == null ? '' : jsonEncode(body),
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

ApiProjectReadinessPort _port(_Adapter adapter) => ApiProjectReadinessPort(
  ApiClient(
    dio: Dio(BaseOptions(baseUrl: 'https://chat.example.com/v1'))
      ..httpClientAdapter = adapter,
    interceptors: const [],
  ),
);

const String _behaviorPath = 'GET /v1/channels/$_forumId/behavior';
const String _channelPath = 'GET /v1/channels/$_forumId';

void main() {
  test(
    'a forum without a binding or an unreadable binding reads as null',
    () async {
      expect(
        await _port(_Adapter({_behaviorPath: (200, _behavior())})).read(_forum),
        isNull,
      );
      expect(
        await _port(
          _Adapter({
            _behaviorPath: (403, {'code': 'forbidden', 'status': 403}),
          }),
        ).read(_forum),
        isNull,
      );
    },
  );

  test('a ready binding reads as ready without further requests', () async {
    final adapter = _Adapter({
      _behaviorPath: (200, _behavior(binding: _binding(ready: true))),
    });
    final view = await _port(adapter).read(_forum);
    expect(view?.ready, isTrue);
    expect(adapter.seen.map((s) => s.$1), [_behaviorPath]);
  });

  test('a chat agent as operator: the cause, the offline operator and the only runner', () async {
    final adapter = _Adapter({
      _behaviorPath: (
        200,
        _behavior(
          binding: _binding(ready: false, reason: reasonOperatorCannotServe),
        ),
      ),
      _channelPath: (200, _channel(effective: _hermesId)),
      'GET /v1/agents/$_hermesId': (
        200,
        _agent(_hermesId, 'Hermes', 'hermes', presence: 'offline'),
      ),
      'GET /v1/agents': (
        200,
        {
          'items': [
            _agent(_hermesId, 'Hermes', 'hermes'),
            _agent(_runnerId, 'Claude Code', 'code_runner'),
          ],
          'next_cursor': 'next',
        },
      ),
      'GET /v1/agents?cursor=next': (
        200,
        {
          'items': [
            _agent(
              '0192f4a0-7b1c-72d4-8bae-000000000007',
              'Old runner',
              'code_runner',
              status: 'disabled',
            ),
          ],
        },
      ),
    });
    final view = (await _port(adapter).read(_forum))!;
    expect(view.ready, isFalse);
    expect(view.reason, reasonOperatorCannotServe);
    expect(view.runnerName, 'Hermes');
    expect(view.runnerOffline, isTrue);
    expect(view.fixRunner?.id.value, _runnerId);
    expect(view.fixRunner?.name, 'Claude Code');
  });

  test('without a readable operator or with two runners the banner still names the cause', () async {
    final adapter = _Adapter({
      _behaviorPath: (
        200,
        _behavior(
          binding: _binding(ready: false, reason: reasonOperatorCannotServe),
        ),
      ),
      'GET /v1/agents': (
        200,
        {
          'items': [
            _agent(_runnerId, 'Claude Code', 'code_runner'),
            _agent(_hermesId, 'Second runner', 'code_runner'),
          ],
        },
      ),
    });
    final view = (await _port(adapter).read(_forum))!;
    expect(view.reason, reasonOperatorCannotServe);
    expect(view.runnerName, isNull);
    expect(view.fixRunner, isNull);
  });

  test('a failed list of agents offers no runner', () async {
    final adapter = _Adapter({
      _behaviorPath: (
        200,
        _behavior(
          binding: _binding(ready: false, reason: reasonOperatorCannotServe),
        ),
      ),
      _channelPath: (200, _channel()),
    });
    final view = (await _port(adapter).read(_forum))!;
    expect(view.runnerName, isNull);
    expect(view.fixRunner, isNull);
  });

  test(
    'setRunner sets the agent as operator against the version it read',
    () async {
      final adapter = _Adapter({
        _channelPath: (200, _channel(effective: _hermesId)),
        'PUT /v1/channels/$_forumId/operator': (
          200,
          _channel(effective: _runnerId)['channel'],
        ),
      });
      await _port(adapter).setRunner(
        _forum,
        d.PrincipalId.parse(_runnerId),
        reason: 'Fix the project',
        purpose: 'Change Shop',
      );
      final put = adapter.seen.last;
      expect(put.$1, 'PUT /v1/channels/$_forumId/operator');
      expect(put.$2, {
        'mode': 'agent',
        'agent_id': _runnerId,
        'reason': 'Fix the project',
      });
      expect(put.$3['If-Match'] ?? put.$3['if-match'], '"4"');
    },
  );

  test(
    'renameFolder keeps the other values of the binding and needs one',
    () async {
      final adapter = _Adapter({
        _behaviorPath: (
          200,
          _behavior(
            binding: _binding(ready: false, reason: 'workspace_exists'),
          ),
        ),
        'PUT /v1/channels/$_forumId/project-binding': (
          200,
          _binding(ready: false, reason: 'provisioning'),
        ),
      });
      await _port(adapter).renameFolder(
        _forum,
        'shop-2',
        reason: 'Fix the project',
        purpose: 'Change Shop',
      );
      final put = adapter.seen.last;
      expect(put.$1, 'PUT /v1/channels/$_forumId/project-binding');
      final body = put.$2! as Map<String, Object?>;
      expect(body['workspace_ref'], 'shop-2');
      expect(body['repository_url'], 'https://git.example.com/shop.git');
      expect(body['default_branch'], 'main');
      expect(body['runner_config'], {'model': 'opus'});
      expect(put.$3['If-Match'] ?? put.$3['if-match'], '"7"');

      await expectLater(
        _port(_Adapter({_behaviorPath: (200, _behavior())})).renameFolder(
          _forum,
          'shop-2',
          reason: 'Fix the project',
          purpose: 'Change Shop',
        ),
        throwsA(isA<NewProjectFailure>()),
      );
    },
  );
}
