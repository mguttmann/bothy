// SPDX-License-Identifier: Apache-2.0

// A7 in the browser (brief M6-08): install the app "Repo Bridge" from a manifest file the runner
// sets on the console's file input, with the consent of 11 4.1, a grant on a chosen channel, the
// server's preview and the step-up; the app token and the key ID are shown once. Then an outgoing
// subscription to the receiver of the Compose network, a message that causes message.created,
// and the delivery the screen shows as succeeded after a reload.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

/// The address of the second subscription at the receiver (webhook.allowed_hosts of the E2E
/// configuration lists webhook-receiver:8443).
const _secondHook = 'https://webhook-receiver:8443/hooks/second';

/// The texts below the widget with [key], joined by a space.
String _textBelow(String key) => [
  for (final e
      in find
          .descendant(
            of: byKey(key),
            matching: find.byType(RichText),
            matchRoot: true,
          )
          .evaluate())
    (e.widget as RichText).text.toPlainText(),
].join(' ');

/// The widgets whose key starts with [prefix].
Finder _keyedWith(String prefix) => find.byWidgetPredicate(
  (w) => switch (w.key) {
    ValueKey<String>(:final value) => value.startsWith(prefix),
    _ => false,
  },
);

/// Whether a delivery on screen shows the state succeeded, in the words of the console's
/// catalogue (the console runs in English, see openConsole).
bool _deliverySucceeded() {
  final succeeded = lookupAppLocalizations(const Locale('en'))
      .adminHookDeliverySucceeded;
  return _keyedWith('delivery.status.').evaluate().any((e) {
    final key = (e.widget.key! as ValueKey<String>).value;
    return _textBelow(key).trim() == succeeded;
  });
}

/// The ID of the installed app with [slug] (listApps with the console session).
Future<String> _appId(String slug) async {
  final apps = itemsOf(await consoleApi('GET', '/apps'), 'listApps');
  final app = apps.where((a) => a['slug'] == slug).toList();
  expect(app, hasLength(1), reason: 'listApps names $slug once');
  return app.single['id']! as String;
}

/// The ID of the subscription of [appId] to [url] (listWebhookSubscriptions).
Future<String> _subscriptionTo(String appId, String url) async {
  final subs = itemsOf(
    await consoleApi('GET', '/apps/$appId/webhooks'),
    'listWebhookSubscriptions',
  );
  final sub = subs.where((s) => s['url'] == url).toList();
  expect(sub, hasLength(1), reason: 'one subscription goes to $url');
  return sub.single['id']! as String;
}

/// Posts [text] into the main thread of [channelId] as the owner (getChannel, createMessage with
/// a fresh client_msg_id; createMessage takes no Idempotency-Key), which causes message.created.
Future<void> _postMessage(String channelId, String text) async {
  final channel = await consoleApi('GET', '/channels/$channelId');
  // getChannel answers a ChannelView: the channel sits under `channel`.
  final view = channel.json as Map?;
  final thread = (view?['channel'] as Map?)?['main_thread_id'];
  expect(channel.status, 200, reason: 'getChannel');
  expect(thread, isA<String>(), reason: 'the channel names its main thread');
  final sent = await consoleApi(
    'POST',
    '/threads/$thread/messages',
    body: {'client_msg_id': uuid7(), 'content_md': text},
  );
  expect(sent.status, anyOf(200, 201), reason: 'createMessage');
}

void main() {
  patrolTest(
    'A7 install an app from a manifest file with step-up and see the token once',
    ($) async {
      final seed = E2eSeed.read();
      final builds = seed.channel('engineeringBuilds');
      await openConsole($, '/apps');
      await waitFor($, byKey('apps.list'));
      await tapKey($, 'apps.install');
      await waitFor($, byKey('install.dialog'));
      await chooseManifest($, 'repo-bridge.json');
      await waitFor($, byKey('consent.view'));

      // The required scope is checked and fixed; the optional scopes and the event start
      // checked (the manifest asks for them) and the owner keeps them.
      expect(checkboxOf('consent.scope.messages:write'), (
        value: true,
        enabled: false,
      ));
      for (final key in [
        'consent.scope.messages:read',
        'consent.scope.events:subscribe',
        'consent.event.message.created',
      ]) {
        expect(checkboxOf(key).value, isTrue, reason: '$key starts checked');
      }
      await pick($, 'consent.grant.0', 'consent.grant.0.channel.${builds.id}');
      await typeKey($, 'consent.tokenName', 'Build bridge');
      await typeKey($, 'consent.tokenLastDay', dayFromToday(30));
      await typeKey($, 'consent.reason', 'Build results for engineering');
      await tapKey($, 'install.submit');

      // The grants step: the server's preview of the chosen grant, then grants and token.
      await waitFor($, byKey('install.preview.0'));
      expect(
        _textBelow('install.preview.0'),
        contains(builds.name),
        reason: 'the preview names the chosen channel',
      );
      await tapKey($, 'install.createGrants');
      await waitFor($, byKey('appToken.secret'));
      var token = selectableTextOf('appToken.secret');
      hideInFailures(token);
      expect(
        _textBelow('appToken.keyId').trim(),
        isNotEmpty,
        reason: 'the key ID of the webhook signatures is shown with the token',
      );
      await tapKey($, 'appToken.secretClose');
      await waitGone($, byKey('appToken.secretDialog'));
      // Shown once: after the close nothing on the page shows any part of the token.
      expect(byKey('appToken.secret'), findsNothing);
      expectNoTrace(token, 'app token');
      expectStepUp();

      // An outgoing subscription of the app to the receiver, then a message that the app may
      // read (grant on the channel, messages:read, message.created) and its delivery.
      final appId = await _appId('repo-bridge');
      await openConsole($, '/apps/$appId');
      await waitFor($, byKey('appPage'));
      // The app page shows the token's prefix at most, never the token.
      expectNoTrace(token, 'app token');
      token = '';
      await tapKey($, 'subs.create');
      await waitFor($, byKey('subs.form'));
      await typeKey($, 'subs.url', _secondHook);
      await setCheckbox($, 'subs.event.message.created', true);
      await tapKey($, 'subs.submit');
      await waitGone($, byKey('subs.form'));
      expectStepUp();
      final subId = await _subscriptionTo(appId, _secondHook);
      await waitFor($, byKey('subs.row.$subId'));

      await _postMessage(builds.id, 'Build 42 passed on main');
      await tapKey($, 'subs.deliveries.$subId');
      await waitFor($, byKey('deliveries.view'));
      // The worker delivers within seconds; a retry after a refused first attempt follows
      // webhook.retry_backoff_base (10 s), so the wait allows a few of them.
      final end = DateTime.now().add(const Duration(minutes: 2));
      while (!_deliverySucceeded()) {
        if (!DateTime.now().isBefore(end)) {
          fail('no delivery shows succeeded; ${screenText()}');
        }
        await $.tester.pump(const Duration(seconds: 2));
        await tapKey($, 'deliveries.reload');
      }
      forgetHidden();
    },
  );
}
