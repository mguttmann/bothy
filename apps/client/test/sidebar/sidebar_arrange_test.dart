// SPDX-License-Identifier: Apache-2.0

// The pure rules of the sidebar ordering (brief M4-51; design note sidebar-ordering.md
// D-M4-51-3, D-M4-51-4, D-M4-51-9): a personal list with the rows not in it first, the order by
// activity with ties kept, the held order, a move among the shown rows of a longer list, and the
// activity of a channel and a space.

import 'dart:math';

import 'package:client/sidebar/sidebar_arrange.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';

void main() {
  group('applyPersonalOrder', () {
    test('rows in the list follow it; rows not in it come first in the '
        'default order', () {
      final out = applyPersonalOrder(
        ['a', 'b', 'c', 'd'],
        ['c', 'a'],
        (s) => s,
      );
      expect(out, ['b', 'd', 'c', 'a']);
    });

    test('null or empty keeps the default order; unknown IDs are ignored', () {
      expect(applyPersonalOrder(['a', 'b'], null, (s) => s), ['a', 'b']);
      expect(applyPersonalOrder(['a', 'b'], const [], (s) => s), ['a', 'b']);
      expect(applyPersonalOrder(['a', 'b'], ['x', 'b', 'a'], (s) => s), [
        'b',
        'a',
      ]);
    });
  });

  group('orderByActivity', () {
    test('newest first; equal times keep the given order', () {
      final at = {
        'a': t0,
        'b': t0.add(const Duration(minutes: 5)),
        'c': t0,
        'd': t0.add(const Duration(minutes: 1)),
      };
      expect(orderByActivity(['a', 'b', 'c', 'd'], (s) => at[s]!), [
        'b',
        'd',
        'a',
        'c',
      ]);
    });

    test('property: a permutation, sorted newest first, stable', () {
      final random = Random(51);
      for (var round = 0; round < 200; round++) {
        final n = random.nextInt(12);
        final items = List.generate(n, (i) => i);
        final at = {
          for (final i in items)
            i: t0.add(Duration(minutes: random.nextInt(4))),
        };
        final out = orderByActivity(items, (i) => at[i]!);
        expect(out.toSet(), items.toSet());
        for (var i = 1; i < out.length; i++) {
          final cmp = at[out[i - 1]]!.compareTo(at[out[i]]!);
          expect(cmp >= 0, isTrue);
          if (cmp == 0) expect(out[i - 1] < out[i], isTrue);
        }
      }
    });
  });

  group('holdOrder', () {
    test(
      'held rows keep their held order; rows new to the level come first',
      () {
        expect(holdOrder(['c', 'a', 'x', 'b'], ['a', 'b', 'c'], (s) => s), [
          'x',
          'a',
          'b',
          'c',
        ]);
      },
    );

    test('without a held order the fresh order applies', () {
      expect(holdOrder(['c', 'a'], const [], (s) => s), ['c', 'a']);
    });
  });

  group('moveAmong', () {
    test('a move within a full list', () {
      expect(moveAmong(['a', 'b', 'c'], ['a', 'b', 'c'], 'a', 2), [
        'b',
        'c',
        'a',
      ]);
      expect(moveAmong(['a', 'b', 'c'], ['a', 'b', 'c'], 'c', 0), [
        'c',
        'a',
        'b',
      ]);
      expect(moveAmong(['a', 'b', 'c'], ['a', 'b', 'c'], 'b', 1), [
        'a',
        'b',
        'c',
      ]);
    });

    test('rows not shown keep their place relative to the shown ones '
        '(D-M4-51-9)', () {
      // Shown: a, b, c; behind "Show more": d, e.
      final full = ['a', 'b', 'c', 'd', 'e'];
      expect(moveAmong(full, ['a', 'b', 'c'], 'a', 2), [
        'b',
        'c',
        'a',
        'd',
        'e',
      ]);
      expect(moveAmong(full, ['a', 'b', 'c'], 'c', 0), [
        'c',
        'a',
        'b',
        'd',
        'e',
      ]);
    });

    test('rows on screen that are not in the order (a selected done post, a '
        'held row) are skipped (review of M4-51)', () {
      expect(moveAmong(['a', 'b'], ['a', 'x', 'b'], 'b', 0), ['b', 'a']);
      expect(moveAmong(['a', 'b'], ['a', 'x', 'b'], 'b', 1), ['a', 'b']);
      expect(moveAmong(['a', 'b'], ['a', 'b', 'k'], 'a', 2), ['b', 'a']);
      expect(moveAmong(['a'], ['x', 'a'], 'a', 0), ['a']);
    });

    test('an index beyond the ends is clamped; an unknown row changes '
        'nothing', () {
      expect(moveAmong(['a', 'b'], ['a', 'b'], 'a', 9), ['b', 'a']);
      expect(moveAmong(['a', 'b'], ['a', 'b'], 'b', -3), ['b', 'a']);
      expect(moveAmong(['a', 'b'], ['a', 'b'], 'x', 0), ['a', 'b']);
    });

    test('property: a move keeps every row once and puts the row at its '
        'place among the shown rows', () {
      final random = Random(7);
      for (var round = 0; round < 300; round++) {
        final n = 1 + random.nextInt(10);
        final full = List.generate(n, (i) => i)..shuffle(random);
        final shownCount = 1 + random.nextInt(n);
        final shown = full.take(shownCount).toList();
        final item = shown[random.nextInt(shown.length)];
        final to = random.nextInt(shown.length);
        final out = moveAmong(full, shown, item, to);
        expect(out.length, n);
        expect(out.toSet(), full.toSet());
        final newShown = [
          for (final i in out)
            if (shown.contains(i)) i,
        ];
        expect(newShown.indexOf(item), to);
      }
    });
  });

  group('activity', () {
    test(
      'a channel: the newest activity of its threads, else its creation',
      () {
        final c = channel(1, kind: ChannelKind.forum);
        expect(channelActivity(c, const []), c.createdAt);
        expect(
          channelActivity(c, [
            thread(1, activeMinutes: 4),
            thread(2, activeMinutes: 9),
            thread(3, channelN: 2, activeMinutes: 99),
          ]),
          t0.add(const Duration(minutes: 9)),
        );
      },
    );

    test('a space: the newest of its channels, else its creation', () {
      final s = space(1);
      expect(spaceActivity(s, const []), s.createdAt);
      expect(
        spaceActivity(s, [t0, t0.add(const Duration(hours: 2))]),
        t0.add(const Duration(hours: 2)),
      );
    });
  });

  test(
    'SidebarArrangement: the defaults of 01 11 (chats on, the rest off)',
    () {
      const a = SidebarArrangement();
      expect(a.isAuto(SidebarLevel.spaces), isFalse);
      expect(a.isAuto(SidebarLevel.channels), isFalse);
      expect(a.isAuto(SidebarLevel.chats), isTrue);
    },
  );
}
