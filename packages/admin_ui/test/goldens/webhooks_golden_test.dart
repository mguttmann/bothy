// SPDX-License-Identifier: Apache-2.0

// Goldens of the webhooks of M6-08 (brief M6-08 Tests): the subscriptions of an app with the
// disabled ones and their reasons, the deliveries of a subscription, the incoming webhooks of
// a channel and the dialog that creates one; light and dark, text scaling 1 and 2, German and
// English. One file per view keeps each file below repo.max_file_kb.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../admin/support.dart';
import '../apps/hooks_support.dart';
import '../incoming_webhooks/incoming_support.dart';

Widget _frame(Widget child) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 760,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

FakeAdminRepository _repository() =>
    FakeAdminRepository(prompt: StepUpDialogPrompt(() => null))
      ..stepUpFresh = true;

final Map<String, Widget Function()> _views = {
  'subscriptions': () {
    final r = _repository();
    final app = seedBridgeHooks(r);
    return _frame(
      AppSubscriptionsSection(
        repository: r,
        app: app,
        timeZone: 'Europe/Berlin',
        clock: testClock,
      ),
    );
  },
  'deliveries': () {
    final r = _repository();
    final app = seedBridgeHooks(r);
    final sub = r.subscriptionsByApp[app.id]!.first;
    r.deliveriesBySubscription[sub.id] = [
      hookDelivery(
        'd3',
        seq: 5303,
        status: DeliveryStatus.pending,
        attempt: 3,
        responseCode: 503,
        lastError: 'throttled',
        nextAttemptAt: DateTime.utc(2026, 10, 17, 9, 5),
        createdAt: DateTime.utc(2026, 10, 17, 8, 50),
      ),
      hookDelivery(
        'd2',
        seq: 5302,
        responseCode: 204,
        deliveredAt: DateTime.utc(2026, 10, 16, 7, 1),
      ),
      hookDelivery(
        'd1',
        seq: 5300,
        eventType: null,
        status: DeliveryStatus.dead,
        attempt: 4,
        lastError: 'event_expired',
        createdAt: DateTime.utc(2026, 10, 1, 7),
      ),
    ];
    r.deliveryPageSize = 2;
    return _frame(
      DeliveriesView(
        repository: r,
        subscription: sub,
        timeZone: 'Europe/Berlin',
        clock: testClock,
      ),
    );
  },
  'incoming_list': () {
    final r = _repository();
    seedIncoming(r);
    return _frame(
      IncomingWebhooksView(
        repository: r,
        timeZone: 'Europe/Berlin',
        clock: testClock,
      ),
    );
  },
  'incoming_create': () {
    final r = _repository();
    seedIncoming(r);
    return CreateIncomingDialog(
      repository: r,
      channel: campaigns,
      apps: [r.appsById['app-repo-bridge']!],
      offerIncomingApp: true,
    );
  },
};

void main() {
  for (final MapEntry(key: name, value: view) in _views.entries) {
    goldenTest(
      '$name in light and dark, text scale 1 and 2, de and en',
      fileName: 'webhooks_$name',
      pumpBeforeTest: dsPumpIcons,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
