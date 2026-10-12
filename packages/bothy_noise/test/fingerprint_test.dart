// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';
import 'dart:io';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:test/test.dart';

import 'support.dart';

List<int> sequence(int from) => List<int>.generate(32, (i) => from + i);

void main() {
  test('RFC 4648 inputs in the Crockford alphabet', () {
    const cases = {
      '': '',
      'f': 'CR',
      'fo': 'CSQG',
      'foo': 'CSQPY',
      'foob': 'CSQPYRG',
      'fooba': 'CSQPYRK1',
      'foobar': 'CSQPYRK1E8',
    };
    cases.forEach((input, want) {
      expect(crockfordBase32(utf8.encode(input)), want, reason: input);
    });
  });

  test('known vector', () async {
    expect(
      await hostKeyFingerprint(sequence(0x00), sequence(0x20)),
      'F0CC-6QS6-RF5D-3BSH-CAZQ-PXMR-J8RA-AHK5-1HF0-9S02-1GW6-BW3J-HK90',
    );
  });

  test('display form and key order', () async {
    final fp = await hostKeyFingerprint(sequence(0x40), sequence(0x60));
    final groups = fp.split('-');
    expect(groups, hasLength(13));
    for (final g in groups) {
      expect(RegExp(r'^[0-9A-HJKMNP-TV-Z]{4}$').hasMatch(g), isTrue, reason: g);
    }
    expect(await hostKeyFingerprint(sequence(0x60), sequence(0x40)), isNot(fp));
  });

  test('wrong key lengths are rejected', () async {
    for (final n in [0, 31, 33]) {
      await expectLater(
        hostKeyFingerprint(List<int>.filled(n, 0), sequence(0)),
        throwsA(
          isA<NoiseException>().having(
            (e) => e.error,
            'error',
            NoiseError.invalidKey,
          ),
        ),
      );
      await expectLater(
        hostKeyFingerprint(sequence(0), List<int>.filled(n, 0)),
        throwsA(
          isA<NoiseException>().having(
            (e) => e.error,
            'error',
            NoiseError.invalidKey,
          ),
        ),
      );
    }
  });

  test('fingerprint vectors match', () async {
    final file = File('${testdataDir().path}/fingerprint.json');
    final data = jsonDecode(file.readAsStringSync()) as Map<String, Object?>;
    expect(data['format'], 1);
    final vectors = (data['vectors']! as List<Object?>)
        .cast<Map<String, Object?>>();
    expect(vectors, hasLength(9));
    for (final v in vectors) {
      final fp = await hostKeyFingerprint(
        fromHex(v['noise_public']! as String),
        fromHex(v['ed25519_public']! as String),
      );
      expect(fp, v['fingerprint']);
    }
  });
}
