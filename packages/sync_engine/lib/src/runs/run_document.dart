// SPDX-License-Identifier: Apache-2.0

/// `RunDocument`: a read-only view over a run document (`run_document.v1`, 04 11.1) with typed
/// getters for what the live box shows (09 5.4).
library;

import 'dart:collection';

import 'package:domain/domain.dart';

/// Status of a plan item (API `RunPlan.items[].status`, schema `run_plan.v1`).
enum PlanItemStatus implements Coded {
  /// Not started.
  pending('pending'),

  /// Being worked on.
  inProgress('in_progress'),

  /// Done.
  completed('completed'),

  /// A value of a newer server.
  unknown('');

  const PlanItemStatus(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static PlanItemStatus fromCode(String code) => _decode(values, code, unknown);
}

/// State of a follow-up message of a run (API `RunFollowup.state`, 03 `run_followups`).
enum FollowupState implements Coded {
  /// Waiting for delivery.
  queued('queued'),

  /// Delivered to the adapter.
  delivered('delivered'),

  /// Taken into the running work.
  absorbed('absorbed'),

  /// Kept for after the run.
  deferred('deferred'),

  /// Dropped.
  discarded('discarded'),

  /// A value of a newer server.
  unknown('');

  const FollowupState(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static FollowupState fromCode(String code) => _decode(values, code, unknown);
}

T _decode<T extends Coded>(List<T> values, String code, T unknown) {
  for (final value in values) {
    if (code.isNotEmpty && value.code == code) return value;
  }
  return unknown;
}

/// The last retry of a model API call (API `RunApiRetry`), shown while set (09 5.4).
final class RunApiRetryView {
  const RunApiRetryView._({
    required this.attempt,
    this.maxRetries,
    this.retryDelayMs,
    this.error,
  });

  /// Number of the attempt.
  final int attempt;

  /// Highest number of attempts, when the runtime reports it.
  final int? maxRetries;

  /// Wait before the next attempt in milliseconds.
  final int? retryDelayMs;

  /// Error code of the provider, for example `overloaded`.
  final String? error;
}

/// Metrics of a run (API `RunMetrics`, schema `run_metrics.v1`); every field is optional.
final class RunMetricsView {
  RunMetricsView._(Map<String, Object?> raw)
    : contextPct = _num(raw['context_pct'])?.toDouble(),
      tokensIn = _int(raw['tokens_in']),
      tokensOut = _int(raw['tokens_out']),
      tokensCacheRead = _int(raw['tokens_cache_read']),
      tokensCacheWrite = _int(raw['tokens_cache_write']),
      tokensCacheWrite1h = _int(raw['tokens_cache_write_1h']),
      subagents = _int(raw['subagents']),
      backgroundJobs = _int(raw['background_jobs']),
      apiRetries = _int(raw['api_retries']),
      lastApiRetry = _retry(raw['last_api_retry']);

  /// Used share of the context window in percent.
  final double? contextPct;

  /// Input tokens.
  final int? tokensIn;

  /// Output tokens.
  final int? tokensOut;

  /// Tokens read from the prompt cache.
  final int? tokensCacheRead;

  /// Cache writes with 5 minutes (or without a stated duration).
  final int? tokensCacheWrite;

  /// Cache writes with 1 hour.
  final int? tokensCacheWrite1h;

  /// Number of subagents the runtime reports.
  final int? subagents;

  /// Number of background jobs the runtime reports.
  final int? backgroundJobs;

  /// Number of API retries so far.
  final int? apiRetries;

  /// The last API retry while it is set (`metrics.last_api_retry`).
  final RunApiRetryView? lastApiRetry;

  static RunApiRetryView? _retry(Object? raw) {
    if (raw is! Map<String, Object?>) return null;
    final attempt = _int(raw['attempt']);
    if (attempt == null) return null;
    return RunApiRetryView._(
      attempt: attempt,
      maxRetries: _int(raw['max_retries']),
      retryDelayMs: _int(raw['retry_delay_ms']),
      error: _text(raw['error']),
    );
  }
}

/// An item of the plan of a run (API `RunPlan.items[]`).
final class RunPlanItemView {
  const RunPlanItemView._(this.id, this.text, this.status);

  /// ID of the item, chosen by the adapter.
  final String id;

  /// Text of the item.
  final String text;

  /// Status of the item.
  final PlanItemStatus status;
}

/// A follow-up message of a run (API `RunFollowup`).
final class RunFollowupView {
  const RunFollowupView._(this.messageId, this.state);

  /// ID of the follow-up message; the key of the entry in `followups`.
  final String messageId;

  /// Delivery state.
  final FollowupState state;
}

/// A step of a run (API `RunStep`): tool call, status, subagent, workflow or background job.
final class RunStepView {
  RunStepView._(String key, Map<String, Object?> raw)
    : id = _text(raw['id']) ?? key,
      ordinal = _int(raw['ordinal']),
      parentStepId = _text(raw['parent_step_id']),
      kind = StepKind.fromCode(_text(raw['kind']) ?? ''),
      name = _text(raw['name']),
      title = _text(raw['title']),
      status = StepStatus.fromCode(_text(raw['status']) ?? ''),
      detail = _map(raw['detail']),
      startedAt = _time(raw['started_at']),
      finishedAt = _time(raw['finished_at']);

