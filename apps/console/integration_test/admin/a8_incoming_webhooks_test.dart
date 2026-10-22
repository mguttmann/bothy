// SPDX-License-Identifier: Apache-2.0

// A8 in the browser (brief M6-08): an incoming webhook for a channel with the "incoming app" of
// 11 8.1 (the owner's option), its address shown once, a post to it from the page without any
// credential, the message read back through the API, the revocation, and the next post answered
// 404. The address carries the token: it lives in a local variable only, is never printed and
// leaves no trace on screen after the dialog closed (the "shown once" test of 08 12).

import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

/// The ID of the incoming webhook in its address `<origin>/api/v1/hooks/<id>/<token>` (11 8.1);
/// the ID is no secret, the token after it is.
String _webhookId(String address) {
  final segments = Uri.parse(address).pathSegments;
  final at = segments.indexOf('hooks');
  if (at < 0 || segments.length != at + 3) {
    fail('the address has not the form .../hooks/<id>/<token>');
  }
  return segments[at + 1];
}

void main() {
  patrolTest('A8 an incoming webhook shown once, posted to and revoked', (
    $,
  ) async {
    final seed = E2eSeed.read();
    final general = seed.channel('supportGeneral');
    await openConsole($, '/apps/incoming-webhooks');
    await waitFor($, byKey('incoming.view'));
    await pick($, 'incoming.channel', 'incoming.channel.${general.id}');
    await tapKey($, 'incoming.create');
    await waitFor($, byKey('incoming.dialog'));
    await typeKey($, 'incoming.name', 'Monitoring alerts');
    // The owner composes a new incoming app (Design note 5): install, grant, webhook.
    await tapKey($, 'incoming.mode.newApp');
    await typeKey($, 'incoming.reason', 'Alerts of the monitoring');
    await tapKey($, 'incoming.submit');
    await waitFor($, byKey('incoming.secret'));
    var address = selectableTextOf('incoming.secret');
    hideInFailures(address);
    expect(
      address.startsWith('${Uri.base.origin}/api/v1/hooks/'),
      isTrue,
      reason: 'the address is an incoming webhook path of this server',
    );
    final webhookId = _webhookId(address);
    // The token is the last segment; origin, path and ID of the address are no secret.
    final token = Uri.parse(address).pathSegments.last;
    await tapKey($, 'incoming.secretClose');
    await waitGone($, byKey('incoming.secretDialog'));
    expect(byKey('incoming.secret'), findsNothing);
    expectNoTrace(token, 'webhook token');
    expectStepUp();

    // An outside sender: no cookie, no CSRF header, the token in the path only.
    const content = 'Disk usage above threshold on build.example.com';
    final posted = await postWithoutSession(address, {'content_md': content});
    expect(posted.status, 201, reason: 'postIncomingWebhook');
    final result = posted.json as Map?;
    final thread = result?['thread_id'];
    expect(thread, isA<String>(), reason: 'the answer names the thread');
    final messages = itemsOf(
      await consoleApi('GET', '/threads/$thread/messages'),
      'listThreadMessages',
    );
    expect(
      messages
          .where((m) => m['id'] == result?['message_id'])
          .map((m) => m['content_md']),
      [content],
      reason: 'the message of the webhook is in the channel',
    );

    // Reopening the screen shows the webhook, but no part of its token.
    await openConsole($, '/apps/incoming-webhooks');
    await pick($, 'incoming.channel', 'incoming.channel.${general.id}');
    await waitFor($, byKey('incoming.row.$webhookId'));
    expectNoTrace(token, 'webhook token');

    await tapKey($, 'incoming.revoke.$webhookId');
    await typeKey($, 'confirm.reason', 'The monitoring moved');
    await tapKey($, 'confirm.submit');
    await waitGone($, byKey('incoming.row.$webhookId'));
    expectStepUp();
    final after = await postWithoutSession(address, {'content_md': content});
    expect(after.status, 404, reason: 'a revoked webhook answers 404');
    address = '';
    forgetHidden();
  });
}
