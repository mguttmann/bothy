// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'user_setting.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UserSetting extends UserSetting {
  @override
  final String key;
  @override
  final JsonObject? value;
  @override
  final int version;
  @override
  final String? updatedByDeviceId;
  @override
  final DateTime? updatedAt;

  factory _$UserSetting([void Function(UserSettingBuilder)? updates]) =>
      (UserSettingBuilder()..update(updates))._build();

  _$UserSetting._({
    required this.key,
    this.value,
    required this.version,
    this.updatedByDeviceId,
    this.updatedAt,
  }) : super._();
  @override
  UserSetting rebuild(void Function(UserSettingBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UserSettingBuilder toBuilder() => UserSettingBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UserSetting &&
        key == other.key &&
        value == other.value &&
        version == other.version &&
        updatedByDeviceId == other.updatedByDeviceId &&
        updatedAt == other.updatedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, key.hashCode);
    _$hash = $jc(_$hash, value.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, updatedByDeviceId.hashCode);
    _$hash = $jc(_$hash, updatedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'UserSetting')
          ..add('key', key)
          ..add('value', value)
          ..add('version', version)
          ..add('updatedByDeviceId', updatedByDeviceId)
          ..add('updatedAt', updatedAt))
        .toString();
  }
}

class UserSettingBuilder implements Builder<UserSetting, UserSettingBuilder> {
  _$UserSetting? _$v;

  String? _key;
  String? get key => _$this._key;
  set key(String? key) => _$this._key = key;

  JsonObject? _value;
  JsonObject? get value => _$this._value;
  set value(JsonObject? value) => _$this._value = value;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  String? _updatedByDeviceId;
  String? get updatedByDeviceId => _$this._updatedByDeviceId;
  set updatedByDeviceId(String? updatedByDeviceId) =>
      _$this._updatedByDeviceId = updatedByDeviceId;

  DateTime? _updatedAt;
  DateTime? get updatedAt => _$this._updatedAt;
  set updatedAt(DateTime? updatedAt) => _$this._updatedAt = updatedAt;

  UserSettingBuilder() {
    UserSetting._defaults(this);
  }

  UserSettingBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _key = $v.key;
      _value = $v.value;
      _version = $v.version;
      _updatedByDeviceId = $v.updatedByDeviceId;
      _updatedAt = $v.updatedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UserSetting other) {
    _$v = other as _$UserSetting;
  }

  @override
  void update(void Function(UserSettingBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UserSetting build() => _build();

  _$UserSetting _build() {
    final _$result =
        _$v ??
        _$UserSetting._(
          key: BuiltValueNullFieldError.checkNotNull(
            key,
            r'UserSetting',
            'key',
          ),
          value: value,
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'UserSetting',
            'version',
          ),
          updatedByDeviceId: updatedByDeviceId,
          updatedAt: updatedAt,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
