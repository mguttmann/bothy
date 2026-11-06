// SPDX-License-Identifier: Apache-2.0

// The decisions of the walk through Photos to the share sheet of the newest photo (brief M5-06
// Tests), apart from the native automation that carries them out (share_flow.dart), so a unit
// test can hold them without a simulator (test/share/photos_walk_test.dart): what a round reads
// from the tree Photos shows (readPhotosView) and which steps follow (PhotosWalk).

import 'dart:ui' show Offset, Rect;

/// One element of the accessibility tree Photos shows: its type (the name of patrol's
/// `IOSElementType`), identifier, label, frame in points and children.
class PhotosElement {
  const PhotosElement({
    this.type = 'other',
    this.identifier = '',
    this.label = '',
    required this.frame,
    this.children = const [],
  });

  final String type;
  final String identifier;
  final String label;
  final Rect frame;
  final List<PhotosElement> children;
}

/// The identifier of the scroll view of Photos' viewer of one photo (the "one-up" view).
const String photosViewer = 'OneUpMainPagingView';

/// What [root], the tree of Photos taken at once (its root is the application, whose frame is
/// the screen), shows to the walk; the viewer counts while it lies wholly on screen. One snapshot per round: a query per element costs Photos up
/// to 25 s for its grid on the hosted simulator, a snapshot of the whole tree a fraction of a
/// second (V1-08). The newest photo is the last cell of the grid (`PXGGridLayout-Info`), or else
/// the last image labelled as a photo; the photo of the viewer is not a cell of the grid.
PhotosView readPhotosView(PhotosElement root, {bool answeredPrompt = false}) {
  final screen = root.frame;
  final all = <PhotosElement>[];
  final grid = <PhotosElement>[];
  void walk(PhotosElement e, bool inViewer) {
    all.add(e);
    final viewer = inViewer || e.identifier == photosViewer;
    if (!viewer) grid.add(e);
    for (final c in e.children) {
      walk(c, viewer);
    }
  }

  walk(root, false);
  bool has(String identifier) => all.any((e) => e.identifier == identifier);

  Offset? share;
  for (final e in all) {
    if (e.label != 'Share') continue;
    share = _centerOnScreen(e.frame, screen);
    if (share != null) break;
  }

  PhotosElement? newest;
  for (final e in grid) {
    if (e.identifier == 'PXGGridLayout-Info') newest = e;
  }
  if (newest == null) {
    for (final e in grid) {
      if (e.type == 'image' && e.label.startsWith('Photo,')) newest = e;
    }
  }

  return PhotosView(
    answeredPrompt: answeredPrompt,
    share: share,
    viewer: all.any(
      (e) =>
          e.identifier == photosViewer &&
          _centerOnScreen(e.frame, screen) != null,
    ),
    newest: newest == null ? null : _centerOnScreen(newest.frame, screen),
    newestExists: newest != null,
    headerClose: has('header.closeButton'),
    libraryTab: has('LibraryTab'),
    libraryAbove: has('all_photos_grid'),
  );
}

/// The center of [frame] as fractions of [screen] when it lies wholly on screen, else null.
Offset? _centerOnScreen(Rect frame, Rect screen) {
  final inside =
      screen.right > 0 &&
      screen.bottom > 0 &&
      frame.left >= 0 &&
      frame.top >= 0 &&
      frame.right <= screen.right &&
      frame.bottom <= screen.bottom &&
      frame.right > frame.left &&
      frame.bottom > frame.top;
  if (!inside) return null;
  return Offset(
    frame.center.dx / screen.right,
    frame.center.dy / screen.bottom,
  );
}

/// What one round of the walk saw in Photos; centers are fractions of the screen and only set
/// for elements wholly on screen.
class PhotosView {
  const PhotosView({
    this.answeredPrompt = false,
    this.share,
    this.viewer = false,
    this.newest,
    this.newestExists = false,
    this.headerClose = false,
    this.libraryTab = false,
    this.libraryAbove = false,
  });

  /// The round answered the welcome sheet of Photos or the notification prompt.
  final bool answeredPrompt;

  /// The Share button of the photo viewer, when it lies wholly on screen.
  final Offset? share;

  /// Whether Photos shows its viewer of one photo ([photosViewer]). Its Share button is on
  /// screen only while the viewer shows its bars; a tap on the photo shows or hides them.
  final bool viewer;

  /// The newest photo of the library grid.
  final Offset? newest;

  /// Whether Photos shows the newest photo at all, on screen or off.
  final bool newestExists;

  /// The close button of a header (`header.closeButton`, a tip at the first start).
  final bool headerClose;

  /// The Library tab of the versions with a tab bar (`LibraryTab`).
  final bool libraryTab;

  /// The "Library" element `all_photos_grid` of iOS 18, which Photos shows when it opened
  /// scrolled down to the collections below the grid.
  final bool libraryAbove;
}

