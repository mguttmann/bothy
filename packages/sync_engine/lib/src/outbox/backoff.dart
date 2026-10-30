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

/// The wait before retry number [retry] after a refusal that named [retryAfter]: the server's
/// wait plus [fullJitterBackoff], or the backoff alone without one (04 13.2 item 4, addendum of
/// 2026-10-30).
///
/// `Retry-After` is a floor, never the exact time: the server names the seconds until one
/// token of the bucket is back (06 9 item 5), so every lane refused at once would come back
/// at once and use up the bucket again. The jitter spreads them; its bound doubles with
/// [retry], so a lane that keeps hitting the limit backs off further.
///
/// Example: `retryWait(retryAfter: Duration(seconds: 1), base: base, cap: cap, retry: 0,
/// random: random)` lies in `[1 s, 1 s + base]`.
Duration retryWait({
  Duration? retryAfter,
  required Duration base,
  required Duration cap,
  required int retry,
  required Random random,
}) {
  final jitter = fullJitterBackoff(
    base: base,
    cap: cap,
    retry: retry,
    random: random,
  );
  return retryAfter == null ? jitter : retryAfter + jitter;
}
