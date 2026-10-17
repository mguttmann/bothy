// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_presence_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentPresencePayload extends AgentPresencePayload {
  @override
  final String agentId;
  @override
  final AgentRuntimeState state;
  @override
  final BillingMode? reportedBillingMode;

  factory _$AgentPresencePayload([
    void Function(AgentPresencePayloadBuilder)? updates,
  ]) => (AgentPresencePayloadBuilder()..update(updates))._build();

  _$AgentPresencePayload._({
    required this.agentId,
    required this.state,
    this.reportedBillingMode,
  }) : super._();
  @override
  AgentPresencePayload rebuild(
    void Function(AgentPresencePayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentPresencePayloadBuilder toBuilder() =>
      AgentPresencePayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentPresencePayload &&
        agentId == other.agentId &&
        state == other.state &&
        reportedBillingMode == other.reportedBillingMode;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, reportedBillingMode.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentPresencePayload')
          ..add('agentId', agentId)
          ..add('state', state)
          ..add('reportedBillingMode', reportedBillingMode))
        .toString();
  }
}

class AgentPresencePayloadBuilder
    implements Builder<AgentPresencePayload, AgentPresencePayloadBuilder> {
  _$AgentPresencePayload? _$v;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  AgentRuntimeState? _state;
  AgentRuntimeState? get state => _$this._state;
  set state(AgentRuntimeState? state) => _$this._state = state;

  BillingMode? _reportedBillingMode;
  BillingMode? get reportedBillingMode => _$this._reportedBillingMode;
  set reportedBillingMode(BillingMode? reportedBillingMode) =>
      _$this._reportedBillingMode = reportedBillingMode;

  AgentPresencePayloadBuilder() {
    AgentPresencePayload._defaults(this);
  }

  AgentPresencePayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _agentId = $v.agentId;
      _state = $v.state;
      _reportedBillingMode = $v.reportedBillingMode;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentPresencePayload other) {
    _$v = other as _$AgentPresencePayload;
  }

  @override
  void update(void Function(AgentPresencePayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentPresencePayload build() => _build();

  _$AgentPresencePayload _build() {
    final _$result =
        _$v ??
        _$AgentPresencePayload._(
          agentId: BuiltValueNullFieldError.checkNotNull(
            agentId,
            r'AgentPresencePayload',
            'agentId',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'AgentPresencePayload',
            'state',
          ),
          reportedBillingMode: reportedBillingMode,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
