// SPDX-License-Identifier: Apache-2.0

// The shared steps of the share device tests (brief M5-06 Tests): the app with the platform ports
// of sharing (on iOS the Pigeon glue of the Share Extension and the platform's device keys, which
// the extension signs with; on Android the share receiver of the runner) against the fake server
// of M4-17, and the walk through the system apps that share a file into it: Photos on iOS, the
// file app (DocumentsUI) on Android. The device tests run through `patrol test` (README "Device
// tests of sharing"); the wrappers prepare the sample files first.

import 'dart:io';

import 'package:client/platform/device_key_signer.dart';
import 'package:client/share/share_android.dart';
import 'package:client/share/share_apple.dart';
import 'package:device_key/device_key.dart' as dk;
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import '../../../test/support/until.dart' show testPatience;
import '../../phone/support/phone_flow.dart';
import 'photos_walk.dart';

export '../../phone/support/phone_flow.dart';

/// The bundle identifier of Photos.
const String photosApp = 'com.apple.mobileslideshow';

/// The name of the sample file the wrappers put into Photos (iOS) and into the downloads
/// (Android).
const String sampleImage = 'share-sample.jpg';

/// The text sample of the Android wrapper (`text/plain`).
const String sampleText = 'share-sample.txt';

/// The PDF sample of the Android wrapper (`application/pdf`).
const String samplePdf = 'share-sample.pdf';

/// A sample whose type the app does not list (`application/octet-stream`).
const String sampleUnsupported = 'share-sample.bin';

/// The app's name in the system share sheet (the neutral branding).
const String appName = 'ExampleChat';

/// How long a native step may wait for its view: the [testPatience], because the system apps
/// answer slowly on a loaded machine; a view that never comes fails the step after it.
const Duration nativeWait = testPatience;

/// Starts the phone app with the platform ports of sharing.
Future<TestApp> startShareApp(WidgetTester tester) {
  if (Platform.isIOS) {
    return TestApp.start(
      tester,
      platform: 'ios',
      appleShare: AppleShare(host: ShareAppleHostApi()),
      deviceKeys: PlatformDeviceKeyStore(
        dk.DeviceKeyClient(),
        () => const dk.StepUpPrompt(title: 'Confirm', negativeButton: 'Cancel'),
      ),
    );
  }
  return TestApp.start(
    tester,
    platform: 'android',
    androidShare: AndroidShare(host: ShareAndroidHostApi()),
  );
}

/// A forum `server` with the thread `Gateway: resume ...` that Alex can write in, the target of
/// the tests (01 5.3 M6).
({String forum, String thread}) seedTarget(TestApp app) {
  final world = app.fake.server.world;
  final seed = app.fake.seed;
  final forum = world.addForum(seed.space, 'server', {seed.alex.id: 'write'});
  final thread = world.addThread(
    forum,
    'Gateway: resume after restart',
    createdBy: seed.alex.id,
  );
  return (forum: forum, thread: thread);
}

/// Waits until the app created the child session of the Share Extension (08 7.4).
Future<void> untilChildSession(TestApp app) => until(
  app.tester,
  'the child session of the Share Extension',
  () => app.fake.server.world.sessions.values.any(
    (s) => s.clientComponent == 'share_ext' && !s.revoked,
  ),
);

/// The messages of [thread] that carry an attachment.
List<Map<String, Object?>> messagesWithFiles(TestApp app, String thread) => [
  for (final m in app.fake.server.world.messagesOf(thread))
    if ((m['attachments'] as List?)?.isNotEmpty ?? false) m,
];

/// The native automation of the share tests: every native step waits up to [nativeWait], so the
/// connection to the automation server must outlast it (patrol's default of 60 s would end a
/// longer wait as a timeout of the connection).
final PlatformAutomatorConfig shareAutomatorConfig =
    PlatformAutomatorConfig.fromOptions(
      findTimeout: nativeWait,
      connectionTimeout: nativeWait + const Duration(minutes: 1),
    );

/// Opens Photos at the library and the share sheet of the newest photo (the sample the
/// wrapper added), then the app's Share Extension. The first start of Photos shows a welcome
/// sheet and a notification prompt; both are answered when they appear. Photos differs between
/// iOS versions (a tab bar in some, one scrolling library in others), so the newest photo is
/// found by the grid's identifier, after the Library tab where there is one, or by its label;
/// a failing step names what Photos showed.
Future<void> shareNewestPhoto(PatrolIntegrationTester $) async {
  try {
    await _shareNewestPhoto($);
  } on Object catch (e) {
    var shown = '(no view tree)';
    try {
      final tree = await $.platform.ios.getNativeViews(
        null,
        iosInstalledApps: [photosApp],
      );
      shown = _describe(tree.roots);
    } on Object {
      // The tree is only a diagnosis.
    }
    fail('$e\nPhotos showed:\n$shown');
  }
}

