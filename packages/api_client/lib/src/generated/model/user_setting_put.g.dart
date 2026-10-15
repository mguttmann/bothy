// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'user_setting_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UserSettingPut extends UserSettingPut {
  @override
  final JsonObject? value;

  factory _$UserSettingPut([void Function(UserSettingPutBuilder)? updates]) =>
      (UserSettingPutBuilder()..update(updates))._build();

  _$UserSettingPut._({this.value}) : super._();
  @override
  UserSettingPut rebuild(void Function(UserSettingPutBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UserSettingPutBuilder toBuilder() => UserSettingPutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UserSettingPut && value == other.value;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, value.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'UserSettingPut',
    )..add('value', value)).toString();
  }
}

class UserSettingPutBuilder
    implements Builder<UserSettingPut, UserSettingPutBuilder> {
  _$UserSettingPut? _$v;

  JsonObject? _value;
  JsonObject? get value => _$this._value;
  set value(JsonObject? value) => _$this._value = value;

  UserSettingPutBuilder() {
    UserSettingPut._defaults(this);
  }

  UserSettingPutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _value = $v.value;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UserSettingPut other) {
    _$v = other as _$UserSettingPut;
  }

  @override
  void update(void Function(UserSettingPutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UserSettingPut build() => _build();

  _$UserSettingPut _build() {
    final _$result = _$v ?? _$UserSettingPut._(value: value);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
