// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'user_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UserPayload extends UserPayload {
  @override
  final User user;

  factory _$UserPayload([void Function(UserPayloadBuilder)? updates]) =>
      (UserPayloadBuilder()..update(updates))._build();

  _$UserPayload._({required this.user}) : super._();
  @override
  UserPayload rebuild(void Function(UserPayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UserPayloadBuilder toBuilder() => UserPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UserPayload && user == other.user;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, user.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'UserPayload',
    )..add('user', user)).toString();
  }
}

class UserPayloadBuilder implements Builder<UserPayload, UserPayloadBuilder> {
  _$UserPayload? _$v;

  UserBuilder? _user;
  UserBuilder get user => _$this._user ??= UserBuilder();
  set user(UserBuilder? user) => _$this._user = user;

  UserPayloadBuilder() {
    UserPayload._defaults(this);
  }

  UserPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _user = $v.user.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UserPayload other) {
    _$v = other as _$UserPayload;
  }

  @override
  void update(void Function(UserPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UserPayload build() => _build();

  _$UserPayload _build() {
    _$UserPayload _$result;
    try {
      _$result = _$v ?? _$UserPayload._(user: user.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'user';
        user.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'UserPayload',
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
