// SPDX-License-Identifier: Apache-2.0

// An image from another app lands in the chosen thread (brief M5-06 Tests, PLAN row M5-06;
// 01 5.3 M6, 5.4; 15 12). iOS: the sample photo in Photos goes through the system share sheet
// into the app's Share Extension, which finds "server > Gateway: resume ..." by search and sends
// by itself with its child session (upload, then the message with its own client_msg_id).
// Android: the sample image in the file app goes through the share chooser into the app's
// share sheet, which sends through the outbox. Either way the fake server of M4-17 receives the
// upload and the message with the attachment in that thread, and the next share shows the
// thread at the top of the recent targets.

import 'dart:io';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support/share_flow.dart';

void main() {
  patrolTest(
    'an image from another app lands in the chosen thread',
    platformAutomatorConfig: shareAutomatorConfig,
    ($) async {
      final app = await startShareApp($.tester);
      final target = seedTarget(app);
      try {
        await signIn(app);
        if (Platform.isIOS) {
          await untilChildSession(app);
          await shareNewestPhoto($);
          await sendFromExtension($, query: 'Gateway');
        } else {
          final chooser = await openAndroidChooser(
            $,
            sampleImage,
            offered: true,
          );
          expect(chooserOffersApp(chooser), isTrue);
          await pickAppInChooser($);
          await sendFromAppSheet(app, query: 'Gateway');
        }
        await untilInBackground(
          'the message with the image in the thread',
          () => messagesWithFiles(app, target.thread).isNotEmpty,
        );
        final sent = messagesWithFiles(app, target.thread);
        expect(sent, hasLength(1));
        final file =
            ((sent.single['attachments']! as List).single as Map)['file']!
                as Map;
        expect(file['mime_type'], 'image/jpeg');
        expect(sent.single['client_msg_id'], isNotNull);

        // The next share offers the thread first (client.share.recent_targets).
        if (Platform.isIOS) {
          await shareNewestPhoto($);
          expect(await firstExtensionTarget($), contains('Gateway'));
          await $.platform.ios.tap(
            const IOSSelector(identifier: 'share.cancel'),
            appId: photosApp,
            timeout: nativeWait,
          );
        } else {
          await openAndroidChooser($, sampleImage, offered: true);
          await pickAppInChooser($);
          await app.waitFor(find.byKey(const ValueKey('share.target.0')));
          expect(
            find.descendant(
              of: find.byKey(const ValueKey('share.target.0')),
              matching: find.textContaining('Gateway'),
            ),
            findsOneWidget,
          );
          await app.tap('share.cancel');
        }
        expect(app.fake.server.violations, isEmpty);
      } finally {
        await app.stop();
      }
    },
  );
}
