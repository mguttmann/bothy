// SPDX-License-Identifier: Apache-2.0

// The shared start of the phone integration tests (briefs M5-02 and M5-03 Tests): the app with the
// ports of an iOS build, or of an Android build on an Android device or emulator, against the
// fake server of M4-17 in a phone window. On the iOS simulator the database files get their
// protection class through the runner's Pigeon channel; on Android (private app storage) and on
// the Flutter tester (no runner) that port stays a no-op. The files are the same on both
// platforms; only going back differs ([goBack]).

import 'dart:io';

import 'package:client/files/file_ports.dart';
import 'package:client/lock/app_lock_port.dart';
import 'package:client/platform/ios_platform.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../../test/support/widgets.dart' show en;
import '../../support/test_app.dart';

export '../../support/test_app.dart';

/// The paths the app gave the protection class, in order, on every host.
final List<String> protectedPaths = [];

/// Whether the tests run in the iOS simulator, which keeps no protection class on its files
/// (only a device does): there the tests check the call, on a device the attribute.
bool get inSimulator =>
    Platform.isIOS && Directory.systemTemp.path.contains('/CoreSimulator/');

/// The platform of the phone build under test: `android` on an Android device or emulator,
/// else `ios` (the iOS simulator and the Flutter tester).
String get phonePlatform => Platform.isAndroid ? 'android' : 'ios';

/// Starts the phone app; [lifecycle] replaces the always-foreground lifecycle of the tests and
/// [files] the ports of files, voice and photos (brief M5-07); [appLock] the system
/// authentication of the app lock (none by default).
Future<TestApp> startPhone(
  WidgetTester tester, {
  AppLifecycle? lifecycle,
  FilePorts? files,
  AppLockPort appLock = const NoAppLockPort(),
}) {
  protectedPaths.clear();
  final ios = Platform.isIOS ? IosPlatformApi() : null;
  return TestApp.start(
    tester,
    platform: phonePlatform,
    lifecycle: lifecycle,
    files: files,
    appLock: appLock,
    protectFiles: (paths) async {
      protectedPaths.addAll(paths);
      await ios?.protectFiles(paths);
    },
  );
}

/// Runs [body] with the target platform of the phone: on a device or simulator its own; on the
/// Flutter tester iOS (the Cupertino pages and their edge swipe), as on the simulator; the
/// Flutter tester's default would be Android.
Future<void> asPhone(Future<void> Function() body) async {
  if (Platform.isIOS || Platform.isAndroid) return body();
  debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
  try {
    await body();
  } finally {
    debugDefaultTargetPlatformOverride = null;
  }
}

/// Logs in as Alex and waits for the phone shell with a live engine.
Future<void> signIn(TestApp app) async {
  await app.connect();
  await app.waitFor(find.byKey(const ValueKey('e2.loginName')));
  await app.loginWithPassword();
  await app.waitFor(find.byKey(const ValueKey('phone.shell')));
  await app.waitLive();
}

/// The composer's text field.
final Finder composerField = find.descendant(
  of: find.byKey(const ValueKey('composer')),
  matching: find.byType(TextField),
);

/// Types [text] and sends it with the send button (Enter adds a line on the phone, 01 11).
Future<void> sendText(WidgetTester tester, String text) async {
  await tester.enterText(composerField, text);
  await tester.pump();
  await tester.tap(
    find.byWidgetPredicate(
      (w) => w is DsTappable && w.label == en.composerSend,
    ),
  );
  await tester.pump();
}

/// Pumps until [condition] holds, with the bound of [TestApp.waitFor].
Future<void> until(
  WidgetTester tester,
  String what,
  bool Function() condition,
) async {
  final end = DateTime.now().add(const Duration(seconds: 30));
  while (!condition()) {
    if (DateTime.now().isAfter(end)) fail('timed out: $what');
    await tester.pump(const Duration(milliseconds: 50));
  }
}

/// The edge swipe back of iOS: a finger down at the left edge, moved right across most of the
/// screen in steps of one frame, then lifted.
///
/// iOS (and the Cupertino route) offers the gesture only once the page's push transition has
/// ended, so the swipe waits for that first; on a loaded machine the transition can still run
/// when the test gets here.
Future<void> swipeBack(WidgetTester tester) async {
  final navigator = tester.state<NavigatorState>(find.byType(Navigator).last);
  ModalRoute<Object?>? top;
  navigator.popUntil((route) {
    top ??= route as ModalRoute<Object?>;
    return true;
  });
  await until(
    tester,
    'the push transition of the top page ended',
    () => top!.animation!.status == AnimationStatus.completed,
  );
  final gesture = await tester.startGesture(const Offset(2, 400));
  for (var i = 0; i < 10; i++) {
    await gesture.moveBy(const Offset(30, 0));
    await tester.pump(const Duration(milliseconds: 16));
  }
  await gesture.up();
  await tester.pump();
}

/// Goes back one page as a person does: system back on Android (01 5.4), the edge swipe on
/// iOS ([swipeBack]).
Future<void> goBack(WidgetTester tester) async {
  if (phonePlatform == 'android') {
    await tester.binding.handlePopRoute();
    await tester.pump();
    return;
  }
  await swipeBack(tester);
}

/// Waits until [condition] holds without pumping, for the time the app is in the background
/// (`hidden` or `paused`), when the binding schedules no frames; the bound is that of [until].
Future<void> untilInBackground(String what, bool Function() condition) async {
  final end = DateTime.now().add(const Duration(seconds: 30));
  while (!condition()) {
    if (DateTime.now().isAfter(end)) fail('timed out: $what');
    await Future<void>.delayed(const Duration(milliseconds: 20));
  }
}
