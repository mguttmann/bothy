// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'team_member_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TeamMemberCreate extends TeamMemberCreate {
  @override
  final String agentId;
  @override
  final String homeChannelId;
  @override
  final String handle;

  factory _$TeamMemberCreate([
    void Function(TeamMemberCreateBuilder)? updates,
  ]) => (TeamMemberCreateBuilder()..update(updates))._build();

  _$TeamMemberCreate._({
    required this.agentId,
    required this.homeChannelId,
    required this.handle,
  }) : super._();
  @override
  TeamMemberCreate rebuild(void Function(TeamMemberCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TeamMemberCreateBuilder toBuilder() =>
      TeamMemberCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TeamMemberCreate &&
        agentId == other.agentId &&
        homeChannelId == other.homeChannelId &&
        handle == other.handle;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, homeChannelId.hashCode);
    _$hash = $jc(_$hash, handle.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TeamMemberCreate')
          ..add('agentId', agentId)
          ..add('homeChannelId', homeChannelId)
          ..add('handle', handle))
        .toString();
  }
}

class TeamMemberCreateBuilder
    implements Builder<TeamMemberCreate, TeamMemberCreateBuilder> {
  _$TeamMemberCreate? _$v;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _homeChannelId;
  String? get homeChannelId => _$this._homeChannelId;
  set homeChannelId(String? homeChannelId) =>
      _$this._homeChannelId = homeChannelId;

  String? _handle;
  String? get handle => _$this._handle;
  set handle(String? handle) => _$this._handle = handle;

  TeamMemberCreateBuilder() {
    TeamMemberCreate._defaults(this);
  }

  TeamMemberCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _agentId = $v.agentId;
      _homeChannelId = $v.homeChannelId;
      _handle = $v.handle;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TeamMemberCreate other) {
    _$v = other as _$TeamMemberCreate;
  }

  @override
  void update(void Function(TeamMemberCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TeamMemberCreate build() => _build();

  _$TeamMemberCreate _build() {
    final _$result =
        _$v ??
        _$TeamMemberCreate._(
          agentId: BuiltValueNullFieldError.checkNotNull(
            agentId,
            r'TeamMemberCreate',
            'agentId',
          ),
          homeChannelId: BuiltValueNullFieldError.checkNotNull(
            homeChannelId,
            r'TeamMemberCreate',
            'homeChannelId',
          ),
          handle: BuiltValueNullFieldError.checkNotNull(
            handle,
            r'TeamMemberCreate',
            'handle',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
