// SPDX-License-Identifier: Apache-2.0

/// The sections of the statistics screen A7 as plain data: the rows of each chart and the table
/// with the same values (docs/spec/25-visualization.md 7.4; brief M6-06 Design notes 1 and 2).
///
/// Nothing here computes a statistic: a section lists the server's buckets, sums counters and
/// cost amounts of one mode over the buckets of the range (the totals of the server's own
/// numbers) and formats them. Rates, percentiles and counts of people are never summed or
/// averaged; they appear per bucket as the server delivered them. No row names a person.
library;

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart' show dsReasonText;
import 'package:l10n/l10n.dart';

import 'stats_range.dart';
import 'stats_source.dart';

/// One part of a bar: its [value] (the length), the [text] of the value and its color, either a
/// run [state] (completed, failed, cancelled) or a position in the series roles [colorIndex].
final class StatsBarPart {
  /// Creates the part; exactly one of [state] and [colorIndex] colors it.
  const StatsBarPart({
    required this.value,
    required this.text,
    this.state,
    this.colorIndex,
  }) : assert((state == null) != (colorIndex == null), 'one color');

  /// The length of the part.
  final double value;

  /// The value as the table shows it.
  final String text;

  /// The run state of a part of "Runs by result".
  final api.RunState? state;

  /// The series role of any other part.
  final int? colorIndex;
}

/// One row of a section chart: its [label] and its parts, stacked.
final class StatsBarRow {
  /// Creates the row.
  const StatsBarRow({required this.label, required this.parts});

  /// The row label, for example the agent name.
  final String label;

  /// The parts from left to right.
  final List<StatsBarPart> parts;

  /// The label with the text of every part, which the chart shows and screen readers read.
  String get text => [label, for (final p in parts) p.text].join(', ');
}

/// One section of A7: a [title], the chart [rows], the [table] with the same values and an
/// optional [note] below the chart. A section without rows shows "No data in this range".
final class StatsSectionData {
  /// Creates the section.
  const StatsSectionData({
    required this.key,
    required this.title,
    required this.rows,
    required this.table,
    this.note,
  });

  /// The key of the section in the widget tree, for example `runs`.
  final String key;

  /// The heading.
  final String title;

  /// The chart rows.
  final List<StatsBarRow> rows;

  /// The table view.
  final VizTableData table;

  /// A note below the chart, or null.
  final String? note;
}

/// The texts and formats the sections need: the locale's [l10n] and [format], the time zone
/// and current time for the bucket labels, and the names of agents and spaces.
final class StatsTexts {
  /// Creates the texts.
  StatsTexts({
    required this.l10n,
    required this.format,
    required this.timeZone,
    required this.bucketZone,
    required this.now,
    required this.agentNames,
    required this.spaceName,
  });

  /// The interface texts.
  final AppLocalizations l10n;

  /// Numbers, money and times.
  final L10nFormat format;

  /// The time zone of the signed-in person, for hour buckets (instants).
  final String timeZone;

  /// The server's `stats.time_zone`, in which it starts day, week and month buckets; their
  /// labels name the calendar day of this zone.
  final String bucketZone;

  /// The current time, for "Today" and "Yesterday".
  final DateTime now;

  /// The display name per agent ID; an unknown agent shows its ID.
  final Map<String, String> agentNames;

  /// The name of a space by its ID.
  final String Function(String spaceId) spaceName;

  /// The name of the agent [id].
  String agent(String? id) => id == null ? '' : agentNames[id] ?? id;

  /// The last calendar day of a range of whole [days] that ends at [to] (exclusive).
  /// A day is not always 24 hours long, so the day is the one that holds the last instant.
  String lastDay(DateTime to, StatsDays days) => bucket(
    days.start(days.dayOf(to.subtract(const Duration(microseconds: 1)))),
    StatsBucketSize.day,
  );

  /// The start of a bucket: the day ("Today", a weekday, a date), with the time for hours.
  String bucket(DateTime start, StatsBucketSize size) {
    String day(String zone) =>
        format.relativeDay(start, now: now, timeZone: zone);
    String time(String zone) => format.time(start, timeZone: zone);
    try {
      return size == StatsBucketSize.hour
          ? '${day(timeZone)} ${time(timeZone)}'
          : day(bucketZone);
    } on ArgumentError {
      // An unknown zone of the person shows the instant in UTC rather than nothing.
      return size == StatsBucketSize.hour
          ? '${day('UTC')} ${time('UTC')}'
          : day('UTC');
    }
  }

