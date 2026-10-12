// SPDX-License-Identifier: Apache-2.0

// Every public input of type List<int> rejects values outside 0 to 255 instead of truncating
// them as Uint8List.fromList would; the Go side can only receive bytes (D-M2-38-21).

import 'dart:typed_data';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:bothy_noise/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

Matcher throwsNoise(NoiseError error) =>
    throwsA(isA<NoiseException>().having((e) => e.error, 'error', error));

/// [base] with 0x100 added to its first value: equal after truncation, but not a byte list.
List<int> widened(List<int> base) => [base[0] + 0x100, ...base.skip(1)];

void main() {
  final ids = Uint8List(16);

  test('ids, keys and fingerprint inputs', () async {
    expect(
      () => buildPrologue(widened(ids), ids),
      throwsNoise(NoiseError.invalidId),
    );
    expect(
      () => buildPrologue(ids, [-1, ...ids.skip(1)]),
      throwsNoise(NoiseError.invalidId),
    );
    await expectLater(
      ephemeralKeyHash(widened(Uint8List(32))),
      throwsNoise(NoiseError.invalidKey),
    );
    await expectLater(
      hostKeyFingerprint(widened(Uint8List(32)), Uint8List(32)),
      throwsNoise(NoiseError.invalidKey),
    );
    await expectLater(
      hostKeyFingerprint(Uint8List(32), widened(Uint8List(32))),
      throwsNoise(NoiseError.invalidKey),
    );
    expect(() => crockfordBase32([0x166]), throwsArgumentError);
    expect(crockfordBase32([0x66]), 'CR');
  });

  test('handshake inputs', () async {
    final static = await keyPairOf(await sha256Of('test/static'));
    final hostKey = await publicOf(static);
    final eph = await keyPairOf(await sha256Of('test/eph-i'));
    await expectLater(
      NkInitiator.start(
        hostStaticPublicKey: widened(hostKey),
        sessionId: ids,
        hostId: ids,
        ephemeral: eph,
      ),
      throwsNoise(NoiseError.invalidKey),
    );
    await expectLater(
      NkResponder.start(
        staticKeyPair: static,
        sessionId: widened(ids),
        hostId: ids,
      ),
      throwsNoise(NoiseError.invalidId),
    );

    var init = await NkInitiator.start(
      hostStaticPublicKey: hostKey,
      sessionId: ids,
      hostId: ids,
      ephemeral: eph,
    );
    await expectLater(init.writeMessage1([0x100]), throwsArgumentError);
    // A refused local payload changes no state.
    final m1 = await init.writeMessage1(const [1]);

    var resp = await NkResponder.start(
      staticKeyPair: static,
      sessionId: ids,
      hostId: ids,
    );
    await expectLater(
      resp.readMessage1(widened(m1)),
      throwsNoise(NoiseError.handshakeFailed),
    );
    await expectLater(resp.readMessage1(m1), throwsNoise(NoiseError.broken));

    resp = await NkResponder.start(
      staticKeyPair: static,
      sessionId: ids,
      hostId: ids,
    );
    await resp.readMessage1(m1);
    await expectLater(resp.writeMessage2([0x100]), throwsArgumentError);
    final (m2, hostT) = await resp.writeMessage2(const [2]);
    await expectLater(
      init.readMessage2(widened(m2)),
      throwsNoise(NoiseError.handshakeFailed),
    );
    await expectLater(init.readMessage2(m2), throwsNoise(NoiseError.broken));

    init = await NkInitiator.start(
      hostStaticPublicKey: hostKey,
      sessionId: ids,
      hostId: ids,
      ephemeral: eph,
    );
    final resp2 = await NkResponder.start(
      staticKeyPair: static,
      sessionId: ids,
      hostId: ids,
    );
    await resp2.readMessage1(await init.writeMessage1(const []));
    final (m2b, host) = await resp2.writeMessage2(const []);
    final device = (await init.readMessage2(m2b)).transport;

    await expectLater(device.encrypt([0x100]), throwsArgumentError);
    // The refused call used no nonce and did not break the transport.
    final msg = await device.encrypt(const [1, 2, 3]);
    await expectLater(
      host.decrypt(widened(msg)),
      throwsNoise(NoiseError.decryptFailed),
    );
    await expectLater(host.decrypt(msg), throwsNoise(NoiseError.broken));
    expect(hostT, isNotNull);
  });
}
