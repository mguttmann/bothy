// SPDX-License-Identifier: Apache-2.0

// The REST side of the audit log (brief M4-29): the filters become the query parameters of
// `listAuditEvents`, the answers map to the screen types, the chain check sends its range, the
// export first reads one entry of the range (so the step-up and a 403 arrive on JSON) and then
// passes the server's bytes through unchanged. A Dio adapter answers in place of a server, so
// no socket opens.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// Answers each request with [answer] and records it.
class _Adapter implements HttpClientAdapter {
  _Adapter(this.answer);

  final ResponseBody Function(RequestOptions options) answer;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    return answer(options);
  }

  @override
  void close({bool force = false}) {}
}

ResponseBody _json(Object body, {int status = 200}) => ResponseBody.fromString(
  jsonEncode(body),
  status,
  headers: {
    Headers.contentTypeHeader: [
      status >= 400 ? 'application/problem+json' : 'application/json',
    ],
  },
);

const Map<String, Object?> _event = {
  'seq': 7,
  'id': '0192f4a0-7b1c-7000-8000-000000000007',
  'occurred_at': '2026-10-17T08:00:00.000Z',
  'actor_principal_id': '0192f4a0-7b1c-7000-8000-000000000001',
  'action': 'agent.invoke',
  'decision': 'deny',
  'deny_reason': 'missing_grant',
  'resource_kind': 'run',
  'resource_id': '0192f4a0-7b1c-7000-8000-000000000002',
  'details': {'reason': 'ignored for a denial', 'count': 3},
  'correlation_id': '4bf92f35-77b3-4da6-a3ce-929d0e0e4736',
  'prev_hash': 'aa',
  'hash': 'bb',
};

