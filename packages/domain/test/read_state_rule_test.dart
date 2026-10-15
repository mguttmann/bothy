// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('a normal update never decreases the read position', () {
    final merged = mergeReadState(
      readState(50, version: 3),
      readState(40, version: 4, mentions: 2),
    );
    expect(merged.lastReadSeq, const ThreadSeq(50));
    expect(merged.version, const ResourceVersion(4));
    expect(merged.mentionCount, 2);
    expect(merged.unreadCount, isNull);
  });

  test(
    'a normal update raises the read position and takes the server counters',
    () {
      final merged = mergeReadState(
        readState(40, version: 3),
        readState(50, version: 4, mentions: 1, unread: 0),
      );
      expect(merged.lastReadSeq, const ThreadSeq(50));
      expect(merged.mentionCount, 1);
      expect(merged.unreadCount, 0);
    },
  );

  test('an older update changes nothing, also after a reset', () {
    final reset = mergeReadState(
      readState(50, version: 4),
      readState(20, version: 5, unread: 7, mentions: 1),
      reset: true,
    );
    expect(mergeReadState(reset, readState(50, version: 4)), reset);
  });

  test('an older update keeps the local state', () {
    final local = readState(40, version: 5, mentions: 3);
    expect(mergeReadState(local, readState(45, version: 4)), local);
  });

  test('an explicit reset with a newer version lowers it', () {
    final merged = mergeReadState(
      readState(50, version: 3),
      readState(20, version: 4),
      reset: true,
    );
    expect(merged.lastReadSeq, const ThreadSeq(20));
    expect(merged.version, const ResourceVersion(4));
  });

  test('a stale reset changes nothing', () {
    final local = readState(50, version: 4);
    expect(
      mergeReadState(local, readState(20, version: 4), reset: true),
      local,
    );
  });
}
