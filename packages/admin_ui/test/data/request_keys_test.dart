// SPDX-License-Identifier: Apache-2.0

// The `Idempotency-Key` of a creating request across repetitions (04 5.2; 06 7; 15 1 item 3):
// after a lost answer the same request goes out again with the same key, so the server answers
// from its record instead of creating twice; a final answer drops the key, and a changed
// request gets a new one. A scripted adapter answers; no socket is opened.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

const String _group = '0192f4a0-7b1c-7a00-8000-000000000e01';
const String _at = '2026-10-18T09:00:00.000Z';

/// Answers every request with the next outcome of [script]: a status with a body, or null for
/// a connection that broke before an answer arrived. Records the keys it saw.
class _ScriptedAdapter implements HttpClientAdapter {
  _ScriptedAdapter(this.script);

  final List<(int, Object?)?> script;
  final List<String?> keys = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (requestStream != null) await requestStream.drain<void>();
    keys.add(options.headers['Idempotency-Key'] as String?);
    final next = script.removeAt(0);
    if (next == null) {
      throw DioException.connectionError(
        requestOptions: options,
        reason: 'lost',
      );
    }
    final (status, body) = next;
    return ResponseBody.fromString(
      jsonEncode(body),
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

Map<String, Object?> _groupJson() => {
  'id': _group,
  'name': 'Support',
  'description': 'First line',
  'source': 'local',
  'created_at': _at,
  'member_count': 1,
};

Map<String, Object?> _problem(int status, String code) => {
  'type': 'about:blank',
  'title': code,
  'status': status,
  'code': code,
};

void main() {
  late _ScriptedAdapter adapter;
  late AdminRepository repository;
  var made = 0;

  void use(List<(int, Object?)?> script) {
    adapter = _ScriptedAdapter(script);
    made = 0;
    repository = ApiAdminRepository(
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
      newIdempotencyKey: () => '0192f4a0-0000-7000-8000-00000000000${made++}',
    );
  }

  Future<void> create(String name) async {
    try {
      await repository.createGroup(name, description: '', purpose: 'Group');
    } on SelfActionFailed {
      // The test reads the keys the adapter saw.
    }
  }

  test('a repetition after a lost answer sends the same key', () async {
    use([null, (201, _groupJson())]);
    await create('Support');
    await create('Support');
    expect(adapter.keys, hasLength(2));
    expect(adapter.keys[1], adapter.keys[0]);
  });

  test(
    'a repetition after 503 or 409 request_in_progress keeps the key',
    () async {
      use([
        (503, _problem(503, 'unavailable')),
        (409, _problem(409, 'request_in_progress')),
        (201, _groupJson()),
      ]);
      for (var i = 0; i < 3; i++) {
        await create('Support');
      }
      expect(adapter.keys.toSet(), hasLength(1));
    },
  );

  test('a final answer drops the key', () async {
    use([(409, _problem(409, 'conflict')), (201, _groupJson())]);
    await create('Support');
    await create('Support');
    expect(adapter.keys[1], isNot(adapter.keys[0]));
  });

  test('a changed request gets a new key', () async {
    use([null, (201, _groupJson())]);
    await create('Support');
    await create('Second line');
    expect(adapter.keys[1], isNot(adapter.keys[0]));
  });

  test('a success drops the key', () async {
    use([(201, _groupJson()), (201, _groupJson())]);
    await create('Support');
    await create('Support');
    expect(adapter.keys[1], isNot(adapter.keys[0]));
  });

  test('the install of an app keeps its key after a lost answer', () async {
    use([null, (500, _problem(500, 'internal_error')), null]);
    final upload = ManifestUpload.read(
      'a.json',
      Uint8List.fromList(utf8.encode('{"slug":"repo-bridge"}')),
    );
    for (var i = 0; i < 3; i++) {
      try {
        await repository.installApp(
          upload,
          grantedScopes: const ['messages:write'],
          eventSubscriptions: const [],
          reason: 'Bridge',
          purpose: 'Install',
        );
      } on SelfActionFailed {
        // The test reads the keys the adapter saw.
      }
    }
    expect(adapter.keys, hasLength(3));
    expect(adapter.keys.toSet(), hasLength(1));
  });
}
