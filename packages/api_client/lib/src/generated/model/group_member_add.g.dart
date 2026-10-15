// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'group_member_add.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GroupMemberAdd extends GroupMemberAdd {
  @override
  final String userId;
  @override
  final DateTime? expiresAt;

  factory _$GroupMemberAdd([void Function(GroupMemberAddBuilder)? updates]) =>
      (GroupMemberAddBuilder()..update(updates))._build();

  _$GroupMemberAdd._({required this.userId, this.expiresAt}) : super._();
  @override
  GroupMemberAdd rebuild(void Function(GroupMemberAddBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GroupMemberAddBuilder toBuilder() => GroupMemberAddBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GroupMemberAdd &&
        userId == other.userId &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GroupMemberAdd')
          ..add('userId', userId)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class GroupMemberAddBuilder
    implements Builder<GroupMemberAdd, GroupMemberAddBuilder> {
  _$GroupMemberAdd? _$v;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  GroupMemberAddBuilder() {
    GroupMemberAdd._defaults(this);
  }

  GroupMemberAddBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _userId = $v.userId;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GroupMemberAdd other) {
    _$v = other as _$GroupMemberAdd;
  }

  @override
  void update(void Function(GroupMemberAddBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GroupMemberAdd build() => _build();

  _$GroupMemberAdd _build() {
    final _$result =
        _$v ??
        _$GroupMemberAdd._(
          userId: BuiltValueNullFieldError.checkNotNull(
            userId,
            r'GroupMemberAdd',
            'userId',
          ),
          expiresAt: expiresAt,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
