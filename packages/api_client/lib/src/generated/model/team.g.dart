// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'team.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Team extends Team {
  @override
  final String channelId;
  @override
  final String sponsorId;
  @override
  final int memberTurns;
  @override
  final int turnBudget;
  @override
  final TeamPauseReason? pauseReason;
  @override
  final BuiltList<TeamMember> members;
  @override
  final DateTime createdAt;
  @override
  final int version;

  factory _$Team([void Function(TeamBuilder)? updates]) =>
      (TeamBuilder()..update(updates))._build();

  _$Team._({
    required this.channelId,
    required this.sponsorId,
    required this.memberTurns,
    required this.turnBudget,
    this.pauseReason,
    required this.members,
    required this.createdAt,
    required this.version,
  }) : super._();
  @override
  Team rebuild(void Function(TeamBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TeamBuilder toBuilder() => TeamBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Team &&
        channelId == other.channelId &&
        sponsorId == other.sponsorId &&
        memberTurns == other.memberTurns &&
        turnBudget == other.turnBudget &&
        pauseReason == other.pauseReason &&
        members == other.members &&
        createdAt == other.createdAt &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, sponsorId.hashCode);
    _$hash = $jc(_$hash, memberTurns.hashCode);
    _$hash = $jc(_$hash, turnBudget.hashCode);
    _$hash = $jc(_$hash, pauseReason.hashCode);
    _$hash = $jc(_$hash, members.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Team')
          ..add('channelId', channelId)
          ..add('sponsorId', sponsorId)
          ..add('memberTurns', memberTurns)
          ..add('turnBudget', turnBudget)
          ..add('pauseReason', pauseReason)
          ..add('members', members)
          ..add('createdAt', createdAt)
          ..add('version', version))
        .toString();
  }
}

class TeamBuilder implements Builder<Team, TeamBuilder> {
  _$Team? _$v;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _sponsorId;
  String? get sponsorId => _$this._sponsorId;
  set sponsorId(String? sponsorId) => _$this._sponsorId = sponsorId;

  int? _memberTurns;
  int? get memberTurns => _$this._memberTurns;
  set memberTurns(int? memberTurns) => _$this._memberTurns = memberTurns;

  int? _turnBudget;
  int? get turnBudget => _$this._turnBudget;
  set turnBudget(int? turnBudget) => _$this._turnBudget = turnBudget;

  TeamPauseReason? _pauseReason;
  TeamPauseReason? get pauseReason => _$this._pauseReason;
  set pauseReason(TeamPauseReason? pauseReason) =>
      _$this._pauseReason = pauseReason;

  ListBuilder<TeamMember>? _members;
  ListBuilder<TeamMember> get members =>
      _$this._members ??= ListBuilder<TeamMember>();
  set members(ListBuilder<TeamMember>? members) => _$this._members = members;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  TeamBuilder() {
    Team._defaults(this);
  }

  TeamBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelId = $v.channelId;
      _sponsorId = $v.sponsorId;
      _memberTurns = $v.memberTurns;
      _turnBudget = $v.turnBudget;
      _pauseReason = $v.pauseReason;
      _members = $v.members.toBuilder();
      _createdAt = $v.createdAt;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Team other) {
    _$v = other as _$Team;
  }

  @override
  void update(void Function(TeamBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Team build() => _build();

  _$Team _build() {
    _$Team _$result;
    try {
      _$result =
          _$v ??
          _$Team._(
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'Team',
              'channelId',
            ),
            sponsorId: BuiltValueNullFieldError.checkNotNull(
              sponsorId,
              r'Team',
              'sponsorId',
            ),
            memberTurns: BuiltValueNullFieldError.checkNotNull(
              memberTurns,
              r'Team',
              'memberTurns',
            ),
            turnBudget: BuiltValueNullFieldError.checkNotNull(
              turnBudget,
              r'Team',
              'turnBudget',
            ),
            pauseReason: pauseReason,
            members: members.build(),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'Team',
              'createdAt',
            ),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'Team',
              'version',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'members';
        members.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(r'Team', _$failedField, e.toString());
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
