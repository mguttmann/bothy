// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

/// The known vector of M2-38 (24 4.3).
const String known =
    'F0CC-6QS6-RF5D-3BSH-CAZQ-PXMR-J8RA-AHK5-1HF0-9S02-1GW6-BW3J-HK90';

/// The fingerprints of the shared test vectors (24 4.3: server, host service and apps check
/// against the same file).
List<String> vectorFingerprints() {
  final json = jsonDecode(
    readRepoFile('services/host/testdata/noise/fingerprint.json'),
  ) as Map<String, Object?>;
  return [
    for (final v in json['vectors']! as List<Object?>)
      (v! as Map<String, Object?>)['fingerprint']! as String,
  ];
}

void main() {
  test('the known vector is in the shared test vectors', () {
    expect(vectorFingerprints(), contains(known));
  });

  test('the known vector gives 13 groups of four', () {
    final groups = fingerprintGroups(known);
    expect(groups, hasLength(13));
    expect(groups.first, 'F0CC');
    expect(groups.last, 'HK90');
    expect(groups.every((g) => g.length == 4), isTrue);
  });

  test('every shared vector displays as its groups joined by one space', () {
    for (final fp in vectorFingerprints()) {
      expect(fingerprintDisplay(fp), fp.replaceAll('-', ' '));
      expect(fingerprintGroups(fp), fp.split('-'));
    }
  });

  test('display form of the known vector', () {
    expect(
      fingerprintDisplay(known),
      'F0CC 6QS6 RF5D 3BSH CAZQ PXMR J8RA AHK5 1HF0 9S02 1GW6 BW3J HK90',
    );
  });

  test('separators and case do not matter for grouping', () {
    expect(fingerprintGroups('f0cc6qs6 rf5d'), ['F0CC', '6QS6', 'RF5D']);
    expect(fingerprintGroups('f0-cc 6q-s6'), ['F0CC', '6QS6']);
    expect(
      fingerprintDisplay(known.toLowerCase().replaceAll('-', '')),
      fingerprintDisplay(known),
    );
  });

  test('a length not divisible by four leaves a shorter last group', () {
    expect(fingerprintGroups('ABCDEF'), ['ABCD', 'EF']);
    expect(fingerprintDisplay('ABCDEFG'), 'ABCD EFG');
  });

  test('empty input gives no groups and empty forms', () {
    expect(fingerprintGroups(''), isEmpty);
    expect(fingerprintGroups(' - '), isEmpty);
    expect(fingerprintDisplay(''), '');
    expect(fingerprintShort(''), '');
  });

  test('short form of the header: the first two groups and " .."', () {
    expect(fingerprintShortGroups, 2);
    expect(fingerprintShort(known), 'F0CC 6QS6 ..');
    expect(fingerprintShort('7kq2-9xta-m4pb-1c8d'), '7KQ2 9XTA ..');
  });

  test('short form with another group count', () {
    expect(fingerprintShort(known, groups: 4), 'F0CC 6QS6 RF5D 3BSH ..');
    expect(fingerprintShort(known, groups: 13), fingerprintDisplay(known));
  });

  test('short form without " .." when nothing is cut off', () {
    expect(fingerprintShort('7KQ2'), '7KQ2');
    expect(fingerprintShort('7KQ2-9XTA'), '7KQ2 9XTA');
    expect(fingerprintShort('7KQ2-9X'), '7KQ2 9X');
    expect(fingerprintShort('7KQ2-9XTA-M'), '7KQ2 9XTA ..');
  });

  test('a short form of zero or fewer groups is rejected', () {
    expect(() => fingerprintShort(known, groups: 0), throwsArgumentError);
    expect(() => fingerprintShort(known, groups: -1), throwsArgumentError);
  });

  test('equality ignores separators and case', () {
    expect(fingerprintsEqual(known, known.toLowerCase()), isTrue);
    expect(fingerprintsEqual(known, known.replaceAll('-', ' ')), isTrue);
    expect(fingerprintsEqual(known, known.replaceAll('-', '')), isTrue);
    expect(fingerprintsEqual('', ' - '), isTrue);
  });

  test('different keys are not equal', () {
    final vectors = vectorFingerprints();
    expect(fingerprintsEqual(vectors[0], vectors[1]), isFalse);
    expect(
      fingerprintsEqual(known, known.substring(0, known.length - 1)),
      isFalse,
    );
    expect(fingerprintsEqual(known, ''), isFalse);
  });
}
