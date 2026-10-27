// SPDX-License-Identifier: Apache-2.0

// Sending offline on the phone against the real server (brief M5-11 Design note 2, PLAN row
// M5-11; 01 5.3 M7 and 6.6; 04 5.1; 15 7.5): the app loses its connections to the stack, Alex
// sends in general, the message waits marked "Sending" under the banner with the waiting
// changes, Sam does not have it; the connections come back and Sam receives it exactly once.
//
// The cut is the in-process network of e2e_network.dart (as in the desktop suite of M4-34): the
// app's every connection to the server goes through a tunnel on the device's loopback, and
// cutting it ends the open connections and refuses new ones, while the server stays up and
// patrol's own connection to its automation server is not tunneled. The device's network itself
// stays up, as on a phone whose server is out of reach. tool/mobile_e2e_suite.sh runs it.

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';
import 'package:sync_engine/sync_engine.dart' show ConnectionPhase;

import '../../integration_test/phone/support/phone_flow.dart' show sendText;
import '../../test/support/widgets.dart' show en;
import '../support/e2e_keys.dart';
import 'support/phone_run.dart';

void main() {
  patrolTest(
    'offline: the message waits under the banner, after the reconnect Sam has it once',
    platformAutomatorConfig: phoneAutomatorConfig,
    ($) => PhoneRun.run($, (run) async {
      final world = run.world;
      final app = await run.startApp();
      await run.signInWithCode();
      await app.tap('sidebar.channel.${world.generalId}');
      await app.waitFor(
        find.byKey(ValueKey('history.${world.generalThreadId}')),
      );
      await app.waitFor(find.byKey(const ValueKey('composer')));

      final text = 'Offline check ${randomNonce().substring(0, 8)}';
      Future<int> samHas() async =>
          (await run.sam.messages(world.generalThreadId))
              .where((m) => m.contentMd == text)
              .length;

      run.network.cut();
      await app.until(
        'the connection lost',
        () => app.state.services!.engine.state.phase != ConnectionPhase.live,
      );
      await sendText(app.tester, text);
      final banner = find.byKey(const ValueKey('shell.offline'));
      // The banner's text with one or more waiting changes (0 is "Offline" alone).
      final waitingTexts = {
        for (var n = 1; n <= 20; n++) en.phoneOfflineWaiting(n),
      };
      // Without its connection the engine waits between attempts to connect; the banner shows
      // while it waits, with the changes waiting in the outbox (the message among them).
      await app.until(
        'the message marked "Sending" under the banner with waiting changes',
        () =>
            find.text(en.messageSending).evaluate().isNotEmpty &&
            find
                .descendant(
                  of: banner,
                  matching: find.byWidgetPredicate(
                    (w) => w is Text && waitingTexts.contains(w.data),
                  ),
                )
                .evaluate()
                .isNotEmpty,
      );
      expect(
        await samHas(),
        0,
        reason: 'the message waits on the phone, not on the server',
      );

      run.network.restore();
      await app.waitLive();
      await app.until(
        'nothing waits any more and the banner is gone',
        () =>
            find.text(en.messageSending).evaluate().isEmpty &&
            banner.evaluate().isEmpty,
      );
      await app.untilAsync('Sam receives the message', () async {
        return await samHas() > 0;
      });
      expect(await samHas(), 1, reason: 'the message arrives exactly once');
    }),
  );
}
