// SPDX-License-Identifier: Apache-2.0

// The REST side of editing a built-in agent (brief A1-09 requirements 12 to 16; 26 4.1, 8.4;
// 07 2.13): the agent's state carries its identity, runner and status; the built-in agents
// are the agents of kind claude_agent; the versions come newest first over every page; an
// older version is read with `version`; putDefinition sends If-Match of the edited version
// and the edit purpose; setAgentHost sends If-Match, the runner, the reason and the purpose;
// disableAgent sends the person's reason with step-up; the owner names come with the context;
// a 412 keeps its code for the reload.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' hide AgentPage;
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

typedef _Seen = ({RequestOptions options, Object? json});

class _Adapter implements HttpClientAdapter {
  _Adapter(this.routes);

  final Map<String, (int, Object?)> routes;
  final List<_Seen> seen = [];

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
    final query = options.uri.query;
    final key =
        '${options.method} ${options.uri.path}${query.isEmpty ? '' : '?$query'}';
    final (status, answer) =
        routes[key] ?? (throw StateError('no route for $key'));
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
const String _sam = '0192f4a0-7b1c-7f89-8d6b-6960453241bd';
const String _runner = '0192f4a0-7b1c-7d07-84b0-358556a08a99';
const String _other = '0192f4a0-7b1c-7d07-84b0-358556a08a96';
const String _notes = '0192f4a0-7b1c-7d07-84b0-358556a08a97';
const String _helper = '0192f4a0-7b1c-7d07-84b0-358556a08a98';
const String _at = '2026-11-07T09:30:00.000Z';

Map<String, Object?> _agent(
  String id,
  String handle, {
  String kind = 'claude_agent',
  String status = 'active',
  String toolClass = 'chat',
  String? hostedBy = _runner,
  int version = 1,
}) => {
  'id': id,
  'handle': handle,
  'display_name': 'The $handle',
  'status': status,
  'adapter_kind': kind,
  'description': 'Keeps $handle',
  'billing_mode': 'organization',
  'tool_class': toolClass,
  'presence': {'state': 'online'},
  'operator_config': <String, Object?>{},
  'hosted_by': ?hostedBy,
  'version': version,
};

const Map<String, Object?> _definition = {
  'instructions': 'Keep the notes of Sam.',
  'skills': <Object?>[],
  'mcp_servers': <Object?>[],
  'settings': {'tool_class_wish': 'chat'},
  'origin': {'kind': 'manual'},
};

Map<String, Object?> _version(int n, {String kind = 'manual'}) => {
  'agent_id': _notes,
  'version': n,
  'origin_kind': kind,
  'created_at': _at,
  'created_by': _alex,
};

void main() {
  late _Adapter adapter;
  late ApiNewAgentPort port;

  ApiNewAgentPort build(Map<String, (int, Object?)> routes) {
    adapter = _Adapter(routes);
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
    return port = ApiNewAgentPort(
      client,
      purpose: (name) => 'Create the agent $name',
      reason: (name) => 'New agent $name',
      purposeOf: (change, name) => '${change.name} purpose $name',
      reasonOf: (change, name) => '${change.name} reason $name',
    );
  }

  _Seen seenOf(String method, String path) => adapter.seen.lastWhere(
    (s) => s.options.method == method && s.options.uri.path == path,
  );

  test('the state carries the identity, the runner and the status', () async {
    build({
      'GET /api/v1/agents/$_notes': (
        200,
        _agent(_notes, 'notes', status: 'disabled'),
      ),
    });
    final state = await port.agent(_notes);
    expect(state.name, 'The notes');
    expect(state.handle, 'notes');
    expect(state.description, 'Keeps notes');
    expect(state.hostedBy, _runner);
    expect(state.active, isFalse);
  });

  test('the built-in agents are the agents of kind claude_agent', () async {
    build({
      'GET /api/v1/agents': (
        200,
        {
          'items': [
            _agent(_runner, 'dev-runner', kind: 'code_runner', hostedBy: null),
            _agent(_notes, 'notes'),
            _agent(_helper, 'helper', status: 'disabled'),
          ],
        },
      ),
    });
    final agents = await port.builtinAgents();
    expect([for (final a in agents) a.id], [_notes, _helper]);
  });

  test('the versions come newest first over every page', () async {
    build({
      'GET /api/v1/agents/$_notes/definition/versions': (
        200,
        {
          'items': [_version(3), _version(2, kind: 'template')],
          'next_cursor': 'c2',
        },
      ),
      'GET /api/v1/agents/$_notes/definition/versions?cursor=c2': (
        200,
        {
          'items': [_version(1, kind: 'interview')],
        },
      ),
    });
    final versions = await port.versions(_notes);
    expect([for (final v in versions) v.version], [3, 2, 1]);
    expect(versions[1].origin, DraftOrigin.template);
    expect(versions[2].origin, DraftOrigin.interview);
    expect(versions.first.createdBy, _alex);
    expect(versions.first.createdAt, DateTime.parse(_at));
  });

  test('an older version is read with the query version', () async {
    build({
      'GET /api/v1/agents/$_notes/definition?version=2': (
        200,
        {..._version(2), 'definition': _definition},
      ),
    });
    final v = await port.definitionVersion(_notes, 2);
    expect(v.version, 2);
    expect(v.json, _definition);
  });

  test(
    'an edit sends If-Match of the edited version and its purpose',
    () async {
      build({
        'PUT /api/v1/agents/$_notes/definition': (
          200,
          {..._version(5), 'definition': _definition},
        ),
      });
      await port.putDefinition(
        _notes,
        _definition,
        ifMatch: 4,
        name: 'Notes',
        change: AgentChangeKind.edit,
      );
      final seen = seenOf('PUT', '/api/v1/agents/$_notes/definition');
      expect(seen.options.headers['If-Match'], '"4"');
      expect(seen.options.extra[stepUpPurposeExtra], 'edit purpose Notes');
      expect(seen.json, _definition);
    },
  );

  test('the class step of an edit sends the edit reason', () async {
    build({
      'PUT /api/v1/agents/$_notes/tool-class': (
        200,
        _agent(_notes, 'notes', toolClass: 'host', version: 4),
      ),
    });
    await port.setToolClass(
      const NewAgentState(
        id: _notes,
        version: 3,
        toolClass: AgentToolClass.chat,
      ),
      AgentToolClass.host,
      name: 'Notes',
      change: AgentChangeKind.edit,
    );
    final seen = seenOf('PUT', '/api/v1/agents/$_notes/tool-class');
    expect(seen.options.headers['If-Match'], '"3"');
    expect((seen.json! as Map)['reason'], 'edit reason Notes');
    expect(seen.options.extra[stepUpPurposeExtra], 'edit purpose Notes');
  });

  test('without purposeOf the creation texts stay', () async {
    adapter = _Adapter({
      'PUT /api/v1/agents/$_notes/definition': (
        200,
        {..._version(1), 'definition': _definition},
      ),
    });
    final plain = ApiNewAgentPort(
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
        adapter: adapter,
      ),
      purpose: (name) => 'Create the agent $name',
      reason: (name) => 'New agent $name',
    );
    await plain.putDefinition(
      _notes,
      _definition,
      name: 'Notes',
      change: AgentChangeKind.edit,
    );
    expect(
      seenOf(
        'PUT',
        '/api/v1/agents/$_notes/definition',
      ).options.extra[stepUpPurposeExtra],
      'Create the agent Notes',
    );
  });

