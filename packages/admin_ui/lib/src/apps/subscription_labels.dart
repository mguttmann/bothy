// SPDX-License-Identifier: Apache-2.0

/// The texts of outgoing webhook subscriptions and their deliveries (11 6, 7; D-M2-26-6; brief
/// M6-08 Ruling 4): states, statuses, error codes and why a subscription is disabled.
library;

import 'package:l10n/l10n.dart';

import '../data/apps_repository.dart';

/// The text of the subscription state [state].
String subscriptionStateText(AppLocalizations l, SubscriptionState state) =>
    switch (state) {
      SubscriptionState.active => l.adminHookStateActive,
      SubscriptionState.paused => l.adminHookStatePaused,
      SubscriptionState.disabled => l.adminHookStateDisabled,
      SubscriptionState.unknown => l.adminHookStateUnknown,
    };

/// The text of the delivery status [status].
String deliveryStatusText(AppLocalizations l, DeliveryStatus status) =>
    switch (status) {
      DeliveryStatus.pending => l.adminHookDeliveryPending,
      DeliveryStatus.succeeded => l.adminHookDeliverySucceeded,
      DeliveryStatus.failed => l.adminHookDeliveryFailed,
      DeliveryStatus.dead => l.adminHookDeliveryDead,
      DeliveryStatus.unknown => l.adminHookDeliveryUnknown,
    };

/// The text of the `last_error` code [code] of a delivery (the closed list of D-M2-26-6); a
/// code of a newer server is shown as the code, so support can still read it.
///
/// Example: `deliveryErrorText(l, 'timeout')` is "timed out" in English.
String deliveryErrorText(AppLocalizations l, String code) => switch (code) {
  'timeout' => l.adminHookErrorTimeout,
  'connection_failed' => l.adminHookErrorConnectionFailed,
  'tls_failed' => l.adminHookErrorTlsFailed,
  'address_blocked' => l.adminHookErrorAddressBlocked,
  'host_not_allowed' => l.adminHookErrorHostNotAllowed,
  'http_status' => l.adminHookErrorHttpStatus,
  'throttled' => l.adminHookErrorThrottled,
  'gone' => l.adminHookErrorGone,
  'not_authorized' => l.adminHookErrorNotAuthorized,
  'not_subscribed' => l.adminHookErrorNotSubscribed,
  'event_expired' => l.adminHookErrorEventExpired,
  _ => code,
};

/// Why the disabled [subscription] is disabled, read from the server's values (Ruling 4): its
/// [newest] finished delivery ended with `event_expired` (a gap in the event log), or failed
/// because the receiver answered `410`; otherwise, with failures in a row, how many. The API
/// has no cause field, so the client reads the cause from what the server stored; without any
/// of these the text says the server names none.
String disabledReasonText(
  AppLocalizations l,
  AppSubscription subscription,
  AppDelivery? newest,
) {
  if (newest?.lastError == 'event_expired') return l.adminHookReasonGap;
  if (newest?.status == DeliveryStatus.failed &&
      (newest?.responseCode == 410 || newest?.lastError == 'gone')) {
    return l.adminHookReasonGone;
  }
  final failures = subscription.consecutiveFailures;
  if (failures > 0) return l.adminHookReasonFailures(failures);
  return l.adminHookReasonNone;
}
