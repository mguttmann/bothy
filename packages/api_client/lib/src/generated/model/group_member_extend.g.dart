// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'group_member_extend.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GroupMemberExtend extends GroupMemberExtend {
  @override
  final DateTime expiresAt;

  factory _$GroupMemberExtend([
    void Function(GroupMemberExtendBuilder)? updates,
  ]) => (GroupMemberExtendBuilder()..update(updates))._build();

  _$GroupMemberExtend._({required this.expiresAt}) : super._();
  @override
  GroupMemberExtend rebuild(void Function(GroupMemberExtendBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GroupMemberExtendBuilder toBuilder() =>
      GroupMemberExtendBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GroupMemberExtend && expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'GroupMemberExtend',
    )..add('expiresAt', expiresAt)).toString();
  }
}

class GroupMemberExtendBuilder
    implements Builder<GroupMemberExtend, GroupMemberExtendBuilder> {
  _$GroupMemberExtend? _$v;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  GroupMemberExtendBuilder() {
    GroupMemberExtend._defaults(this);
  }

  GroupMemberExtendBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GroupMemberExtend other) {
    _$v = other as _$GroupMemberExtend;
  }

  @override
  void update(void Function(GroupMemberExtendBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GroupMemberExtend build() => _build();

  _$GroupMemberExtend _build() {
    final _$result =
        _$v ??
        _$GroupMemberExtend._(
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'GroupMemberExtend',
            'expiresAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
