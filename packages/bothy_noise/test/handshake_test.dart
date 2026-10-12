// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';
import 'dart:typed_data';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:bothy_noise/src/handshake.dart' show debugUseKeyExchange;
import 'package:bothy_noise/src/transport.dart' show setTransportNonces;
import 'package:bothy_noise/testing.dart';
import 'package:cryptography/cryptography.dart';
import 'package:cryptography/dart.dart' show DartX25519;
import 'package:test/test.dart';

import 'support.dart';

/// X25519 whose shared secret fails like BrowserX25519 does in the web console for a
/// low-order key (a StateError around the OperationError of Web Crypto).
class ThrowingX25519 extends DartX25519 {
  const ThrowingX25519();

  @override
  Future<SecretKey> sharedSecretKey({
    required KeyPair keyPair,
    required PublicKey remotePublicKey,
  }) => throw StateError('sharedSecretKey failed: OperationError');
}

Matcher throwsNoise(NoiseError error) =>
    throwsA(isA<NoiseException>().having((e) => e.error, 'error', error));

const sessionText = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e11';
const hostText = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e12';

class Pair {
  Pair(this.device, this.host);
  final NoiseTransport device;
  final NoiseTransport host;
}

Future<SimpleKeyPair> testKey(String label) async =>
    keyPairOf(await sha256Of(label));

Future<Pair> handshakePair() async {
  final static = await testKey('test/static');
  final init = await NkInitiator.start(
    hostStaticPublicKey: await publicOf(static),
    sessionId: uuidToBytes(sessionText),
    hostId: uuidToBytes(hostText),
    ephemeral: await testKey('test/eph-i'),
  );
  final resp = await NkResponder.start(
    staticKeyPair: static,
    sessionId: uuidToBytes(sessionText),
    hostId: uuidToBytes(hostText),
  );
  final m1 = await init.writeMessage1(utf8.encode('token'));
  expect(await resp.readMessage1(m1), utf8.encode('token'));
  expect(
    resp.initiatorEphemeralPublicKey,
    await publicOf(await testKey('test/eph-i')),
  );
  final (m2, hostT) = await resp.writeMessage2(utf8.encode('{}'));
  final result = await init.readMessage2(m2);
  expect(result.payload, utf8.encode('{}'));
  expect(result.handshakeHash, hasLength(32));
  return Pair(result.transport, hostT);
}

