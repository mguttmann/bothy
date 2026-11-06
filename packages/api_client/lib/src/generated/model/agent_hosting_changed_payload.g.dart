// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_hosting_changed_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentHostingChangedPayload extends AgentHostingChangedPayload {
  @override
  final String agentId;

  factory _$AgentHostingChangedPayload([
    void Function(AgentHostingChangedPayloadBuilder)? updates,
  ]) => (AgentHostingChangedPayloadBuilder()..update(updates))._build();

  _$AgentHostingChangedPayload._({required this.agentId}) : super._();
  @override
  AgentHostingChangedPayload rebuild(
    void Function(AgentHostingChangedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentHostingChangedPayloadBuilder toBuilder() =>
      AgentHostingChangedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentHostingChangedPayload && agentId == other.agentId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'AgentHostingChangedPayload',
    )..add('agentId', agentId)).toString();
  }
}

class AgentHostingChangedPayloadBuilder
    implements
        Builder<AgentHostingChangedPayload, AgentHostingChangedPayloadBuilder> {
  _$AgentHostingChangedPayload? _$v;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  AgentHostingChangedPayloadBuilder() {
    AgentHostingChangedPayload._defaults(this);
  }

  AgentHostingChangedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _agentId = $v.agentId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentHostingChangedPayload other) {
    _$v = other as _$AgentHostingChangedPayload;
  }

  @override
  void update(void Function(AgentHostingChangedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentHostingChangedPayload build() => _build();

  _$AgentHostingChangedPayload _build() {
    final _$result =
        _$v ??
        _$AgentHostingChangedPayload._(
          agentId: BuiltValueNullFieldError.checkNotNull(
            agentId,
            r'AgentHostingChangedPayload',
            'agentId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
