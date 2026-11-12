// SPDX-License-Identifier: Apache-2.0

// The API port of the search over the client the app builds (`createApiClient` with its
// interceptors and the read retry of the session) and a scripted network: a file hit as the
// server delivers it opens the message it is attached to, the search is asked exactly once
// whatever the answer is (the person sees the reason at once and decides, the read retry of
// the session stays out), a timeout and a rate limit keep their meaning, a request nobody
// waits for is cancelled, and the filters of 14 4.4 reach the server as parameters.

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' as rest;
import 'package:client/search/search_port.dart';
import 'package:client/search/search_view.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';

/// The body the server of this tree answered to `GET /search?q=otter&type=files` in
/// `services/server/test/attachments` (`TestSearch_FileHitNamesItsThreadAndMessage`): the file
/// is the attached form without `scope_channel_id` (D-M3-07-5), and the hit names the thread
/// and the message.
const String realFileHitPage = '''
{"items":[{"channel_name":"Allgemein","file":{"created_at":"2026-11-12T05:12:00.543Z","id":"01a1ce2b-831f-71cb-945a-1e9b6bd8a43e","mime_type":"text/plain","original_name":"otter plan.txt","purpose":"attachment","scan_status":"clean","sha256":"914cb64a1bfac4c6f9507c9d26b81ed91757fa051e3f8508140053834d1dda81","size_bytes":5,"thumbnails":[],"transcript_status":"none","upload_state":"complete","uploaded_by":"01a1ce2b-7d7d-774e-9907-87d43c5e477a"},"kind":"file","message_id":"01a1ce2b-8389-7516-b9a2-f656a4c6795a","score":0.19177963144629448,"segments":[{"match":true,"text":"otter"},{"match":false,"text":" plan.txt"}],"space_name":"Support","thread_id":"01a1ce2b-830c-727f-95e6-e90c3b2ca0a5","thread_title":"Otter planning"}],"candidates_capped":false}
''';

/// One scripted answer of the network.
typedef Answer = ({int status, String body, Map<String, String> headers});

/// A network that answers the requests in order from [answers], or fails every request with
/// a connection error when [down] is set, or never answers when [hang] is set (until the
/// request is cancelled).
class ScriptedNetwork implements HttpClientAdapter {
  final List<Answer> answers = [];
  final List<RequestOptions> requests = [];
  bool down = false;
  bool hang = false;
  int cancelled = 0;

  void json(int status, String body, {Map<String, String> headers = const {}}) {
    answers.add((status: status, body: body, headers: headers));
  }

