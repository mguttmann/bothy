// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_wake_input_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentWakeInputPayload extends AgentWakeInputPayload {
  @override
  final String runId;
  @override
  final String threadId;
  @override
  final String wakeId;
  @override
  final AgentWakeFacts wake;

  factory _$AgentWakeInputPayload([
    void Function(AgentWakeInputPayloadBuilder)? updates,
  ]) => (AgentWakeInputPayloadBuilder()..update(updates))._build();

  _$AgentWakeInputPayload._({
    required this.runId,
    required this.threadId,
    required this.wakeId,
    required this.wake,
  }) : super._();
  @override
  AgentWakeInputPayload rebuild(
    void Function(AgentWakeInputPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentWakeInputPayloadBuilder toBuilder() =>
      AgentWakeInputPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentWakeInputPayload &&
        runId == other.runId &&
        threadId == other.threadId &&
        wakeId == other.wakeId &&
        wake == other.wake;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, wakeId.hashCode);
    _$hash = $jc(_$hash, wake.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentWakeInputPayload')
          ..add('runId', runId)
          ..add('threadId', threadId)
          ..add('wakeId', wakeId)
          ..add('wake', wake))
        .toString();
  }
}

class AgentWakeInputPayloadBuilder
    implements Builder<AgentWakeInputPayload, AgentWakeInputPayloadBuilder> {
  _$AgentWakeInputPayload? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _wakeId;
  String? get wakeId => _$this._wakeId;
  set wakeId(String? wakeId) => _$this._wakeId = wakeId;

  AgentWakeFactsBuilder? _wake;
  AgentWakeFactsBuilder get wake => _$this._wake ??= AgentWakeFactsBuilder();
  set wake(AgentWakeFactsBuilder? wake) => _$this._wake = wake;

  AgentWakeInputPayloadBuilder() {
    AgentWakeInputPayload._defaults(this);
  }

  AgentWakeInputPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _threadId = $v.threadId;
      _wakeId = $v.wakeId;
      _wake = $v.wake.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentWakeInputPayload other) {
    _$v = other as _$AgentWakeInputPayload;
  }

  @override
  void update(void Function(AgentWakeInputPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentWakeInputPayload build() => _build();

  _$AgentWakeInputPayload _build() {
    _$AgentWakeInputPayload _$result;
    try {
      _$result =
          _$v ??
          _$AgentWakeInputPayload._(
            runId: BuiltValueNullFieldError.checkNotNull(
              runId,
              r'AgentWakeInputPayload',
              'runId',
            ),
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'AgentWakeInputPayload',
              'threadId',
            ),
            wakeId: BuiltValueNullFieldError.checkNotNull(
              wakeId,
              r'AgentWakeInputPayload',
              'wakeId',
            ),
            wake: wake.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'wake';
        wake.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentWakeInputPayload',
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
