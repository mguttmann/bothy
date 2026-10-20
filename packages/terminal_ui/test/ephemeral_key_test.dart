// SPDX-License-Identifier: Apache-2.0

// R-CL-15 (brief M4-35 Tests, ephemeral_key_test): the ephemeral private key is used for the
// handshake and destroyed (its bytes zeroed by `cryptography`) right after message 2; a
// cancelled or refused opening destroys it as well.

import 'package:bothy_noise/bothy_noise.dart';
import 'package:cryptography/cryptography.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';

void main() {
  test(
    'the private key is destroyed after message 2 and unusable afterwards',
    () async {
      final keys = <SimpleKeyPair>[];
      final h = await Harness.create(
        newEphemeralKey: () async {
          final pair = await generateEphemeralKeyPair();
          keys.add(pair);
          return pair;
        },
      );
      await h.open();
      await h.connected();
      expect(keys, hasLength(1));
      expect(keys.single.hasBeenDestroyed, isTrue);
      await expectLater(keys.single.extract(), throwsA(anything));

      await h.controller.detach();
      await h.controller.attach();
      await h.connected();
      expect(keys, hasLength(2));
      expect(keys.every((k) => k.hasBeenDestroyed), isTrue);
    },
  );

  test('a cancelled step-up and a mismatch destroy the key too', () async {
    final keys = <SimpleKeyPair>[];
    Future<SimpleKeyPair> factory() async {
      final pair = await generateEphemeralKeyPair();
      keys.add(pair);
      return pair;
    }

    final cancelled = await Harness.create(
      cancelStepUp: true,
      newEphemeralKey: factory,
    );
    await cancelled.open();
    final mismatch = await Harness.create(newEphemeralKey: factory);
    mismatch.api.grantGeneration = 5;
    await mismatch.open();
    expect(keys, hasLength(2));
    expect(keys.every((k) => k.hasBeenDestroyed), isTrue);
    expect(mismatch.controller.state.phase, TerminalPhase.blocked);
  });
}
