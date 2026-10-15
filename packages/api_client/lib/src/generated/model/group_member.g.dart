// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'group_member.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GroupMember extends GroupMember {
  @override
  final PrincipalRef user;
  @override
  final String? addedBy;
  @override
  final DateTime addedAt;
  @override
  final DateTime? expiresAt;

  factory _$GroupMember([void Function(GroupMemberBuilder)? updates]) =>
      (GroupMemberBuilder()..update(updates))._build();

  _$GroupMember._({
    required this.user,
    this.addedBy,
    required this.addedAt,
    this.expiresAt,
  }) : super._();
  @override
  GroupMember rebuild(void Function(GroupMemberBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GroupMemberBuilder toBuilder() => GroupMemberBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GroupMember &&
        user == other.user &&
        addedBy == other.addedBy &&
        addedAt == other.addedAt &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, user.hashCode);
    _$hash = $jc(_$hash, addedBy.hashCode);
    _$hash = $jc(_$hash, addedAt.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GroupMember')
          ..add('user', user)
          ..add('addedBy', addedBy)
          ..add('addedAt', addedAt)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class GroupMemberBuilder implements Builder<GroupMember, GroupMemberBuilder> {
  _$GroupMember? _$v;

  PrincipalRefBuilder? _user;
  PrincipalRefBuilder get user => _$this._user ??= PrincipalRefBuilder();
  set user(PrincipalRefBuilder? user) => _$this._user = user;

  String? _addedBy;
  String? get addedBy => _$this._addedBy;
  set addedBy(String? addedBy) => _$this._addedBy = addedBy;

  DateTime? _addedAt;
  DateTime? get addedAt => _$this._addedAt;
  set addedAt(DateTime? addedAt) => _$this._addedAt = addedAt;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  GroupMemberBuilder() {
    GroupMember._defaults(this);
  }

  GroupMemberBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _user = $v.user.toBuilder();
      _addedBy = $v.addedBy;
      _addedAt = $v.addedAt;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GroupMember other) {
    _$v = other as _$GroupMember;
  }

  @override
  void update(void Function(GroupMemberBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GroupMember build() => _build();

  _$GroupMember _build() {
    _$GroupMember _$result;
    try {
      _$result =
          _$v ??
          _$GroupMember._(
            user: user.build(),
            addedBy: addedBy,
            addedAt: BuiltValueNullFieldError.checkNotNull(
              addedAt,
              r'GroupMember',
              'addedAt',
            ),
            expiresAt: expiresAt,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'user';
        user.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GroupMember',
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
