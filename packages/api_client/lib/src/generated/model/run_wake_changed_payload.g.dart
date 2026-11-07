// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_changed_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunWakeChangedPayload extends RunWakeChangedPayload {
  @override
  final String wakeId;
  @override
  final String threadId;
  @override
  final String agentId;
  @override
  final String triggeredBy;
  @override
  final RunWakeState state;
  @override
  final String? stateReason;
  @override
  final String? noteMessageId;
  @override
  final int? noteThreadSeq;

  factory _$RunWakeChangedPayload([
    void Function(RunWakeChangedPayloadBuilder)? updates,
  ]) => (RunWakeChangedPayloadBuilder()..update(updates))._build();

  _$RunWakeChangedPayload._({
    required this.wakeId,
    required this.threadId,
    required this.agentId,
    required this.triggeredBy,
    required this.state,
    this.stateReason,
    this.noteMessageId,
    this.noteThreadSeq,
  }) : super._();
  @override
  RunWakeChangedPayload rebuild(
    void Function(RunWakeChangedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  RunWakeChangedPayloadBuilder toBuilder() =>
      RunWakeChangedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunWakeChangedPayload &&
        wakeId == other.wakeId &&
        threadId == other.threadId &&
        agentId == other.agentId &&
        triggeredBy == other.triggeredBy &&
        state == other.state &&
        stateReason == other.stateReason &&
        noteMessageId == other.noteMessageId &&
        noteThreadSeq == other.noteThreadSeq;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, wakeId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, triggeredBy.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, stateReason.hashCode);
    _$hash = $jc(_$hash, noteMessageId.hashCode);
    _$hash = $jc(_$hash, noteThreadSeq.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunWakeChangedPayload')
          ..add('wakeId', wakeId)
          ..add('threadId', threadId)
          ..add('agentId', agentId)
          ..add('triggeredBy', triggeredBy)
          ..add('state', state)
          ..add('stateReason', stateReason)
          ..add('noteMessageId', noteMessageId)
          ..add('noteThreadSeq', noteThreadSeq))
        .toString();
  }
}

class RunWakeChangedPayloadBuilder
    implements Builder<RunWakeChangedPayload, RunWakeChangedPayloadBuilder> {
  _$RunWakeChangedPayload? _$v;

  String? _wakeId;
  String? get wakeId => _$this._wakeId;
  set wakeId(String? wakeId) => _$this._wakeId = wakeId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _triggeredBy;
  String? get triggeredBy => _$this._triggeredBy;
  set triggeredBy(String? triggeredBy) => _$this._triggeredBy = triggeredBy;

  RunWakeState? _state;
  RunWakeState? get state => _$this._state;
  set state(RunWakeState? state) => _$this._state = state;

  String? _stateReason;
  String? get stateReason => _$this._stateReason;
  set stateReason(String? stateReason) => _$this._stateReason = stateReason;

  String? _noteMessageId;
  String? get noteMessageId => _$this._noteMessageId;
  set noteMessageId(String? noteMessageId) =>
      _$this._noteMessageId = noteMessageId;

  int? _noteThreadSeq;
  int? get noteThreadSeq => _$this._noteThreadSeq;
  set noteThreadSeq(int? noteThreadSeq) =>
      _$this._noteThreadSeq = noteThreadSeq;

  RunWakeChangedPayloadBuilder() {
    RunWakeChangedPayload._defaults(this);
  }

  RunWakeChangedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _wakeId = $v.wakeId;
      _threadId = $v.threadId;
      _agentId = $v.agentId;
      _triggeredBy = $v.triggeredBy;
      _state = $v.state;
      _stateReason = $v.stateReason;
      _noteMessageId = $v.noteMessageId;
      _noteThreadSeq = $v.noteThreadSeq;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunWakeChangedPayload other) {
    _$v = other as _$RunWakeChangedPayload;
  }

  @override
  void update(void Function(RunWakeChangedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunWakeChangedPayload build() => _build();

  _$RunWakeChangedPayload _build() {
    final _$result =
        _$v ??
        _$RunWakeChangedPayload._(
          wakeId: BuiltValueNullFieldError.checkNotNull(
            wakeId,
            r'RunWakeChangedPayload',
            'wakeId',
          ),
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'RunWakeChangedPayload',
            'threadId',
          ),
          agentId: BuiltValueNullFieldError.checkNotNull(
            agentId,
            r'RunWakeChangedPayload',
            'agentId',
          ),
          triggeredBy: BuiltValueNullFieldError.checkNotNull(
            triggeredBy,
            r'RunWakeChangedPayload',
            'triggeredBy',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'RunWakeChangedPayload',
            'state',
          ),
          stateReason: stateReason,
          noteMessageId: noteMessageId,
          noteThreadSeq: noteThreadSeq,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