/// One step of the walk.
enum PhotosStep {
  /// Tap the Share button; the walk is done.
  tapShare,

  /// Tap the newest photo.
  tapPhoto,

  /// Tap the middle of the viewer, which shows its bars with the Share button (or hides them;
  /// the next tap shows them again).
  showBars,

  /// Swipe up through the grid, towards the newest photo at its end.
  swipeUp,

  /// Tap the header's close button (repeated while the walk took no other step, not counted).
  closeHeader,

  /// Tap the Library tab.
  libraryTab,

  /// Swipe down towards the grid above the collections.
  swipeDown,

  /// Leave Photos for the home screen and open it again.
  reopen,
}

/// The walk's state over its rounds. Photos draws at its own pace, so a step repeats in every
/// round in which its view still shows, and a photo is tapped only when two rounds in a row saw
/// it at the same place (not while the grid still scrolls, and not in the round that answered a
/// prompt, whose sheet may have covered the grid).
///
/// The viewer of a photo comes before the grid: Photos may keep the grid in the tree under its
/// viewer, and its viewer may open without its bars, so without the Share button on screen
/// (PR run 42178688293: the walk saw the grid's cells under the viewer, waited two rounds of
/// 20 s each and tapped the photo again, which showed the bars, only at its bound). A viewer
/// without Share on screen in two rounds in a row gets a tap in its middle ([PhotosStep.showBars]);
/// it is never idle and never swiped (a swipe down closes it).
///
/// A walk never stands still: a round in which Photos shows none of the walk's views is idle,
/// and after [idleRounds] idle rounds in a row the walk acts by itself, by turns a swipe up
/// through the grid and a reopen of Photos. On the hosted simulator Photos once kept its grid
/// out of the accessibility tree after its welcome sheet closed (`Ignoring failure to get
/// hierarchy for remote element ... kAXErrorServerNotFound` from then on, main run 40671022325):
/// the walk then saw the grid's frame but no photo and only looked until its bound ended. A
/// scroll makes the grid lay out its cells anew and a reopen brings Photos back to the
/// foreground; whether either restores the tree on the hosted simulator is not proven, but the
/// walk no longer waits out its bound without trying, and a failure names what it tried.
class PhotosWalk {
  PhotosWalk({this.idleRounds = 5});

  /// How many idle rounds in a row make the walk act by itself.
  final int idleRounds;

  /// How often each step was taken, by name, for the message of a failure.
  final Map<String, int> steps = {};

  Offset? _seen;
  int _idle = 0;
  int _bare = 0;
  bool _reopenNext = false;

  /// The steps of the round that saw [view], in order.
  List<PhotosStep> next(PhotosView view) {
    if (view.share != null) return _take([PhotosStep.tapShare]);
    if (view.viewer) {
      _seen = null;
      _idle = 0;
      _bare++;
      if (_bare < 2) return const [];
      _bare = 0;
      return _take([PhotosStep.showBars]);
    }
    _bare = 0;
    final photo = view.newest;
    if (photo != null) {
      _idle = 0;
      if (view.answeredPrompt) {
        _seen = null;
        return const [];
      }
      if (photo == _seen) {
        _seen = null;
        return _take([PhotosStep.tapPhoto]);
      }
      _seen = photo;
      return const [];
    }
    _seen = null;
    if (view.newestExists) return _take([PhotosStep.swipeUp]);
    final out = [
      if (steps.isEmpty && view.headerClose) PhotosStep.closeHeader,
      if (view.libraryTab) PhotosStep.libraryTab,
      if (view.libraryAbove) PhotosStep.swipeDown,
    ];
    if (out.isNotEmpty) return _take(out);
    if (view.answeredPrompt) {
      _idle = 0;
      return const [];
    }
    _idle++;
    if (_idle < idleRounds) return const [];
    final step = _reopenNext ? PhotosStep.reopen : PhotosStep.swipeUp;
    _reopenNext = !_reopenNext;
    return _take([step]);
  }

  /// The steps taken so far, for the message of a failure.
  String get described =>
      [for (final e in steps.entries) '${e.key} x${e.value}'].join(', ');

  List<PhotosStep> _take(List<PhotosStep> out) {
    _idle = 0;
    for (final s in out) {
      final name = _names[s];
      if (name != null) steps[name] = (steps[name] ?? 0) + 1;
    }
    return out;
  }

  static const Map<PhotosStep, String> _names = {
    PhotosStep.tapPhoto: 'photo',
    PhotosStep.showBars: 'bars',
    PhotosStep.swipeUp: 'swipe up',
    PhotosStep.libraryTab: 'LibraryTab',
    PhotosStep.swipeDown: 'swipe down',
    PhotosStep.reopen: 'reopen',
  };
}
