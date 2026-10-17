// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'user_created.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UserCreated extends UserCreated {
  @override
  final User user;
  @override
  final EnrollmentCode? enrollment;

  factory _$UserCreated([void Function(UserCreatedBuilder)? updates]) =>
      (UserCreatedBuilder()..update(updates))._build();

  _$UserCreated._({required this.user, this.enrollment}) : super._();
  @override
  UserCreated rebuild(void Function(UserCreatedBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UserCreatedBuilder toBuilder() => UserCreatedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UserCreated &&
        user == other.user &&
        enrollment == other.enrollment;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, user.hashCode);
    _$hash = $jc(_$hash, enrollment.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'UserCreated')
          ..add('user', user)
          ..add('enrollment', enrollment))
        .toString();
  }
}

class UserCreatedBuilder implements Builder<UserCreated, UserCreatedBuilder> {
  _$UserCreated? _$v;

  UserBuilder? _user;
  UserBuilder get user => _$this._user ??= UserBuilder();
  set user(UserBuilder? user) => _$this._user = user;

  EnrollmentCodeBuilder? _enrollment;
  EnrollmentCodeBuilder get enrollment =>
      _$this._enrollment ??= EnrollmentCodeBuilder();
  set enrollment(EnrollmentCodeBuilder? enrollment) =>
      _$this._enrollment = enrollment;

  UserCreatedBuilder() {
    UserCreated._defaults(this);
  }

  UserCreatedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _user = $v.user.toBuilder();
      _enrollment = $v.enrollment?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UserCreated other) {
    _$v = other as _$UserCreated;
  }

  @override
  void update(void Function(UserCreatedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UserCreated build() => _build();

  _$UserCreated _build() {
    _$UserCreated _$result;
    try {
      _$result =
          _$v ??
          _$UserCreated._(user: user.build(), enrollment: _enrollment?.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'user';
        user.build();
        _$failedField = 'enrollment';
        _enrollment?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'UserCreated',
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