Future<void> _shareNewestPhoto(PatrolIntegrationTester $) async {
  final ios = $.platform.ios;
  await $.platform.mobile.pressHome();
  await ios.openPlatformApp(iosAppId: AppleApp.photos);
  // Photos draws at its own pace: the welcome sheet and the notification prompt may appear at
  // any point of the walk, so each round answers whichever is shown. Photos opens where it was
  // left: in the viewer of the sample after an earlier share of this test (its Share button is
  // on screen), or in the library, maybe without photos on screen: a Library tab in some
  // versions; on iOS 18 one scrolling view (with the "Library" element all_photos_grid) that
  // opens scrolled down to the collections, below the grid, so the walk swipes it down towards
  // the grid.
  //
  // Each round tells PhotosWalk what it saw and carries out its steps (photos_walk.dart: when
  // to tap a photo, and that a walk that sees none of its views acts by itself). Photos and the
  // share button are tapped at the center of what is on screen (patrol taps an element it
  // cannot hit at its top-left corner, and a tap that waits for a missing element ends the
  // native automation). The rounds end at the share sheet or when nativeWait has passed; a
  // failure names the steps.
  final end = DateTime.now().add(nativeWait);
  final walk = PhotosWalk();
  var done = false;
  while (!done) {
    var answered = await _optionalTap(
      $,
      const IOSSelector(label: 'Continue'),
      photosApp,
    );
    if (await _optionalTap(
      $,
      const IOSSelector(label: 'Don’t Allow'),
      'com.apple.springboard',
    )) {
      answered = true;
    }
    final screen = await _screen(ios);
    if (screen != null) {
      final share = await _onScreen(
        ios,
        const IOSSelector(label: 'Share'),
        screen,
      );
      final newest = share == null
          ? await _newestPhoto(ios, screen)
          : (center: null, exists: false);
      final others = share == null && newest.center == null && !newest.exists;
      final view = PhotosView(
        answeredPrompt: answered,
        share: share,
        newest: newest.center,
        newestExists: newest.exists,
        headerClose:
            others &&
            await _shown(
              ios,
              const IOSSelector(identifier: 'header.closeButton'),
            ),
        libraryTab:
            others &&
            await _shown(ios, const IOSSelector(identifier: 'LibraryTab')),
        libraryAbove:
            others &&
            await _shown(ios, const IOSSelector(identifier: 'all_photos_grid')),
      );
      for (final step in walk.next(view)) {
        switch (step) {
          case PhotosStep.tapShare:
            await ios.tapAt(share!, appId: photosApp);
            done = true;
          case PhotosStep.tapPhoto:
            await ios.tapAt(newest.center!, appId: photosApp);
          case PhotosStep.swipeUp:
            await ios.swipe(
              from: const Offset(0.5, 0.7),
              to: const Offset(0.5, 0.3),
              appId: photosApp,
            );
          case PhotosStep.closeHeader:
            await _optionalTap(
              $,
              const IOSSelector(identifier: 'header.closeButton'),
              photosApp,
            );
          case PhotosStep.libraryTab:
            await _optionalTap(
              $,
              const IOSSelector(identifier: 'LibraryTab'),
              photosApp,
            );
          case PhotosStep.swipeDown:
            await ios.swipe(
              from: const Offset(0.5, 0.3),
              to: const Offset(0.5, 0.9),
              appId: photosApp,
            );
          case PhotosStep.reopen:
            await $.platform.mobile.pressHome();
            await ios.openPlatformApp(iosAppId: AppleApp.photos);
        }
      }
    }
    if (done) break;
    if (DateTime.now().isAfter(end)) {
      fail('Photos shows no Share button (steps: ${walk.described})');
    }
    await Future<void>.delayed(const Duration(milliseconds: 200));
  }
  await _tapAppInShareSheet($);
  await ios.waitUntilVisible(
    const IOSSelector(identifier: 'share.cancel'),
    appId: photosApp,
    timeout: nativeWait,
  );
}

/// Whether Photos shows [selector] now, on screen or off.
Future<bool> _shown(IOSAutomator ios, IOSSelector selector) async =>
    (await ios.getNativeViews(selector, appId: photosApp)).roots.isNotEmpty;

