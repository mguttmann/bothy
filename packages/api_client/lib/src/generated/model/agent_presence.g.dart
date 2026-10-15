// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_presence.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentPresence extends AgentPresence {
  @override
  final String agentId;
  @override
  final AgentRuntimeState state;
  @override
  final BillingMode? reportedBillingMode;

  factory _$AgentPresence([void Function(AgentPresenceBuilder)? updates]) =>
      (AgentPresenceBuilder()..update(updates))._build();

  _$AgentPresence._({
    required this.agentId,
    required this.state,
    this.reportedBillingMode,
  }) : super._();
  @override
  AgentPresence rebuild(void Function(AgentPresenceBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentPresenceBuilder toBuilder() => AgentPresenceBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentPresence &&
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
    return (newBuiltValueToStringHelper(r'AgentPresence')
          ..add('agentId', agentId)
          ..add('state', state)
          ..add('reportedBillingMode', reportedBillingMode))
        .toString();
  }
}

class AgentPresenceBuilder
    implements Builder<AgentPresence, AgentPresenceBuilder> {
  _$AgentPresence? _$v;

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

  AgentPresenceBuilder() {
    AgentPresence._defaults(this);
  }

  AgentPresenceBuilder get _$this {
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
  void replace(AgentPresence other) {
    _$v = other as _$AgentPresence;
  }

  @override
  void update(void Function(AgentPresenceBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentPresence build() => _build();

  _$AgentPresence _build() {
    final _$result =
        _$v ??
        _$AgentPresence._(
          agentId: BuiltValueNullFieldError.checkNotNull(
            agentId,
            r'AgentPresence',
            'agentId',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'AgentPresence',
            'state',
          ),
          reportedBillingMode: reportedBillingMode,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
