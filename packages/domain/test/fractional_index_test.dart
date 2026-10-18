// SPDX-License-Identifier: Apache-2.0

// The order key of the personal sidebar order (brief M4-19 Design note 2): the same vectors and
// properties as the server's `fracindex` (M1-52), so a key the client sends is never refused.

import 'dart:math';

import 'package:domain/domain.dart';
import 'package:test/test.dart';

void main() {
  final zeros26 = '0' * 26;
  final zeros25 = '0' * 25;

  test('the normative vectors of M1-52 and the server tests', () {
    final cases = <(String, String, String)>[
      ('', '', 'a0'),
      ('a0', '', 'a1'),
      ('', 'a0', 'Zz'),
      ('a0', 'a1', 'a0V'),
      ('a1', '', 'a2'),
      ('az', '', 'b00'),
      ('Zz', 'a0', 'ZzV'),
      ('a0V', 'a1', 'a0l'),
      ('a0', 'a0V', 'a0G'),
      ('a0', 'a0G', 'a08'),
      ('b00', '', 'b01'),
      ('', 'Zz', 'Zy'),
      ('', 'a0V', 'a0'),
      ('', 'A${zeros26}1', 'A${zeros26}0V'),
      ('A${zeros26}1', 'A${zeros25}1', 'A${zeros26}W'),
      ('z' * 27, '', '${'z' * 27}V'),
    ];
    for (final (a, b, want) in cases) {
      expect(
        FractionalIndex.between(a.isEmpty ? null : a, b.isEmpty ? null : b),
        want,
        reason: 'between("$a", "$b")',
      );
    }
  });

  test('a lower bound not below the upper bound and invalid keys throw', () {
    final cases = <(String, String)>[
      ('a1', 'a0'),
      ('a0', 'a0'),
      ('a00', ''),
      ('', 'a10'),
      ('!', ''),
      ('a', ''),
      ('A$zeros26', ''),
      ('a0', 'b'),
    ];
    for (final (a, b) in cases) {
      expect(
        () =>
            FractionalIndex.between(a.isEmpty ? null : a, b.isEmpty ? null : b),
        throwsArgumentError,
        reason: 'between("$a", "$b")',
      );
    }
  });

  test('validity follows the key grammar', () {
    for (final k in [
      'a0',
      'a0V',
      'Zz',
      'b00',
      'z' * 27,
      'A${zeros25}1',
      'a01',
      'A${zeros26}1',
    ]) {
      expect(FractionalIndex.isValid(k), isTrue, reason: k);
    }
    for (final k in [
      '',
      'a',
      'a00',
      'a0V0',
      'b0',
      'Z',
      'A$zeros26',
      'a-',
      'aä0',
      '0',
      'a0 ',
    ]) {
      expect(FractionalIndex.isValid(k), isFalse, reason: k);
    }
  });

  test(
    'random inserts lie strictly between their neighbours in byte order',
    () {
      final random = Random(52);
      final keys = <String>['a0'];
      for (var n = 0; n < 2000; n++) {
        final i = random.nextInt(keys.length + 1);
        final a = i > 0 ? keys[i - 1] : null;
        final b = i < keys.length ? keys[i] : null;
        final k = FractionalIndex.between(a, b);
        expect(FractionalIndex.isValid(k), isTrue, reason: k);
        if (a != null) {
          expect(k.compareTo(a), greaterThan(0), reason: '$a < $k');
        }
        if (b != null) {
          expect(k.compareTo(b), lessThan(0), reason: '$k < $b');
        }
        keys.insert(i, k);
      }
      // Dart compares strings by UTF-16 code units; for the ASCII digits that is the byte order
      // the server's COLLATE "C" uses.
      expect(keys, List.of(keys)..sort());
    },
  );

  test('repeated inserts at the start and the end keep short keys', () {
    String? last;
    String? first;
    for (var n = 0; n < 10000; n++) {
      last = FractionalIndex.between(last, null);
      first = FractionalIndex.between(null, first);
    }
    expect(last!.length, lessThanOrEqualTo(4));
    expect(first!.length, lessThanOrEqualTo(4));
  });
}