/// The identifiers and labels of [views], one line per view that has either, for a failure
/// message.
String _describe(List<IOSNativeView> views, [String indent = '']) {
  final out = StringBuffer();
  for (final v in views) {
    if (v.identifier.isNotEmpty || v.label.isNotEmpty) {
      out.writeln('$indent${v.elementType.name} ${v.identifier} "${v.label}"');
    }
    out.write(_describe(v.children, '$indent '));
  }
  return out.toString();
}

/// The label of the first target of the Share Extension's sheet.
Future<String> firstExtensionTarget(PatrolIntegrationTester $) async {
  final views = await $.platform.ios.getNativeViews(
    const IOSSelector(identifier: 'share.target.0'),
    appId: photosApp,
  );
  return views.roots.isEmpty ? '' : views.roots.first.label;
}

/// In the Share Extension's sheet: searches [query], picks the first hit and sends. The comment
/// stays empty: the sheet's multi-line comment field is a text view, which the native text entry
/// of patrol does not type into, and the message needs no text with an attachment.
Future<void> sendFromExtension(
  PatrolIntegrationTester $, {
  required String query,
}) async {
  final ios = $.platform.ios;
  await ios.enterText(
    const IOSSelector(identifier: 'share.search'),
    text: query,
    appId: photosApp,
    timeout: nativeWait,
  );
  await ios.waitUntilVisible(
    IOSSelector(identifier: 'share.target.0', labelContains: query),
    appId: photosApp,
    timeout: nativeWait,
  );
  await ios.tap(
    const IOSSelector(identifier: 'share.target.0'),
    appId: photosApp,
    timeout: nativeWait,
  );
  await ios.tap(
    const IOSSelector(identifier: 'share.send'),
    appId: photosApp,
    timeout: nativeWait,
  );
}

/// In the app's own share sheet (Android): searches [query], picks the first hit, comments and
/// sends through the outbox.
Future<void> sendFromAppSheet(
  TestApp app, {
  required String query,
  String comment = 'See error above',
}) async {
  await app.waitFor(find.byKey(const ValueKey('share.sheet')));
  await app.tester.enterText(find.byKey(const ValueKey('share.search')), query);
  await app.waitFor(
    find.descendant(
      of: find.byKey(const ValueKey('share.target.0')),
      matching: find.textContaining(query),
    ),
  );
  await app.tap('share.target.0');
  await app.tester.enterText(
    find.byKey(const ValueKey('share.comment')),
    comment,
  );
  await app.tap('share.send');
}

/// The package of the system's file app on the test images.
const List<String> fileApps = [
  'com.google.android.documentsui',
  'com.android.documentsui',
];

/// Opens the file app at the downloads (the wrapper put the samples there), selects
/// [fileName] by its type icon and opens the share chooser; returns the chooser's views. The
/// chooser fills its targets after it appears, so with [offered] the wait lasts until it lists
/// the app of this repository; without, it ends at the first chooser or other app shown.
Future<AndroidGetNativeViewsResponse> openAndroidChooser(
  PatrolIntegrationTester $,
  String fileName, {
  bool offered = false,
}) async {
  final android = $.platform.android;
  await $.platform.mobile.pressHome();
  String? package;
  for (final app in fileApps) {
    try {
      await android.openPlatformApp(androidAppId: app);
      package = app;
      break;
    } on Object {
      // The other image's file app.
    }
  }
  if (package == null) fail('no file app among $fileApps');
  await android.waitUntilVisible(
    AndroidSelector(text: fileName),
    timeout: nativeWait,
  );
  // DocumentsUI keeps the selection of an earlier share; it is cleared, so only [fileName] is
  // shared. Its own action mode has the close button (newer images), the framework's action
  // mode the platform's one (API 26).
  for (final owner in [package, 'android']) {
    final keep = AndroidSelector(
      resourceName: '$owner:id/action_mode_close_button',
    );
    if ((await android.getNativeViews(keep)).roots.isNotEmpty) {
      await android.tap(keep, timeout: nativeWait);
      break;
    }
  }
  // A long press on a file selects it in DocumentsUI (a tap opens it); patrol has no long
  // press, and a swipe that stays on its point for 150 steps of 5 ms is one. The file's name
  // is found in the shown tree, whose IDs differ between DocumentsUI's layouts and images.
  final tree = (await android.getNativeViews(null)).roots;
  Rectangle? title;
  var width = 0.0;
  var height = 0.0;
  void walk(List<AndroidNativeView> nodes) {
    for (final n in nodes) {
      final b = n.visibleBounds;
      if (b.maxX > width) width = b.maxX;
      if (b.maxY > height) height = b.maxY;
      if ((n.resourceName ?? '').endsWith(':id/title') && n.text == fileName) {
        title = b;
      }
      walk(n.children);
    }
  }

  walk(tree);
  final row = title;
  if (row == null) fail('$fileName is not listed');
  // The share action of the selection: `action_menu_share` in newer DocumentsUI, `menu_share`
  // in older ones (API 26); both carry the description "Share".
  final share = AndroidSelector(
    contentDescription: 'Share',
    applicationPackage: package,
  );
  final at = Offset(
    (row.minX + row.maxX) / 2 / width,
    (row.minY + row.maxY) / 2 / height,
  );
  await android.swipe(from: at, to: at, steps: 150);
  try {
    await android.waitUntilVisible(share, timeout: nativeWait);
  } on Object catch (e) {
    fail(
      'no selection after a long press at $at ($row of $width x $height): $e',
    );
  }
  await android.tap(share, timeout: nativeWait);
  // The chooser opens, or, when only one other app takes the type (API 26 for an unknown
  // type), the system starts that app directly: then the views are that app's, without the
  // app of this repository. With many apps (newer images) the chooser shows only its first
  // rows, so while it lacks the app of this repository the wait drags its list up by 40 % of
  // the screen in 150 steps (about 0.75 s), a drag rather than a fling, so no row passes unseen.
  final end = DateTime.now().add(nativeWait);
  while (true) {
    final views = await android.getNativeViews(null);
    if (offered
        ? chooserOffersApp(views)
        : _isChooser(views.roots) || _isOtherApp(views.roots, package)) {
      return views;
    }
    if (offered && _isChooser(views.roots)) {
      await android.swipe(
        from: const Offset(0.5, 0.85),
        to: const Offset(0.5, 0.45),
        steps: 150,
      );
    }
    if (DateTime.now().isAfter(end)) {
      final what = offered
          ? 'no share chooser with $appName'
          : 'no share chooser';
      fail('$what after sharing $fileName: ${chooserTexts(views)}');
    }
    await Future<void>.delayed(const Duration(milliseconds: 200));
  }
}

