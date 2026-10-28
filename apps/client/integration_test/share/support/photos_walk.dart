// SPDX-License-Identifier: Apache-2.0

// The decisions of the walk through Photos to the share sheet of the newest photo (brief M5-06
// Tests), apart from the native automation that carries them out (share_flow.dart), so a unit
// test can hold them without a simulator (test/share/photos_walk_test.dart).

import 'dart:ui' show Offset;

/// What one round of the walk saw in Photos; centers are fractions of the screen and only set
/// for elements wholly on screen.
class PhotosView {
  const PhotosView({
    this.answeredPrompt = false,
    this.share,
    this.newest,
    this.newestExists = false,
    this.headerClose = false,
    this.libraryTab = false,
    this.libraryAbove = false,
  });

  /// The round answered the welcome sheet of Photos or the notification prompt.
  final bool answeredPrompt;

  /// The Share button of the photo viewer.
  final Offset? share;

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
/// it at the same place (not while the grid still scrolls).
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
  bool _reopenNext = false;

  /// The steps of the round that saw [view], in order.
  List<PhotosStep> next(PhotosView view) {
    if (view.share != null) return _take([PhotosStep.tapShare]);
    final photo = view.newest;
    if (photo != null) {
      _idle = 0;
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
    PhotosStep.swipeUp: 'swipe up',
    PhotosStep.libraryTab: 'LibraryTab',
    PhotosStep.swipeDown: 'swipe down',
    PhotosStep.reopen: 'reopen',
  };
}
