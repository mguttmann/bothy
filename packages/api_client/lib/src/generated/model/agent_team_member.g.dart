// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_team_member.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentTeamMember extends AgentTeamMember {
  @override
  final String memberId;
  @override
  final String handle;
  @override
  final String agentId;
  @override
  final String homeChannelId;

  factory _$AgentTeamMember([void Function(AgentTeamMemberBuilder)? updates]) =>
      (AgentTeamMemberBuilder()..update(updates))._build();

  _$AgentTeamMember._({
    required this.memberId,
    required this.handle,
    required this.agentId,
    required this.homeChannelId,
  }) : super._();
  @override
  AgentTeamMember rebuild(void Function(AgentTeamMemberBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentTeamMemberBuilder toBuilder() => AgentTeamMemberBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentTeamMember &&
        memberId == other.memberId &&
        handle == other.handle &&
        agentId == other.agentId &&
        homeChannelId == other.homeChannelId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, memberId.hashCode);
    _$hash = $jc(_$hash, handle.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, homeChannelId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentTeamMember')
          ..add('memberId', memberId)
          ..add('handle', handle)
          ..add('agentId', agentId)
          ..add('homeChannelId', homeChannelId))
        .toString();
  }
}

class AgentTeamMemberBuilder
    implements Builder<AgentTeamMember, AgentTeamMemberBuilder> {
  _$AgentTeamMember? _$v;

  String? _memberId;
  String? get memberId => _$this._memberId;
  set memberId(String? memberId) => _$this._memberId = memberId;

  String? _handle;
  String? get handle => _$this._handle;
  set handle(String? handle) => _$this._handle = handle;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _homeChannelId;
  String? get homeChannelId => _$this._homeChannelId;
  set homeChannelId(String? homeChannelId) =>
      _$this._homeChannelId = homeChannelId;

  AgentTeamMemberBuilder() {
    AgentTeamMember._defaults(this);
  }

  AgentTeamMemberBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _memberId = $v.memberId;
      _handle = $v.handle;
      _agentId = $v.agentId;
      _homeChannelId = $v.homeChannelId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentTeamMember other) {
    _$v = other as _$AgentTeamMember;
  }

  @override
  void update(void Function(AgentTeamMemberBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentTeamMember build() => _build();

  _$AgentTeamMember _build() {
    final _$result =
        _$v ??
        _$AgentTeamMember._(
          memberId: BuiltValueNullFieldError.checkNotNull(
            memberId,
            r'AgentTeamMember',
            'memberId',
          ),
          handle: BuiltValueNullFieldError.checkNotNull(
            handle,
            r'AgentTeamMember',
            'handle',
          ),
          agentId: BuiltValueNullFieldError.checkNotNull(
            agentId,
            r'AgentTeamMember',
            'agentId',
          ),
          homeChannelId: BuiltValueNullFieldError.checkNotNull(
            homeChannelId,
            r'AgentTeamMember',
            'homeChannelId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
