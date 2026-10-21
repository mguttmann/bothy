// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:console/console.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import 'terminal_fakes.dart';

const _hello = {
  'protocol_versions': [1],
  'dpop_nonce': 'nonce-1',
  'server_time': '2026-10-20T10:00:00Z',
};

/// The terminal API of a signed-in console over [b].
ConsoleTerminalApi _api(TestBrowser b) {
  terminalRoutes(b.server);
  final c = ConsoleController(b.environment);
  var n = 0;
  return ConsoleTerminalApi(
    c.api,
    newIdempotencyKey: () => '0192f4a0-7b1c-7000-8000-00000000000${n++}',
  );
}

TerminalSessionOpen _open() => TerminalSessionOpen(
  (b) => b
    ..profile = 'shell'
    ..cols = 80
    ..rows = 24
    ..clientEphemeralKeyHash = 'aGFzaA',
);

void main() {
  group('ConsoleTerminalApi', () {
    test('records the device of every grant by its stream path', () async {
      final b = TestBrowser();
      final api = _api(b);
      expect(api.deviceIdFor(streamPath), isNull);
      final grant = await api.openSession(buildHostId, _open());
      expect(grant.streamPath, streamPath);
      expect(api.deviceIdFor(streamPath), consoleDeviceId);
      final other = '0192f4a0-7b1c-7848-8501-1716705c7dbf';
      b
          .server
          .routes['POST /api/v1/terminal/sessions/$terminalSessionId/attach'] = (
        _,
      ) => Answer(200, grantBody(deviceId: other));
      await api.attachSession(
        terminalSessionId,
        TerminalSessionAttach(
          (b) => b
            ..cols = 80
            ..rows = 24
            ..clientEphemeralKeyHash = 'aGFzaA',
        ),
      );
      expect(api.deviceIdFor(streamPath), other);
    });

    test('sends the CSRF header, never a token, and closes with an Idempotency-Key', () async {
      final b = TestBrowser();
      final api = _api(b);
      await api.getHost(buildHostId);
      await api.openSession(buildHostId, _open());
      await api.closeSession(terminalSessionId);
      final open = b.server
          .to('POST', '/api/v1/hosts/$buildHostId/terminal/sessions')
          .single;
      expect(open.headers['x-console-request'], '1');
      expect(open.headers.containsKey('authorization'), isFalse);
      expect(open.headers.containsKey('dpop'), isFalse);
      final close = b.server
          .to('POST', '/api/v1/terminal/sessions/$terminalSessionId/close')
          .single;
      expect(close.headers['idempotency-key'], isNotEmpty);
      expect((close.body! as Map)['reason'], isNotEmpty);
    });

    test(
      'leaves step_up_required of an open to the terminal controller',
      () async {
        final b = TestBrowser();
        final api = _api(b);
        b.server.routes['POST /api/v1/hosts/$buildHostId/terminal/sessions'] =
            (_) => const Answer(401, {
              'code': 'step_up_required',
              'status': 401,
              'reason': 'passkey_required',
            });
        Object? error;
        try {
          await api.openSession(buildHostId, _open());
        } on Object catch (e) {
          error = e;
        }
        expect(error, isA<DioException>());
        expect(ApiProblem.of(error!)?.code, 'step_up_required');
        // No action step-up started: the console's adapter never asked for a challenge.
        expect(b.server.to('POST', '/api/v1/auth/step-up/challenge'), isEmpty);
      },
    );
  });

  group('ConsoleTerminalTransport', () {
    test(
      'connects to the stream path on the same origin, wss for https',
      () async {
        final sockets = FakeSockets();
        final t = ConsoleTerminalTransport(
          origin: Uri.parse('https://chat.example.com'),
          openSocket: sockets.connect,
          deviceIdFor: (_) => consoleDeviceId,
        );
        await t.connect(streamPath);
        expect(
          sockets.opened.single.url,
          Uri.parse('wss://chat.example.com$streamPath'),
        );
        final plain = ConsoleTerminalTransport(
          origin: Uri.parse('http://localhost:8443'),
          openSocket: sockets.connect,
          deviceIdFor: (_) => consoleDeviceId,
        );
        await plain.connect(streamPath);
        expect(
          sockets.opened.last.url,
          Uri.parse('ws://localhost:8443$streamPath'),
        );
      },
    );

    test(
      'identify carries the recorded device and no token and no DPoP proof',
      () async {
        final sockets = FakeSockets();
        final paths = <String>[];
        final t = ConsoleTerminalTransport(
          origin: Uri.parse('https://chat.example.com'),
          openSocket: sockets.connect,
          deviceIdFor: (path) {
            paths.add(path);
            return consoleDeviceId;
          },
        );
        await t.connect(streamPath);
        final identify = await t.identify(_hello);
        expect(identify, {'protocol_version': 1, 'device_id': consoleDeviceId});
        expect(identify.containsKey('access_token'), isFalse);
        expect(identify.containsKey('dpop'), isFalse);
        expect(paths, [streamPath]);
      },
    );

    test('identify without a recorded device fails', () async {
      final sockets = FakeSockets();
      final t = ConsoleTerminalTransport(
        origin: Uri.parse('https://chat.example.com'),
        openSocket: sockets.connect,
        deviceIdFor: (_) => null,
      );
      await expectLater(t.identify(_hello), throwsStateError);
      await t.connect(streamPath);
      await expectLater(t.identify(_hello), throwsStateError);
    });

    test('text until identify, then only binary', () async {
      final sockets = FakeSockets();
      final t = ConsoleTerminalTransport(
        origin: Uri.parse('https://chat.example.com'),
        openSocket: sockets.connect,
        deviceIdFor: (_) => consoleDeviceId,
      );
      final channel = await t.connect(streamPath);
      final socket = sockets.opened.single;
      // The channel delivers what the server sends: hello as text, Noise messages as bytes.
      final received = <Object>[];
      final sub = channel.messages.listen(received.add);
      socket.push(jsonEncode({'op': 'hello', 'd': _hello}));
      socket.push(Uint8List.fromList([1, 2, 3]));
      await pumpEventQueue();
      expect(received.first, isA<String>());
      expect(received.last, Uint8List.fromList([1, 2, 3]));
      final d = await t.identify(_hello);
      channel.sendText(jsonEncode({'op': 'identify', 'd': d}));
      channel.sendBinary(Uint8List.fromList([4, 5]));
      expect(() => channel.sendText('{"op":"identify"}'), throwsStateError);
      channel.sendBinary(Uint8List.fromList([6]));
      expect(socket.sent, hasLength(3));
      final first = socket.sent.first as SentText;
      final sent = jsonDecode(first.text) as Map<String, Object?>;
      expect(sent['op'], 'identify');
      expect(sent['d'], {'protocol_version': 1, 'device_id': consoleDeviceId});
      expect(socket.sent.skip(1), everyElement(isA<SentBinary>()));
      await channel.close();
      expect(socket.closedWith, 1000);
      await sub.cancel();
    });

    test('a binary message before identify is refused', () async {
      final sockets = FakeSockets();
      final t = ConsoleTerminalTransport(
        origin: Uri.parse('https://chat.example.com'),
        openSocket: sockets.connect,
        deviceIdFor: (_) => consoleDeviceId,
      );
      final channel = await t.connect(streamPath);
      expect(() => channel.sendBinary(Uint8List(1)), throwsStateError);
      expect(sockets.opened.single.sent, isEmpty);
    });
  });
}
