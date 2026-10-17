// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_session_changed_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentSessionChangedPayload extends AgentSessionChangedPayload {
  @override
  final String threadId;
  @override
  final AgentSession agentSession;

  factory _$AgentSessionChangedPayload([
    void Function(AgentSessionChangedPayloadBuilder)? updates,
  ]) => (AgentSessionChangedPayloadBuilder()..update(updates))._build();

  _$AgentSessionChangedPayload._({
    required this.threadId,
    required this.agentSession,
  }) : super._();
  @override
  AgentSessionChangedPayload rebuild(
    void Function(AgentSessionChangedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentSessionChangedPayloadBuilder toBuilder() =>
      AgentSessionChangedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentSessionChangedPayload &&
        threadId == other.threadId &&
        agentSession == other.agentSession;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, agentSession.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentSessionChangedPayload')
          ..add('threadId', threadId)
          ..add('agentSession', agentSession))
        .toString();
  }
}

class AgentSessionChangedPayloadBuilder
    implements
        Builder<AgentSessionChangedPayload, AgentSessionChangedPayloadBuilder> {
  _$AgentSessionChangedPayload? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  AgentSessionBuilder? _agentSession;
  AgentSessionBuilder get agentSession =>
      _$this._agentSession ??= AgentSessionBuilder();
  set agentSession(AgentSessionBuilder? agentSession) =>
      _$this._agentSession = agentSession;

  AgentSessionChangedPayloadBuilder() {
    AgentSessionChangedPayload._defaults(this);
  }

  AgentSessionChangedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _agentSession = $v.agentSession.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentSessionChangedPayload other) {
    _$v = other as _$AgentSessionChangedPayload;
  }

  @override
  void update(void Function(AgentSessionChangedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentSessionChangedPayload build() => _build();

  _$AgentSessionChangedPayload _build() {
    _$AgentSessionChangedPayload _$result;
    try {
      _$result =
          _$v ??
          _$AgentSessionChangedPayload._(
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'AgentSessionChangedPayload',
              'threadId',
            ),
            agentSession: agentSession.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'agentSession';
        agentSession.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentSessionChangedPayload',
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
