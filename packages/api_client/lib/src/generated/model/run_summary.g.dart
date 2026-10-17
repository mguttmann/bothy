// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_summary.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunSummary extends RunSummary {
  @override
  final String id;
  @override
  final String threadId;
  @override
  final String channelId;
  @override
  final String agentId;
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
  final String? statusLine;
  @override
  final String? summary;
  @override
  final Money cost;
  @override
  final int stepCount;
  @override
  final int? durationMs;
  @override
  final DateTime queuedAt;
  @override
  final DateTime? startedAt;
  @override
  final DateTime? finishedAt;

  factory _$RunSummary([void Function(RunSummaryBuilder)? updates]) =>
      (RunSummaryBuilder()..update(updates))._build();

  _$RunSummary._({
    required this.id,
    required this.threadId,
    required this.channelId,
    required this.agentId,
    required this.triggeredBy,
    required this.state,
    this.stateReason,
    required this.version,
    required this.billingModeExpected,
    this.billingModeUsed,
    this.statusLine,
    this.summary,
    required this.cost,
    required this.stepCount,
    this.durationMs,
    required this.queuedAt,
    this.startedAt,
    this.finishedAt,
  }) : super._();
  @override
  RunSummary rebuild(void Function(RunSummaryBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunSummaryBuilder toBuilder() => RunSummaryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunSummary &&
        id == other.id &&
        threadId == other.threadId &&
        channelId == other.channelId &&
        agentId == other.agentId &&
        triggeredBy == other.triggeredBy &&
        state == other.state &&
        stateReason == other.stateReason &&
        version == other.version &&
        billingModeExpected == other.billingModeExpected &&
        billingModeUsed == other.billingModeUsed &&
        statusLine == other.statusLine &&
        summary == other.summary &&
        cost == other.cost &&
        stepCount == other.stepCount &&
        durationMs == other.durationMs &&
        queuedAt == other.queuedAt &&
        startedAt == other.startedAt &&
        finishedAt == other.finishedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, triggeredBy.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, stateReason.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, billingModeExpected.hashCode);
    _$hash = $jc(_$hash, billingModeUsed.hashCode);
    _$hash = $jc(_$hash, statusLine.hashCode);
    _$hash = $jc(_$hash, summary.hashCode);
    _$hash = $jc(_$hash, cost.hashCode);
    _$hash = $jc(_$hash, stepCount.hashCode);
    _$hash = $jc(_$hash, durationMs.hashCode);
    _$hash = $jc(_$hash, queuedAt.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, finishedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunSummary')
          ..add('id', id)
          ..add('threadId', threadId)
          ..add('channelId', channelId)
          ..add('agentId', agentId)
          ..add('triggeredBy', triggeredBy)
          ..add('state', state)
          ..add('stateReason', stateReason)
          ..add('version', version)
          ..add('billingModeExpected', billingModeExpected)
          ..add('billingModeUsed', billingModeUsed)
          ..add('statusLine', statusLine)
          ..add('summary', summary)
          ..add('cost', cost)
          ..add('stepCount', stepCount)
          ..add('durationMs', durationMs)
          ..add('queuedAt', queuedAt)
          ..add('startedAt', startedAt)
          ..add('finishedAt', finishedAt))
        .toString();
  }
}

class RunSummaryBuilder implements Builder<RunSummary, RunSummaryBuilder> {
  _$RunSummary? _$v;

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

  String? _statusLine;
  String? get statusLine => _$this._statusLine;
  set statusLine(String? statusLine) => _$this._statusLine = statusLine;

  String? _summary;
  String? get summary => _$this._summary;
  set summary(String? summary) => _$this._summary = summary;

  MoneyBuilder? _cost;
  MoneyBuilder get cost => _$this._cost ??= MoneyBuilder();
  set cost(MoneyBuilder? cost) => _$this._cost = cost;

  int? _stepCount;
  int? get stepCount => _$this._stepCount;
  set stepCount(int? stepCount) => _$this._stepCount = stepCount;

  int? _durationMs;
  int? get durationMs => _$this._durationMs;
  set durationMs(int? durationMs) => _$this._durationMs = durationMs;

  DateTime? _queuedAt;
  DateTime? get queuedAt => _$this._queuedAt;
  set queuedAt(DateTime? queuedAt) => _$this._queuedAt = queuedAt;

  DateTime? _startedAt;
  DateTime? get startedAt => _$this._startedAt;
  set startedAt(DateTime? startedAt) => _$this._startedAt = startedAt;

  DateTime? _finishedAt;
  DateTime? get finishedAt => _$this._finishedAt;
  set finishedAt(DateTime? finishedAt) => _$this._finishedAt = finishedAt;

  RunSummaryBuilder() {
    RunSummary._defaults(this);
  }

  RunSummaryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _agentId = $v.agentId;
      _triggeredBy = $v.triggeredBy;
      _state = $v.state;
      _stateReason = $v.stateReason;
      _version = $v.version;
      _billingModeExpected = $v.billingModeExpected;
      _billingModeUsed = $v.billingModeUsed;
      _statusLine = $v.statusLine;
      _summary = $v.summary;
      _cost = $v.cost.toBuilder();
      _stepCount = $v.stepCount;
      _durationMs = $v.durationMs;
      _queuedAt = $v.queuedAt;
      _startedAt = $v.startedAt;
      _finishedAt = $v.finishedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunSummary other) {
    _$v = other as _$RunSummary;
  }

  @override
  void update(void Function(RunSummaryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunSummary build() => _build();

  _$RunSummary _build() {
    _$RunSummary _$result;
    try {
      _$result =
          _$v ??
          _$RunSummary._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'RunSummary', 'id'),
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'RunSummary',
              'threadId',
            ),
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'RunSummary',
              'channelId',
            ),
            agentId: BuiltValueNullFieldError.checkNotNull(
              agentId,
              r'RunSummary',
              'agentId',
            ),
            triggeredBy: BuiltValueNullFieldError.checkNotNull(
              triggeredBy,
              r'RunSummary',
              'triggeredBy',
            ),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'RunSummary',
              'state',
            ),
            stateReason: stateReason,
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'RunSummary',
              'version',
            ),
            billingModeExpected: BuiltValueNullFieldError.checkNotNull(
              billingModeExpected,
              r'RunSummary',
              'billingModeExpected',
            ),
            billingModeUsed: billingModeUsed,
            statusLine: statusLine,
            summary: summary,
            cost: cost.build(),
            stepCount: BuiltValueNullFieldError.checkNotNull(
              stepCount,
              r'RunSummary',
              'stepCount',
            ),
            durationMs: durationMs,
            queuedAt: BuiltValueNullFieldError.checkNotNull(
              queuedAt,
              r'RunSummary',
              'queuedAt',
            ),
            startedAt: startedAt,
            finishedAt: finishedAt,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'cost';
        cost.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunSummary',
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
