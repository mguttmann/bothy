// SPDX-License-Identifier: Apache-2.0

// The dart:io transport against a WebSocket server on 127.0.0.1 with port 0 (brief M4-13,
// Boundaries): text frames both ways, the close code and reason of the server, and a failed
// upgrade.

import 'dart:async';
import 'dart:io';

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

void main() {
  late HttpServer server;
  late Uri url;
  final received = <String>[];

  setUp(() async {
    received.clear();
    server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    url = Uri.parse('ws://127.0.0.1:${server.port}/api/v1/gateway');
    server.listen((request) async {
      if (request.uri.path != '/api/v1/gateway') {
        request.response.statusCode = HttpStatus.notFound;
        await request.response.close();
        return;
      }
      final ws = await WebSocketTransformer.upgrade(request);
      ws.add('{"op":"hello","d":{}}');
      ws.listen((m) async {
        received.add(m as String);
        await ws.close(4013, 'restart');
      });
    });
  });
  tearDown(() => server.close(force: true));

  test('frames flow both ways and the server close code arrives', () async {
    final channel = await IoGatewayTransport().connect(url);
    final frames = <String>[];
    final done = Completer<void>();
    channel.frames.listen(frames.add, onDone: done.complete);
    await Future<void>.delayed(const Duration(milliseconds: 50));
    channel.send('{"op":"heartbeat","d":{"last_seq":1}}');
    await done.future.timeout(const Duration(seconds: 5));
    expect(frames, ['{"op":"hello","d":{}}']);
    expect(received, ['{"op":"heartbeat","d":{"last_seq":1}}']);
    expect(channel.closeCode, 4013);
    expect(channel.closeReason, 'restart');
  });

  test('the client closes with its code', () async {
    final channel = await IoGatewayTransport().connect(url);
    await channel.close(1000);
    await channel.frames.drain<void>().timeout(const Duration(seconds: 5));
  });

  test('a failed upgrade throws', () async {
    await expectLater(
      IoGatewayTransport().connect(url.replace(path: '/other')),
      throwsA(anything),
    );
  });
}