bool _isChooser(List<AndroidNativeView> nodes) => nodes.any(
  (n) => n.resourceName == 'android:id/text1' || _isChooser(n.children),
);

/// Whether the shown views are another app's: neither the file app's nor a chooser's (`android`
/// on API 26, `com.android.intentresolver` on newer images); the status bar's views
/// (`com.android.systemui`) do not count.
bool _isOtherApp(List<AndroidNativeView> roots, String fileApp) {
  final packages = <String>{};
  void walk(List<AndroidNativeView> nodes) {
    for (final n in nodes) {
      final p = n.applicationPackage;
      if (p != null && p.isNotEmpty) packages.add(p);
      walk(n.children);
    }
  }

  walk(roots);
  packages.remove('com.android.systemui');
  return packages.isNotEmpty &&
      !packages.contains(fileApp) &&
      !packages.contains('android') &&
      !packages.contains('com.android.intentresolver');
}

/// Whether the chooser in [views] offers the app.
bool chooserOffersApp(AndroidGetNativeViewsResponse views) {
  bool walk(List<AndroidNativeView> nodes) =>
      nodes.any((n) => n.text == appName || walk(n.children));
  return walk(views.roots);
}

/// The texts of the chooser's views, for the reason of a failed expectation.
String chooserTexts(AndroidGetNativeViewsResponse views) {
  final texts = <String>[];
  void walk(List<AndroidNativeView> nodes) {
    for (final n in nodes) {
      if ((n.text ?? '').isNotEmpty) texts.add(n.text!);
      walk(n.children);
    }
  }

  walk(views.roots);
  return texts.join(' | ');
}

/// Picks the app in the open share chooser; the app's share sheet opens.
Future<void> pickAppInChooser(PatrolIntegrationTester $) => $.platform.android
    .tap(const AndroidSelector(text: appName), timeout: nativeWait);

