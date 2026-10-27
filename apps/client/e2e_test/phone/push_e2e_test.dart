// SPDX-License-Identifier: Apache-2.0

// Push on the phone against the real server (brief M5-11 Design note 2 and product requirement
// 12, PLAN row M5-11; 13 3.1, 4.4, 4.6, 6.1, 6.2; 15 10 and 11). Alex follows a post of the
// forum (the own level of the thread is `all`, set through the API). The app signs in, opening
// the post asks for the notification permission, which the system dialog grants (patrol's native
// automation), and the app registers its push token with the server. The app goes to the
// background, which closes its gateway connection, so the server routes Sam's next message in
// that thread as a push: to the fake provider of the stack (fake FCM or fake APNs), whose
// capture the runner's push relay hands to the device (an `adb` broadcast into the debug-only
// receiver on Android, `xcrun simctl push` on iOS). The push carries IDs only; the native part
// (the preview worker on Android, the Notification Service Extension on iOS) fetches the text
// from the real server, and the notification shows it (on Android; the iOS simulator shows the
// payload's own text, see [_iosPlaceholder]). Tapping it opens the thread at that message. tool/mobile_e2e_suite.sh runs it; the relay is started by `make e2e-mobile`
// (tools/e2e/mobile_e2e.py). On Android it needs API 33 or later, where the app asks for the
// permission.

import 'dart:io';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';
import 'package:sync_engine/sync_engine.dart' show ConnectionPhase;

import '../../integration_test/share/support/share_flow.dart' show nativeWait;
import '../support/e2e_keys.dart';
import 'support/phone_run.dart';

/// How long the suite waits for the notification of Sam's message before Sam writes again.
///
/// The server learns that a connection closed only at the next flush of its presence state
/// (`cluster.presence_flush_interval`), so a message that comes right after the app went to the
/// background may still be routed to the closed connection and never become a push (13 4.6,
/// D-M3-23-3). Sam then writes again; a message inside the coalescing window of the first one
/// goes out at the window's end (13 4.5). This is the pace of that repetition, never the bound
/// of the test: the wait ends at the first notification and fails only after `patience`.
const Duration _writeAgainAfter = Duration(seconds: 30);

/// The text an iOS simulator shows for Sam's push: the body of the payload's `loc-key`
/// (`PUSH_BODY_MESSAGE` of `ios/Localizations/en.lproj/Localizable.strings`). `xcrun simctl push`
/// hands the notification to the system without running the Notification Service Extension
/// (Xcode 27), so the simulator cannot show the fetched text; the fetch of the extension is
/// proven by the ClientKit tests of M5-04, not by this suite (docs/decisions.d/M5-11.md).
const String _iosPlaceholder = 'New message';

void main() {
  patrolTest(
    "push: the permission dialog, Sam's message as a notification with its text, "
    'the tap opens the thread at the message',
    platformAutomatorConfig: phoneAutomatorConfig,
    ($) => PhoneRun.run($, (run) async {
      final world = run.world;
      final nonce = randomNonce().substring(0, 8);

      // The precondition through the API: a post of Alex in the forum that Alex follows.
      final thread = await run.owner.createPost(
        world.forumId,
        'Push $nonce',
        'Watching the gateway $nonce',
      );
      await run.network.direct(
        () => run.owner.setLevel('thread', thread, 'all'),
      );
      expect(await run.owner.levelOf('thread', thread), 'all');

      // The preview worker and the extension sign the preview fetch with the app's `dpop` key.
      final app = await run.startApp(push: true, nativeKeys: true);
      await run.signInWithCode();
      final engine = app.state.services!.engine;

      // Opening a thread is the moment the app asks for the permission (15 10.2, D-M5-05-6).
      await app.tap('sidebar.channel.${world.forumId}');
      await app.tap('postList.post.$thread');
      // While the system dialog is up the app is inactive and shows its privacy cover (15 11), so
      // the dialog is answered before the thread can be seen.
      expect(
        await $.platform.mobile.isPermissionDialogVisible(timeout: nativeWait),
        isTrue,
        reason:
            'the system asks for the notification permission when a thread opens '
            '(a fresh install, a build with push)',
      );
      await $.platform.mobile.grantPermissionWhenInUse();
      await app.waitFor(find.byKey(ValueKey('history.$thread')));
      await app.untilAsync(
        'the push registration of the app',
        () async => await run.pushRegistrationId() != null,
      );

      // In the background the app closes its gateway connection (15 6.3, 13 4.6).
      await $.platform.mobile.pressHome();
      await untilInBackgroundAsync(
        'the gateway connection closed in the background',
        () async => engine.state.phase == ConnectionPhase.suspended,
      );
      await $.platform.mobile.openNotifications();

      // Sam's messages by their text; each text is unique in the run.
      final sent = <String, String>{};
      Future<void> write() async {
        final text = 'Push check ${sent.length + 1} for $nonce';
        sent[text] = (await run.sam.post(thread, text)).id;
      }

      await write();
      var since = Stopwatch()..start();
      var onScreen = 0;
      String? shown;
      await untilInBackgroundAsync(
        "a notification with the text of Sam's message",
        () async {
          final notifications = await $.platform.mobile.getNotifications();
          onScreen = notifications.length;
          for (final n in notifications) {
            // iOS names the notification only in its accessibility label (`raw`).
            final text = Platform.isIOS
                ? n.raw ?? ''
                : '${n.title}\n${n.content}';
            if (Platform.isIOS && text.contains(_iosPlaceholder)) {
              shown = _iosPlaceholder;
              return true;
            }
            for (final candidate in sent.keys) {
              if (text.contains(candidate)) {
                shown = candidate;
                return true;
              }
            }
          }
          if (since.elapsed >= _writeAgainAfter) {
            await write();
            since = Stopwatch()..start();
          }
          return false;
        },
        // Names no content: the texts on screen stay out of the log.
        detail: () =>
            '($onScreen notifications on screen, ${sent.length} messages of Sam)',
      );

      await $.platform.mobile.tapOnNotificationBySelector(
        Selector(textContains: shown),
        timeout: nativeWait,
      );
      await untilInBackgroundAsync(
        'the app in the foreground after the tap',
        () async => run.resumed,
      );
      // On Android the notification names its message; on the iOS simulator it shows the
      // payload's own text, so the thread opens at the message of the last push of Sam.
      final messages = Platform.isIOS ? sent.values.toList() : [sent[shown]!];
      Finder highlighted(String message) => find.descendant(
        of: find.byKey(ValueKey('message.$message')),
        matching: find.byKey(const ValueKey('history.highlight')),
      );
      await app.until(
        'the thread open at the message of the notification',
        () => messages.any(
          (m) =>
              find
                  .byKey(ValueKey('historyColumn.$thread.$m'))
                  .evaluate()
                  .isNotEmpty &&
              highlighted(m).evaluate().isNotEmpty,
        ),
      );
    }),
  );
}
