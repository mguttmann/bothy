// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

LoadedRanges _ranges(List<(int, int)> pairs) {
  var r = LoadedRanges.empty;
  for (final (a, b) in pairs) {
    r = r.add(ThreadSeq(a), ThreadSeq(b));
  }
  return r;
}

void main() {
  test('adjacent ranges merge', () {
    expect(_ranges([(1, 5), (6, 9)]).ranges, [
      SeqRange(const ThreadSeq(1), const ThreadSeq(9)),
    ]);
  });

  test('overlapping ranges merge, also across several', () {
    final r = _ranges([(10, 12), (1, 3), (20, 25), (2, 21)]);
    expect(r.ranges, [SeqRange(const ThreadSeq(1), const ThreadSeq(25))]);
  });

  test('disjoint ranges stay sorted', () {
    final r = _ranges([(20, 25), (1, 3), (10, 12)]);
    expect(r.ranges.map((e) => e.toString()), ['1..3', '10..12', '20..25']);
  });

  test('contains', () {
    final r = _ranges([(1, 3), (10, 12)]);
    expect(r.contains(const ThreadSeq(3)), isTrue);
    expect(r.contains(const ThreadSeq(4)), isFalse);
    expect(r.contains(const ThreadSeq(10)), isTrue);
  });

  test('gaps up to the last message', () {
    final r = _ranges([(3, 5), (8, 9)]);
    expect(r.gapsUpTo(const ThreadSeq(12)).map((e) => e.toString()), [
      '1..2',
      '6..7',
      '10..12',
    ]);
    expect(r.gapsUpTo(const ThreadSeq(4)).map((e) => e.toString()), ['1..2']);
    expect(_ranges([(1, 9)]).gapsUpTo(const ThreadSeq(9)), isEmpty);
    expect(LoadedRanges.empty.gapsUpTo(const ThreadSeq(0)), isEmpty);
  });

  test('fits at the edge and with a gap', () {
    final r = _ranges([(1, 5)]);
    expect(r.fits(const ThreadSeq(6)), isTrue);
    expect(r.fits(const ThreadSeq(5)), isTrue);
    expect(r.fits(const ThreadSeq(7)), isFalse);
    expect(LoadedRanges.empty.fits(const ThreadSeq(1)), isTrue);
    expect(LoadedRanges.empty.fits(const ThreadSeq(2)), isFalse);
  });

  test('invalid ranges are rejected', () {
    expect(
      () => LoadedRanges.empty.add(const ThreadSeq(5), const ThreadSeq(4)),
      throwsArgumentError,
    );
    expect(
      () => LoadedRanges.empty.add(const ThreadSeq(0), const ThreadSeq(4)),
      throwsArgumentError,
    );
  });

  test('ranges compare by value', () {
    final a = SeqRange(const ThreadSeq(1), const ThreadSeq(2));
    expect(a, SeqRange(const ThreadSeq(1), const ThreadSeq(2)));
    expect(
      a.hashCode,
      SeqRange(const ThreadSeq(1), const ThreadSeq(2)).hashCode,
    );
  });
}
