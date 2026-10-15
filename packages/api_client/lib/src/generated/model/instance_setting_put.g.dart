// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'instance_setting_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InstanceSettingPut extends InstanceSettingPut {
  @override
  final JsonObject? value;

  factory _$InstanceSettingPut([
    void Function(InstanceSettingPutBuilder)? updates,
  ]) => (InstanceSettingPutBuilder()..update(updates))._build();

  _$InstanceSettingPut._({this.value}) : super._();
  @override
  InstanceSettingPut rebuild(
    void Function(InstanceSettingPutBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  InstanceSettingPutBuilder toBuilder() =>
      InstanceSettingPutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InstanceSettingPut && value == other.value;
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
      r'InstanceSettingPut',
    )..add('value', value)).toString();
  }
}

class InstanceSettingPutBuilder
    implements Builder<InstanceSettingPut, InstanceSettingPutBuilder> {
  _$InstanceSettingPut? _$v;

  JsonObject? _value;
  JsonObject? get value => _$this._value;
  set value(JsonObject? value) => _$this._value = value;

  InstanceSettingPutBuilder() {
    InstanceSettingPut._defaults(this);
  }

  InstanceSettingPutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _value = $v.value;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InstanceSettingPut other) {
    _$v = other as _$InstanceSettingPut;
  }

  @override
  void update(void Function(InstanceSettingPutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InstanceSettingPut build() => _build();

  _$InstanceSettingPut _build() {
    final _$result = _$v ?? _$InstanceSettingPut._(value: value);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
