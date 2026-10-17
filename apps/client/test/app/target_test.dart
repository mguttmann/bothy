// SPDX-License-Identifier: Apache-2.0

// The target lookup of links (15 11; 01 6.7): first the local database, then REST; 404 is
// "Not found or no access", no answer is "unknown".

import 'dart:typed_data';

import 'package:api_client/api_client.dart' hide Space;
import 'package:client/app/router.dart';
import 'package:client/app/target_resolver.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

class StatusAdapter implements HttpClientAdapter {
  StatusAdapter(this.status);

  final int? status;
  final List<String> paths = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    paths.add(options.uri.path);
    final s = status;
    if (s == null) {
      throw DioException.connectionError(requestOptions: options, reason: 'offline');
    }
    return ResponseBody.fromString(
      s == 404 ? '{"code":"not_found","status":404}' : '{}',
      s,
      headers: {
        Headers.contentTypeHeader: ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

ApiClient api(StatusAdapter adapter) => createApiClient(
  baseUrl: Uri.parse('https://chat.example.com'),
  traceContext: () => TraceContext(traceId: '1' * 32, sampled: false),
  readRetry: ReadRetryPolicy(base: Duration.zero, cap: Duration.zero, maxAttempts: 1),
  adapter: adapter,
);

const String id = '0193f2a7-0000-7000-8000-000000000001';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  Future<TargetStatus> resolve(String path, int? status, [StatusAdapter? seen]) =>
      TargetResolver(db, api(seen ?? StatusAdapter(status))).resolve(parseLocation(Uri.parse(path)));

  test('a target REST answers with 404 or 403 is not found, one it returns is visible', () async {
    for (final kind in ['s', 'c', 't']) {
      expect(await resolve('/$kind/$id', 404), TargetStatus.notFound, reason: kind);
      expect(await resolve('/$kind/$id', 403), TargetStatus.notFound, reason: kind);
    }
    final adapter = StatusAdapter(200);
    expect(await resolve('/t/$id?m=$id', 200, adapter), isNot(TargetStatus.notFound));
    expect(adapter.paths.single, '/api/v1/threads/$id');
  });

  test('offline the status is unknown', () async {
    expect(await resolve('/c/$id', null), TargetStatus.unknown);
  });

  test('locations without a target are visible, an unknown one is not found', () async {
    for (final path in ['/', '/running', '/search?q=x', '/admin/x', '/onboarding/login']) {
      expect(await resolve(path, 500), TargetStatus.visible, reason: path);
    }
    expect(await resolve('/x', 200), TargetStatus.notFound);
  });

  test('a target in the local database needs no REST call', () async {
    final adapter = StatusAdapter(404);
    final space = SpaceId.parse(id);
    await db.structureDao.upsertSpaceIfNewer(
      Space(
        id: space,
        name: 'Support',
        sortKey: 'a',
        createdAt: DateTime.utc(2026, 10, 17),
        version: const ResourceVersion(1),
      ),
    );
    expect(
      await TargetResolver(db, api(adapter)).resolve(SpaceLocation(space)),
      TargetStatus.visible,
    );
    expect(adapter.paths, isEmpty);
  });
}
