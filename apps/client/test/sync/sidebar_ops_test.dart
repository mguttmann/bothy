// SPDX-License-Identifier: Apache-2.0

// The writes of the personal sidebar orders (M4-51, 04 14): `SidebarWriter` changes the local
// row at once and queues the write; the executor of `putMySidebarSpace` passes `channel_order`,
// the executor of `putMySidebarChannel` runs in the lane of its channel and stores the answer
// by the version rule unless a later write of the same channel waits.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' show ApiClient;
import 'package:auth/auth.dart' show AuthSession;
import 'package:client/sync/sidebar_ops.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/engine.dart';

/// Answers each request by method and path from [routes] and records the request bodies.
class _Adapter implements HttpClientAdapter {
  _Adapter(this.routes);

  final Map<String, Map<String, Object?>> routes;

  /// The requests seen, as `METHOD path` with the decoded JSON body.
  final List<(String, Object?)> seen = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final key = '${options.method} ${options.uri.path}';
    final bytes = requestStream == null
        ? <int>[]
        : [for (final chunk in await requestStream.toList()) ...chunk];
    seen.add((key, bytes.isEmpty ? null : jsonDecode(utf8.decode(bytes))));
    final body = routes[key];
    return ResponseBody.fromString(
      body == null ? '' : jsonEncode(body),
      body == null ? 404 : 200,
      headers: {
        if (body != null) Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

String _id(int n) =>
    '0192f4a0-7b1c-78f9-8074-${n.toRadixString(16).padLeft(12, '0')}';

void main() {
  final space = SpaceId.parse(_id(0x101));
  final ch1 = ChannelId.parse(_id(0x201));
  final ch2 = ChannelId.parse(_id(0x202));
  final t1 = ThreadId.parse(_id(0x301));
  final t2 = ThreadId.parse(_id(0x302));
  final otherDevice = _id(0xd2);

  late LocalDatabase db;
  late ({SyncEngine engine, AuthSession auth}) idle;
  late SidebarWriter writer;

  setUp(() {
    db = openInMemoryForTests();
    idle = idleEngine(db);
    writer = SidebarWriter(idle.engine, db);
  });
  tearDown(() async {
    await idle.engine.dispose();
    await idle.auth.dispose();
    await db.close();
  });

  ApiClient client(_Adapter adapter) => ApiClient(
    dio: Dio(BaseOptions(baseUrl: 'https://chat.example.com'))
      ..httpClientAdapter = adapter,
  );

  Future<OutboxResponse> run(OutboxExecutors executors, OutboxEntry e) =>
      executors.lookup(e.kind)!.send(e, ApiCallContext(idempotencyKey: e.id));

  test('orderChannels writes the local row at once, then the outbox', () async {
    await writer.orderChannels(space, [ch2, ch1], collapsed: true);
    final row = (await db.structureDao.watchUserSpaceStates().first).single;
    expect(row.channelOrder, [ch2, ch1]);
    expect(row.collapsed, isTrue);
    final entry = (await db.outboxDao.allEntries()).single;
    expect(entry.kind, SidebarOps.putMySidebarSpace);
    expect(jsonDecode(entry.payload), {
      'space_id': space.value,
      'collapsed': true,
      'channel_order': [ch2.value, ch1.value],
    });
  });

  test('orderThreads writes the local row at once, then the outbox in the lane of the channel', () async {
    await writer.orderThreads(ch1, [t2, t1]);
    final row = (await db.structureDao.watchUserChannelStates().first).single;
    expect(row.channelId, ch1);
    expect(row.threadOrder, [t2, t1]);
    final entry = (await db.outboxDao.allEntries()).single;
    expect(entry.kind, SidebarOps.putMySidebarChannel);
    expect(jsonDecode(entry.payload), {
      'channel_id': ch1.value,
      'thread_order': [t2.value, t1.value],
    });
    final executors = OutboxExecutors();
    SidebarOps.register(executors, client(_Adapter({})), db);
    expect(executors.laneOf(entry), 'sidebar-channel:${ch1.value}');
  });

  test(
    'the space executor sends channel_order and stores the answer',
    () async {
      final adapter = _Adapter({
        'PUT /me/sidebar/spaces/${space.value}': {
          'space_id': space.value,
          'collapsed': false,
          'channel_order': [ch2.value, ch1.value],
          'version': 4,
          'updated_by_device_id': otherDevice,
        },
      });
      final executors = OutboxExecutors();
      SidebarOps.register(executors, client(adapter), db);
      await writer.orderChannels(space, [ch2, ch1], collapsed: false);
      final entry = (await db.outboxDao.allEntries()).single;
      final response = await run(executors, entry);
      expect(response.status, 200);
      expect(adapter.seen.single.$2, {
        'collapsed': false,
        'channel_order': [ch2.value, ch1.value],
      });
      final row = (await db.structureDao.watchUserSpaceStates().first).single;
      expect(row.version, const ResourceVersion(4));
      expect(row.channelOrder, [ch2, ch1]);
    },
  );

  test('the channel executor stores the answer by version unless a later write waits', () async {
    Map<String, Object?> state(List<ThreadId> order, int version) => {
      'channel_id': ch1.value,
      'thread_order': [for (final t in order) t.value],
      'version': version,
      'updated_by_device_id': otherDevice,
    };
    final routes = {
      'PUT /me/sidebar/channels/${ch1.value}': state([t2, t1], 3),
    };
    final adapter = _Adapter(routes);
    final executors = OutboxExecutors();
    SidebarOps.register(executors, client(adapter), db);

    await writer.orderThreads(ch1, [t2, t1]);
    final first = (await db.outboxDao.allEntries()).single;
    expect((await run(executors, first)).status, 200);
    expect(adapter.seen.single.$1, 'PUT /me/sidebar/channels/${ch1.value}');
    expect(adapter.seen.single.$2, {
      'thread_order': [t2.value, t1.value],
    });
    var row = (await db.structureDao.watchUserChannelStates().first).single;
    expect(row.version, const ResourceVersion(3));
    await db.outboxDao.delete(first.id);

    // A second write and a third that waits behind it: the answer of the second is not
    // stored, the row keeps the newer local order.
    await writer.orderThreads(ch1, [t1]);
    await writer.orderThreads(ch1, [t1, t2]);
    routes['PUT /me/sidebar/channels/${ch1.value}'] = state([t1], 4);
    final pending = [
      for (final e in await db.outboxDao.allEntries())
        if (e.id != first.id) e,
    ];
    await run(executors, pending.first);
    row = (await db.structureDao.watchUserChannelStates().first).single;
    expect(row.threadOrder, [t1, t2]);
    expect(row.version, const ResourceVersion(3));
  });
}
