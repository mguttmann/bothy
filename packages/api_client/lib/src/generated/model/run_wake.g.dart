// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunWake extends RunWake {
  @override
  final String id;
  @override
  final String agentId;
  @override
  final String threadId;
  @override
  final String sourceRunId;
  @override
  final String triggeredBy;
  @override
  final RunWakeKind kind;
  @override
  final RunWakeReason reason;
  @override
  final String label;
  @override
  final RunWakeCondition condition;
  @override
  final int? outage;
  @override
  final DateTime notBefore;
  @override
  final DateTime deadline;
  @override
  final RunWakeState state;
  @override
  final String? stateReason;
  @override
  final String? resultText;
  @override
  final String? firedRunId;
  @override
  final String? firedIntoRunId;
  @override
  final DateTime createdAt;
  @override
  final DateTime? endedAt;
  @override
  final int version;

  factory _$RunWake([void Function(RunWakeBuilder)? updates]) =>
      (RunWakeBuilder()..update(updates))._build();

  _$RunWake._({
    required this.id,
    required this.agentId,
    required this.threadId,
    required this.sourceRunId,
    required this.triggeredBy,
    required this.kind,
    required this.reason,
    required this.label,
    required this.condition,
    this.outage,
    required this.notBefore,
    required this.deadline,
    required this.state,
    this.stateReason,
    this.resultText,
    this.firedRunId,
    this.firedIntoRunId,
    required this.createdAt,
    this.endedAt,
    required this.version,
  }) : super._();
  @override
  RunWake rebuild(void Function(RunWakeBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunWakeBuilder toBuilder() => RunWakeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunWake &&
        id == other.id &&
        agentId == other.agentId &&
        threadId == other.threadId &&
        sourceRunId == other.sourceRunId &&
        triggeredBy == other.triggeredBy &&
        kind == other.kind &&
        reason == other.reason &&
        label == other.label &&
        condition == other.condition &&
        outage == other.outage &&
        notBefore == other.notBefore &&
        deadline == other.deadline &&
        state == other.state &&
        stateReason == other.stateReason &&
        resultText == other.resultText &&
        firedRunId == other.firedRunId &&
        firedIntoRunId == other.firedIntoRunId &&
        createdAt == other.createdAt &&
        endedAt == other.endedAt &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, sourceRunId.hashCode);
    _$hash = $jc(_$hash, triggeredBy.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, label.hashCode);
    _$hash = $jc(_$hash, condition.hashCode);
    _$hash = $jc(_$hash, outage.hashCode);
    _$hash = $jc(_$hash, notBefore.hashCode);
    _$hash = $jc(_$hash, deadline.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, stateReason.hashCode);
    _$hash = $jc(_$hash, resultText.hashCode);
    _$hash = $jc(_$hash, firedRunId.hashCode);
    _$hash = $jc(_$hash, firedIntoRunId.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, endedAt.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunWake')
          ..add('id', id)
          ..add('agentId', agentId)
          ..add('threadId', threadId)
          ..add('sourceRunId', sourceRunId)
          ..add('triggeredBy', triggeredBy)
          ..add('kind', kind)
          ..add('reason', reason)
          ..add('label', label)
          ..add('condition', condition)
          ..add('outage', outage)
          ..add('notBefore', notBefore)
          ..add('deadline', deadline)
          ..add('state', state)
          ..add('stateReason', stateReason)
          ..add('resultText', resultText)
          ..add('firedRunId', firedRunId)
          ..add('firedIntoRunId', firedIntoRunId)
          ..add('createdAt', createdAt)
          ..add('endedAt', endedAt)
          ..add('version', version))
        .toString();
  }
}

class RunWakeBuilder implements Builder<RunWake, RunWakeBuilder> {
  _$RunWake? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _sourceRunId;
  String? get sourceRunId => _$this._sourceRunId;
  set sourceRunId(String? sourceRunId) => _$this._sourceRunId = sourceRunId;

  String? _triggeredBy;
  String? get triggeredBy => _$this._triggeredBy;
  set triggeredBy(String? triggeredBy) => _$this._triggeredBy = triggeredBy;

  RunWakeKind? _kind;
  RunWakeKind? get kind => _$this._kind;
  set kind(RunWakeKind? kind) => _$this._kind = kind;