void main() {
  late _Adapter adapter;
  late ApiAdminRepository repository;

  void serve(ResponseBody Function(RequestOptions options) answer) {
    adapter = _Adapter(answer);
    final dio = Dio(BaseOptions(baseUrl: 'http://192.0.2.1/api/v1'))
      ..httpClientAdapter = adapter;
    repository = ApiAdminRepository(
      ApiClient(
        dio: dio,
        interceptors: [BinaryProblemInterceptor(), ProblemInterceptor()],
      ),
      newIdempotencyKey: () => '0192f4a0-7b1c-7000-8000-0000000000ff',
    );
  }

  test('the filters become the query parameters and the page maps', () async {
    serve(
      (_) => _json({
        'items': [_event],
        'next_cursor': 'c2',
      }),
    );
    final page = await repository.auditEvents(
      AuditFilter(
        action: 'agent.invoke',
        actorId: '0192f4a0-7b1c-7000-8000-000000000001',
        result: AuditResult.deny,
        resourceKind: 'run',
        resourceId: '0192f4a0-7b1c-7000-8000-000000000002',
        from: DateTime.utc(2026, 10, 16),
        to: DateTime.utc(2026, 10, 18),
      ),
      cursor: 'c1',
      purpose: 'Read the audit log',
    );

    final q = adapter.requests.single.uri.queryParameters;
    expect(adapter.requests.single.uri.path, '/api/v1/audit/events');
    expect(q['action'], 'agent.invoke');
    expect(q['actor_id'], '0192f4a0-7b1c-7000-8000-000000000001');
    expect(q['decision'], 'deny');
    expect(q['resource_kind'], 'run');
    expect(q['resource_id'], '0192f4a0-7b1c-7000-8000-000000000002');
    expect(q['from'], startsWith('2026-10-16T00:00:00'));
    expect(q['to'], startsWith('2026-10-18T00:00:00'));
    expect(q['cursor'], 'c1');
    expect(q.containsKey('limit'), isFalse, reason: 'the server default');
    expect(
      adapter.requests.single.extra['auth.step_up_purpose'],
      'Read the audit log',
    );

    final e = page.items.single;
    expect(page.nextCursor, 'c2');
    expect(e.seq, 7);
    expect(e.result, AuditResult.deny);
    expect(e.reason, 'missing_grant');
    expect(e.details['count'], 3);
    expect(e.correlationId, '4bf92f35-77b3-4da6-a3ce-929d0e0e4736');
    expect(e.prevHash, 'aa');
    expect(e.hash, 'bb');
  });

  test('the other decisions map too', () async {
    serve(
      (_) => _json({
        'items': [
          {..._event, 'decision': 'allow'},
          {..._event, 'decision': 'error'},
        ],
      }),
    );
    final page = await repository.auditEvents(
      const AuditFilter(result: AuditResult.error),
      purpose: 'p',
    );
    expect(page.items.map((e) => e.result), [
      AuditResult.allow,
      AuditResult.error,
    ]);
    expect(adapter.requests.single.uri.queryParameters['decision'], 'error');
  });

  test('a refusal arrives as SelfActionFailed with the problem', () async {
    serve(
      (_) => _json({
        'code': 'forbidden',
        'reason': 'owner_only',
        'correlation_id': '4bf92f35-77b3-4da6-a3ce-929d0e0e4736',
      }, status: 403),
    );
    await expectLater(
      repository.auditEvents(const AuditFilter(), purpose: 'p'),
      throwsA(
        isA<SelfActionFailed>()
            .having((f) => f.problem?.status, 'status', 403)
            .having((f) => f.problem?.reason, 'reason', 'owner_only'),
      ),
    );
  });

  test('the chain check sends its range and maps the result', () async {
    serve(
      (_) => _json({
        'ok': false,
        'checked_from_seq': 3,
        'checked_to_seq': 5,
        'first_broken_seq': 5,
        'failure_reason': 'seq_gap',
        'checkpoints_compared': 1,
        'last_checkpoint_seq': 2,
        'warnings': ['checkpoint_sink_not_configured'],
      }),
    );
    final r = await repository.verifyAuditChain(
      fromSeq: 3,
      toSeq: 9,
      purpose: 'Check',
    );
    expect(adapter.requests.single.data, {'from_seq': 3, 'to_seq': 9});
    expect(r.ok, isFalse);
    expect(r.firstBrokenSeq, 5);
    expect(r.failureReason, 'seq_gap');
    expect(r.lastCheckpointSeq, 2);
    expect(r.warnings, ['checkpoint_sink_not_configured']);

    for (final reason in [
      'hash_mismatch',
      'link_mismatch',
      'checkpoint_mismatch',
      'checkpoint_invalid',
    ]) {
      serve(
        (_) => _json({
          'ok': false,
          'checked_from_seq': 1,
          'checked_to_seq': 1,
          'first_broken_seq': 1,
          'failure_reason': reason,
          'checkpoints_compared': 0,
          'warnings': <String>[],
        }),
      );
      final check = await repository.verifyAuditChain(purpose: 'Check');
      expect(check.failureReason, reason);
      expect(adapter.requests.single.data, isEmpty);
    }
  });

  test(
    'the export reads one entry first, then streams the bytes unchanged',
    () async {
      final ndjson =
          '${jsonEncode(_event)}\n${jsonEncode({..._event, 'seq': 8})}\n';
      serve(
        (o) => o.path.endsWith('/audit/export')
            ? ResponseBody.fromString(
                ndjson,
                200,
                headers: {
                  Headers.contentTypeHeader: ['application/x-ndjson'],
                },
              )
            : _json({
                'items': [_event],
              }),
      );
      final bytes = await repository
          .exportAudit(
            from: DateTime.utc(2026, 10, 16),
            to: DateTime.utc(2026, 10, 18),
            purpose: 'Export',
          )
          .expand((c) => c)
          .toList();

      expect(utf8.decode(bytes), ndjson);
      expect(adapter.requests.map((r) => r.uri.path), [
        '/api/v1/audit/events',
        '/api/v1/audit/export',
      ]);
      expect(adapter.requests.first.uri.queryParameters['limit'], '1');
      final export = adapter.requests.last;
      expect(export.uri.queryParameters['from'], '2026-10-16T00:00:00.000Z');
      expect(export.uri.queryParameters['to'], '2026-10-18T00:00:00.000Z');
      expect(export.extra['auth.step_up_purpose'], 'Export');
      expect(
        (export.extra['secure'] as List).map((s) => (s as Map)['name']),
        contains('dpopAuth'),
      );
    },
  );

  test('a refused export stream reads its problem from the body', () async {
    serve(
      (o) => o.path.endsWith('/audit/export')
          ? _json({'code': 'forbidden', 'reason': 'owner_only'}, status: 403)
          : _json({'items': <Object>[]}),
    );
    await expectLater(
      repository
          .exportAudit(
            from: DateTime.utc(2026, 10, 16),
            to: DateTime.utc(2026, 10, 18),
            purpose: 'Export',
          )
          .drain<void>(),
      throwsA(
        isA<SelfActionFailed>()
            .having((f) => f.problem?.status, 'status', 403)
            .having((f) => f.problem?.reason, 'reason', 'owner_only'),
      ),
    );
  });

  test(
    'a refusal of the first read stops the export before the stream',
    () async {
      serve((_) => _json({'code': 'forbidden'}, status: 403));
      await expectLater(
        repository
            .exportAudit(
              from: DateTime.utc(2026, 10, 16),
              to: DateTime.utc(2026, 10, 18),
              purpose: 'Export',
            )
            .drain<void>(),
        throwsA(isA<SelfActionFailed>()),
      );
      expect(adapter.requests.map((r) => r.uri.path), ['/api/v1/audit/events']);
    },
  );
}
