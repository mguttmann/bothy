// SPDX-License-Identifier: Apache-2.0

// Sharing an image into a thread on the phone against the real server (brief M5-11 Design
// note 2, PLAN row M5-11; 01 5.3 M6 and 5.4; 15 12). iOS: the sample photo in Photos goes
// through the system share sheet into the app's Share Extension, which finds general by search
// and sends by itself with its child session (upload, then the message), trusting the stack's
// test CA as a root of the simulator. Android: the sample image in the file app goes through the
// share chooser into the app's share sheet, which sends through the outbox. Either way Sam
// receives the message with the image in general. tool/mobile_e2e_suite.sh puts the sample into
// Photos or the downloads and runs it.

import 'dart:io';

import 'package:api_client/api_client.dart'
    show Message, SessionClientComponentEnum;
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import '../../integration_test/share/support/share_flow.dart'
    show
        chooserOffersApp,
        openAndroidChooser,
        pickAppInChooser,
        sampleImage,
        sendFromExtension,
        shareNewestPhoto;
import '../support/e2e_app.dart';
import '../support/e2e_keys.dart';
import 'support/phone_run.dart';

void main() {
  patrolTest(
    'share: an image from another app lands in general for Sam',
    platformAutomatorConfig: phoneAutomatorConfig,
    ($) => PhoneRun.run($, (run) async {
      final world = run.world;
      // On iOS the Share Extension signs with the app's `dpop` key of the shared keychain group.
      final app = await run.startApp(share: true, nativeKeys: Platform.isIOS);
      await run.signInWithCode();

      Future<List<Message>> withImage() async => [
        for (final m in await run.sam.messages(world.generalThreadId))
          if (m.attachments.isNotEmpty) m,
      ];
      final before = {for (final m in await withImage()) m.id};
      final comment = 'Shared photo ${randomNonce().substring(0, 8)}';

      if (Platform.isIOS) {
        final device = app.deviceId;
        await app.untilAsync(
          'the child session of the Share Extension',
          () async {
            final sessions = await run.network.direct(
              () => run.owner.api.getDevicesApi().listMySessions(),
            );
            return sessions.data!.items.any(
              (s) =>
                  s.deviceId == device &&
                  s.clientComponent == SessionClientComponentEnum.shareExt,
            );
          },
        );
        // From here Photos and the Share Extension are in front and the app is in the
        // background; the waits poll without pumping.
        await shareNewestPhoto($);
        await sendFromExtension($, query: 'general');
      } else {
        final chooser = await openAndroidChooser($, sampleImage, offered: true);
        expect(chooserOffersApp(chooser), isTrue);
        await pickAppInChooser($);
        await _sendFromAppSheet(app, query: 'general', comment: comment);
      }

      Message? shared;
      await untilInBackgroundAsync(
        'the shared image for Sam in general',
        () async {
          shared = (await withImage())
              .where((m) => !before.contains(m.id))
              .firstOrNull;
          return shared != null;
        },
      );
      final file = shared!.attachments.single.file;
      expect(file.mimeType, 'image/jpeg');
      if (Platform.isAndroid) {
        // The comment of the app's sheet travels with the image (the extension's comment is a
        // text view that patrol cannot type into, see share_flow.dart).
        expect(shared!.contentMd == comment, isTrue, reason: 'the comment');
      }
      expect(
        (await withImage()).where((m) => !before.contains(m.id)),
        hasLength(1),
        reason: 'the image arrives once',
      );
    }),
  );
}

/// In the app's own share sheet (Android): searches [query], picks the first hit, writes
/// [comment] and sends through the outbox. The sheet of share_flow.dart for [E2eApp].
Future<void> _sendFromAppSheet(
  E2eApp app, {
  required String query,
  required String comment,
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