  /// A count.
  String count(num v) => format.count(v.round());

  /// A share given as a fraction (0 to 1), as the server delivers rates.
  String rate(num v) => format.percent(v * 100);

  /// Milliseconds.
  String millis(num v) => '${format.count(v.round())} ms';

  /// A cost amount in micro units of [currency].
  String money(int micros, String currency) =>
      format.money(micros / 1000000, currency);

  /// A number of bytes in B, KB, MB or GB.
  String bytes(num value) {
    const unit = 1024;
    final b = value.round();
    String one(double v) => _decimal(v);
    if (b < unit) return l10n.fileSizeBytes('$b');
    if (b < unit * unit) return l10n.fileSizeKilobytes(one(b / unit));
    if (b < unit * unit * unit) {
      return l10n.fileSizeMegabytes(one(b / (unit * unit)));
    }
    return l10n.fileSizeGigabytes(one(b / (unit * unit * unit)));
  }

  String _decimal(double v) {
    final text = v.toStringAsFixed(v >= 100 ? 0 : 1);
    return format.locale.languageCode == 'en'
        ? text
        : text.replaceAll('.', ',');
  }

  /// The billing mode.
  String mode(api.BillingMode mode) => mode == api.BillingMode.organization
      ? l10n.vizBadgeOrganization
      : l10n.vizBadgePersonal;

  /// The cost label of a sum of [mode]: `personal` is always "information only",
  /// `organization` "Estimate" while any summed entry is one (admin-statistics.md 2).
  String? costLabel(api.BillingMode mode, {required bool estimate}) =>
      mode == api.BillingMode.organization
      ? (estimate ? l10n.vizCostEstimate : null)
      : l10n.vizCostInformationOnly;

  /// The label of a metric of `getErrorStats`.
  String errorMetric(String name) => switch (name) {
    'run_failure_rate' => l10n.statsErrorRunFailure,
    'invocation_denials' => l10n.statsErrorInvocationDenials,
    'webhook_failure_rate' => l10n.statsErrorWebhookFailure,
    'scan_errors' => l10n.statsErrorScanErrors,
    'scan_infected' => l10n.statsErrorScanInfected,
    _ when name.startsWith('push_failure_rate_') => l10n.statsErrorPushFailure(
      name.substring('push_failure_rate_'.length),
    ),
    _ => l10n.vizMetricUnknown(name),
  };

  /// The text of a failure reason code of a run.
  String reason(String code) => dsReasonText(l10n, code) ?? code;
}

/// The cell of [value] formatted by [text], sorted by the number.
VizCell _cell(num value, String text) => VizCell(text, sortBy: value);

/// The total of a counter [metric] per group over every bucket of [series], in the order of
/// first appearance.
Map<String?, double> _totals(api.StatsSeries series, String metric) {
  final out = <String?, double>{};
  for (final b in series.buckets) {
    final v = b.metrics[metric];
    if (v == null) continue;
    out[b.groupKey] = (out[b.groupKey] ?? 0) + v;
  }
  return out;
}

/// "Runs by result": per bucket the completed, failed and cancelled runs of every agent
/// together, stacked; the table lists the same per bucket.
StatsSectionData runsSection(StatsTexts t, api.StatsSeries runs) {
  final l = t.l10n;
  final size = _size(runs.granularity);
  const results = [
    ('completed', api.RunState.completed),
    ('failed', api.RunState.failed),
    ('cancelled', api.RunState.cancelled),
  ];
  final byStart = <DateTime, Map<String, double>>{};
  for (final b in runs.buckets) {
    final sums = byStart.putIfAbsent(b.start, () => {});
    for (final (metric, _) in results) {
      final v = b.metrics[metric];
      if (v != null) sums[metric] = (sums[metric] ?? 0) + v;
    }
  }
  final starts = byStart.keys.toList()..sort();
  final rows = <StatsBarRow>[];
  final cells = <List<VizCell>>[];
  for (final start in starts) {
    final sums = byStart[start]!;
    final label = t.bucket(start, size);
    rows.add(
      StatsBarRow(
        label: label,
        parts: [
          for (final (metric, state) in results)
            StatsBarPart(
              value: sums[metric] ?? 0,
              text: '${_stateText(l, state)} ${t.count(sums[metric] ?? 0)}',
              state: state,
            ),
        ],
      ),
    );
    cells.add([
      VizCell(label, sortBy: start.microsecondsSinceEpoch),
      for (final (metric, _) in results)
        _cell(sums[metric] ?? 0, t.count(sums[metric] ?? 0)),
    ]);
  }
  return StatsSectionData(
    key: 'runs',
    title: l.statsSectionRuns,
    rows: rows,
    table: VizTableData(
      caption: l.statsSectionRuns,
      columns: [
        l.statsColumnPeriod,
        l.liveBoxStateCompleted,
        l.liveBoxStateFailed,
        l.liveBoxStateCancelled,
      ],
      rows: cells,
    ),
  );
}

