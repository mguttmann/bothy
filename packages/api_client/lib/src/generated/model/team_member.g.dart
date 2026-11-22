// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'team_member.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TeamMember extends TeamMember {
  @override
  final String id;
  @override
  final String handle;
  @override
  final String agentId;
  @override
  final String homeChannelId;
  @override
  final String workThreadId;
  @override
  final DateTime createdAt;

  factory _$TeamMember([void Function(TeamMemberBuilder)? updates]) =>
      (TeamMemberBuilder()..update(updates))._build();

  _$TeamMember._({
    required this.id,
    required this.handle,
    required this.agentId,
    required this.homeChannelId,
    required this.workThreadId,
    required this.createdAt,
  }) : super._();
  @override
  TeamMember rebuild(void Function(TeamMemberBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TeamMemberBuilder toBuilder() => TeamMemberBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TeamMember &&
        id == other.id &&
        handle == other.handle &&
        agentId == other.agentId &&
        homeChannelId == other.homeChannelId &&
        workThreadId == other.workThreadId &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, handle.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, homeChannelId.hashCode);
    _$hash = $jc(_$hash, workThreadId.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TeamMember')
          ..add('id', id)
          ..add('handle', handle)
          ..add('agentId', agentId)
          ..add('homeChannelId', homeChannelId)
          ..add('workThreadId', workThreadId)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class TeamMemberBuilder implements Builder<TeamMember, TeamMemberBuilder> {
  _$TeamMember? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

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

  String? _workThreadId;
  String? get workThreadId => _$this._workThreadId;
  set workThreadId(String? workThreadId) => _$this._workThreadId = workThreadId;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  TeamMemberBuilder() {
    TeamMember._defaults(this);
  }

  TeamMemberBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _handle = $v.handle;
      _agentId = $v.agentId;
      _homeChannelId = $v.homeChannelId;
      _workThreadId = $v.workThreadId;
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TeamMember other) {
    _$v = other as _$TeamMember;
  }

  @override
  void update(void Function(TeamMemberBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TeamMember build() => _build();

  _$TeamMember _build() {
    final _$result =
        _$v ??
        _$TeamMember._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'TeamMember', 'id'),
          handle: BuiltValueNullFieldError.checkNotNull(
            handle,
            r'TeamMember',
            'handle',
          ),
          agentId: BuiltValueNullFieldError.checkNotNull(
            agentId,
            r'TeamMember',
            'agentId',
          ),
          homeChannelId: BuiltValueNullFieldError.checkNotNull(
            homeChannelId,
            r'TeamMember',
            'homeChannelId',
          ),
          workThreadId: BuiltValueNullFieldError.checkNotNull(
            workThreadId,
            r'TeamMember',
            'workThreadId',
          ),
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'TeamMember',
            'createdAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
