// SPDX-License-Identifier: Apache-2.0

/// The collector of the app metrics (docs/spec/15-client-architektur.md 17, R-CL-10; ADR 0057;
/// brief M4-31 Design note 1).
///
/// The app measures only the closed list of docs/observability/signals.md 4.6a: [ClientMetric]
/// names every entry with its kind, so a name outside the list does not compile. Histograms
/// use the bounds "durations up to seconds" of signals.md 4.7 ([histogramBounds]); a value above
/// the top bound counts in the top bucket, because a bucket carries only a numeric `le` and the
/// server accepts only these bounds. Nothing but the value, and for reconnects the close code,
/// is recorded; the attributes of a batch come from the build and the appearance when it is
/// sent ([MetricsAttributes]).
library;

/// The kind of an app metric (`ClientMetric.kind`).
enum MetricKind {
  /// Buckets with upper bounds and counts.
  histogram,

  /// An increment since the last batch.
  counter,

  /// The last value.
  gauge,
}

/// The app metrics of signals.md 4.6a; `client_metrics_test.dart` holds this enum equal to the
/// table.
enum ClientMetric {
  /// Cold start until a usable sidebar from the cache (seconds).
  coldStartDuration('client.cold_start.duration', MetricKind.histogram),

  /// Opening a thread from the cache until the first messages are visible (seconds).
  threadOpenDuration('client.thread_open.duration', MetricKind.histogram),

  /// Sending until the local display (seconds).
  sendLocalDuration('client.send_local.duration', MetricKind.histogram),

  /// Frames drawn.
  frames('client.frames', MetricKind.counter),

  /// Frames over the frame budget.
  framesOverBudget('client.frames.over_budget', MetricKind.counter),

  /// Duration of a catch-up (seconds).
  catchupDuration('client.catchup.duration', MetricKind.histogram),

  /// Events received during catch-up.
  catchupEvents('client.catchup.events', MetricKind.counter),

  /// `resync_required` handled.
  resync('client.resync', MetricKind.counter),

  /// Reconnects by the close code of the previous connection.
  reconnects('client.reconnects', MetricKind.counter),

  /// Entries of the outbox that are not final.
  outboxDepth('client.outbox.depth', MetricKind.gauge),

  /// Push until display (mobile apps, M5).
  pushDisplayDuration('client.push_display.duration', MetricKind.histogram),

  /// A placeholder instead of the push text (mobile apps, M5).
  pushPlaceholder('client.push.placeholder', MetricKind.counter);

  const ClientMetric(this.wireName, this.kind);

  /// The name in the app and on the wire.
  final String wireName;

  /// The kind.
  final MetricKind kind;
}

/// The bounds "durations up to seconds" of signals.md 4.7, the only bounds the server accepts
/// for an app histogram; `client_metrics_test.dart` holds them equal to the table.
const List<double> histogramBounds = [
  0.005,
  0.01,
  0.025,
  0.05,
  0.075,
  0.1,
  0.25,
  0.5,
  0.75,
  1,
  2.5,
  5,
  7.5,
  10,
];

/// One entry of a batch (schema `ClientMetric`): [value] for counters and gauges, [buckets]
/// (upper bound to count, only non-empty buckets) for histograms, [closeCode] only for
/// [ClientMetric.reconnects].
final class MetricEntry {
  /// Creates an entry.
  const MetricEntry(this.metric, {this.value, this.buckets, this.closeCode});

  /// The metric.
  final ClientMetric metric;

  /// The counter increment or gauge value.
  final double? value;

  /// The histogram buckets by upper bound.
  final Map<double, int>? buckets;

  /// The close code of a reconnect.
  final int? closeCode;

  /// The entry as JSON of the contract.
  Map<String, Object?> toJson() => {
    'name': metric.wireName,
    'kind': metric.kind.name,
    if (closeCode != null) 'close_code': closeCode,
    if (value != null) 'value': value,
    if (buckets != null)
      'buckets': [
        for (final b in buckets!.entries) {'le': b.key, 'count': b.value},
      ],
  };
}

