// SPDX-License-Identifier: Apache-2.0

// Frames `{op, d}` of 04 2.3 with the generated models: unknown ops and fields are ignored,
// outgoing frames carry no `null`, broken frames are decode errors (brief M4-13, Tests).

import 'dart:convert';

import 'package:api_client/api_client.dart';
import 'package:built_collection/built_collection.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import '../support/fakes.dart';

bool _hasNull(Object? v) => switch (v) {
  null => true,
  Map() => v.values.any(_hasNull),
  List() => v.any(_hasNull),
  _ => false,
};

void main() {
  test('an unknown op is ignored', () {
    expect(
      decodeFrame(jsonEncode({'op': 'future.op', 'd': <String, Object?>{}})),
      isNull,
    );
  });

  test('unknown fields are ignored', () {
    final d = helloData()..['new_field'] = {'x': 1};
    final f = decodeFrame(jsonEncode({'op': 'hello', 'd': d}));
    expect(f, isA<HelloFrame>());
    expect((f! as HelloFrame).model.dpopNonce, 'nonce-1');
  });

  test('every server op decodes', () {
    final frames = {
      'hello': helloData(),
      'ready': readyData(),
      'resumed': resumedData(),
      'dispatch': {'type': 'x', 'seq': 1},
      'cursor': {'seq': 3},
      'catchup.complete': {'head_seq': 3},
      'resync_required': {
        'reason': 'retention',
        'head_seq': 3,
        'sync_epoch': 'e',
      },
      'heartbeat_ack': {'server_time': '2026-10-15T10:00:00.000Z'},
      'reauth_required': {
        'expires_at': '2026-10-15T10:00:00.000Z',
        'dpop_nonce': 'n',
      },
      'error': {'op': 'focus', 'code': 'validation_failed'},
    };
    for (final e in frames.entries) {
      final f = decodeFrame(jsonEncode({'op': e.key, 'd': e.value}));
      expect(f?.op, e.key);
    }
  });

  test('frames that are not JSON or have no op throw a decode error', () {
    for (final text in ['{', '[]', '{"d": {}}', '{"op": 3}', '"x"']) {
      expect(() => decodeFrame(text), throwsA(isA<FrameDecodeError>()));
    }
    expect(
      () => decodeFrame(
        jsonEncode({
          'op': 'hello',
          'd': {'dpop_nonce': 1},
        }),
      ),
      throwsA(isA<FrameDecodeError>()),
    );
  });

  test('a decode error message carries no frame content', () {
    try {
      decodeFrame('{"secret-value"');
      fail('no error');
    } on FrameDecodeError catch (e) {
      expect(e.toString(), isNot(contains('secret-value')));
    }
  });

  test('outgoing frames contain no null', () {
    final frames = <GatewayFrame>[
      IdentifyFrame(
        GatewayIdentify(
          (b) => b
            ..protocolVersion = 1
            ..accessToken = 't'
            ..dpop = 'p'
            ..deviceId = deviceId,
        ),
      ),
      ResumeFrame(
        GatewayResume(
          (b) => b
            ..gatewaySessionId = 'gs'
            ..accessToken = 't'
            ..dpop = 'p'
            ..cursor = 1,
        ),
      ),
      HeartbeatFrame(GatewayHeartbeat((b) => b..lastSeq = 0)),
      ReauthFrame(
        GatewayReauth(
          (b) => b
            ..accessToken = 't'
            ..dpop = 'p',
        ),
      ),
      FocusFrame(
        GatewayFocus((b) => b..threadIds = ListBuilder<String>(['a'])),
      ),
      TypingStartFrame(GatewayTyping((b) => b..threadId = 'a')),
      TypingStopFrame(GatewayTyping((b) => b..threadId = 'a')),
    ];
    for (final f in frames) {
      final json = jsonDecode(encodeFrame(f)) as Map<String, Object?>;
      expect(json['op'], f.op);
      expect(_hasNull(json), isFalse, reason: f.op);
    }
    final identify =
        jsonDecode(encodeFrame(frames.first)) as Map<String, Object?>;
    expect((identify['d']! as Map).containsKey('cursor'), isFalse);
  });

  test('the connection skips a broken frame and keeps the socket', () async {
    final h = Harness();
    addTearDown(h.dispose);
    final ch = await h.goLive();
    ch.pushRaw('not json');
    ch.pushRaw(jsonEncode({'op': 'something.new', 'd': <String, Object?>{}}));
    await settle();
    expect(ch.isClosed, isFalse);
    expect(h.connection.decodeErrors, 1);
    expect(h.phase, ConnectionPhase.live);
    expect(h.log.where((l) => l.contains('not json')), isEmpty);
  });
}