  void problem(
    int status,
    String code, {
    String? reason,
    Map<String, String> headers = const {},
  }) => json(
    status,
    jsonEncode({
      'type': 'https://chat.example.com/problems/$code',
      'title': code,
      'status': status,
      'code': code,
      'reason': ?reason,
    }),
    headers: headers,
  );

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    if (down) {
      throw DioException.connectionError(
        requestOptions: options,
        reason: 'no route',
      );
    }
    if (hang) {
      await cancelFuture;
      cancelled++;
      throw DioException.requestCancelled(
        requestOptions: options,
        reason: 'cancelled',
      );
    }
    final a = answers.removeAt(0);
    return ResponseBody.fromString(
      a.body,
      a.status,
      headers: {
        Headers.contentTypeHeader: [
          a.status < 400 ? Headers.jsonContentType : 'application/problem+json',
        ],
        for (final h in a.headers.entries) h.key: [h.value],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

/// The port over the client of the app: the interceptors of `createApiClient` and the read
/// retry with the backoff keys of the session (2 s to 5 min, nine attempts); [sleeps] records
/// every wait of a retry instead of waiting.
({ApiSearchPort port, ScriptedNetwork network, List<Duration> sleeps})
client() {
  final network = ScriptedNetwork();
  final sleeps = <Duration>[];
  final api = rest.createApiClient(
    baseUrl: Uri.parse('https://chat.example.com'),
    traceContext: () => rest.TraceContext(
      traceId: '0af7651916cd43dd8448eb211c80319c',
      sampled: false,
    ),
    readRetry: rest.ReadRetryPolicy.fromBackoff(
      base: const Duration(seconds: 2),
      cap: const Duration(minutes: 5),
      sleep: (d) async => sleeps.add(d),
    ),
    adapter: network,
  );
  return (port: ApiSearchPort(api), network: network, sleeps: sleeps);
}

Future<SearchFailure> failureOf(Future<Object?> call) => call.then(
  (_) => throw StateError('expected a SearchFailure'),
  onError: (Object e) => e as SearchFailure,
);

void main() {
  test(
    'a file hit of the server opens the message it is attached to',
    () async {
      final c = client();
      c.network.json(200, realFileHitPage);
      final page = await c.port.search(
        const SearchRequest(q: 'otter', type: SearchType.files),
      );
      final hit = page.items.single;
      expect(hit.type, SearchHitType.file);
      expect(hit.title, 'otter plan.txt');
      expect(hit.path, ['Support', 'Allgemein', 'Otter planning']);
      expect(
        hit.threadId,
        ThreadId.parse('01a1ce2b-830c-727f-95e6-e90c3b2ca0a5'),
      );
      expect(
        hit.messageId,
        MessageId.parse('01a1ce2b-8389-7516-b9a2-f656a4c6795a'),
      );
      expect(hit.channelId, isNull);
      expect(
        SearchHitTile.targetOf(hit),
        '/t/01a1ce2b-830c-727f-95e6-e90c3b2ca0a5'
        '?m=01a1ce2b-8389-7516-b9a2-f656a4c6795a',
      );
    },
  );

  test('a search timeout is asked once and keeps its meaning', () async {
    final c = client();
    c.network.problem(
      503,
      'unavailable',
      reason: 'search_timeout',
      headers: {'Retry-After': '1'},
    );
    final failure = await failureOf(
      c.port.search(const SearchRequest(q: 'deploy')),
    );
    expect(failure.kind, SearchFailureKind.timeout);
    expect(c.network.requests, hasLength(1));
    expect(c.sleeps, isEmpty);
  });

  test('a rate limit is asked once and names the wait', () async {
    final c = client();
    c.network.problem(429, 'rate_limited', headers: {'Retry-After': '7'});
    final failure = await failureOf(
      c.port.search(const SearchRequest(q: 'deploy')),
    );
    expect(failure.kind, SearchFailureKind.rateLimited);
    expect(failure.retryAfter, const Duration(seconds: 7));
    expect(c.network.requests, hasLength(1));
    expect(c.sleeps, isEmpty);
  });

  test('503 without the reason of the search stays a plain failure', () async {
    final c = client();
    c.network.problem(503, 'unavailable');
    final failure = await failureOf(
      c.port.search(const SearchRequest(q: 'deploy')),
    );
    expect(failure.kind, SearchFailureKind.failed);
    expect(c.network.requests, hasLength(1));
  });

  test('a connection error is offline after one request', () async {
    final c = client();
    c.network.down = true;
    final failure = await failureOf(
      c.port.search(const SearchRequest(q: 'deploy')),
    );
    expect(failure.kind, SearchFailureKind.offline);
    expect(c.network.requests, hasLength(1));
    expect(c.sleeps, isEmpty);
  });

  test('cancelling ends the request on the network', () async {
    final c = client();
    c.network.hang = true;
    final cancel = SearchCancel();
    final call = failureOf(
      c.port.search(const SearchRequest(q: 'deploy'), cancel: cancel),
    );
    await pumpEventQueue();
    expect(c.network.requests, hasLength(1));
    expect(c.network.cancelled, 0);
    cancel.cancel();
    await call;
    expect(c.network.cancelled, 1);
    expect(c.network.requests, hasLength(1));
  });

  test(
    'the filters of 14 4.4 are parameters, agent_id and tag_id too',
    () async {
      final c = client();
      c.network.json(200, '{"items":[],"candidates_capped":false}');
      await c.port.search(
        SearchRequest(
          q: 'deploy',
          type: SearchType.threads,
          sort: SearchSort.newest,
          spaceIds: [spaceId(1)],
          channelIds: [channelId(1)],
          agentId: robin,
          tagIds: [tagId(1), tagId(2)],
          threadState: ThreadState.open,
        ),
      );
      final query = c.network.requests.single.uri.queryParametersAll;
      expect(query['q'], ['deploy']);
      expect(query['type'], ['threads']);
      expect(query['sort'], ['newest']);
      expect(query['space_id'], [spaceId(1).value]);
      expect(query['channel_id'], [channelId(1).value]);
      expect(query['agent_id'], [robin.value]);
      expect(query['tag_id'], [tagId(1).value, tagId(2).value]);
      expect(query['thread_state'], ['open']);
      expect(query.containsKey('author_id'), isFalse);
    },
  );
}
