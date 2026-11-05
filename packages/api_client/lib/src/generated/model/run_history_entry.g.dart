// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_history_entry.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunHistoryEntry extends RunHistoryEntry {
  @override
  final String id;
  @override
  final String threadId;
  @override
  final String channelId;
  @override
  final String? spaceId;
  @override
  final String agentId;
  @override
  final RunState state;
  @override
  final String? stateReason;
  @override
  final BillingMode billingModeExpected;
  @override
  final BillingMode? billingModeUsed;
  @override
  final String? model;
  @override
  final String? summary;
  @override
  final Money cost;
  @override
  final RunMetrics metrics;
  @override
  final int stepCount;
  @override
  final DateTime queuedAt;
  @override
  final DateTime? startedAt;
  @override
  final DateTime finishedAt;
  @override
  final int? durationMs;
  @override
  final StatsRunChat? chat;

  factory _$RunHistoryEntry([void Function(RunHistoryEntryBuilder)? updates]) =>
      (RunHistoryEntryBuilder()..update(updates))._build();

  _$RunHistoryEntry._({
    required this.id,
    required this.threadId,
    required this.channelId,
    this.spaceId,
    required this.agentId,
    required this.state,
    this.stateReason,
    required this.billingModeExpected,
    this.billingModeUsed,
    this.model,
    this.summary,
    required this.cost,
    required this.metrics,
    required this.stepCount,
    required this.queuedAt,
    this.startedAt,
    required this.finishedAt,
    this.durationMs,
    this.chat,
  }) : super._();
  @override
  RunHistoryEntry rebuild(void Function(RunHistoryEntryBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunHistoryEntryBuilder toBuilder() => RunHistoryEntryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunHistoryEntry &&
        id == other.id &&
        threadId == other.threadId &&
        channelId == other.channelId &&
        spaceId == other.spaceId &&
        agentId == other.agentId &&
        state == other.state &&
        stateReason == other.stateReason &&
        billingModeExpected == other.billingModeExpected &&
        billingModeUsed == other.billingModeUsed &&
        model == other.model &&
        summary == other.summary &&
        cost == other.cost &&
        metrics == other.metrics &&
        stepCount == other.stepCount &&
        queuedAt == other.queuedAt &&
        startedAt == other.startedAt &&
        finishedAt == other.finishedAt &&
        durationMs == other.durationMs &&
        chat == other.chat;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, stateReason.hashCode);
    _$hash = $jc(_$hash, billingModeExpected.hashCode);
    _$hash = $jc(_$hash, billingModeUsed.hashCode);
    _$hash = $jc(_$hash, model.hashCode);
    _$hash = $jc(_$hash, summary.hashCode);
    _$hash = $jc(_$hash, cost.hashCode);
    _$hash = $jc(_$hash, metrics.hashCode);
    _$hash = $jc(_$hash, stepCount.hashCode);
    _$hash = $jc(_$hash, queuedAt.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, finishedAt.hashCode);
    _$hash = $jc(_$hash, durationMs.hashCode);
    _$hash = $jc(_$hash, chat.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunHistoryEntry')
          ..add('id', id)
          ..add('threadId', threadId)
          ..add('channelId', channelId)
          ..add('spaceId', spaceId)
          ..add('agentId', agentId)
          ..add('state', state)
          ..add('stateReason', stateReason)
          ..add('billingModeExpected', billingModeExpected)
          ..add('billingModeUsed', billingModeUsed)
          ..add('model', model)
          ..add('summary', summary)
          ..add('cost', cost)
          ..add('metrics', metrics)
          ..add('stepCount', stepCount)
          ..add('queuedAt', queuedAt)
          ..add('startedAt', startedAt)
          ..add('finishedAt', finishedAt)
          ..add('durationMs', durationMs)
          ..add('chat', chat))
        .toString();
  }
}