/// The attributes of a batch: platform, app version, density and theme (signals.md 4.6a).
/// Nothing else describes the device or the person.
final class MetricsAttributes {
  /// Creates the attributes.
  const MetricsAttributes({
    required this.platform,
    required this.appVersion,
    this.density,
    this.theme,
  });

  /// `macos`, `windows`, `ios`, `android` or `web`.
  final String platform;

  /// The version of the build; the server keeps only the major version.
  final String appVersion;

  /// `comfortable`, `compact` or `touch`, or null.
  final String? density;

  /// `light` or `dark`, or null.
  final String? theme;
}

/// A batch of entries with its attributes (schema `ClientMetricsBatch`).
final class MetricsBatch {
  /// Creates a batch.
  const MetricsBatch(this.attributes, this.entries);

  /// The attributes.
  final MetricsAttributes attributes;

  /// The entries.
  final List<MetricEntry> entries;

  /// The batch as JSON of the contract.
  Map<String, Object?> toJson() => {
    'platform': attributes.platform,
    'app_version': attributes.appVersion,
    if (attributes.density != null) 'density': attributes.density,
    if (attributes.theme != null) 'theme': attributes.theme,
    'metrics': [for (final e in entries) e.toJson()],
  };
}

/// Collects the app metrics in memory until the sender takes them (brief M4-31 Design note 1).
///
/// Example:
/// ```dart
/// final metrics = ClientMetrics();
/// metrics.record(ClientMetric.threadOpenDuration, 0.12);
/// metrics.record(ClientMetric.reconnects, 1, closeCode: 4009);
/// final entries = metrics.take();
/// ```
final class ClientMetrics {
  final Map<ClientMetric, List<int>> _histograms = {};
  final Map<(ClientMetric, int?), double> _counters = {};
  final Map<ClientMetric, double> _gauges = {};

  /// Records [value] for [metric]: a histogram value in seconds, a counter increment or a
  /// gauge value. [closeCode] is kept only for [ClientMetric.reconnects]. A negative or
  /// non-finite value is ignored, and so is a negative counter increment.
  void record(ClientMetric metric, num value, {int? closeCode}) {
    final v = value.toDouble();
    if (!v.isFinite) return;
    switch (metric.kind) {
      case MetricKind.histogram:
        if (v < 0) return;
        final counts = _histograms.putIfAbsent(
          metric,
          () => List.filled(histogramBounds.length, 0),
        );
        var i = histogramBounds.indexWhere((b) => v <= b);
        // Above the top bound: the top bucket (Design note 1, the clamp of the README).
        if (i < 0) i = histogramBounds.length - 1;
        counts[i]++;
      case MetricKind.counter:
        if (v < 0) return;
        final key = (
          metric,
          metric == ClientMetric.reconnects ? closeCode : null,
        );
        _counters[key] = (_counters[key] ?? 0) + v;
      case MetricKind.gauge:
        _gauges[metric] = v;
    }
  }

  /// Records [elapsed] for a histogram in seconds.
  void recordDuration(ClientMetric metric, Duration elapsed) =>
      record(metric, elapsed.inMicroseconds / Duration.microsecondsPerSecond);

  /// Whether nothing was recorded since the last [take].
  bool get isEmpty =>
      _histograms.isEmpty && _counters.isEmpty && _gauges.isEmpty;

  /// Returns the entries recorded since the last call and starts empty again.
  List<MetricEntry> take() {
    final out = <MetricEntry>[
      for (final h in _histograms.entries)
        MetricEntry(
          h.key,
          buckets: {
            for (var i = 0; i < histogramBounds.length; i++)
              if (h.value[i] > 0) histogramBounds[i]: h.value[i],
          },
        ),
      for (final c in _counters.entries)
        MetricEntry(c.key.$1, value: c.value, closeCode: c.key.$2),
      for (final g in _gauges.entries) MetricEntry(g.key, value: g.value),
    ];
    _histograms.clear();
    _counters.clear();
    _gauges.clear();
    return out;
  }
}
