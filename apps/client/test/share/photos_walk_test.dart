// SPDX-License-Identifier: Apache-2.0

// The decisions of the walk through Photos in the iOS share device tests
// (integration_test/share/support/photos_walk.dart), without a simulator.

import 'package:flutter_test/flutter_test.dart';

import '../../integration_test/share/support/photos_walk.dart';

void main() {
  test('the walk acts after five idle rounds (PL/58)', () {
    expect(PhotosWalk().idleRounds, 5);
  });

  test('a photo is tapped when two rounds see it at the same place', () {
    final walk = PhotosWalk();
    expect(walk.next(const PhotosView(newest: Offset(0.9, 0.8))), isEmpty);
    expect(walk.next(const PhotosView(newest: Offset(0.9, 0.7))), isEmpty);
    expect(walk.next(const PhotosView(newest: Offset(0.9, 0.7))), [
      PhotosStep.tapPhoto,
    ]);
    expect(walk.next(const PhotosView(share: Offset(0.1, 0.9))), [
      PhotosStep.tapShare,
    ]);
    expect(walk.described, 'photo x1');
  });

  test(
    'the walk scrolls to a newest photo off screen and down to the grid',
    () {
      final walk = PhotosWalk();
      expect(
        walk.next(const PhotosView(libraryAbove: true, headerClose: true)),
        [PhotosStep.closeHeader, PhotosStep.swipeDown],
      );
      expect(walk.next(const PhotosView(newestExists: true)), [
        PhotosStep.swipeUp,
      ]);
      expect(walk.next(const PhotosView(headerClose: true, libraryTab: true)), [
        PhotosStep.libraryTab,
      ]);
      expect(walk.described, 'swipe down x1, swipe up x1, LibraryTab x1');
    },
  );

  // Main run 40671022325: after the welcome sheet Photos showed the grid's frame without any
  // photo in the accessibility tree, and the walk only looked for 140 s until its bound.
  test(
    'a walk that sees none of its views acts by turns after the idle rounds',
    () {
      final walk = PhotosWalk(idleRounds: 3);
      final taken = <PhotosStep>[];
      for (var round = 0; round < 12; round++) {
        taken.addAll(walk.next(const PhotosView()));
      }
      expect(taken, [
        PhotosStep.swipeUp,
        PhotosStep.reopen,
        PhotosStep.swipeUp,
        PhotosStep.reopen,
      ]);
      expect(walk.described, 'swipe up x2, reopen x2');
    },
  );

  test(
    'an answered prompt and a step of the walk start the idle rounds anew',
    () {
      final walk = PhotosWalk(idleRounds: 3);
      final rounds = [
        const PhotosView(),
        const PhotosView(),
        const PhotosView(answeredPrompt: true),
        const PhotosView(),
        const PhotosView(),
        const PhotosView(newestExists: true),
        const PhotosView(),
        const PhotosView(),
      ];
      final taken = [for (final v in rounds) ...walk.next(v)];
      expect(taken, [PhotosStep.swipeUp]);
      expect(walk.next(const PhotosView()), [PhotosStep.swipeUp]);
    },
  );

  test('a photo on screen is not idle', () {
    final walk = PhotosWalk(idleRounds: 2);
    for (var round = 0; round < 6; round++) {
      final x = 0.1 * round;
      expect(walk.next(PhotosView(newest: Offset(x, 0.5))), isEmpty);
    }
  });
}
