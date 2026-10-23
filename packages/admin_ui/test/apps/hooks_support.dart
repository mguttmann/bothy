// SPDX-License-Identifier: Apache-2.0

// The outgoing webhooks of the tests (brief M6-08 Tests): Repo Bridge with two granted events
// out of three, one subscription in every state and the newest delivery behind each reason of
// a disabled subscription (Ruling 4: the gap, a 410 of the receiver, failures in a row).

import 'package:admin_ui/admin_ui.dart';

import '../admin/support.dart';

/// The events Repo Bridge may subscribe to; the installation grants the first two.
const List<String> bridgeEvents = [
  'message.created',
  'message.updated',
  'reaction.added',
];

AppSubscription _sub(
  String id, {
  required SubscriptionState state,
  int failures = 0,
  List<String> schemes = const ['v1a'],
  List<String> events = const ['message.created'],
  DateTime? disabledAt,
}) => AppSubscription(
  id: id,
  url: 'https://$allowedHookHost/hooks/$id',
  eventTypes: events,
  state: state,
  consecutiveFailures: failures,
  signatureSchemes: schemes,
  signingKeyKid: 'whk-2026-10',
  createdAt: DateTime.utc(2026, 10, 1, 8),
  disabledAt: disabledAt,
);

/// One delivery [id] of event [seq] with [status]; the remaining members vary per test.
AppDelivery hookDelivery(
  String id, {
  int seq = 5301,
  String? eventType = 'message.created',
  DeliveryStatus status = DeliveryStatus.succeeded,
  int attempt = 1,
  int? responseCode,
  String? lastError,
  DateTime? nextAttemptAt,
  DateTime? createdAt,
  DateTime? deliveredAt,
}) => AppDelivery(
  id: id,
  eventId: 'event-$id',
  eventSeq: seq,
  eventType: eventType,
  attempt: attempt,
  status: status,
  responseCode: responseCode,
  lastError: lastError,
  nextAttemptAt: nextAttemptAt,
  createdAt: createdAt ?? DateTime.utc(2026, 10, 16, 7),
  deliveredAt: deliveredAt,
);

/// Seeds Repo Bridge with subscriptions in every state into [r] and returns the app:
/// `sub-active`, `sub-paused` (two failures, with HMAC), `sub-gap`, `sub-gone`, `sub-failing`
/// (disabled for the three reasons of Ruling 4) and `sub-newer` (a state of a newer server).
AppRecord seedBridgeHooks(FakeAdminRepository r) {
  final app = r.seedApp(
    repoBridgeManifest(events: bridgeEvents),
    grantedScopes: const [
      'messages:write',
      'commands:write',
      'messages:read',
      'events:subscribe',
    ],
    eventSubscriptions: const ['message.created', 'message.updated'],
  );
  final disabledAt = DateTime.utc(2026, 10, 16, 7);
  r.subscriptionsByApp[app.id] = [
    _sub(
      'sub-active',
      state: SubscriptionState.active,
      events: const ['message.created', 'message.updated'],
    ),
    _sub(
      'sub-paused',
      state: SubscriptionState.paused,
      failures: 2,
      schemes: const ['v1a', 'v1'],
    ),
    _sub('sub-gap', state: SubscriptionState.disabled, disabledAt: disabledAt),
    _sub(
      'sub-gone',
      state: SubscriptionState.disabled,
      failures: 1,
      disabledAt: disabledAt,
    ),
    _sub(
      'sub-failing',
      state: SubscriptionState.disabled,
      failures: 25,
      disabledAt: disabledAt,
    ),
    _sub('sub-newer', state: SubscriptionState.unknown),
  ];
  r.deliveriesBySubscription['sub-gap'] = [
    hookDelivery(
      'd-gap',
      eventType: null,
      status: DeliveryStatus.dead,
      lastError: 'event_expired',
    ),
  ];
  r.deliveriesBySubscription['sub-gone'] = [
    hookDelivery(
      'd-gone',
      status: DeliveryStatus.failed,
      responseCode: 410,
      lastError: 'gone',
    ),
  ];
  r.deliveriesBySubscription['sub-failing'] = [
    hookDelivery(
      'd-failing',
      status: DeliveryStatus.dead,
      attempt: 25,
      responseCode: 500,
      lastError: 'http_status',
    ),
  ];
  return app;
}