  test(
    'change runner sends If-Match, the runner, the reason and the purpose',
    () async {
      build({
        'PUT /api/v1/agents/$_notes/host': (
          200,
          _agent(_notes, 'notes', hostedBy: _other, version: 6),
        ),
      });
      final state = await port.setHost(
        const NewAgentState(
          id: _notes,
          version: 5,
          toolClass: AgentToolClass.chat,
        ),
        _other,
        name: 'Notes',
      );
      expect(state.hostedBy, _other);
      final seen = seenOf('PUT', '/api/v1/agents/$_notes/host');
      expect(seen.options.headers['If-Match'], '"5"');
      expect(seen.json, {'hosted_by': _other, 'reason': 'host reason Notes'});
      expect(seen.options.extra[stepUpPurposeExtra], 'host purpose Notes');
    },
  );

  test('a refusal of change runner keeps its reason', () async {
    build({
      'PUT /api/v1/agents/$_notes/host': (
        409,
        {
          'code': 'conflict',
          'status': 409,
          'reason': 'host_tool_class_ceiling',
        },
      ),
    });
    await expectLater(
      port.setHost(
        const NewAgentState(
          id: _notes,
          version: 5,
          toolClass: AgentToolClass.host,
        ),
        _other,
        name: 'Notes',
      ),
      throwsA(
        isA<NewAgentFailure>().having(
          (f) => f.reason,
          'reason',
          'host_tool_class_ceiling',
        ),
      ),
    );
  });

  test('disable sends the reason with step-up', () async {
    build({
      'POST /api/v1/agents/$_notes/disable': (
        200,
        _agent(_notes, 'notes', status: 'disabled', version: 7),
      ),
    });
    final state = await port.disable(
      _notes,
      reason: 'Not needed any more',
      name: 'Notes',
    );
    expect(state.active, isFalse);
    final seen = seenOf('POST', '/api/v1/agents/$_notes/disable');
    expect(seen.json, {'reason': 'Not needed any more'});
    expect(seen.options.extra[stepUpPurposeExtra], 'disable purpose Notes');
  });

  test('a stale definition write is version_conflict', () async {
    build({
      'PUT /api/v1/agents/$_notes/definition': (
        412,
        {'code': 'version_conflict', 'status': 412},
      ),
    });
    await expectLater(
      port.putDefinition(
        _notes,
        _definition,
        ifMatch: 1,
        name: 'Notes',
        change: AgentChangeKind.edit,
      ),
      throwsA(
        isA<NewAgentFailure>().having(
          (f) => f.code,
          'code',
          'version_conflict',
        ),
      ),
    );
  });

  test('the context names the owners', () async {
    build({
      'GET /api/v1/me': (
        200,
        {
          'principal': {
            'id': _alex,
            'kind': 'user',
            'handle': 'alex',
            'display_name': 'Alex',
          },
          'is_owner': true,
          'permission_version': 1,
        },
      ),
      'GET /api/v1/owners': (
        200,
        {
          'items': [
            for (final (id, name) in [(_alex, 'Alex'), (_sam, 'Sam')])
              {
                'user': {
                  'id': id,
                  'kind': 'user',
                  'handle': name.toLowerCase(),
                  'display_name': name,
                },
                'granted_at': _at,
                'reason': 'Founder',
              },
          ],
        },
      ),
      'GET /api/v1/spaces': (200, {'items': <Object?>[]}),
    });
    final context = await port.context();
    expect(context.ownerNames, {_alex: 'Alex', _sam: 'Sam'});
    expect(context.onlyOwner, isFalse);
  });
}