  /// ID of the step; the key in `steps` when the step carries no `id` field.
  final String id;

  /// Position assigned by the server; null while a step has none yet.
  final int? ordinal;

  /// The enclosing step (for example the subagent of a tool call).
  final String? parentStepId;

  /// Kind of the step.
  final StepKind kind;

  /// Technical name, for example the tool name.
  final String? name;

  /// Title shown in the live box.
  final String? title;

  /// Status of the step.
  final StepStatus status;

  /// Details (schema `run_step_detail.v1`), read-only.
  final Map<String, Object?>? detail;

  /// Start time.
  final DateTime? startedAt;

  /// End time.
  final DateTime? finishedAt;

  /// Whether the step has not ended yet (`pending` or `running`).
  bool get isOpen =>
      status == StepStatus.pending || status == StepStatus.running;
}

/// A step with its child steps, both in `ordinal` order.
final class RunStepNode {
  RunStepNode._(this.step, List<RunStepNode> children)
    : children = List.unmodifiable(children);

  /// The step.
  final RunStepView step;

  /// Steps whose `parent_step_id` is this step, ordered by `ordinal`.
  final List<RunStepNode> children;
}

/// A read-only view over one run document (`run_document.v1`, API schema `Run`).
///
/// The getters read the fields the live box shows (09 5.4); a field that is missing or has the
/// wrong type reads as null, empty or `unknown`, never as an error, because the document comes
/// from the server and a newer server may add fields (04 6.1). Unknown fields stay in [raw].
///
/// Instances are immutable: the store creates a new one for every applied change, so a widget
/// compares two states by identity.
///
/// Example:
/// ```dart
/// final d = RunDocument.fromJson(decodedBody);
/// final tool = d.currentTool?.title;
/// ```
final class RunDocument {
  RunDocument._(Map<String, Object?> json, this.version)
    : raw = UnmodifiableMapView(json);

  /// Wraps the decoded document [json] without copying it.
  ///
  /// [json] must not be changed afterwards; the store hands in maps that only it holds.
  /// Throws a [FormatException] when `version` is not an integer, because the client rule of
  /// 04 11.3 item 4 cannot place a document without one.
  factory RunDocument.fromJson(Map<String, Object?> json) {
    final version = json['version'];
    if (version is! int) {
      throw const FormatException('run document without an integer version');
    }
    return RunDocument._(json, ResourceVersion(version));
  }

  /// The document as a map, including fields this version does not know.
  ///
  /// The top level cannot be changed through it; nested maps and lists must not be changed
  /// either, because the next state shares them (copy on write of `applyRunPatch`).
  final Map<String, Object?> raw;

  /// Version of the document (`runs.version`, 04 9 item 3).
  final ResourceVersion version;

  /// ID of the run; null when `id` is missing or no UUID.
  RunId? get id {
    final text = _text(raw['id']);
    if (text == null) return null;
    try {
      return RunId.parse(text);
    } on FormatException {
      return null;
    }
  }

  /// State of the run (09 5.1); `unknown` for a missing or newer value.
  RunState get state => RunState.fromCode(_text(raw['state']) ?? '');

  /// Reason code of the state (09 5.2).
  String? get stateReason => _text(raw['state_reason']);

  /// Current status line.
  String? get statusLine => _text(raw['status_line']);

  /// The last text of the agent, already truncated by the server.
  String? get lastTextExcerpt => _text(raw['last_text_excerpt']);

  /// Model name the adapter reported.
  String? get model => _text(raw['model']);

  /// Summary of the adapter for the collapsed row, when it sent one.
  String? get summary => _text(raw['summary']);

  /// Metrics; empty when the document has none.
  late final RunMetricsView metrics = RunMetricsView._(
    _map(raw['metrics']) ?? const {},
  );

  /// Cost of the run; null when missing or incomplete.
  late final Money? cost = _money(raw['cost']);

  /// Mode the adapter reported for this run.
  BillingMode? get billingModeUsed => _billing(raw['billing_mode_used']);

  /// Mode configured at the last delivery; the badge falls back to it.
  BillingMode? get billingModeExpected =>
      _billing(raw['billing_mode_expected']);

  /// Items of the plan in their order; empty without a plan.
  late final List<RunPlanItemView> planItems = List.unmodifiable(_plan());

  /// Follow-ups in the order of the document.
  late final List<RunFollowupView> followups = List.unmodifiable(_followups());

  /// When the server noticed a stall (09 5.5); null when not set.
  ///
  /// The API schema `Run` does not name this field yet; it is read from the map when present.
  DateTime? get stallWarnedAt => _time(raw['stall_warned_at']);