String _stateText(AppLocalizations l, api.RunState state) => switch (state) {
  api.RunState.completed => l.liveBoxStateCompleted,
  api.RunState.failed => l.liveBoxStateFailed,
  _ => l.liveBoxStateCancelled,
};

/// "Run duration per agent": the chart shows p50 and p95 of the latest bucket of each agent as
/// `getRunStats` delivers them (a percentile of the range cannot be built from the buckets); the
/// table lists every bucket.
StatsSectionData durationSection(StatsTexts t, api.StatsSeries runs) {
  final l = t.l10n;
  final size = _size(runs.granularity);
  final latest = <String?, api.StatsBucket>{};
  final cells = <List<VizCell>>[];
  for (final b in runs.buckets) {
    final p50 = b.metrics['duration_ms_p50'];
    final p95 = b.metrics['duration_ms_p95'];
    if (p50 == null && p95 == null) continue;
    final known = latest[b.groupKey];
    if (known == null || b.start.isAfter(known.start)) latest[b.groupKey] = b;
    cells.add([
      VizCell(t.agent(b.groupKey)),
      VizCell(t.bucket(b.start, size), sortBy: b.start.microsecondsSinceEpoch),
      p50 == null ? VizCell(l.vizNotShown) : _cell(p50, t.millis(p50)),
      p95 == null ? VizCell(l.vizNotShown) : _cell(p95, t.millis(p95)),
    ]);
  }
  final rows = <StatsBarRow>[];
  for (final MapEntry(key: agent, value: b) in latest.entries) {
    final label = '${t.agent(agent)}, ${t.bucket(b.start, size)}';
    for (final (name, metric, color) in [
      ('p50', 'duration_ms_p50', 0),
      ('p95', 'duration_ms_p95', 1),
    ]) {
      final v = b.metrics[metric];
      if (v == null) continue;
      rows.add(
        StatsBarRow(
          label: '$label, $name',
          parts: [StatsBarPart(value: v, text: t.millis(v), colorIndex: color)],
        ),
      );
    }
  }
  return StatsSectionData(
    key: 'duration',
    title: l.statsSectionDuration,
    rows: rows,
    note: rows.isEmpty ? null : l.statsDurationNote,
    table: VizTableData(
      caption: l.statsSectionDuration,
      columns: [
        l.statsColumnAgent,
        l.statsColumnPeriod,
        l.statsColumnMedian,
        l.statsColumnP95,
      ],
      rows: cells,
    ),
  );
}

/// The cost sums per group and billing mode of [costs]; the modes are never added together.
List<({String? group, api.BillingMode mode, int micros, bool estimate})>
costSums(api.CostSeries costs) {
  final micros = <(String?, api.BillingMode), int>{};
  final estimate = <(String?, api.BillingMode), bool>{};
  for (final b in costs.buckets) {
    final key = (b.groupKey, b.billingMode);
    micros[key] = (micros[key] ?? 0) + b.amountMicros;
    estimate[key] = (estimate[key] ?? false) || b.isEstimate;
  }
  return [
    for (final MapEntry(key: (group, mode), value: sum) in micros.entries)
      (
        group: group,
        mode: mode,
        micros: sum,
        estimate: estimate[(group, mode)]!,
      ),
  ];
}

