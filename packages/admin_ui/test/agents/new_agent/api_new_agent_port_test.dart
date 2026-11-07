// SPDX-License-Identifier: Apache-2.0

// The REST side of "New agent" (brief A1-08 Design notes 2 and 3; 26 8.1, 8.2): the offer is
// null on an older server (404 of listAgentTemplates) and without a runner that reports
// hosts_agents; createAgent sends adapter_kind claude_agent, hosted_by, the Idempotency-Key and
// the step-up purpose; the class, billing and operator steps send If-Match and the reason;
// putDefinition sends the definition unchanged; a 404 of getAgentDefinition is "not set up";
// field errors keep their JSON Pointer; the interview sends the turns and maps the answer.

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
    final key = '${options.method} ${options.uri.path}';
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
const String _runner = '0192f4a0-7b1c-7d07-84b0-358556a08a99';
const String _helper = '0192f4a0-7b1c-7d07-84b0-358556a08a98';
const String _notes = '0192f4a0-7b1c-7d07-84b0-358556a08a97';
const String _space = '0192f4a0-7b1c-7fd5-831e-d4370e087874';
const String _channel = '0192f4a0-7b1c-72d4-8bae-7661d2b2f203';
const String _at = '2026-11-07T09:30:00.000Z';

Map<String, Object?> _agent(
  String id,
  String handle, {
  String kind = 'hermes',
  String toolClass = 'chat',
  Map<String, Object?> presence = const {'state': 'online'},
  String? hostedBy,
  int version = 1,
}) => {
  'id': id,
  'handle': handle,
  'display_name': handle.toUpperCase(),
  'status': 'active',
  'adapter_kind': kind,
  'description': '',
  'billing_mode': 'organization',
  'tool_class': toolClass,
  'presence': presence,
  'operator_config': <String, Object?>{},
  'hosted_by': ?hostedBy,
  'version': version,
};

const Map<String, Object?> _definition = {
  'instructions': 'Keep the notes of Sam.',
  'skills': <Object?>[],
  'mcp_servers': [
    {
      'name': 'docs',
      'transport': 'http',
      'url': 'https://mcp.example.com/v1',
      'headers': [
        {'name': 'Authorization', 'secret_ref': 'env:DOCS_TOKEN'},
      ],
    },
  ],
  'settings': {'tool_class_wish': 'scoped', 'max_cost_usd_per_run': 2.5},
  'origin': {'kind': 'manual'},
};

