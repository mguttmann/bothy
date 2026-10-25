// SPDX-License-Identifier: Apache-2.0

// A run of support-agent, served by the Fake-Agent of the stack (brief M4-34 product requirement
// 13, Design note 1): Alex mentions the agent in a forum post; the live box shows the status line
// and the steps; a follow-up during the run is delivered; the agent posts an approval card,
// Alex allows once, the agent replies and the run collapses into its line (04 11, 16.5; 09 11).

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/e2e_app.dart';
import '../support/e2e_context.dart';
import '../support/e2e_keys.dart';
import 'messaging_flows.dart' show sendText;

/// A widget whose key is a [ValueKey] of [prefix] followed by a UUID.
Finder keyedWith(String prefix) => find.byWidgetPredicate(
  (w) =>
      w.key is ValueKey<String> &&
      RegExp(
        '^${RegExp.escape(prefix)}[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\$',
      ).hasMatch((w.key! as ValueKey<String>).value),
);

/// The text of the [ValueKey] of the first widget [finder] finds.
String keyOf(Finder finder) =>
    ((finder.evaluate().first.widget.key!) as ValueKey<String>).value;

/// The trigger state icon of the message [messageId] in [state].
Finder triggerOf(String messageId, DsTriggerState state) => find.descendant(
  of: find.byKey(ValueKey('message.$messageId')),
  matching: find.byWidgetPredicate(
    (w) => w is DsTriggerStateIcon && w.state == state,
  ),
);

/// Registers the agent flow.
void agentFlows(E2eContext c) {
  final world = c.world;

  testWidgets('a run with live box, follow-up and approval card', (
    tester,
  ) async {
    final nonce = randomNonce().substring(0, 8);
    final thread = (await tester.runAsync(
      () => c.owner.createPost(
        world.forumId,
        'Release checklist $nonce',
        'The checklist of the release',
      ),
    ))!;
    final app = await E2eApp.start(tester, world);
    try {
      await app.signIn(await c.alexCode());
      await app.openPost(world.forumId, thread);

      await sendText(tester, '@${world.agent.handle} summarize the checklist');
      final box = keyedWith('livebox.');
      await app.waitFor(box);
      final runId = keyOf(box).substring('livebox.'.length);
      final liveBox = find.byKey(ValueKey('livebox.$runId'));
      await app.waitFor(
        find.descendant(of: liveBox, matching: find.text('Reading the thread')),
      );
      await app.waitFor(
        find.descendant(of: liveBox, matching: find.textContaining('search')),
      );

      // A follow-up during the run reaches the agent (09 7). Only a triggering message
      // becomes one (R-SY-22), and the forum answers only a mention (thread_require_mention).
      final followupText =
          '@${world.agent.handle} also mention the budget $nonce';
      await sendText(tester, followupText);
      String? followup;
      await app.untilAsync('the follow-up on the server', () async {
        final messages = await c.sam.messages(thread);
        followup = messages
            .where((m) => m.contentMd == followupText)
            .firstOrNull
            ?.id;
        return followup != null;
      });
      await app.waitFor(triggerOf(followup!, DsTriggerState.followupDelivered));

      // The approval card: Alex started the run and may approve actions.
      final card = keyedWith('card.');
      await app.waitFor(card);
      final once = find.descendant(
        of: find.byKey(ValueKey(keyOf(card))),
        matching: find.byKey(const ValueKey('ds-component-approval.once')),
      );
      await app.waitFor(once);
      await app.until(
        'the button enabled',
        () => tester.widget<DsButton>(once).onPressed != null,
      );
      await app.tapFinder(once);
      // The choice is taken: the card offers no button any more (used, or no longer
      // available once the run ended), and the agent answers only after it received the
      // interaction (tools/ci/desktop_e2e_agent.py).
      await app.until('the card answered', () => once.evaluate().isEmpty);
      await app.waitFor(find.text('Here is the summary.'));
      await app.waitFor(find.byKey(ValueKey('runline.$runId')));
      expect(liveBox, findsNothing);
    } finally {
      await app.stop();
    }
  });
}
