// SPDX-License-Identifier: Apache-2.0

/// Order of the post list of a forum with the Done section (01 6.1).
library;

import '../entities/channel.dart';
import '../entities/thread.dart';

/// The post list split into open threads and the Done section.
final class ForumOrderResult {
  /// Creates a result.
  const ForumOrderResult({required this.open, required this.done});

  /// Open threads, newest first.
  final List<Thread> open;

  /// Closed threads for the section "Done (n)", in the same order.
  final List<Thread> done;
}

/// Sorting of a forum's post list.
abstract final class ForumOrder {
  /// Sorts [threads] by [sort] descending (newest first) and moves closed threads into a
  /// separate Done list (01 6.1, 03 index `(channel_id, state, last_activity_at DESC)`).
  ///
  /// [ForumSort.created] sorts by creation time; every other value, also
  /// [ForumSort.unknown], by last activity. Equal times sort by ID descending, so the order
  /// is the same on every device.
  static ForumOrderResult sort(List<Thread> threads, ForumSort sort) {
    DateTime key(Thread t) =>
        sort == ForumSort.created ? t.createdAt : t.lastActivityAt;
    int compare(Thread a, Thread b) {
      final byTime = key(b).compareTo(key(a));
      return byTime != 0 ? byTime : b.id.value.compareTo(a.id.value);
    }

    final open = threads.where((t) => t.state != ThreadState.closed).toList()
      ..sort(compare);
    final done = threads.where((t) => t.state == ThreadState.closed).toList()
      ..sort(compare);
    return ForumOrderResult(
      open: List.unmodifiable(open),
      done: List.unmodifiable(done),
    );
  }
}