/// "Costs per agent" or "Costs per space": one row per group and mode, `organization` and
/// `personal` as separate series with their labels.
StatsSectionData costSection(
  StatsTexts t,
  api.CostSeries costs, {
  required bool byAgent,
}) {
  final l = t.l10n;
  final title = byAgent ? l.statsSectionCostsAgent : l.statsSectionCostsSpace;
  final rows = <StatsBarRow>[];
  final cells = <List<VizCell>>[];
  for (final s in costSums(costs)) {
    final name = byAgent
        ? t.agent(s.group)
        : (s.group == null ? '' : t.spaceName(s.group!));
    final label = t.costLabel(s.mode, estimate: s.estimate);
    final amount = t.money(s.micros, costs.currency);
    final text = label == null ? amount : '$amount, $label';
    rows.add(
      StatsBarRow(
        label: '$name, ${t.mode(s.mode)}',
        parts: [
          StatsBarPart(
            value: s.micros.toDouble(),
            text: text,
            colorIndex: s.mode == api.BillingMode.organization ? 0 : 1,
          ),
        ],
      ),
    );
    cells.add([VizCell(name), VizCell(t.mode(s.mode)), _cell(s.micros, text)]);
  }
  return StatsSectionData(
    key: byAgent ? 'costs-agent' : 'costs-space',
    title: title,
    rows: rows,
    table: VizTableData(
      caption: title,
      columns: [
        byAgent ? l.statsColumnAgent : l.statsColumnSpace,
        l.statsColumnMode,
        l.statsColumnAmount,
      ],
      rows: cells,
    ),
  );
}

/// "Failure reasons": the failed runs per `state_reason` code over the range and every agent.
StatsSectionData failuresSection(StatsTexts t, api.StatsSeries runs) {
  final l = t.l10n;
  const prefix = 'failed_reason_';
  final sums = <String, double>{};
  for (final b in runs.buckets) {
    for (final MapEntry(:key, :value) in b.metrics.entries) {
      if (!key.startsWith(prefix)) continue;
      final code = key.substring(prefix.length);
      sums[code] = (sums[code] ?? 0) + value;
    }
  }
  final codes = sums.keys.toList()
    ..sort((a, b) => sums[b]!.compareTo(sums[a]!));
  return StatsSectionData(
    key: 'failures',
    title: l.statsSectionFailures,
    rows: [
      for (final code in codes)
        StatsBarRow(
          label: t.reason(code),
          parts: [
            StatsBarPart(
              value: sums[code]!,
              text: t.count(sums[code]!),
              colorIndex: 0,
            ),
          ],
        ),
    ],
    table: VizTableData(
      caption: l.statsSectionFailures,
      columns: [l.statsColumnReason, l.statsColumnCount],
      rows: [
        for (final code in codes)
          [VizCell(t.reason(code)), _cell(sums[code]!, t.count(sums[code]!))],
      ],
    ),
  );
}

/// "Tool calls by tool": calls and failed calls per tool name over the range.
StatsSectionData toolsSection(StatsTexts t, api.StatsSeries tools) {
  final l = t.l10n;
  final calls = _totals(tools, 'calls');
  final failed = _totals(tools, 'failed');
  final names = calls.keys.toList()
    ..sort((a, b) => calls[b]!.compareTo(calls[a]!));
  return StatsSectionData(
    key: 'tools',
    title: l.statsSectionTools,
    rows: [
      for (final name in names)
        StatsBarRow(
          label: name ?? '',
          parts: [
            StatsBarPart(
              value: calls[name]! - (failed[name] ?? 0),
              text: '${l.statsColumnCalls} ${t.count(calls[name]!)}',
              colorIndex: 0,
            ),
            StatsBarPart(
              value: failed[name] ?? 0,
              text: '${l.statsColumnFailed} ${t.count(failed[name] ?? 0)}',
              colorIndex: 1,
            ),
          ],
        ),
    ],
    table: VizTableData(
      caption: l.statsSectionTools,
      columns: [l.statsColumnTool, l.statsColumnCalls, l.statsColumnFailed],
      rows: [
        for (final name in names)
          [
            VizCell(name ?? ''),
            _cell(calls[name]!, t.count(calls[name]!)),
            _cell(failed[name] ?? 0, t.count(failed[name] ?? 0)),
          ],
      ],
    ),
  );
}