void main() {
  test('constants', () {
    expect(noiseProtocolName, 'Noise_NK_25519_ChaChaPoly_SHA256');
    expect(prologueLabel, 'bothy-terminal/1');
    expect(maxNoiseMessageLength, 65535);
    expect(noiseTagLength, 16);
    expect(maxTransportPlaintextLength, 65519);
  });

  test('prologue bytes', () {
    final p = buildPrologue(
      uuidToBytes('0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f'),
      uuidToBytes('0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e10'),
    );
    expect(p, [
      ...ascii.encode('bothy-terminal/1'),
      0x01,
      0x92,
      0xf4,
      0xa0,
      0x7b,
      0x1c,
      0x7b,
      0x22,
      0x9d,
      0x4f,
      0x5a,
      0x6b,
      0x7c,
      0x8d,
      0x9e,
      0x0f, //
      0x01,
      0x92,
      0xf4,
      0xa0,
      0x7b,
      0x1c,
      0x7b,
      0x22,
      0x9d,
      0x4f,
      0x5a,
      0x6b,
      0x7c,
      0x8d,
      0x9e,
      0x10,
    ]);
    expect(
      () => buildPrologue(Uint8List(15), Uint8List(16)),
      throwsNoise(NoiseError.invalidId),
    );
    expect(
      () => buildPrologue(Uint8List(16), Uint8List(17)),
      throwsNoise(NoiseError.invalidId),
    );
  });

  test('uuidToBytes cases', () {
    expect(uuidToBytes(sessionText.toUpperCase()), uuidToBytes(sessionText));
    for (final bad in [
      sessionText.substring(0, 35),
      '${sessionText}0',
      '{$sessionText}',
      'urn:uuid:$sessionText',
      '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e1g',
      '0192f4a07-b1c-7b22-9d4f-5a6b7c8d9e11',
      '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e1 ',
      '+192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e11',
      '',
    ]) {
      expect(
        () => uuidToBytes(bad),
        throwsNoise(NoiseError.invalidId),
        reason: '${bad.length} characters',
      );
    }
  });

  test('ephemeralKeyHash literal', () async {
    expect(
      await ephemeralKeyHash(List<int>.generate(32, (i) => i)),
      'Yw3NKWbEM2aRElRIu7JbT_QSpJxzLbLIq8G4WBvXEN0',
    );
    await expectLater(
      ephemeralKeyHash(Uint8List(31)),
      throwsNoise(NoiseError.invalidKey),
    );
    await expectLater(
      ephemeralKeyHash(Uint8List(33)),
      throwsNoise(NoiseError.invalidKey),
    );
    final kp = await generateEphemeralKeyPair();
    expect(await ephemeralKeyHash(await publicOf(kp)), hasLength(43));
  });

  test('handshake and transport in both directions', () async {
    final pair = await handshakePair();
    for (var i = 0; i < 3; i++) {
      expect(await pair.host.decrypt(await pair.device.encrypt([1, i])), [
        1,
        i,
      ]);
      expect(await pair.device.decrypt(await pair.host.encrypt([2, i])), [
        2,
        i,
      ]);
    }
  });

  test('encrypt results follow the call order', () async {
    final pair = await handshakePair();
    final pending = [
      for (var i = 0; i < 20; i++) pair.device.encrypt([i]),
    ];
    final messages = await Future.wait(pending);
    for (var i = 0; i < messages.length; i++) {
      expect(await pair.host.decrypt(messages[i]), [i]);
    }
  });

  test('message limits', () async {
    final pair = await handshakePair();
    final max = Uint8List(maxTransportPlaintextLength);
    final msg = await pair.device.encrypt(max);
    expect(msg, hasLength(maxNoiseMessageLength));
    expect(await pair.host.decrypt(msg), hasLength(max.length));
    await expectLater(
      pair.device.encrypt(Uint8List(max.length + 1)),
      throwsNoise(NoiseError.messageTooLong),
    );
    expect(await pair.host.decrypt(await pair.device.encrypt([7])), [7]);

    final p2 = await handshakePair();
    await expectLater(
      p2.host.decrypt(Uint8List(maxNoiseMessageLength + 1)),
      throwsNoise(NoiseError.messageTooLong),
    );
    final p3 = await handshakePair();
    await expectLater(
      p3.host.decrypt(Uint8List(noiseTagLength - 1)),
      throwsNoise(NoiseError.decryptFailed),
    );

    final static = await testKey('test/static');
    Future<NkInitiator> newInit() async => NkInitiator.start(
      hostStaticPublicKey: await publicOf(static),
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
      ephemeral: await testKey('test/eph-i'),
    );
    var init = await newInit();
    await expectLater(
      init.writeMessage1(Uint8List(maxNoiseMessageLength - 48 + 1)),
      throwsNoise(NoiseError.messageTooLong),
    );
    init = await newInit();
    expect(
      await init.writeMessage1(Uint8List(maxNoiseMessageLength - 48)),
      hasLength(maxNoiseMessageLength),
    );
    var resp = await NkResponder.start(
      staticKeyPair: static,
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
    );
    await expectLater(
      resp.readMessage1(Uint8List(maxNoiseMessageLength + 1)),
      throwsNoise(NoiseError.messageTooLong),
    );
    resp = await NkResponder.start(
      staticKeyPair: static,
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
    );
    init = await newInit();
    await resp.readMessage1(await init.writeMessage1(const []));
    await expectLater(
      resp.writeMessage2(Uint8List(maxNoiseMessageLength - 48 + 1)),
      throwsNoise(NoiseError.messageTooLong),
    );
    await expectLater(
      init.readMessage2(Uint8List(maxNoiseMessageLength + 1)),
      throwsNoise(NoiseError.messageTooLong),
    );
  });

  test('zero key rejected', () async {
    final init = await NkInitiator.start(
      hostStaticPublicKey: Uint8List(32),
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
      ephemeral: await testKey('test/eph-i'),
    );
    await expectLater(
      init.writeMessage1(const []),
      throwsNoise(NoiseError.handshakeFailed),
    );
    final resp = await NkResponder.start(
      staticKeyPair: await testKey('test/static'),
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
    );
    await expectLater(
      resp.readMessage1(Uint8List(48)),
      throwsNoise(NoiseError.handshakeFailed),
    );
    final static = await testKey('test/static');
    final init2 = await NkInitiator.start(
      hostStaticPublicKey: await publicOf(static),
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
      ephemeral: await testKey('test/eph-i'),
    );
    await init2.writeMessage1(const []);
    await expectLater(
      init2.readMessage2(Uint8List(48)),
      throwsNoise(NoiseError.handshakeFailed),
    );
  });

  test('invalid keys', () async {
    await expectLater(
      NkInitiator.start(
        hostStaticPublicKey: Uint8List(31),
        sessionId: Uint8List(16),
        hostId: Uint8List(16),
        ephemeral: await testKey('x'),
      ),
      throwsNoise(NoiseError.invalidKey),
    );
    await expectLater(
      NkInitiator.start(
        hostStaticPublicKey: Uint8List(32),
        sessionId: Uint8List(15),
        hostId: Uint8List(16),
        ephemeral: await testKey('x'),
      ),
      throwsNoise(NoiseError.invalidId),
    );
    final ed = await Ed25519().newKeyPair();
    await expectLater(
      NkInitiator.start(
        hostStaticPublicKey: Uint8List(32),
        sessionId: Uint8List(16),
        hostId: Uint8List(16),
        ephemeral: ed,
      ),
      throwsNoise(NoiseError.invalidKey),
    );
    await expectLater(
      NkResponder.start(
        staticKeyPair: ed,
        sessionId: Uint8List(16),
        hostId: Uint8List(16),
      ),
      throwsNoise(NoiseError.invalidKey),
    );
  });

  test('broken after failure', () async {
    final pair = await handshakePair();
    final msg = await pair.device.encrypt([1, 2, 3]);
    final tampered = Uint8List.fromList(msg)..[0] ^= 1;
    await expectLater(
      pair.host.decrypt(tampered),
      throwsNoise(NoiseError.decryptFailed),
    );
    await expectLater(pair.host.decrypt(msg), throwsNoise(NoiseError.broken));
    await expectLater(pair.host.encrypt([1]), throwsNoise(NoiseError.broken));

    final static = await testKey('test/static');
    final resp = await NkResponder.start(
      staticKeyPair: static,
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
    );
    final init = await NkInitiator.start(
      hostStaticPublicKey: await publicOf(static),
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
      ephemeral: await testKey('test/eph-i'),
    );
    final m1 = await init.writeMessage1(utf8.encode('token'));
    final bad = Uint8List.fromList(m1)..[m1.length - 1] ^= 1;
    await expectLater(
      resp.readMessage1(bad),
      throwsNoise(NoiseError.handshakeFailed),
    );
    await expectLater(resp.readMessage1(m1), throwsNoise(NoiseError.broken));
    await expectLater(
      resp.writeMessage2(const []),
      throwsNoise(NoiseError.broken),
    );
    await expectLater(
      init.readMessage2(Uint8List(10)),
      throwsNoise(NoiseError.handshakeFailed),
    );
    await expectLater(
      init.readMessage2(Uint8List(10)),
      throwsNoise(NoiseError.broken),
    );
    await expectLater(
      init.writeMessage1(const []),
      throwsNoise(NoiseError.broken),
    );
  });

  test('nonce limit', () async {
    final pair = await handshakePair();
    // The last usable nonce is 2^64 - 2 (high 0xffffffff, low 0xfffffffe).
    setTransportNonces(pair.device, sendHigh: 0xffffffff, sendLow: 0xfffffffe);
    setTransportNonces(
      pair.host,
      receiveHigh: 0xffffffff,
      receiveLow: 0xfffffffe,
    );
    final msg = await pair.device.encrypt([9]);
    expect(await pair.host.decrypt(msg), [9]);
    await expectLater(
      pair.device.encrypt([9]),
      throwsNoise(NoiseError.nonceExhausted),
    );
    await expectLater(
      pair.host.decrypt(msg),
      throwsNoise(NoiseError.nonceExhausted),
    );
    // A carry from the low into the high word.
    final p2 = await handshakePair();
    setTransportNonces(p2.device, sendHigh: 0, sendLow: 0xffffffff);
    setTransportNonces(p2.host, receiveHigh: 0, receiveLow: 0xffffffff);
    expect(await p2.host.decrypt(await p2.device.encrypt([1])), [1]);
    expect(await p2.host.decrypt(await p2.device.encrypt([2])), [2]);
  });

  test('call order', () async {
    final static = await testKey('test/static');
    final init = await NkInitiator.start(
      hostStaticPublicKey: await publicOf(static),
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
      ephemeral: await testKey('test/eph-i'),
    );
    await expectLater(
      init.readMessage2(Uint8List(48)),
      throwsNoise(NoiseError.invalidState),
    );
    final m1 = await init.writeMessage1(const []);
    await expectLater(
      init.writeMessage1(const []),
      throwsNoise(NoiseError.invalidState),
    );
    final resp = await NkResponder.start(
      staticKeyPair: static,
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
    );
    await expectLater(
      resp.writeMessage2(const []),
      throwsNoise(NoiseError.invalidState),
    );
    expect(
      () => resp.initiatorEphemeralPublicKey,
      throwsNoise(NoiseError.invalidState),
    );
    await resp.readMessage1(m1);
    await expectLater(
      resp.readMessage1(m1),
      throwsNoise(NoiseError.invalidState),
    );
    final (m2, _) = await resp.writeMessage2(const []);
    await expectLater(
      resp.writeMessage2(const []),
      throwsNoise(NoiseError.invalidState),
    );
    await init.readMessage2(m2);
    await expectLater(
      init.readMessage2(m2),
      throwsNoise(NoiseError.invalidState),
    );
  });

  test('errors carry no content', () async {
    const canary = 'CANARY-7f3a91';
    final static = await testKey('test/static');
    final eph = await testKey('test/eph-i');
    final secrets = <String>[];
    for (final b in [
      await sha256Of('test/static'),
      await publicOf(static),
      await sha256Of('test/eph-i'),
      await publicOf(eph),
    ]) {
      secrets.addAll([
        toHex(b),
        toHex(b).toUpperCase(),
        base64Url.encode(b).replaceAll('=', ''),
        base64.encode(b),
      ]);
    }
    final texts = <String>[];
    Future<void> collect(Future<Object?> Function() f) async {
      try {
        await f();
        fail('no error');
      } on NoiseException catch (e) {
        texts.add(e.toString());
      }
    }

    final init = await NkInitiator.start(
      hostStaticPublicKey: await publicOf(static),
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
      ephemeral: eph,
    );
    final m1 = await init.writeMessage1(utf8.encode(canary));
    final resp = await NkResponder.start(
      staticKeyPair: static,
      sessionId: Uint8List(16)..[0] = 1,
      hostId: Uint8List(16),
    );
    await collect(() => resp.readMessage1(m1));
    await collect(() => resp.readMessage1(m1));
    final resp2 = await NkResponder.start(
      staticKeyPair: static,
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
    );
    await collect(
      () => resp2.readMessage1([...utf8.encode(canary * 6000), ...m1]),
    );
    await collect(() => ephemeralKeyHash(utf8.encode(canary)));
    await collect(() async => uuidToBytes(canary));
    final pair = await handshakePair();
    final msg = await pair.device.encrypt(utf8.encode(canary));
    msg[0] ^= 1;
    await collect(() => pair.host.decrypt(msg));
    await collect(() => pair.host.decrypt(msg));
    await collect(() => pair.device.encrypt(utf8.encode(canary * 6000)));
    expect(texts, hasLength(8));
    for (final t in texts) {
      expect(
        t.contains('7f3a91'),
        isFalse,
        reason: 'an error text carries the payload',
      );
      for (final s in secrets) {
        expect(
          t.contains(s.substring(0, 16)),
          isFalse,
          reason: 'an error text carries key material',
        );
      }
    }
  });

  test('a failing curve function is a handshake failure and breaks', () async {
    final static = await testKey('test/static');
    final hostKey = await publicOf(static);
    final eph = await testKey('test/eph-i');
    final init = await NkInitiator.start(
      hostStaticPublicKey: hostKey,
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
      ephemeral: eph,
    );
    final good = await NkInitiator.start(
      hostStaticPublicKey: hostKey,
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
      ephemeral: eph,
    );
    final m1 = await good.writeMessage1(const []);
    final resp = await NkResponder.start(
      staticKeyPair: static,
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
    );
    final resp2 = await NkResponder.start(
      staticKeyPair: static,
      sessionId: Uint8List(16),
      hostId: Uint8List(16),
    );
    await resp2.readMessage1(m1);
    debugUseKeyExchange(const ThrowingX25519());
    addTearDown(() => debugUseKeyExchange(null));
    await expectLater(
      init.writeMessage1(const []),
      throwsNoise(NoiseError.handshakeFailed),
    );
    await expectLater(
      init.writeMessage1(const []),
      throwsNoise(NoiseError.broken),
    );
    await expectLater(
      resp.readMessage1(m1),
      throwsNoise(NoiseError.handshakeFailed),
    );
    await expectLater(resp.readMessage1(m1), throwsNoise(NoiseError.broken));
    await expectLater(
      resp2.writeMessage2(const []),
      throwsNoise(NoiseError.handshakeFailed),
    );
    await expectLater(
      resp2.writeMessage2(const []),
      throwsNoise(NoiseError.broken),
    );
    debugUseKeyExchange(null);
    final m2 = (await resp2Pair(static, m1)).$1;
    debugUseKeyExchange(const ThrowingX25519());
    await expectLater(
      good.readMessage2(m2),
      throwsNoise(NoiseError.handshakeFailed),
    );
    await expectLater(good.readMessage2(m2), throwsNoise(NoiseError.broken));
  });

  test('overlapping decrypt calls honour an earlier failure', () async {
    final pair = await handshakePair();
    final a = await pair.device.encrypt([1, 2, 3]);
    final b = await pair.device.encrypt([4, 5, 6]);
    final tampered = Uint8List.fromList(a)..[0] ^= 1;
    final first = pair.host.decrypt(tampered);
    final second = pair.host.decrypt(b);
    await expectLater(first, throwsNoise(NoiseError.decryptFailed));
    await expectLater(second, throwsNoise(NoiseError.broken));
  });
}

Future<(Uint8List, NoiseTransport)> resp2Pair(
  SimpleKeyPair static,
  List<int> m1,
) async {
  final r = await NkResponder.start(
    staticKeyPair: static,
    sessionId: Uint8List(16),
    hostId: Uint8List(16),
  );
  await r.readMessage1(m1);
  return r.writeMessage2(const []);
}