  RunWakeReason? _reason;
  RunWakeReason? get reason => _$this._reason;
  set reason(RunWakeReason? reason) => _$this._reason = reason;

  String? _label;
  String? get label => _$this._label;
  set label(String? label) => _$this._label = label;

  RunWakeConditionBuilder? _condition;
  RunWakeConditionBuilder get condition =>
      _$this._condition ??= RunWakeConditionBuilder();
  set condition(RunWakeConditionBuilder? condition) =>
      _$this._condition = condition;

  int? _outage;
  int? get outage => _$this._outage;
  set outage(int? outage) => _$this._outage = outage;

  DateTime? _notBefore;
  DateTime? get notBefore => _$this._notBefore;
  set notBefore(DateTime? notBefore) => _$this._notBefore = notBefore;

  DateTime? _deadline;
  DateTime? get deadline => _$this._deadline;
  set deadline(DateTime? deadline) => _$this._deadline = deadline;

  RunWakeState? _state;
  RunWakeState? get state => _$this._state;
  set state(RunWakeState? state) => _$this._state = state;

  String? _stateReason;
  String? get stateReason => _$this._stateReason;
  set stateReason(String? stateReason) => _$this._stateReason = stateReason;

  String? _resultText;
  String? get resultText => _$this._resultText;
  set resultText(String? resultText) => _$this._resultText = resultText;

  String? _firedRunId;
  String? get firedRunId => _$this._firedRunId;
  set firedRunId(String? firedRunId) => _$this._firedRunId = firedRunId;

  String? _firedIntoRunId;
  String? get firedIntoRunId => _$this._firedIntoRunId;
  set firedIntoRunId(String? firedIntoRunId) =>
      _$this._firedIntoRunId = firedIntoRunId;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _endedAt;
  DateTime? get endedAt => _$this._endedAt;
  set endedAt(DateTime? endedAt) => _$this._endedAt = endedAt;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  RunWakeBuilder() {
    RunWake._defaults(this);
  }

  RunWakeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _agentId = $v.agentId;
      _threadId = $v.threadId;
      _sourceRunId = $v.sourceRunId;
      _triggeredBy = $v.triggeredBy;
      _kind = $v.kind;
      _reason = $v.reason;
      _label = $v.label;
      _condition = $v.condition.toBuilder();
      _outage = $v.outage;
      _notBefore = $v.notBefore;
      _deadline = $v.deadline;
      _state = $v.state;
      _stateReason = $v.stateReason;
      _resultText = $v.resultText;
      _firedRunId = $v.firedRunId;
      _firedIntoRunId = $v.firedIntoRunId;
      _createdAt = $v.createdAt;
      _endedAt = $v.endedAt;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunWake other) {
    _$v = other as _$RunWake;
  }

  @override
  void update(void Function(RunWakeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunWake build() => _build();

  _$RunWake _build() {
    _$RunWake _$result;
    try {
      _$result =
          _$v ??
          _$RunWake._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'RunWake', 'id'),
            agentId: BuiltValueNullFieldError.checkNotNull(
              agentId,
              r'RunWake',
              'agentId',
            ),
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'RunWake',
              'threadId',
            ),
            sourceRunId: BuiltValueNullFieldError.checkNotNull(
              sourceRunId,
              r'RunWake',
              'sourceRunId',
            ),
            triggeredBy: BuiltValueNullFieldError.checkNotNull(
              triggeredBy,
              r'RunWake',
              'triggeredBy',
            ),
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'RunWake',
              'kind',
            ),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'RunWake',
              'reason',
            ),
            label: BuiltValueNullFieldError.checkNotNull(
              label,
              r'RunWake',
              'label',
            ),
            condition: condition.build(),
            outage: outage,
            notBefore: BuiltValueNullFieldError.checkNotNull(
              notBefore,
              r'RunWake',
              'notBefore',
            ),
            deadline: BuiltValueNullFieldError.checkNotNull(
              deadline,
              r'RunWake',
              'deadline',
            ),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'RunWake',
              'state',
            ),
            stateReason: stateReason,
            resultText: resultText,
            firedRunId: firedRunId,
            firedIntoRunId: firedIntoRunId,
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'RunWake',
              'createdAt',
            ),
            endedAt: endedAt,
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'RunWake',
              'version',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'condition';
        condition.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunWake',
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