class RunHistoryEntryBuilder
    implements Builder<RunHistoryEntry, RunHistoryEntryBuilder> {
  _$RunHistoryEntry? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

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

  RunState? _state;
  RunState? get state => _$this._state;
  set state(RunState? state) => _$this._state = state;

  String? _stateReason;
  String? get stateReason => _$this._stateReason;
  set stateReason(String? stateReason) => _$this._stateReason = stateReason;

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

  String? _summary;
  String? get summary => _$this._summary;
  set summary(String? summary) => _$this._summary = summary;

  MoneyBuilder? _cost;
  MoneyBuilder get cost => _$this._cost ??= MoneyBuilder();
  set cost(MoneyBuilder? cost) => _$this._cost = cost;

  RunMetricsBuilder? _metrics;
  RunMetricsBuilder get metrics => _$this._metrics ??= RunMetricsBuilder();
  set metrics(RunMetricsBuilder? metrics) => _$this._metrics = metrics;

  int? _stepCount;
  int? get stepCount => _$this._stepCount;
  set stepCount(int? stepCount) => _$this._stepCount = stepCount;

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

  StatsRunChatBuilder? _chat;
  StatsRunChatBuilder get chat => _$this._chat ??= StatsRunChatBuilder();
  set chat(StatsRunChatBuilder? chat) => _$this._chat = chat;

  RunHistoryEntryBuilder() {
    RunHistoryEntry._defaults(this);
  }

  RunHistoryEntryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _spaceId = $v.spaceId;
      _agentId = $v.agentId;
      _state = $v.state;
      _stateReason = $v.stateReason;
      _billingModeExpected = $v.billingModeExpected;
      _billingModeUsed = $v.billingModeUsed;
      _model = $v.model;
      _summary = $v.summary;
      _cost = $v.cost.toBuilder();
      _metrics = $v.metrics.toBuilder();
      _stepCount = $v.stepCount;
      _queuedAt = $v.queuedAt;
      _startedAt = $v.startedAt;
      _finishedAt = $v.finishedAt;
      _durationMs = $v.durationMs;
      _chat = $v.chat?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunHistoryEntry other) {
    _$v = other as _$RunHistoryEntry;
  }

  @override
  void update(void Function(RunHistoryEntryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunHistoryEntry build() => _build();

  _$RunHistoryEntry _build() {
    _$RunHistoryEntry _$result;
    try {
      _$result =
          _$v ??
          _$RunHistoryEntry._(
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'RunHistoryEntry',
              'id',
            ),
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'RunHistoryEntry',
              'threadId',
            ),
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'RunHistoryEntry',
              'channelId',
            ),
            spaceId: spaceId,
            agentId: BuiltValueNullFieldError.checkNotNull(
              agentId,
              r'RunHistoryEntry',
              'agentId',
            ),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'RunHistoryEntry',
              'state',
            ),
            stateReason: stateReason,
            billingModeExpected: BuiltValueNullFieldError.checkNotNull(
              billingModeExpected,
              r'RunHistoryEntry',
              'billingModeExpected',
            ),
            billingModeUsed: billingModeUsed,
            model: model,
            summary: summary,
            cost: cost.build(),
            metrics: metrics.build(),
            stepCount: BuiltValueNullFieldError.checkNotNull(
              stepCount,
              r'RunHistoryEntry',
              'stepCount',
            ),
            queuedAt: BuiltValueNullFieldError.checkNotNull(
              queuedAt,
              r'RunHistoryEntry',
              'queuedAt',
            ),
            startedAt: startedAt,
            finishedAt: BuiltValueNullFieldError.checkNotNull(
              finishedAt,
              r'RunHistoryEntry',
              'finishedAt',
            ),
            durationMs: durationMs,
            chat: _chat?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'cost';
        cost.build();
        _$failedField = 'metrics';
        metrics.build();

        _$failedField = 'chat';
        _chat?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunHistoryEntry',
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
