// SPDX-License-Identifier: Apache-2.0

import 'dart:math';

import 'package:api_client/src/hand/problem.dart';
import 'package:dio/dio.dart';

const _readMethods = {'GET', 'HEAD'};
const _attemptExtra = 'api_client.read_attempt';

/// The request option that keeps [ReadRetryInterceptor] out of one read: pass
/// `{noReadRetryExtra: true}` as `extra` of a generated operation whose caller retries by
/// itself, for example an outbox executor, so a refused read is not retried in two layers
/// (FIX-upload-retry-storm: the status read of an upload is retried with its entry).
const String noReadRetryExtra = 'api_client.no_read_retry';

/// How reads are retried: a full-jitter backoff from the caller's configuration keys.
///
/// The apps pass `client.outbox_retry_base` and `client.outbox_retry_cap` (docs/DECISIONS.md
/// D-M4-02-5); the client defines no number of its own. Writes are never retried here: the
/// outbox retries them (15 8).
class ReadRetryPolicy {
  /// Creates a policy with an explicit [maxAttempts] (the first try included).
  ///
  /// [sleep] waits (a fake clock in tests) and [random] draws the jitter; both default to the
  /// real ones. Throws an [ArgumentError] when [maxAttempts] is below 1.
  ReadRetryPolicy({
    required this.base,
    required this.cap,
    required this.maxAttempts,
    Future<void> Function(Duration)? sleep,
    Random? random,
  }) : sleep = sleep ?? Future<void>.delayed,
       _random = random ?? Random() {
    if (maxAttempts < 1) {
      throw ArgumentError.value(
        maxAttempts,
        'maxAttempts',
        'must be at least 1',
      );
    }
  }

  /// Creates a policy whose [maxAttempts] follows from [base] and [cap]: one attempt plus one
  /// retry per doubling of the bound `base * 2^n` until it reaches [cap].
  ///
  /// With base 2 s and cap 5 min that is nine attempts; a zero base or a base at or above the
  /// cap gives one attempt (no retry).
  ///
  /// Example:
  /// ```dart
  /// final policy = ReadRetryPolicy.fromBackoff(
  ///   base: config.clientOutboxRetryBase,
  ///   cap: config.clientOutboxRetryCap,
  /// );
  /// ```
  factory ReadRetryPolicy.fromBackoff({
    required Duration base,
    required Duration cap,
    Future<void> Function(Duration)? sleep,
    Random? random,
  }) {
    var retries = 0;
    if (base > Duration.zero) {
      while (base * pow(2, retries).toInt() < cap) {
        retries++;
      }
    }
    return ReadRetryPolicy(
      base: base,
      cap: cap,
      maxAttempts: retries + 1,
      sleep: sleep,
      random: random,
    );
  }

  /// Base of the backoff.
  final Duration base;

  /// Largest wait of the backoff.
  final Duration cap;

  /// Attempts of one read, the first included.
  final int maxAttempts;

  /// Waits the given time.
  final Future<void> Function(Duration) sleep;

  final Random _random;

  /// The full-jitter wait before retry number [retry] (0 for the first retry): uniform in
  /// `[0, min(cap, base * 2^retry)]`.
  Duration backoff(int retry) {
    final bound = min(
      cap.inMicroseconds,
      base.inMicroseconds * pow(2, min(retry, 40)).toInt(),
    );
    if (bound <= 0) return Duration.zero;
    return Duration(microseconds: (_random.nextDouble() * bound).floor());
  }
}

/// Retries `GET` and `HEAD` after a network error, a timeout, `429` or `503` (except
/// `503 setup_required`, which is final), waiting `Retry-After` plus the full-jitter backoff
/// when the response has it, so reads refused together do not come back together, and the
/// backoff alone otherwise. A request with [noReadRetryExtra] is never retried here.
///
/// It sits last in the chain, so it sees the mapped [ApiProblem], and a retry runs through
/// every interceptor again (a new span ID, a new DPoP proof from M4-09).
class ReadRetryInterceptor extends Interceptor {
  /// Creates the interceptor; [dio] sends the retries, [policy] decides when.
  ReadRetryInterceptor({required this.dio, required this.policy});

  /// The client that sends the retries, the one this interceptor belongs to.
  final Dio dio;

  /// The backoff and the attempt limit.
  final ReadRetryPolicy policy;

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final options = err.requestOptions;
    final attempt = (options.extra[_attemptExtra] as int?) ?? 1;
    if (attempt >= policy.maxAttempts || !_retryable(err)) {
      handler.next(err);
      return;
    }
    final retryAfter =
        ApiProblem.of(err)?.retryAfter ??
        parseRetryAfter(err.response?.headers.value('retry-after'));
    final jitter = policy.backoff(attempt - 1);
    // Retry-After is the floor (06 9 item 3); a date in the past counts as no wait.
    final wait = retryAfter == null || retryAfter.isNegative
        ? jitter
        : retryAfter + jitter;
    await policy.sleep(wait);
    options.extra[_attemptExtra] = attempt + 1;
    try {
      handler.resolve(await dio.fetch<Object?>(options));
    } on DioException catch (e) {
      handler.next(e);
    }
  }

  static bool _retryable(DioException err) {
    if (!_readMethods.contains(err.requestOptions.method.toUpperCase()) ||
        err.requestOptions.extra[noReadRetryExtra] == true) {
      return false;
    }
    switch (err.type) {
      case DioExceptionType.connectionError:
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
      case DioExceptionType.transformTimeout:
        return true;
      case DioExceptionType.badResponse:
        final status = err.response?.statusCode;
        if (status == 429) return true;
        return status == 503 && ApiProblem.of(err)?.code != 'setup_required';
      case DioExceptionType.badCertificate:
      case DioExceptionType.cancel:
      case DioExceptionType.unknown:
        return false;
    }
  }
}
