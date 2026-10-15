// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'instance_setting.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InstanceSetting extends InstanceSetting {
  @override
  final String key;
  @override
  final JsonObject? value;
  @override
  final int version;
  @override
  final String? updatedBy;
  @override
  final DateTime? updatedAt;

  factory _$InstanceSetting([void Function(InstanceSettingBuilder)? updates]) =>
      (InstanceSettingBuilder()..update(updates))._build();

  _$InstanceSetting._({
    required this.key,
    this.value,
    required this.version,
    this.updatedBy,
    this.updatedAt,
  }) : super._();
  @override
  InstanceSetting rebuild(void Function(InstanceSettingBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  InstanceSettingBuilder toBuilder() => InstanceSettingBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InstanceSetting &&
        key == other.key &&
        value == other.value &&
        version == other.version &&
        updatedBy == other.updatedBy &&
        updatedAt == other.updatedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, key.hashCode);
    _$hash = $jc(_$hash, value.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, updatedBy.hashCode);
    _$hash = $jc(_$hash, updatedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'InstanceSetting')
          ..add('key', key)
          ..add('value', value)
          ..add('version', version)
          ..add('updatedBy', updatedBy)
          ..add('updatedAt', updatedAt))
        .toString();
  }
}

class InstanceSettingBuilder
    implements Builder<InstanceSetting, InstanceSettingBuilder> {
  _$InstanceSetting? _$v;

  String? _key;
  String? get key => _$this._key;
  set key(String? key) => _$this._key = key;

  JsonObject? _value;
  JsonObject? get value => _$this._value;
  set value(JsonObject? value) => _$this._value = value;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  String? _updatedBy;
  String? get updatedBy => _$this._updatedBy;
  set updatedBy(String? updatedBy) => _$this._updatedBy = updatedBy;

  DateTime? _updatedAt;
  DateTime? get updatedAt => _$this._updatedAt;
  set updatedAt(DateTime? updatedAt) => _$this._updatedAt = updatedAt;

  InstanceSettingBuilder() {
    InstanceSetting._defaults(this);
  }

  InstanceSettingBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _key = $v.key;
      _value = $v.value;
      _version = $v.version;
      _updatedBy = $v.updatedBy;
      _updatedAt = $v.updatedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InstanceSetting other) {
    _$v = other as _$InstanceSetting;
  }

  @override
  void update(void Function(InstanceSettingBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InstanceSetting build() => _build();

  _$InstanceSetting _build() {
    final _$result =
        _$v ??
        _$InstanceSetting._(
          key: BuiltValueNullFieldError.checkNotNull(
            key,
            r'InstanceSetting',
            'key',
          ),
          value: value,
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'InstanceSetting',
            'version',
          ),
          updatedBy: updatedBy,
          updatedAt: updatedAt,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
