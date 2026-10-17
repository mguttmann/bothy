// SPDX-License-Identifier: Apache-2.0

/// The full-jitter backoff of the outbox (04 13.2 item 4).
library;

import 'dart:math';

/// The wait before retry number [retry] (0 for the first retry) with full jitter: uniform in
/// `[0, min(cap, base * 2^retry)]`.
///
/// [base] and [cap] are `client.outbox_retry_base` and `client.outbox_retry_cap` from
/// `AppConfig` (D-W-115); [random] draws the jitter (seeded in tests). The formula and the use
/// of [random] are the ones of `ReadRetryPolicy.backoff` of `api_client`, so reads and writes
/// back off alike; a test keeps the two equal. A zero or negative bound gives
/// [Duration.zero].
///
/// Example:
/// ```dart
/// final wait = fullJitterBackoff(
///   base: config.outboxRetryBase,
///   cap: config.outboxRetryCap,
///   retry: entry.attempts - 1,
///   random: Random.secure(),
/// );
/// ```
Duration fullJitterBackoff({
  required Duration base,
  required Duration cap,
  required int retry,
  required Random random,
}) {
  // The exponent stops at 40 so the product cannot overflow; the cap is reached long before.
  final bound = min(
    cap.inMicroseconds,
    base.inMicroseconds * pow(2, min(max(retry, 0), 40)).toInt(),
  );
  if (bound <= 0) return Duration.zero;
  return Duration(microseconds: (random.nextDouble() * bound).floor());
}
