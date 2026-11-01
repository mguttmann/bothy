// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_live_run.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatsLiveRunStateEnum _$statsLiveRunStateEnum_queued =
    const StatsLiveRunStateEnum._('queued');
const StatsLiveRunStateEnum _$statsLiveRunStateEnum_running =
    const StatsLiveRunStateEnum._('running');
const StatsLiveRunStateEnum _$statsLiveRunStateEnum_waitingInput =
    const StatsLiveRunStateEnum._('waitingInput');
const StatsLiveRunStateEnum _$statsLiveRunStateEnum_unknownDefaultOpenApi =
    const StatsLiveRunStateEnum._('unknownDefaultOpenApi');

StatsLiveRunStateEnum _$statsLiveRunStateEnumValueOf(String name) {
  switch (name) {
    case 'queued':
      return _$statsLiveRunStateEnum_queued;
    case 'running':
      return _$statsLiveRunStateEnum_running;
    case 'waitingInput':
      return _$statsLiveRunStateEnum_waitingInput;
    case 'unknownDefaultOpenApi':
      return _$statsLiveRunStateEnum_unknownDefaultOpenApi;
    default:
      return _$statsLiveRunStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<StatsLiveRunStateEnum> _$statsLiveRunStateEnumValues =
    BuiltSet<StatsLiveRunStateEnum>(const <StatsLiveRunStateEnum>[
      _$statsLiveRunStateEnum_queued,
      _$statsLiveRunStateEnum_running,
      _$statsLiveRunStateEnum_waitingInput,
      _$statsLiveRunStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<StatsLiveRunStateEnum> _$statsLiveRunStateEnumSerializer =
    _$StatsLiveRunStateEnumSerializer();

class _$StatsLiveRunStateEnumSerializer
    implements PrimitiveSerializer<StatsLiveRunStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'queued': 'queued',
    'running': 'running',
    'waitingInput': 'waiting_input',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'queued': 'queued',
    'running': 'running',
    'waiting_input': 'waitingInput',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatsLiveRunStateEnum];
  @override
  final String wireName = 'StatsLiveRunStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    StatsLiveRunStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatsLiveRunStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatsLiveRunStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$StatsLiveRun extends StatsLiveRun {
  @override
  final String runId;
  @override
  final String threadId;
  @override
  final String channelId;
  @override
  final String spaceId;
  @override
  final String agentId;
  @override
  final StatsLiveRunStateEnum state;
  @override
  final DateTime queuedAt;
  @override
  final DateTime? startedAt;
  @override
  final BillingMode billingMode;
  @override
  final StatsLiveStep? step;
  @override
  final bool approvalPending;
  @override
  final int? costMicros;
  @override
  final String? currency;
  @override
  final bool? costIsEstimate;
  @override
  final BuiltList<StatsLiveTask>? tasks;
  @override
  final bool? tasksTruncated;

  factory _$StatsLiveRun([void Function(StatsLiveRunBuilder)? updates]) =>
      (StatsLiveRunBuilder()..update(updates))._build();

  _$StatsLiveRun._({
    required this.runId,
    required this.threadId,
    required this.channelId,
    required this.spaceId,
    required this.agentId,
    required this.state,
    required this.queuedAt,
    this.startedAt,
    required this.billingMode,
    this.step,
    required this.approvalPending,
    this.costMicros,
    this.currency,
    this.costIsEstimate,
    this.tasks,
    this.tasksTruncated,
  }) : super._();
  @override
  StatsLiveRun rebuild(void Function(StatsLiveRunBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsLiveRunBuilder toBuilder() => StatsLiveRunBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsLiveRun &&
        runId == other.runId &&
        threadId == other.threadId &&
        channelId == other.channelId &&
        spaceId == other.spaceId &&
        agentId == other.agentId &&
        state == other.state &&
        queuedAt == other.queuedAt &&
        startedAt == other.startedAt &&
        billingMode == other.billingMode &&
        step == other.step &&
        approvalPending == other.approvalPending &&
        costMicros == other.costMicros &&
        currency == other.currency &&
        costIsEstimate == other.costIsEstimate &&
        tasks == other.tasks &&
        tasksTruncated == other.tasksTruncated;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, queuedAt.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, step.hashCode);
    _$hash = $jc(_$hash, approvalPending.hashCode);
    _$hash = $jc(_$hash, costMicros.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jc(_$hash, costIsEstimate.hashCode);
    _$hash = $jc(_$hash, tasks.hashCode);
    _$hash = $jc(_$hash, tasksTruncated.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsLiveRun')
          ..add('runId', runId)
          ..add('threadId', threadId)
          ..add('channelId', channelId)
          ..add('spaceId', spaceId)
          ..add('agentId', agentId)
          ..add('state', state)
          ..add('queuedAt', queuedAt)
          ..add('startedAt', startedAt)
          ..add('billingMode', billingMode)
          ..add('step', step)
          ..add('approvalPending', approvalPending)
          ..add('costMicros', costMicros)
          ..add('currency', currency)
          ..add('costIsEstimate', costIsEstimate)
          ..add('tasks', tasks)
          ..add('tasksTruncated', tasksTruncated))
        .toString();
  }
}

class StatsLiveRunBuilder
    implements Builder<StatsLiveRun, StatsLiveRunBuilder> {
  _$StatsLiveRun? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _spaceId;
  String? get spaceId => _$this._spaceId;
  set spaceId(String? spaceId) => _$this._spaceId = spaceId;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  StatsLiveRunStateEnum? _state;
  StatsLiveRunStateEnum? get state => _$this._state;
  set state(StatsLiveRunStateEnum? state) => _$this._state = state;

  DateTime? _queuedAt;
  DateTime? get queuedAt => _$this._queuedAt;
  set queuedAt(DateTime? queuedAt) => _$this._queuedAt = queuedAt;

  DateTime? _startedAt;
  DateTime? get startedAt => _$this._startedAt;
  set startedAt(DateTime? startedAt) => _$this._startedAt = startedAt;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  StatsLiveStepBuilder? _step;
  StatsLiveStepBuilder get step => _$this._step ??= StatsLiveStepBuilder();
  set step(StatsLiveStepBuilder? step) => _$this._step = step;

  bool? _approvalPending;
  bool? get approvalPending => _$this._approvalPending;
  set approvalPending(bool? approvalPending) =>
      _$this._approvalPending = approvalPending;

  int? _costMicros;
  int? get costMicros => _$this._costMicros;
  set costMicros(int? costMicros) => _$this._costMicros = costMicros;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  bool? _costIsEstimate;
  bool? get costIsEstimate => _$this._costIsEstimate;
  set costIsEstimate(bool? costIsEstimate) =>
      _$this._costIsEstimate = costIsEstimate;

  ListBuilder<StatsLiveTask>? _tasks;
  ListBuilder<StatsLiveTask> get tasks =>
      _$this._tasks ??= ListBuilder<StatsLiveTask>();
  set tasks(ListBuilder<StatsLiveTask>? tasks) => _$this._tasks = tasks;

  bool? _tasksTruncated;
  bool? get tasksTruncated => _$this._tasksTruncated;
  set tasksTruncated(bool? tasksTruncated) =>
      _$this._tasksTruncated = tasksTruncated;

  StatsLiveRunBuilder() {
    StatsLiveRun._defaults(this);
  }

  StatsLiveRunBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _spaceId = $v.spaceId;
      _agentId = $v.agentId;
      _state = $v.state;
      _queuedAt = $v.queuedAt;
      _startedAt = $v.startedAt;
      _billingMode = $v.billingMode;
      _step = $v.step?.toBuilder();
      _approvalPending = $v.approvalPending;
      _costMicros = $v.costMicros;
      _currency = $v.currency;
      _costIsEstimate = $v.costIsEstimate;
      _tasks = $v.tasks?.toBuilder();
      _tasksTruncated = $v.tasksTruncated;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsLiveRun other) {
    _$v = other as _$StatsLiveRun;
  }

  @override
  void update(void Function(StatsLiveRunBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsLiveRun build() => _build();

  _$StatsLiveRun _build() {
    _$StatsLiveRun _$result;
    try {
      _$result =
          _$v ??
          _$StatsLiveRun._(
            runId: BuiltValueNullFieldError.checkNotNull(
              runId,
              r'StatsLiveRun',
              'runId',
            ),
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'StatsLiveRun',
              'threadId',
            ),
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'StatsLiveRun',
              'channelId',
            ),
            spaceId: BuiltValueNullFieldError.checkNotNull(
              spaceId,
              r'StatsLiveRun',
              'spaceId',
            ),
            agentId: BuiltValueNullFieldError.checkNotNull(
              agentId,
              r'StatsLiveRun',
              'agentId',
            ),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'StatsLiveRun',
              'state',
            ),
            queuedAt: BuiltValueNullFieldError.checkNotNull(
              queuedAt,
              r'StatsLiveRun',
              'queuedAt',
            ),
            startedAt: startedAt,
            billingMode: BuiltValueNullFieldError.checkNotNull(
              billingMode,
              r'StatsLiveRun',
              'billingMode',
            ),
            step: _step?.build(),
            approvalPending: BuiltValueNullFieldError.checkNotNull(
              approvalPending,
              r'StatsLiveRun',
              'approvalPending',
            ),
            costMicros: costMicros,
            currency: currency,
            costIsEstimate: costIsEstimate,
            tasks: _tasks?.build(),
            tasksTruncated: tasksTruncated,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'step';
        _step?.build();

        _$failedField = 'tasks';
        _tasks?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatsLiveRun',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
