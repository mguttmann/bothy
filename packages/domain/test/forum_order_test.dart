// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  DateTime at(int minutes) => t0.add(Duration(minutes: minutes));

  final threads = [
    thread(1, lastActivityAt: at(5), createdAt: at(1)),
    thread(2, lastActivityAt: at(9), createdAt: at(2)),
    thread(
      3,
      state: ThreadState.closed,
      lastActivityAt: at(7),
      createdAt: at(3),
    ),
    thread(4, lastActivityAt: at(1), createdAt: at(4)),
    thread(
      5,
      state: ThreadState.closed,
      lastActivityAt: at(8),
      createdAt: at(0),
    ),
  ];
  List<String> titles(List<Thread> list) => [for (final t in list) t.title];

  test('open threads by last activity, closed ones in the Done list', () {
    final r = ForumOrder.sort(threads, ForumSort.lastActivity);
    expect(titles(r.open), ['Thread 2', 'Thread 1', 'Thread 4']);
    expect(titles(r.done), ['Thread 5', 'Thread 3']);
  });

  test('by creation', () {
    final r = ForumOrder.sort(threads, ForumSort.created);
    expect(titles(r.open), ['Thread 4', 'Thread 2', 'Thread 1']);
    expect(titles(r.done), ['Thread 3', 'Thread 5']);
  });

  test('equal times sort by ID descending; unknown sorts by activity', () {
    final r = ForumOrder.sort([thread(1), thread(2)], ForumSort.unknown);
    expect(titles(r.open), ['Thread 2', 'Thread 1']);
    expect(r.done, isEmpty);
  });
}