/// Taps the app of this repository in the system share sheet. The sheet's row of apps shows
/// only its first apps (on iOS 18 a newly installed app sits at its end, off screen, where a
/// tap on it never lands), so until the app shows on screen, the walk swipes that row to the
/// left at the height of a system app of the row, and taps the row's "More" (the list of all
/// apps) when it shows there. Every tap goes to the center of an element seen on screen, never
/// to an element of the photo viewer behind the sheet. Bounded by nativeWait; a failure names
/// the steps.
Future<void> _tapAppInShareSheet(PatrolIntegrationTester $) async {
  final ios = $.platform.ios;
  final end = DateTime.now().add(nativeWait);
  final steps = <String, int>{};
  while (true) {
    final size = await _screen(ios);
    if (size != null) {
      final app = await _onScreen(ios, const IOSSelector(label: appName), size);
      if (app != null) {
        await ios.tapAt(app, appId: photosApp);
        return;
      }
      final row = await _shareRowHeight(ios, size);
      final more = row == null
          ? null
          : await _onScreen(
              ios,
              const IOSSelector(label: 'More'),
              size,
              row: row,
            );
      if (more != null) {
        await ios.tapAt(more, appId: photosApp);
        steps['More'] = (steps['More'] ?? 0) + 1;
      } else if (row != null) {
        await ios.swipe(
          from: Offset(0.85, row),
          to: Offset(0.15, row),
          appId: photosApp,
        );
        steps['swipe row'] = (steps['swipe row'] ?? 0) + 1;
      }
    }
    if (DateTime.now().isAfter(end)) {
      final named = [for (final e in steps.entries) '${e.key} x${e.value}'];
      fail('the share sheet shows no $appName (steps: ${named.join(', ')})');
    }
    await Future<void>.delayed(const Duration(milliseconds: 200));
  }
}

/// The centers, as fractions of [screen], of the elements of [selector] that lie wholly on
/// screen, in the order of the query. Elements of a scrolled row or grid exist off screen too.
Future<List<Offset>> _onScreenAll(
  IOSAutomator ios,
  IOSSelector selector,
  Rectangle screen,
) async {
  final views = await ios.getNativeViews(selector, appId: photosApp);
  return [for (final view in views.roots) ?_centerOnScreen(view, screen)];
}

/// The center of [view] as fractions of [screen] when it lies wholly on screen, else null.
Offset? _centerOnScreen(IOSNativeView view, Rectangle screen) {
  final f = view.frame;
  final inside =
      f.minX >= 0 &&
      f.minY >= 0 &&
      f.maxX <= screen.maxX &&
      f.maxY <= screen.maxY &&
      f.maxX > f.minX &&
      f.maxY > f.minY;
  if (!inside) return null;
  return Offset(
    (f.minX + f.maxX) / 2 / screen.maxX,
    (f.minY + f.maxY) / 2 / screen.maxY,
  );
}

/// The first on-screen center of [selector] (see [_onScreenAll]); with [row], only one within
/// 10 % of the screen height of that row; null when none.
Future<Offset?> _onScreen(
  IOSAutomator ios,
  IOSSelector selector,
  Rectangle screen, {
  double? row,
}) async {
  for (final center in await _onScreenAll(ios, selector, screen)) {
    if (row != null && (center.dy - row).abs() > 0.1) continue;
    return center;
  }
  return null;
}

/// The frame of Photos' window, the screen of the fractions; null before Photos shows.
Future<Rectangle?> _screen(IOSAutomator ios) async {
  final views = await ios.getNativeViews(null, appId: photosApp);
  if (views.roots.isEmpty) return null;
  final frame = views.roots.first.frame;
  return frame.maxX > 0 && frame.maxY > 0 ? frame : null;
}

/// The height of the share sheet's row of apps as a fraction of [screen]: the center of a
/// system app that the row of the simulator holds, or null when none shows on screen.
Future<double?> _shareRowHeight(IOSAutomator ios, Rectangle screen) async {
  for (final label in const [
    'Messages',
    'Mail',
    'Notes',
    'Reminders',
    'Freeform',
  ]) {
    final cell = await _onScreen(ios, IOSSelector(label: label), screen);
    if (cell != null) return cell.dy;
  }
  return null;
}

/// Taps [selector] when it is shown and tells whether it did.
Future<bool> _optionalTap(
  PatrolIntegrationTester $,
  IOSSelector selector,
  String appId,
) async {
  final shown = await $.platform.ios.getNativeViews(selector, appId: appId);
  if (shown.roots.isEmpty) return false;
  await $.platform.ios.tap(selector, appId: appId, timeout: nativeWait);
  return true;
}

/// The newest photo: the last cell of the library grid, or else the last image labelled as a
/// photo (the sample the wrapper added is the newest). [center] is its center as fractions of
/// [screen] when it lies wholly on screen; [exists] tells whether Photos shows it at all, on
/// screen or off (a grid of several pages holds cells off screen too).
Future<({Offset? center, bool exists})> _newestPhoto(
  IOSAutomator ios,
  Rectangle screen,
) async {
  for (final selector in const [
    IOSSelector(identifier: 'PXGGridLayout-Info'),
    IOSSelector(elementType: IOSElementType.image, labelStartsWith: 'Photo,'),
  ]) {
    final views = await ios.getNativeViews(selector, appId: photosApp);
    if (views.roots.isEmpty) continue;
    return (center: _centerOnScreen(views.roots.last, screen), exists: true);
  }
  return (center: null, exists: false);
}