  /// Queue time.
  DateTime? get queuedAt => _time(raw['queued_at']);

  /// Start time.
  DateTime? get startedAt => _time(raw['started_at']);

  /// End time.
  DateTime? get finishedAt => _time(raw['finished_at']);

  /// Duration in milliseconds, set after the end.
  int? get durationMs => _int(raw['duration_ms']);

  /// Top-level steps in `ordinal` order, each with its children (04 11.1).
  ///
  /// A step without `ordinal` sorts after those with one, a step whose parent is missing is a
  /// top-level step, and steps on a cycle of parents appear at the top level, so every step
  /// appears exactly once.
  late final List<RunStepNode> stepsInOrder = List.unmodifiable(_tree());

  /// Open steps of kind `subagent`, `workflow` and `background_job` in `ordinal` order
  /// (09 5.4 "Subagents, workflows, background jobs").
  late final List<RunStepView> openSubagents = List.unmodifiable(
    _ordered.where(
      (s) =>
          s.isOpen &&
          (s.kind == StepKind.subagent ||
              s.kind == StepKind.workflow ||
              s.kind == StepKind.backgroundJob),
    ),
  );

  /// The latest step of kind `tool_call` with status `running` (09 5.4 "Current tool").
  late final RunStepView? currentTool = _ordered.reversed
      .where(
        (s) => s.kind == StepKind.toolCall && s.status == StepStatus.running,
      )
      .firstOrNull;

  late final List<RunStepView> _ordered = _steps();

  List<RunStepView> _steps() {
    final source = _map(raw['steps']);
    if (source == null) return const [];
    final steps = <RunStepView>[];
    for (final entry in source.entries) {
      final value = entry.value;
      if (value is Map<String, Object?>) {
        steps.add(RunStepView._(entry.key, value));
      }
    }
    final position = {for (var i = 0; i < steps.length; i++) steps[i]: i};
    // Stable on the document order for equal or missing ordinals.
    int missing(RunStepView s) => s.ordinal == null ? 1 : 0;
    steps.sort((a, b) {
      var order = missing(a).compareTo(missing(b));
      if (order == 0) order = (a.ordinal ?? 0).compareTo(b.ordinal ?? 0);
      if (order == 0) order = position[a]!.compareTo(position[b]!);
      return order;
    });
    return steps;
  }

  List<RunStepNode> _tree() {
    final ids = {for (final s in _ordered) s.id};
    final children = <String, List<RunStepView>>{};
    final roots = <RunStepView>[];
    for (final step in _ordered) {
      final parent = step.parentStepId;
      if (parent == null || parent == step.id || !ids.contains(parent)) {
        roots.add(step);
      } else {
        (children[parent] ??= []).add(step);
      }
    }
    final placed = <String>{};
    RunStepNode build(RunStepView step) {
      placed.add(step.id);
      final kids = [
        for (final child in children[step.id] ?? const <RunStepView>[])
          if (!placed.contains(child.id)) build(child),
      ];
      return RunStepNode._(step, kids);
    }

    final nodes = [for (final root in roots) build(root)];
    // Steps on a parent cycle are reachable from no root; show them at the top level.
    for (final step in _ordered) {
      if (!placed.contains(step.id)) nodes.add(build(step));
    }
    return nodes;
  }

  List<RunPlanItemView> _plan() {
    final items = _map(raw['plan'])?['items'];
    if (items is! List<Object?>) return const [];
    return [
      for (final item in items)
        if (item is Map<String, Object?>)
          RunPlanItemView._(
            _text(item['id']) ?? '',
            _text(item['text']) ?? '',
            PlanItemStatus.fromCode(_text(item['status']) ?? ''),
          ),
    ];
  }

  List<RunFollowupView> _followups() {
    final source = _map(raw['followups']);
    if (source == null) return const [];
    return [
      for (final entry in source.entries)
        if (entry.value case final Map<String, Object?> value)
          RunFollowupView._(
            _text(value['message_id']) ?? entry.key,
            FollowupState.fromCode(_text(value['state']) ?? ''),
          ),
    ];
  }
}

String? _text(Object? value) => value is String ? value : null;

int? _int(Object? value) => value is int ? value : null;

num? _num(Object? value) => value is num ? value : null;

Map<String, Object?>? _map(Object? value) {
  if (value is! Map<String, Object?>) return null;
  return UnmodifiableMapView(value);
}

DateTime? _time(Object? value) =>
    value is String ? DateTime.tryParse(value) : null;

BillingMode? _billing(Object? value) =>
    value is String ? BillingMode.fromCode(value) : null;

Money? _money(Object? value) {
  if (value is! Map<String, Object?>) return null;
  final amount = value['amount_micros'];
  final currency = value['currency'];
  final estimate = value['is_estimate'];
  if (amount is! int || currency is! String || estimate is! bool) return null;
  return Money(amountMicros: amount, currency: currency, isEstimate: estimate);
}
