// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

void main() {
  test('events at or below the cursor are duplicates', () {
    expect(isDuplicate(const GlobalSeq(5), const GlobalSeq(5)), isTrue);
    expect(isDuplicate(const GlobalSeq(4), const GlobalSeq(5)), isTrue);
  });

  test('events above the cursor are new', () {
    expect(isDuplicate(const GlobalSeq(6), const GlobalSeq(5)), isFalse);
  });

  test('without a cursor nothing is a duplicate', () {
    expect(isDuplicate(const GlobalSeq(1), null), isFalse);
  });
}
