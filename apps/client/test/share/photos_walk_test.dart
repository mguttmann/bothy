// SPDX-License-Identifier: Apache-2.0

// The decisions of the walk through Photos in the iOS share device tests
// (integration_test/share/support/photos_walk.dart), without a simulator.

import 'dart:ui' show Rect;

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

  // PR run 42178688293 (jobs 127763141712 and 127782084893) and main run 42044761383: the walk
  // tapped the newest photo, Photos showed its viewer without the Share button on screen, and the
  // walk, which saw the grid cells under the viewer, waited for the photo again and tapped it a
  // second time only shortly before its bound (V1-08).
  test('the viewer without Share on screen is tapped to show its bars', () {
    final walk = PhotosWalk();
    expect(walk.next(const PhotosView(viewer: true)), isEmpty);
    expect(walk.next(const PhotosView(viewer: true)), [PhotosStep.showBars]);
    expect(walk.next(const PhotosView(viewer: true)), isEmpty);
    expect(walk.next(const PhotosView(viewer: true)), [PhotosStep.showBars]);
    expect(walk.next(const PhotosView(share: Offset(0.1, 0.9))), [
      PhotosStep.tapShare,
    ]);
    expect(walk.described, 'bars x2');
  });

  test('the viewer is not idle and is never swiped', () {
    final walk = PhotosWalk(idleRounds: 2);
    final taken = [
      for (var round = 0; round < 6; round++)
        ...walk.next(const PhotosView(viewer: true, libraryAbove: true)),
    ];
    expect(taken, everyElement(PhotosStep.showBars));
  });

  test('an answered prompt makes the photo wait for two more rounds', () {
    final walk = PhotosWalk();
    expect(walk.next(const PhotosView(newest: Offset(0.9, 0.85))), isEmpty);
    expect(
      walk.next(
        const PhotosView(newest: Offset(0.9, 0.85), answeredPrompt: true),
      ),
      isEmpty,
    );
    expect(walk.next(const PhotosView(newest: Offset(0.9, 0.85))), isEmpty);
    expect(walk.next(const PhotosView(newest: Offset(0.9, 0.85))), [
      PhotosStep.tapPhoto,
    ]);
  });

  group('readPhotosView', () {
    const screen = Rect.fromLTRB(0, 0, 402, 874);

    PhotosElement app(List<PhotosElement> children) => PhotosElement(
      type: 'application',
      label: 'Photos',
      frame: screen,
      children: children,
    );

    PhotosElement cell(String label, Rect frame) => PhotosElement(
      type: 'image',
      identifier: 'PXGGridLayout-Info',
      label: label,
      frame: frame,
    );

    // The grid under the viewer, as Photos keeps it in the tree while its viewer shows.
    final grid = PhotosElement(
      type: 'other',
      identifier: 'PXGGridLayout-Group',
      frame: screen,
      children: [
        cell(
          'Photo, October 09, 2009, 9:09 PM',
          const Rect.fromLTRB(242, 620, 321, 699),
        ),
        cell(
          'Photo, November 06, 7:09 AM',
          const Rect.fromLTRB(323, 701, 402, 780),
        ),
      ],
    );

    PhotosElement viewer({required bool bars}) => PhotosElement(
      type: 'scrollView',
      identifier: 'OneUpMainPagingView',
      frame: screen,
      children: [
        const PhotosElement(
          type: 'image',
          label: 'Photo, November 06, 7:09 AM',
          frame: Rect.fromLTRB(0, 300, 402, 600),
        ),
        const PhotosElement(
          type: 'button',
          label: 'Back',
          frame: Rect.fromLTRB(16, 62, 60, 106),
        ),
        const PhotosElement(
          type: 'button',
          label: 'More',
          frame: Rect.fromLTRB(342, 62, 386, 106),
        ),
        PhotosElement(
          type: 'button',
          label: 'Share',
          frame: bars
              ? const Rect.fromLTRB(32, 800, 78, 846)
              : const Rect.fromLTRB(32, 900, 78, 946),
          children: [
            PhotosElement(
              type: 'image',
              identifier: 'square.and.arrow.up',
              label: 'Share',
              frame: bars
                  ? const Rect.fromLTRB(40, 808, 70, 838)
                  : const Rect.fromLTRB(40, 908, 70, 938),
            ),
          ],
        ),
      ],
    );

    test('the viewer with its bars shows the Share button on screen', () {
      final view = readPhotosView(app([grid, viewer(bars: true)]));
      expect(view.share, const Offset(55 / 402, 823 / 874));
      expect(view.viewer, isTrue);
    });

    test('the viewer without its bars has no Share on screen', () {
      final view = readPhotosView(app([grid, viewer(bars: false)]));
      expect(view.share, isNull);
      expect(view.viewer, isTrue);
      // The grid stays in the tree under the viewer; the walk reads the viewer first.
      expect(view.newest, const Offset(362.5 / 402, 740.5 / 874));
    });

    test('the photo of a viewer without a Share button is no newest photo', () {
      const bare = PhotosElement(
        type: 'scrollView',
        identifier: 'OneUpMainPagingView',
        frame: screen,
        children: [
          PhotosElement(
            type: 'image',
            label: 'Photo, November 06, 7:09 AM',
            frame: Rect.fromLTRB(0, 300, 402, 600),
          ),
        ],
      );
      final view = readPhotosView(app(const [bare]));
      expect(view.viewer, isTrue);
      expect(view.share, isNull);
      expect(view.newestExists, isFalse);
    });

    test('a viewer off screen is not the viewer', () {
      final view = readPhotosView(
        app([
          grid,
          const PhotosElement(
            type: 'scrollView',
            identifier: 'OneUpMainPagingView',
            frame: Rect.fromLTRB(402, 0, 804, 874),
          ),
        ]),
      );
      expect(view.viewer, isFalse);
      expect(view.newest, const Offset(362.5 / 402, 740.5 / 874));
    });

    test('the grid gives its last cell as the newest photo', () {
      final view = readPhotosView(app([grid]));
      expect(view.viewer, isFalse);
      expect(view.share, isNull);
      expect(view.newest, const Offset(362.5 / 402, 740.5 / 874));
      expect(view.newestExists, isTrue);
    });

    test('a newest photo off screen exists without a center', () {
      final view = readPhotosView(
        app([
          PhotosElement(
            frame: screen,
            children: [
              cell(
                'Photo, November 06, 7:09 AM',
                const Rect.fromLTRB(323, 937, 402, 1016),
              ),
            ],
          ),
        ]),
      );
      expect(view.newest, isNull);
      expect(view.newestExists, isTrue);
    });

    test('without grid cells the last image labelled as a photo counts', () {
      final view = readPhotosView(
        app([
          const PhotosElement(
            type: 'image',
            label: 'Photo, March 13, 2011, 12:17 AM',
            frame: Rect.fromLTRB(0, 100, 100, 200),
          ),
          const PhotosElement(
            type: 'image',
            label: 'Photo, November 06, 7:09 AM',
            frame: Rect.fromLTRB(100, 100, 200, 200),
          ),
        ]),
      );
      expect(view.newest, const Offset(150 / 402, 150 / 874));
    });

    test('the library views are read by their identifiers', () {
      final view = readPhotosView(
        app(const [
          PhotosElement(
            type: 'button',
            identifier: 'header.closeButton',
            frame: Rect.fromLTRB(350, 60, 390, 100),
          ),
          PhotosElement(
            type: 'button',
            identifier: 'LibraryTab',
            frame: Rect.fromLTRB(0, 800, 100, 874),
          ),
          PhotosElement(
            type: 'button',
            identifier: 'all_photos_grid',
            label: 'Library',
            frame: Rect.fromLTRB(0, 0, 402, 586),
          ),
        ]),
        answeredPrompt: true,
      );
      expect(view.headerClose, isTrue);
      expect(view.libraryTab, isTrue);
      expect(view.libraryAbove, isTrue);
      expect(view.answeredPrompt, isTrue);
      expect(view.viewer, isFalse);
    });

    test('a tree without a frame shows nothing', () {
      final view = readPhotosView(const PhotosElement(frame: Rect.zero));
      expect(view.viewer, isFalse);
      expect(view.share, isNull);
      expect(view.newest, isNull);
    });
  });
}