/// "Error rates": every rate per bucket as the server delivered it; counts beside them.
StatsSectionData errorsSection(StatsTexts t, api.StatsSeries errors) {
  final l = t.l10n;
  final size = _size(errors.granularity);
  final rows = <StatsBarRow>[];
  final cells = <List<VizCell>>[];
  final buckets = errors.buckets.toList()
    ..sort((a, b) => a.start.compareTo(b.start));
  for (final b in buckets) {
    final when = t.bucket(b.start, size);
    final names = b.metrics.keys.toList()..sort();
    for (final (i, name) in names.indexed) {
      final v = b.metrics[name]!;
      final isRate = name.contains('_rate');
      final text = isRate ? t.rate(v) : t.count(v);
      cells.add([
        VizCell(when, sortBy: b.start.microsecondsSinceEpoch),
        VizCell(t.errorMetric(name)),
        _cell(v, text),
      ]);
      // Rates share one axis (0 to 1); counts are only in the table.
      if (isRate) {
        rows.add(
          StatsBarRow(
            label: '$when, ${t.errorMetric(name)}',
            parts: [StatsBarPart(value: v, text: text, colorIndex: i)],
          ),
        );
      }
    }
  }
  return StatsSectionData(
    key: 'errors',
    title: l.statsSectionErrors,
    rows: rows,
    table: VizTableData(
      caption: l.statsSectionErrors,
      columns: [l.statsColumnPeriod, l.statsColumnMetric, l.statsColumnValue],
      rows: cells,
    ),
  );
}

/// "Usage per space": the chart shows the messages per space over the range; the table lists
/// every week with messages, threads, runs, active people ("fewer than the minimum group" where
/// the server withheld the value, `below_min_group`) and the storage of the last week.
StatsSectionData usageSection(StatsTexts t, api.StatsSeries usage) {
  final l = t.l10n;
  final size = _size(usage.granularity);
  final messages = _totals(usage, 'messages');
  final cells = <List<VizCell>>[];
  final buckets = usage.buckets.toList()
    ..sort((a, b) => a.start.compareTo(b.start));
  VizCell countCell(api.StatsBucket b, String metric) {
    if (b.belowMinGroup?.contains(metric) ?? false) {
      return VizCell(l.statsFewerThanMinimum, sortBy: -1);
    }
    final v = b.metrics[metric];
    return v == null ? VizCell(l.vizNotShown) : _cell(v, t.count(v));
  }

  for (final b in buckets) {
    final storage = b.metrics['storage_bytes'];
    cells.add([
      VizCell(b.groupKey == null ? '' : t.spaceName(b.groupKey!)),
      VizCell(t.bucket(b.start, size), sortBy: b.start.microsecondsSinceEpoch),
      countCell(b, 'messages'),
      countCell(b, 'threads_created'),
      countCell(b, 'threads_done'),
      countCell(b, 'runs'),
      countCell(b, 'active_people'),
      storage == null ? VizCell('') : _cell(storage, t.bytes(storage)),
    ]);
  }
  return StatsSectionData(
    key: 'usage',
    title: l.statsSectionUsage,
    rows: [
      for (final MapEntry(key: space, value: v) in messages.entries)
        StatsBarRow(
          label: space == null ? '' : t.spaceName(space),
          parts: [
            StatsBarPart(
              value: v,
              text: '${l.statsColumnMessages} ${t.count(v)}',
              colorIndex: 0,
            ),
          ],
        ),
    ],
    table: VizTableData(
      caption: l.statsSectionUsage,
      columns: [
        l.statsColumnSpace,
        l.statsColumnPeriod,
        l.statsColumnMessages,
        l.statsColumnThreadsCreated,
        l.statsColumnThreadsDone,
        l.statsColumnRuns,
        l.statsColumnActivePeople,
        l.statsColumnStorage,
      ],
      rows: cells,
    ),
  );
}

/// The total of `runs` over every bucket and agent of [runs], for the run tile.
double runTotal(api.StatsSeries runs) =>
    _totals(runs, 'runs').values.fold(0, (a, b) => a + b);

StatsBucketSize _size(api.StatsGranularity g) => switch (g) {
  api.StatsGranularity.hour => StatsBucketSize.hour,
  api.StatsGranularity.week => StatsBucketSize.week,
  api.StatsGranularity.month => StatsBucketSize.month,
  _ => StatsBucketSize.day,
};
