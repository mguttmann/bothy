// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunTriggerKindEnum _$runTriggerKindEnum_message =
    const RunTriggerKindEnum._('message');
const RunTriggerKindEnum _$runTriggerKindEnum_command =
    const RunTriggerKindEnum._('command');
const RunTriggerKindEnum _$runTriggerKindEnum_interaction =
    const RunTriggerKindEnum._('interaction');
const RunTriggerKindEnum _$runTriggerKindEnum_unknownDefaultOpenApi =
    const RunTriggerKindEnum._('unknownDefaultOpenApi');

RunTriggerKindEnum _$runTriggerKindEnumValueOf(String name) {
  switch (name) {
    case 'message':
      return _$runTriggerKindEnum_message;
    case 'command':
      return _$runTriggerKindEnum_command;
    case 'interaction':
      return _$runTriggerKindEnum_interaction;
    case 'unknownDefaultOpenApi':
      return _$runTriggerKindEnum_unknownDefaultOpenApi;
    default:
      return _$runTriggerKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunTriggerKindEnum> _$runTriggerKindEnumValues =
    BuiltSet<RunTriggerKindEnum>(const <RunTriggerKindEnum>[
      _$runTriggerKindEnum_message,
      _$runTriggerKindEnum_command,
      _$runTriggerKindEnum_interaction,
      _$runTriggerKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<RunTriggerKindEnum> _$runTriggerKindEnumSerializer =
    _$RunTriggerKindEnumSerializer();

class _$RunTriggerKindEnumSerializer
    implements PrimitiveSerializer<RunTriggerKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'message': 'message',
    'command': 'command',
    'interaction': 'interaction',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'message': 'message',
    'command': 'command',
    'interaction': 'interaction',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunTriggerKindEnum];
  @override
  final String wireName = 'RunTriggerKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunTriggerKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunTriggerKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunTriggerKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Run extends Run {
  @override
  final String id;
  @override
  final String threadId;
  @override
  final String channelId;
  @override
  final String agentId;
  @override
  final String? agentSessionId;
  @override
  final RunTriggerKindEnum? triggerKind;
  @override
  final String? triggeringMessageId;
  @override
  final String? triggeringCommandId;
  @override
  final String? triggeringInteractionId;
  @override
  final String triggeredBy;
  @override
  final RunState state;
  @override
  final String? stateReason;
  @override
  final int version;
  @override
  final BillingMode billingModeExpected;
  @override
  final BillingMode? billingModeUsed;
  @override
  final String? model;
  @override
  final String? statusLine;
  @override
  final String? lastTextExcerpt;
  @override
  final RunMetrics metrics;
  @override
  final Money cost;
  @override
  final String? summary;
  @override
  final BuiltMap<String, RunStep> steps;
  @override
  final BuiltMap<String, RunFollowup> followups;
  @override
  final DateTime queuedAt;
  @override
  final DateTime? startedAt;
  @override
  final DateTime? finishedAt;
  @override
  final int? durationMs;
  @override
  final RunPlan? plan;

  factory _$Run([void Function(RunBuilder)? updates]) =>
      (RunBuilder()..update(updates))._build();

  _$Run._({
    required this.id,
    required this.threadId,
    required this.channelId,
    required this.agentId,
    this.agentSessionId,
    this.triggerKind,
    this.triggeringMessageId,
    this.triggeringCommandId,
    this.triggeringInteractionId,
    required this.triggeredBy,
    required this.state,
    this.stateReason,
    required this.version,
    required this.billingModeExpected,
    this.billingModeUsed,
    this.model,
    this.statusLine,
    this.lastTextExcerpt,
    required this.metrics,
    required this.cost,
    this.summary,
    required this.steps,
    required this.followups,
    required this.queuedAt,
    this.startedAt,
    this.finishedAt,
    this.durationMs,
    this.plan,
  }) : super._();
  @override
  Run rebuild(void Function(RunBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunBuilder toBuilder() => RunBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Run &&
        id == other.id &&
        threadId == other.threadId &&
        channelId == other.channelId &&
        agentId == other.agentId &&
        agentSessionId == other.agentSessionId &&
        triggerKind == other.triggerKind &&
        triggeringMessageId == other.triggeringMessageId &&
        triggeringCommandId == other.triggeringCommandId &&
        triggeringInteractionId == other.triggeringInteractionId &&
        triggeredBy == other.triggeredBy &&
        state == other.state &&
        stateReason == other.stateReason &&
        version == other.version &&
        billingModeExpected == other.billingModeExpected &&
        billingModeUsed == other.billingModeUsed &&
        model == other.model &&
        statusLine == other.statusLine &&
        lastTextExcerpt == other.lastTextExcerpt &&
        metrics == other.metrics &&
        cost == other.cost &&
        summary == other.summary &&
        steps == other.steps &&
        followups == other.followups &&
        queuedAt == other.queuedAt &&
        startedAt == other.startedAt &&
        finishedAt == other.finishedAt &&
        durationMs == other.durationMs &&
        plan == other.plan;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, agentSessionId.hashCode);
    _$hash = $jc(_$hash, triggerKind.hashCode);
    _$hash = $jc(_$hash, triggeringMessageId.hashCode);
    _$hash = $jc(_$hash, triggeringCommandId.hashCode);
    _$hash = $jc(_$hash, triggeringInteractionId.hashCode);
    _$hash = $jc(_$hash, triggeredBy.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, stateReason.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, billingModeExpected.hashCode);
    _$hash = $jc(_$hash, billingModeUsed.hashCode);
    _$hash = $jc(_$hash, model.hashCode);
    _$hash = $jc(_$hash, statusLine.hashCode);
    _$hash = $jc(_$hash, lastTextExcerpt.hashCode);
    _$hash = $jc(_$hash, metrics.hashCode);
    _$hash = $jc(_$hash, cost.hashCode);
    _$hash = $jc(_$hash, summary.hashCode);
    _$hash = $jc(_$hash, steps.hashCode);
    _$hash = $jc(_$hash, followups.hashCode);
    _$hash = $jc(_$hash, queuedAt.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, finishedAt.hashCode);
    _$hash = $jc(_$hash, durationMs.hashCode);
    _$hash = $jc(_$hash, plan.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Run')
          ..add('id', id)
          ..add('threadId', threadId)
          ..add('channelId', channelId)
          ..add('agentId', agentId)
          ..add('agentSessionId', agentSessionId)
          ..add('triggerKind', triggerKind)
          ..add('triggeringMessageId', triggeringMessageId)
          ..add('triggeringCommandId', triggeringCommandId)
          ..add('triggeringInteractionId', triggeringInteractionId)
          ..add('triggeredBy', triggeredBy)
          ..add('state', state)
          ..add('stateReason', stateReason)
          ..add('version', version)
          ..add('billingModeExpected', billingModeExpected)
          ..add('billingModeUsed', billingModeUsed)
          ..add('model', model)
          ..add('statusLine', statusLine)
          ..add('lastTextExcerpt', lastTextExcerpt)
          ..add('metrics', metrics)
          ..add('cost', cost)
          ..add('summary', summary)
          ..add('steps', steps)
          ..add('followups', followups)
          ..add('queuedAt', queuedAt)
          ..add('startedAt', startedAt)
          ..add('finishedAt', finishedAt)
          ..add('durationMs', durationMs)
          ..add('plan', plan))
        .toString();
  }
}

class RunBuilder implements Builder<Run, RunBuilder> {
  _$Run? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _agentSessionId;
  String? get agentSessionId => _$this._agentSessionId;
  set agentSessionId(String? agentSessionId) =>
      _$this._agentSessionId = agentSessionId;

  RunTriggerKindEnum? _triggerKind;
  RunTriggerKindEnum? get triggerKind => _$this._triggerKind;
  set triggerKind(RunTriggerKindEnum? triggerKind) =>
      _$this._triggerKind = triggerKind;

  String? _triggeringMessageId;
  String? get triggeringMessageId => _$this._triggeringMessageId;
  set triggeringMessageId(String? triggeringMessageId) =>
      _$this._triggeringMessageId = triggeringMessageId;

  String? _triggeringCommandId;
  String? get triggeringCommandId => _$this._triggeringCommandId;
  set triggeringCommandId(String? triggeringCommandId) =>
      _$this._triggeringCommandId = triggeringCommandId;

  String? _triggeringInteractionId;
  String? get triggeringInteractionId => _$this._triggeringInteractionId;
  set triggeringInteractionId(String? triggeringInteractionId) =>
      _$this._triggeringInteractionId = triggeringInteractionId;

  String? _triggeredBy;
  String? get triggeredBy => _$this._triggeredBy;
  set triggeredBy(String? triggeredBy) => _$this._triggeredBy = triggeredBy;

  RunState? _state;
  RunState? get state => _$this._state;
  set state(RunState? state) => _$this._state = state;

  String? _stateReason;
  String? get stateReason => _$this._stateReason;
  set stateReason(String? stateReason) => _$this._stateReason = stateReason;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  BillingMode? _billingModeExpected;
  BillingMode? get billingModeExpected => _$this._billingModeExpected;
  set billingModeExpected(BillingMode? billingModeExpected) =>
      _$this._billingModeExpected = billingModeExpected;

  BillingMode? _billingModeUsed;
  BillingMode? get billingModeUsed => _$this._billingModeUsed;
  set billingModeUsed(BillingMode? billingModeUsed) =>
      _$this._billingModeUsed = billingModeUsed;

  String? _model;
  String? get model => _$this._model;
  set model(String? model) => _$this._model = model;

  String? _statusLine;
  String? get statusLine => _$this._statusLine;
  set statusLine(String? statusLine) => _$this._statusLine = statusLine;

  String? _lastTextExcerpt;
  String? get lastTextExcerpt => _$this._lastTextExcerpt;
  set lastTextExcerpt(String? lastTextExcerpt) =>
      _$this._lastTextExcerpt = lastTextExcerpt;

  RunMetricsBuilder? _metrics;
  RunMetricsBuilder get metrics => _$this._metrics ??= RunMetricsBuilder();
  set metrics(RunMetricsBuilder? metrics) => _$this._metrics = metrics;

  MoneyBuilder? _cost;
  MoneyBuilder get cost => _$this._cost ??= MoneyBuilder();
  set cost(MoneyBuilder? cost) => _$this._cost = cost;

  String? _summary;
  String? get summary => _$this._summary;
  set summary(String? summary) => _$this._summary = summary;

  MapBuilder<String, RunStep>? _steps;
  MapBuilder<String, RunStep> get steps =>
      _$this._steps ??= MapBuilder<String, RunStep>();
  set steps(MapBuilder<String, RunStep>? steps) => _$this._steps = steps;

  MapBuilder<String, RunFollowup>? _followups;
  MapBuilder<String, RunFollowup> get followups =>
      _$this._followups ??= MapBuilder<String, RunFollowup>();
  set followups(MapBuilder<String, RunFollowup>? followups) =>
      _$this._followups = followups;

  DateTime? _queuedAt;
  DateTime? get queuedAt => _$this._queuedAt;
  set queuedAt(DateTime? queuedAt) => _$this._queuedAt = queuedAt;

  DateTime? _startedAt;
  DateTime? get startedAt => _$this._startedAt;
  set startedAt(DateTime? startedAt) => _$this._startedAt = startedAt;

  DateTime? _finishedAt;
  DateTime? get finishedAt => _$this._finishedAt;
  set finishedAt(DateTime? finishedAt) => _$this._finishedAt = finishedAt;

  int? _durationMs;
  int? get durationMs => _$this._durationMs;
  set durationMs(int? durationMs) => _$this._durationMs = durationMs;

  RunPlanBuilder? _plan;
  RunPlanBuilder get plan => _$this._plan ??= RunPlanBuilder();
  set plan(RunPlanBuilder? plan) => _$this._plan = plan;

  RunBuilder() {
    Run._defaults(this);
  }

  RunBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _agentId = $v.agentId;
      _agentSessionId = $v.agentSessionId;
      _triggerKind = $v.triggerKind;
      _triggeringMessageId = $v.triggeringMessageId;
      _triggeringCommandId = $v.triggeringCommandId;
      _triggeringInteractionId = $v.triggeringInteractionId;
      _triggeredBy = $v.triggeredBy;
      _state = $v.state;
      _stateReason = $v.stateReason;
      _version = $v.version;
      _billingModeExpected = $v.billingModeExpected;
      _billingModeUsed = $v.billingModeUsed;
      _model = $v.model;
      _statusLine = $v.statusLine;
      _lastTextExcerpt = $v.lastTextExcerpt;
      _metrics = $v.metrics.toBuilder();
      _cost = $v.cost.toBuilder();
      _summary = $v.summary;
      _steps = $v.steps.toBuilder();
      _followups = $v.followups.toBuilder();
      _queuedAt = $v.queuedAt;
      _startedAt = $v.startedAt;
      _finishedAt = $v.finishedAt;
      _durationMs = $v.durationMs;
      _plan = $v.plan?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Run other) {
    _$v = other as _$Run;
  }

  @override
  void update(void Function(RunBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Run build() => _build();

  _$Run _build() {
    _$Run _$result;
    try {
      _$result =
          _$v ??
          _$Run._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Run', 'id'),
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'Run',
              'threadId',
            ),
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'Run',
              'channelId',
            ),
            agentId: BuiltValueNullFieldError.checkNotNull(
              agentId,
              r'Run',
              'agentId',
            ),
            agentSessionId: agentSessionId,
            triggerKind: triggerKind,
            triggeringMessageId: triggeringMessageId,
            triggeringCommandId: triggeringCommandId,
            triggeringInteractionId: triggeringInteractionId,
            triggeredBy: BuiltValueNullFieldError.checkNotNull(
              triggeredBy,
              r'Run',
              'triggeredBy',
            ),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'Run',
              'state',
            ),
            stateReason: stateReason,
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'Run',
              'version',
            ),
            billingModeExpected: BuiltValueNullFieldError.checkNotNull(
              billingModeExpected,
              r'Run',
              'billingModeExpected',
            ),
            billingModeUsed: billingModeUsed,
            model: model,
            statusLine: statusLine,
            lastTextExcerpt: lastTextExcerpt,
            metrics: metrics.build(),
            cost: cost.build(),
            summary: summary,
            steps: steps.build(),
            followups: followups.build(),
            queuedAt: BuiltValueNullFieldError.checkNotNull(
              queuedAt,
              r'Run',
              'queuedAt',
            ),
            startedAt: startedAt,
            finishedAt: finishedAt,
            durationMs: durationMs,
            plan: _plan?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'metrics';
        metrics.build();
        _$failedField = 'cost';
        cost.build();

        _$failedField = 'steps';
        steps.build();
        _$failedField = 'followups';
        followups.build();

        _$failedField = 'plan';
        _plan?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(r'Run', _$failedField, e.toString());
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
