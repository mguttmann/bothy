// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'settings_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SettingsPayload extends SettingsPayload {
  @override
  final UserSetting setting;
  @override
  final bool? deleted;

  factory _$SettingsPayload([void Function(SettingsPayloadBuilder)? updates]) =>
      (SettingsPayloadBuilder()..update(updates))._build();

  _$SettingsPayload._({required this.setting, this.deleted}) : super._();
  @override
  SettingsPayload rebuild(void Function(SettingsPayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SettingsPayloadBuilder toBuilder() => SettingsPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SettingsPayload &&
        setting == other.setting &&
        deleted == other.deleted;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, setting.hashCode);
    _$hash = $jc(_$hash, deleted.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SettingsPayload')
          ..add('setting', setting)
          ..add('deleted', deleted))
        .toString();
  }
}

class SettingsPayloadBuilder
    implements Builder<SettingsPayload, SettingsPayloadBuilder> {
  _$SettingsPayload? _$v;

  UserSettingBuilder? _setting;
  UserSettingBuilder get setting => _$this._setting ??= UserSettingBuilder();
  set setting(UserSettingBuilder? setting) => _$this._setting = setting;

  bool? _deleted;
  bool? get deleted => _$this._deleted;
  set deleted(bool? deleted) => _$this._deleted = deleted;

  SettingsPayloadBuilder() {
    SettingsPayload._defaults(this);
  }

  SettingsPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _setting = $v.setting.toBuilder();
      _deleted = $v.deleted;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SettingsPayload other) {
    _$v = other as _$SettingsPayload;
  }

  @override
  void update(void Function(SettingsPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SettingsPayload build() => _build();

  _$SettingsPayload _build() {
    _$SettingsPayload _$result;
    try {
      _$result =
          _$v ??
          _$SettingsPayload._(setting: setting.build(), deleted: deleted);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'setting';
        setting.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SettingsPayload',
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
