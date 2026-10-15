// SPDX-License-Identifier: Apache-2.0

/// Loaded ranges of `thread_seq` per thread (15 6.1 `loaded_ranges`, 15 7.3 item 4): a message
/// that does not fit them reveals a gap, which the fetcher loads through REST (04 4).
library;

import '../seq.dart';

/// An inclusive range of thread positions.
final class SeqRange {
  /// Creates the range [from] to [to], both included; throws an [ArgumentError] when [to] is
  /// before [from] or [from] is below 1.
  SeqRange(this.from, this.to) {
    if (from.value < 1 || to < from) {
      throw ArgumentError('a range needs 1 <= from <= to');
    }
  }

  /// First position.
  final ThreadSeq from;

  /// Last position.
  final ThreadSeq to;

  @override
  bool operator ==(Object other) =>
      other is SeqRange && other.from == from && other.to == to;

  @override
  int get hashCode => Object.hash(from, to);

  @override
  String toString() => '${from.value}..${to.value}';
}

/// The loaded ranges of one thread: sorted, disjoint and never adjacent (adjacent ranges are
/// merged). Immutable; every change returns a new value.
final class LoadedRanges {
  const LoadedRanges._(this.ranges);

  /// No range loaded.
  static const LoadedRanges empty = LoadedRanges._(<SeqRange>[]);

  /// The ranges in ascending order.
  final List<SeqRange> ranges;

  /// Returns these ranges plus [from] to [to] (inclusive), merging overlapping and adjacent
  /// ranges. Throws an [ArgumentError] for an invalid range (see [SeqRange]).
  LoadedRanges add(ThreadSeq from, ThreadSeq to) {
    var start = from.value;
    var end = to.value;
    SeqRange(from, to); // validates
    final result = <SeqRange>[];
    var placed = false;
    for (final r in ranges) {
      if (r.to.value + 1 < start) {
        result.add(r);
      } else if (end + 1 < r.from.value) {
        if (!placed) {
          result.add(SeqRange(ThreadSeq(start), ThreadSeq(end)));
          placed = true;
        }
        result.add(r);
      } else {
        // Overlapping or adjacent: widen the new range.
        if (r.from.value < start) start = r.from.value;
        if (r.to.value > end) end = r.to.value;
      }
    }
    if (!placed) result.add(SeqRange(ThreadSeq(start), ThreadSeq(end)));
    return LoadedRanges._(List.unmodifiable(result));
  }

  /// Whether [seq] lies in a loaded range.
  bool contains(ThreadSeq seq) =>
      ranges.any((r) => r.from <= seq && seq <= r.to);

  /// The missing ranges from 1 up to [lastMessageSeq] (the thread's `last_message_seq`).
  List<SeqRange> gapsUpTo(ThreadSeq lastMessageSeq) {
    final gaps = <SeqRange>[];
    var next = 1;
    for (final r in ranges) {
      if (r.from.value > lastMessageSeq.value) break;
      if (r.from.value > next) {
        gaps.add(SeqRange(ThreadSeq(next), ThreadSeq(r.from.value - 1)));
      }
      next = r.to.value + 1;
    }
    if (next <= lastMessageSeq.value) {
      gaps.add(SeqRange(ThreadSeq(next), lastMessageSeq));
    }
    return gaps;
  }

  /// Whether a new message at [seq] continues what is loaded: it is already loaded, directly
  /// follows a loaded range, or is the first message of the thread. Otherwise it opens a gap
  /// (15 7.3 item 4).
  bool fits(ThreadSeq seq) =>
      seq.value == 1 || contains(seq) || ranges.any((r) => r.to.next == seq);
}
