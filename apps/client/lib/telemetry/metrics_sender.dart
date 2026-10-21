// SPDX-License-Identifier: Apache-2.0

/// Sends the app metrics in batches (docs/spec/15-client-architecture.md 17; ADR 0057; brief
/// M4-31 Design note 2).
///
/// Every `client.metrics.flush_interval` and when the app stops, [MetricsSender] takes what
/// [ClientMetrics] collected and, while `privacy.client_metrics` is on, sends it with
/// `submitClientMetrics`, split at [clientMetricsBatchMaxItems] entries. It never goes through
/// the outbox: telemetry is not user data, and the operation is exempt from idempotency keys.
/// A network error keeps the newest batch for the next interval (never more than one), any
/// other error drops the batch, and `403 feature_disabled` stops the sender until the setting changes
/// (docs/DECISIONS.md D-M4-31-2).
library;

import 'dart:async';
import 'dart:io' show SocketException;

import 'package:api_client/api_client.dart' as rest;
import 'package:dio/dio.dart';

import 'client_metrics.dart';

/// `maxItems` of `ClientMetricsBatch.metrics` in docs/api/openapi.yaml; `client_metrics_test.dart`
/// holds it equal to the contract, so the limit has one source.
const int clientMetricsBatchMaxItems = 200;

/// The port to the server: sends one batch; throws what the transport throws.
abstract interface class ClientMetricsApi {
  /// Sends [batch] with `submitClientMetrics`.
  Future<void> submit(MetricsBatch batch);
}

/// [ClientMetricsApi] over the REST client of the signed-in session (DPoP and the access
/// token come from its interceptors).
final class RestClientMetricsApi implements ClientMetricsApi {
  /// Creates the adapter over [api].
  RestClientMetricsApi(this._api);

  final rest.ApiClient _api;

  @override
  Future<void> submit(MetricsBatch batch) async {
    await _api.getStatsApi().submitClientMetrics(
      clientMetricsBatch: toApiBatch(batch),
    );
  }

  /// Converts [batch] into the generated model of the contract.
  static rest.ClientMetricsBatch toApiBatch(MetricsBatch batch) {
    final a = batch.attributes;
    return rest.ClientMetricsBatch((b) {
      b
        ..platform = rest.ClientMetricsBatchPlatformEnum.valueOf(a.platform)
        ..appVersion = a.appVersion
        ..density = a.density == null
            ? null
            : rest.ClientMetricsBatchDensityEnum.valueOf(a.density!)
        ..theme = a.theme == null
            ? null
            : rest.ClientMetricsBatchThemeEnum.valueOf(a.theme!);
      b.metrics.addAll([
        for (final e in batch.entries)
          rest.ClientMetric((m) {
            m
              ..name = e.metric.wireName
              ..kind = rest.ClientMetricKindEnum.valueOf(e.metric.kind.name)
              ..closeCode = e.closeCode
              ..value = e.value;
            final buckets = e.buckets;
            if (buckets != null) {
              m.buckets.addAll([
                for (final bk in buckets.entries)
                  rest.ClientMetricBucketsInner(
                    (x) => x
                      ..le = bk.key
                      ..count = bk.value,
                  ),
              ]);
            }
          }),
      ]);
    });
  }
}

/// Creates the periodic timer of the sender; tests pass a timer they fire by hand.
typedef MetricsTimerFactory = Timer Function(
  Duration interval,
  void Function(Timer) tick,
);

/// Sends the collected metrics at most once per interval (Design note 2).
///
/// Example:
/// ```dart
/// final sender = MetricsSender(metrics: metrics, api: RestClientMetricsApi(auth.api),
///     interval: config.metricsFlushInterval, enabled: () => settings.clientMetrics,
///     attributes: () => attributes)..start();
/// ```
final class MetricsSender {
  /// Creates the sender over [metrics] and [api]; [interval] is
  /// `client.metrics.flush_interval`, [enabled] reads `privacy.client_metrics`, [attributes]
  /// the platform, version, density and theme at the moment of sending. [log] receives one
  /// line per failed send, never a value.
  MetricsSender({
    required this._metrics,
    required this._api,
    required this._interval,
    required this._enabled,
    required this._attributes,
    MetricsTimerFactory? timer,
    this._log,
  }) : _timerFactory = timer ?? Timer.periodic;

  final ClientMetrics _metrics;
  final ClientMetricsApi _api;
  final Duration _interval;
  final bool Function() _enabled;
  final MetricsAttributes Function() _attributes;
  final MetricsTimerFactory _timerFactory;
  final void Function(String line)? _log;

  Timer? _timer;
  MetricsBatch? _pending;
  bool _stoppedByServer = false;
  Future<void>? _running;

  /// Whether the server answered `403 feature_disabled`; nothing is sent until
  /// [settingChanged].
  bool get stoppedByServer => _stoppedByServer;

  /// Starts the timer; a second call does nothing.
  void start() {
    _timer ??= _timerFactory(_interval, (_) => unawaited(flush()));
  }

  /// `privacy.client_metrics` changed: a stop after `403 feature_disabled` ends.
  void settingChanged() => _stoppedByServer = false;

  /// Sends what was collected since the last flush; a flush already running is awaited
  /// instead of starting a second one.
  Future<void> flush() =>
      _running ??= _flush().whenComplete(() => _running = null);

  /// Sends what is left and stops the timer (the app quits or signs out).
  Future<void> stop() async {
    _timer?.cancel();
    await flush();
  }

  Future<void> _flush() async {
    final entries = _metrics.take();
    if (!_enabled() || _stoppedByServer) {
      // The person turned it off, or the server did: nothing is kept for later (01 11).
      _pending = null;
      return;
    }
    final batches = <MetricsBatch>[?_pending];
    _pending = null;
    if (entries.isNotEmpty) {
      final attributes = _attributes();
      for (var i = 0; i < entries.length; i += clientMetricsBatchMaxItems) {
        final end = i + clientMetricsBatchMaxItems;
        batches.add(
          MetricsBatch(
            attributes,
            entries.sublist(i, end < entries.length ? end : entries.length),
          ),
        );
      }
    }
    for (final batch in batches) {
      try {
        await _api.submit(batch);
      } on DioException catch (e) {
        if (_isNetwork(e)) {
          // The rest of this flush would fail the same way: the newest batch waits for the
          // next interval, every older one is dropped (one batch, never more).
          _pending = batches.last;
          _log?.call('client metrics: network error, one batch kept');
          return;
        }
        final problem =
            rest.ApiProblem.of(e) ??
            (e.response == null
                ? null
                : rest.ApiProblem.fromResponse(e.response!));
        if (problem?.code == 'feature_disabled') {
          _stoppedByServer = true;
          _log?.call('client metrics: feature_disabled, stopped');
          return;
        }
        _log?.call('client metrics: batch dropped: ${e.type.name}');
      } on Object catch (e) {
        _log?.call('client metrics: batch dropped: ${e.runtimeType}');
      }
    }
  }

  static bool _isNetwork(DioException e) =>
      e.response == null &&
      (e.type == DioExceptionType.connectionError ||
          e.type == DioExceptionType.connectionTimeout ||
          e.type == DioExceptionType.sendTimeout ||
          e.type == DioExceptionType.receiveTimeout ||
          e.error is SocketException);
}
