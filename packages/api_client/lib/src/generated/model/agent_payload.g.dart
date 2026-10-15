// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentPayload extends AgentPayload {
  @override
  final Agent agent;

  factory _$AgentPayload([void Function(AgentPayloadBuilder)? updates]) =>
      (AgentPayloadBuilder()..update(updates))._build();

  _$AgentPayload._({required this.agent}) : super._();
  @override
  AgentPayload rebuild(void Function(AgentPayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentPayloadBuilder toBuilder() => AgentPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentPayload && agent == other.agent;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, agent.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'AgentPayload',
    )..add('agent', agent)).toString();
  }
}

class AgentPayloadBuilder
    implements Builder<AgentPayload, AgentPayloadBuilder> {
  _$AgentPayload? _$v;

  AgentBuilder? _agent;
  AgentBuilder get agent => _$this._agent ??= AgentBuilder();
  set agent(AgentBuilder? agent) => _$this._agent = agent;

  AgentPayloadBuilder() {
    AgentPayload._defaults(this);
  }

  AgentPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _agent = $v.agent.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentPayload other) {
    _$v = other as _$AgentPayload;
  }

  @override
  void update(void Function(AgentPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentPayload build() => _build();

  _$AgentPayload _build() {
    _$AgentPayload _$result;
    try {
      _$result = _$v ?? _$AgentPayload._(agent: agent.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'agent';
        agent.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentPayload',
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