Map<String, Object?> _template() => {
  'slug': 'writing-helper',
  'source': 'shipped',
  'name': 'Writing helper',
  'description': 'Helps with texts',
  'version': '1.0.0',
  'required_secrets': ['DOCS_TOKEN'],
  'definition': _definition,
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
    );
  }

  _Seen seenOf(String method, String path) => adapter.seen.lastWhere(
    (s) => s.options.method == method && s.options.uri.path == path,
  );

  final runnerCard = _agent(
    _runner,
    'dev-runner',
    kind: 'code_runner',
    presence: {
      'state': 'online',
      'hosts_agents': 'host',
      'billing_modes_supported': ['organization'],
    },
  );

  group('the offer', () {
    test('is null on a server without listAgentTemplates', () async {
      build({
        'GET /api/v1/agent-templates': (404, {'code': 'not_found'}),
        'GET /api/v1/agents': (
          200,
          {
            'items': [runnerCard],
          },
        ),
      });
      expect(await port.offer(), isNull);
    });

    test('is null without a runner that reports hosts_agents', () async {
      build({
        'GET /api/v1/agent-templates': (
          200,
          {
            'items': [_template()],
          },
        ),
        'GET /api/v1/agents': (
          200,
          {
            'items': [_agent(_helper, 'helper')],
          },
        ),
      });
      expect(await port.offer(), isNull);
    });

    test('lists the hosting runners and the templates', () async {
      build({
        'GET /api/v1/agent-templates': (
          200,
          {
            'items': [_template()],
          },
        ),
        'GET /api/v1/agents': (
          200,
          {
            'items': [runnerCard, _agent(_helper, 'helper')],
          },
        ),
      });
      final offer = (await port.offer())!;
      expect(offer.runners.single.id, _runner);
      expect(offer.runners.single.ceiling, AgentToolClass.host);
      expect(offer.templates.single.slug, 'writing-helper');
      expect(offer.templates.single.requiredSecrets, ['DOCS_TOKEN']);
      expect(offer.templates.single.definition, _definition);
    });
  });

  test(
    'createAgent sends the built-in kind, the host, the key and the purpose',
    () async {
      build({
        'POST /api/v1/agents': (
          201,
          _agent(_notes, 'notes', kind: 'claude_agent', hostedBy: _runner),
        ),
      });
      final state = await port.createAgent(
        handle: 'notes',
        displayName: 'Notes',
        description: 'Keeps notes',
        hostedBy: _runner,
        idempotencyKey: '0192f4a0-7b1c-7000-8000-000000000001',
      );
      expect(state.id, _notes);
      final seen = seenOf('POST', '/api/v1/agents');
      expect(seen.json, {
        'handle': 'notes',
        'display_name': 'Notes',
        'adapter_kind': 'claude_agent',
        'description': 'Keeps notes',
        'hosted_by': _runner,
      });
      expect(
        seen.options.headers['Idempotency-Key'],
        '0192f4a0-7b1c-7000-8000-000000000001',
      );
      expect(seen.options.extra[stepUpPurposeExtra], 'Create the agent Notes');
    },
  );

  test('the class step sends If-Match and the reason', () async {
    build({
      'PUT /api/v1/agents/$_notes/tool-class': (
        200,
        _agent(
          _notes,
          'notes',
          kind: 'claude_agent',
          toolClass: 'host',
          version: 3,
        ),
      ),
    });
    final next = await port.setToolClass(
      const NewAgentState(
        id: _notes,
        version: 2,
        toolClass: AgentToolClass.chat,
      ),
      AgentToolClass.host,
      name: 'Notes',
    );
    expect(next.toolClass, AgentToolClass.host);
    final seen = seenOf('PUT', '/api/v1/agents/$_notes/tool-class');
    expect(seen.options.headers['If-Match'], '"2"');
    expect(seen.json, {'tool_class': 'host', 'reason': 'New agent Notes'});
  });

  test('the definition goes unchanged; a missing one is not set up', () async {
    build({
      'PUT /api/v1/agents/$_notes/definition': (
        200,
        {
          'agent_id': _notes,
          'version': 1,
          'origin_kind': 'manual',
          'created_at': _at,
          'created_by': _alex,
          'definition': _definition,
        },
      ),
      'GET /api/v1/agents/$_notes/definition': (404, {'code': 'not_found'}),
    });
    final stored = await port.putDefinition(_notes, _definition, name: 'Notes');
    expect(stored.version, 1);
    expect(stored.json, _definition);
    final seen = seenOf('PUT', '/api/v1/agents/$_notes/definition');
    expect(seen.json, _definition);
    expect(seen.options.headers.containsKey('If-Match'), isFalse);
    expect(await port.definition(_notes), isNull);
  });

  test('a refused definition keeps the pointers of its field errors', () async {
    build({
      'PUT /api/v1/agents/$_notes/definition': (
        400,
        {
          'code': 'validation_failed',
          'errors': [
            {
              'pointer': '/mcp_servers/0/transport',
              'code': 'tool_class_too_low',
            },
          ],
        },
      ),
    });
    await expectLater(
      port.putDefinition(_notes, _definition, name: 'Notes'),
      throwsA(
        isA<NewAgentFailure>()
            .having((f) => f.code, 'code', 'validation_failed')
            .having((f) => f.errors, 'errors', [
              const FieldIssue(
                '/mcp_servers/0/transport',
                'tool_class_too_low',
              ),
            ]),
      ),
    );
  });

  test('a conflict keeps its reason', () async {
    build({
      'PUT /api/v1/agents/$_notes/tool-class': (
        409,
        {'code': 'conflict', 'reason': 'host_tool_class_ceiling'},
      ),
    });
    await expectLater(
      port.setToolClass(
        const NewAgentState(
          id: _notes,
          version: 2,
          toolClass: AgentToolClass.chat,
        ),
        AgentToolClass.host,
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

  test('the channel is created and gets the agent as operator', () async {
    final channel = {
      'id': _channel,
      'space_id': _space,
      'kind': 'text',
      'name': 'notes',
      'topic': '',
      'sort_key': 'a1',
      'read_only': false,
      'project_bound': false,
      'operator': {'mode': 'inherit'},
      'version': 4,
      'settings_version': 1,
      'created_at': _at,
    };
    build({
      'POST /api/v1/spaces/$_space/channels': (201, channel),
      'GET /api/v1/channels/$_channel': (
        200,
        {'channel': channel, 'level': 'trigger', 'capabilities': <String>[]},
      ),
      'PUT /api/v1/channels/$_channel/operator': (200, channel),
    });
    final id = await port.createChannel(
      _space,
      'notes',
      idempotencyKey: '0192f4a0-7b1c-7000-8000-000000000002',
      name: 'Notes',
    );
    expect(id, _channel);
    expect(seenOf('POST', '/api/v1/spaces/$_space/channels').json, {
      'kind': 'text',
      'name': 'notes',
    });
    await port.setOperator(_channel, _notes, name: 'Notes');
    final op = seenOf('PUT', '/api/v1/channels/$_channel/operator');
    expect(op.options.headers['If-Match'], '"4"');
    expect(op.json, {
      'mode': 'agent',
      'agent_id': _notes,
      'reason': 'New agent Notes',
    });
  });

  test(
    'the interview sends the turns and maps a draft with its errors and cost',
    () async {
      build({
        'POST /api/v1/agents/interview': (
          200,
          {
            'outcome': 'draft',
            'draft': {
              ..._definition,
              'name': 'Notes',
              'handle': 'notes',
              'description': 'Keeps notes',
              'channel_suggestion': {
                'space_name': 'Team',
                'channel_name': 'notes',
              },
            },
            'errors': [
              {'pointer': '/instructions', 'code': 'too_long'},
            ],
            'cost_estimate': {'amount_micros': 4000, 'currency': 'USD'},
          },
        ),
      });
      final reply = await port.interview(const [
        InterviewTurn(InterviewRole.builder, 'What should it do?'),
        InterviewTurn(InterviewRole.owner, 'Keep notes'),
      ], wantDraft: true);
      expect(seenOf('POST', '/api/v1/agents/interview').json, {
        'turns': [
          {'role': 'builder', 'text': 'What should it do?'},
          {'role': 'owner', 'text': 'Keep notes'},
        ],
        'want_draft': true,
      });
      expect(reply.outcome, InterviewOutcome.draft);
      expect(reply.draft!['handle'], 'notes');
      expect(reply.errors, [const FieldIssue('/instructions', 'too_long')]);
      expect(reply.costMicros, 4000);
    },
  );

  test('a fallback names its reason', () async {
    build({
      'POST /api/v1/agents/interview': (
        200,
        {'outcome': 'fallback', 'fallback_reason': 'builder_timeout'},
      ),
    });
    final reply = await port.interview(const []);
    expect(reply.outcome, InterviewOutcome.fallback);
    expect(reply.fallbackReason, 'builder_timeout');
  });
}
