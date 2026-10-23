// SPDX-License-Identifier: Apache-2.0

// The REST source of the import (brief M8-08 Design notes 1 to 4): createImport sends the
// converted mapping as plain JSON with kind discord, an Idempotency-Key and the step-up purpose;
// the state steps carry an Idempotency-Key and the purpose; setImportLead sends the Discord
// channel and the lead; the items are read over every page with their wire values; the operator
// step sends If-Match and resolve_i1. A routing adapter answers; no socket is opened.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

typedef _Answer = (int, Object?);

/// Answers each request with [route]; records what it saw.
class _Adapter implements HttpClientAdapter {
  _Adapter(this.route);

  final _Answer Function(RequestOptions options) route;
  final List<({RequestOptions options, List<int> body})> seen = [];

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
    seen.add((options: options, body: bytes));
    final (status, answer) = route(options);
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

const String _source = '0192f4a0-7b1c-78d2-8a6a-ec465435b8a0';
const String _channel = '0192f4a0-7b1c-72d4-8bae-7661d2b2f203';
const String _agent = '0192f4a0-7b1c-72d4-8bae-7661d2b2f999';

Map<String, Object?> _sourceJson({String state = 'draft'}) => {
  'id': _source,
  'kind': 'discord',
  'name': 'Old community server',
  'config': {
    'version': 1,
    'guild_id': '100',
    'channels': [
      {
        'discord_channel': '301',
        'target': {
          'create': {'name': 'tickets'},
        },
        'planned_operator': 'agent:texter',
        'lead': 'discord',
      },
    ],
  },
  'state': state,
  'stats': {
    'server': {
      'channel': {'posted': 2},
    },
  },
  'cursor': <String, Object?>{},
  'created_at': '2026-10-17T08:00:00.000Z',
};

Map<String, Object?> _channelJson({String? operator, int version = 3}) => {
  'id': _channel,
  'kind': 'text',
  'name': 'tickets',
  'topic': '',
  'sort_key': 'a0',
  'read_only': true,
  'operator': {
    'mode': operator == null ? 'none' : 'agent',
    'agent_id': ?operator,
    'effective_agent_id': ?operator,
  },
  'version': version,
  'settings_version': 1,
  'created_at': '2026-10-17T08:00:00.000Z',
};

Map<String, Object?> _item(String key, String kind, String state) => {
  'id': '0192f4a0-7b1c-7e43-8919-6113546a4c7${key.length % 10}',
  'origin_key': key,
  'item_kind': kind,
  'state': state,
  'target_id': _channel,
  'created_at': '2026-10-17T08:00:00.000Z',
};

void main() {
  late _Adapter adapter;
  late ApiImportRepository repository;
  var keys = 0;

  void setUpWith(_Answer Function(RequestOptions o) route) {
    adapter = _Adapter(route);
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
    repository = ApiImportRepository(
      client,
      newIdempotencyKey: () => 'key-${++keys}',
    );
  }

  Map<String, Object?> jsonBody(int i) =>
      jsonDecode(utf8.decode(adapter.seen[i].body)) as Map<String, Object?>;

  test(
    'createImport sends the mapping as plain JSON with key and purpose',
    () async {
      setUpWith((_) => (201, _sourceJson()));
      final config = {
        'version': 1,
        'guild_id': '100',
        'extra': [1, 'two'],
      };
      final created = await repository.createImportSource(
        name: 'Old community server',
        config: config,
        purpose: 'Add the import source',
      );
      expect(created.id, _source);
      expect(created.state, ImportState.draft);
      final o = adapter.seen.single.options;
      expect('${o.method} ${o.uri.path}', 'POST /api/v1/imports');
      expect(o.headers['Idempotency-Key'], 'key-1');
      expect(o.extra[stepUpPurposeExtra], 'Add the import source');
      expect(jsonBody(0), {
        'kind': 'discord',
        'name': 'Old community server',
        'config': config,
      });
    },
  );

  test('a 400 keeps every field error with its pointer', () async {
    setUpWith(
      (_) => (
        400,
        {
          'type': 'https://bothy.dev/problems/validation_failed',
          'title': 'Validation failed',
          'status': 400,
          'code': 'validation_failed',
          'errors': [
            {
              'pointer': '/config/channels/0/planned_operator',
              'code': 'unknown_handle',
            },
            {'pointer': '/config/guild_id', 'code': 'not_digits'},
          ],
        },
      ),
    );
    await expectLater(
      repository.createImportSource(name: 'x', config: const {}, purpose: 'p'),
      throwsA(
        isA<SelfActionFailed>().having(
          (e) => [for (final f in e.problem!.errors) '${f.pointer} ${f.code}'],
          'errors',
          [
            '/config/channels/0/planned_operator unknown_handle',
            '/config/guild_id not_digits',
          ],
        ),
      ),
    );
  });

  test('the state steps carry a key and the purpose', () async {
    setUpWith(
      (o) => (o.method == 'POST' ? 200 : 404, _sourceJson(state: 'running')),
    );
    final steps = {
      'start': () => repository.startImport(_source, purpose: 'p-start'),
      'pause': () => repository.pauseImport(_source, purpose: 'p-pause'),
      'cancel': () => repository.cancelImport(_source, purpose: 'p-cancel'),
      'complete': () =>
          repository.completeImport(_source, purpose: 'p-complete'),
    };
    var i = 0;
    for (final MapEntry(key: step, value: call) in steps.entries) {
      await call();
      final o = adapter.seen[i++].options;
      expect(
        '${o.method} ${o.uri.path}',
        'POST /api/v1/imports/$_source/$step',
      );
      expect(o.headers['Idempotency-Key'], 'key-$i');
      expect(o.extra[stepUpPurposeExtra], 'p-$step');
    }
  });

  test('a 409 keeps its reason', () async {
    setUpWith(
      (_) => (
        409,
        {
          'type': 'https://bothy.dev/problems/conflict',
          'title': 'Conflict',
          'status': 409,
          'code': 'conflict',
          'reason': 'lead_not_${ImportLead.here.wire}',
        },
      ),
    );
    await expectLater(
      repository.completeImport(_source, purpose: 'p'),
      throwsA(
        isA<SelfActionFailed>().having(
          (e) => e.problem?.reason,
          'reason',
          'lead_not_${ImportLead.here.wire}',
        ),
      ),
    );
  });

  test('setImportLead sends the Discord channel and the lead', () async {
    setUpWith((_) => (200, _sourceJson(state: 'running')));
    await repository.setImportLead(
      _source,
      discordChannel: '301',
      lead: ImportLead.here,
      purpose: 'p-lead',
    );
    final o = adapter.seen.single.options;
    expect('${o.method} ${o.uri.path}', 'PUT /api/v1/imports/$_source/leads');
    expect(o.extra[stepUpPurposeExtra], 'p-lead');
    expect(jsonBody(0), {
      'source_channel': '301',
      'lead': ImportLead.here.wire,
    });
  });

  test('the items are read over every page with their wire values', () async {
    setUpWith((o) {
      final cursor = o.uri.queryParameters['cursor'];
      return cursor == null
          ? (
              200,
              {
                'items': [_item('forum_tag:301:1', 'forum_tag', 'posted')],
                'next_cursor': 'c2',
              },
            )
          : (
              200,
              {
                'items': [_item('channel:301', 'channel', 'failed')],
              },
            );
    });
    final items = await repository.importItems(_source);
    expect(
      [for (final i in items) '${i.itemKind} ${i.state}'],
      ['forum_tag posted', 'channel failed'],
    );
    expect(adapter.seen, hasLength(2));
    expect(adapter.seen.last.options.uri.queryParameters['cursor'], 'c2');
  });

  test('the operator step sends If-Match, the reason and resolve_i1', () async {
    setUpWith(
      (o) => o.method == 'GET'
          ? (
              200,
              {
                'channel': _channelJson(),
                'level': 'manage',
                'capabilities': <String>[],
              },
            )
          : (200, _channelJson(operator: _agent, version: 4)),
    );
    final before = await repository.importChannel(_channel);
    expect(before.name, 'tickets');
    expect(before.readOnly, isTrue);
    expect(before.operatorAgentId, isNull);
    expect(before.version, 3);

    final after = await repository.setImportChannelOperator(
      _channel,
      agentId: _agent,
      version: 3,
      reason: 'Planned operator',
      resolveI1: const I1Resolution([_agent], reason: 'Switch first'),
      purpose: 'p-operator',
    );
    expect(after.operatorAgentId, _agent);
    final o = adapter.seen.last.options;
    expect(
      '${o.method} ${o.uri.path}',
      'PUT /api/v1/channels/$_channel/operator',
    );
    expect(o.headers['If-Match'], '"3"');
    expect(o.extra[stepUpPurposeExtra], 'p-operator');
    expect(jsonBody(1), {
      'mode': 'agent',
      'agent_id': _agent,
      'reason': 'Planned operator',
      'resolve_i1': {
        'switch_to_organization': [_agent],
        'reason': 'Switch first',
      },
    });

    await repository.setImportChannelOperator(
      _channel,
      agentId: null,
      version: 4,
      reason: 'Discord leads again',
      purpose: 'p',
    );
    expect(jsonBody(2), {'mode': 'none', 'reason': 'Discord leads again'});
  });

  test('a snapshot reads counters, channels and cancellation', () async {
    setUpWith(
      (_) => (
        200,
        {
          'items': [_sourceJson()],
        },
      ),
    );
    final page = await repository.importSources();
    final s = page.items.single;
    expect(s.name, 'Old community server');
    expect(
      [for (final c in s.serverCounters) '${c.path}=${c.value}'],
      ['channel.posted=2'],
    );
    expect(s.channels.single.plannedAgentHandle, 'texter');
    expect(s.channels.single.createName, 'tickets');
    expect(s.cancelled, isFalse);
  });
}
