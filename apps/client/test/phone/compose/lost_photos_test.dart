// SPDX-License-Identifier: Apache-2.0

// The photos of a pick that Android ended the app during (docs/decisions.d/
// AUD-client-phone-01.md; brief M5-07 requirement 16): the system may end the app while the
// camera or the photo picker is in front and hands the result to the next start. The composer
// that asked is gone with the app, so the app deletes those files when the phone shell of an
// Android build first shows, once per start; no photo stays outside the app's outgoing folder.

import 'dart:io';

import 'package:client/files/providers.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/phone/compose/media_picker.dart';
import 'package:client/phone/phone_shell.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/file_fakes.dart';
import '../support/phone_app.dart';

void main() {
  late Directory root;
  late Directory cache;
  late FakeMediaPort port;

  setUp(() {
    root = Directory.systemTemp.createTempSync('lost-photos');
    cache = Directory('${root.path}/picker-cache')..createSync();
    port = FakeMediaPort();
  });
  tearDown(() => root.deleteSync(recursive: true));

  /// A file the system picker wrote into the app's cache.
  RawMedia cached(String name) {
    final f = File('${cache.path}/$name')..writeAsBytesSync([0xFF, 0xD8, 0xFF]);
    return RawMedia(path: f.path, name: name);
  }

  group('MediaPicker.discardLost', () {
    MediaPicker picker() =>
        MediaPicker(port: port, scratch: () async => Directory(root.path));

    test('deletes every file of the lost pick and takes it only once', () async {
      port.lost = [cached('capture.jpg'), cached('scaled_capture.jpg')];
      await picker().discardLost();
      expect(cache.listSync(), isEmpty);
      expect(port.lostTaken, 1);
      // The platform hands a lost pick out once; a second start finds nothing.
      await picker().discardLost();
      expect(port.lostTaken, 2);
    });

    test('a file the system removed meanwhile and a platform that cannot be '
        'asked change nothing', () async {
      final gone = cached('gone.jpg');
      File(gone.path).deleteSync();
      final kept = cached('kept.jpg');
      port.lost = [gone, kept];
      await picker().discardLost();
      expect(cache.listSync(), isEmpty);

      final other = cached('other.jpg');
      port.lostError = StateError('no activity');
      await picker().discardLost();
      expect(File(other.path).existsSync(), isTrue);
    });
  });

  group('the phone shell', () {
    testWidgets('of an Android build deletes the lost photos when it first '
        'shows, once per start', (tester) async {
      final lost = cached('capture.jpg');
      port.lost = [lost];
      final app = await PhoneApp.start(
        tester,
        root: root,
        platform: 'android',
        overrides: [
          filePortsProvider.overrideWithValue(fakeFilePorts(media: port)),
        ],
      );
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 20)),
      );
      await tester.pump();
      expect(find.byType(PhoneShell), findsOneWidget);
      expect(File(lost.path).existsSync(), isFalse);
      expect(port.lostTaken, 1);
      expect(port.asked, isEmpty, reason: 'no picker opens');

      // The shell is built anew (the app lock, a login): the start already asked.
      app.control.show(const OnboardingState(step: OnboardingStep.server));
      await tester.pumpAndSettle();
      expect(find.byType(PhoneShell), findsNothing);
      app.control.show(const OnboardingState(step: OnboardingStep.signedIn));
      await tester.pumpAndSettle();
      expect(find.byType(PhoneShell), findsOneWidget);
      expect(port.lostTaken, 1);
    });

    testWidgets('of an iOS build asks nothing: only Android hands a pick to '
        'the next start', (tester) async {
      await PhoneApp.start(
        tester,
        root: root,
        overrides: [
          filePortsProvider.overrideWithValue(fakeFilePorts(media: port)),
        ],
      );
      await tester.pump();
      expect(find.byType(PhoneShell), findsOneWidget);
      expect(port.lostTaken, 0);
    });
  });
}
