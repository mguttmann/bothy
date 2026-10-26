// SPDX-License-Identifier: Apache-2.0

// A share without the server goes to the inbox and arrives exactly once (brief M5-06 Tests;
// 15 12; 04 13.2). The fake server drops the Share Extension's first request without an
// answer, as a lost network does; the extension writes the entry with its client_msg_id into
// the share inbox of the App Group. When the app starts again with the server reachable, it
// takes the entry over into its outbox, and the message arrives exactly once with
// the extension's client_msg_id. iOS only: on Android the app's own sheet sends through the
// outbox, whose offline behaviour the phone tests of M5-03 prove.

import 'dart:io';

import 'package:client/src/generated/share_apple_api.g.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support/share_flow.dart';

void main() {
  patrolTest(
    'the inbox entry arrives exactly once with its client_msg_id',
    skip: !Platform.isIOS,
    platformAutomatorConfig: shareAutomatorConfig,
    ($) async {
      final app = await startShareApp($.tester);
      final target = seedTarget(app);
      final host = ShareAppleHostApi();
      try {
        await signIn(app);
        await untilChildSession(app);
        // The extension's upload is created, then its connection drops before the answer.
        app.fake.server.faults.dropResponseAfterCommit('createUpload');
        await shareNewestPhoto($);
        await sendFromExtension($, query: 'Gateway');

        var inbox = <ShareInboxEntryRecord>[];
        await untilInBackground('the entry in the share inbox', () {
          host.readInbox().then((entries) => inbox = entries);
          return inbox.isNotEmpty;
        });
        final clientMsgId = inbox.single.clientMsgId;
        expect(messagesWithFiles(app, target.thread), isEmpty);

        // The app starts again with the server reachable and takes the entry over.
        await $.platform.mobile.openApp();
        await app.restart();
        await until(
          app.tester,
          'the taken-over message',
          () => messagesWithFiles(app, target.thread).isNotEmpty,
        );
        final sent = messagesWithFiles(app, target.thread);
        expect(sent, hasLength(1));
        expect(sent.single['client_msg_id'], clientMsgId);
        expect(await host.readInbox(), isEmpty);
        expect(app.fake.server.violations, isEmpty);
      } finally {
        await app.stop();
      }
    },
  );
}
