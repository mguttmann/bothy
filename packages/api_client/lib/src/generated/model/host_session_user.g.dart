// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_session_user.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostSessionUser extends HostSessionUser {
  @override
  final String userId;
  @override
  final String osUser;
  @override
  final DateTime createdAt;

  factory _$HostSessionUser([void Function(HostSessionUserBuilder)? updates]) =>
      (HostSessionUserBuilder()..update(updates))._build();

  _$HostSessionUser._({
    required this.userId,
    required this.osUser,
    required this.createdAt,
  }) : super._();
  @override
  HostSessionUser rebuild(void Function(HostSessionUserBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostSessionUserBuilder toBuilder() => HostSessionUserBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostSessionUser &&
        userId == other.userId &&
        osUser == other.osUser &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, osUser.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostSessionUser')
          ..add('userId', userId)
          ..add('osUser', osUser)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class HostSessionUserBuilder
    implements Builder<HostSessionUser, HostSessionUserBuilder> {
  _$HostSessionUser? _$v;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _osUser;
  String? get osUser => _$this._osUser;
  set osUser(String? osUser) => _$this._osUser = osUser;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  HostSessionUserBuilder() {
    HostSessionUser._defaults(this);
  }

  HostSessionUserBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _userId = $v.userId;
      _osUser = $v.osUser;
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostSessionUser other) {
    _$v = other as _$HostSessionUser;
  }

  @override
  void update(void Function(HostSessionUserBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostSessionUser build() => _build();

  _$HostSessionUser _build() {
    final _$result =
        _$v ??
        _$HostSessionUser._(
          userId: BuiltValueNullFieldError.checkNotNull(
            userId,
            r'HostSessionUser',
            'userId',
          ),
          osUser: BuiltValueNullFieldError.checkNotNull(
            osUser,
            r'HostSessionUser',
            'osUser',
          ),
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'HostSessionUser',
            'createdAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
