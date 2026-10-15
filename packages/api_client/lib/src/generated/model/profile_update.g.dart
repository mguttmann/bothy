// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'profile_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ProfileUpdate extends ProfileUpdate {
  @override
  final String? displayName;
  @override
  final String? locale;
  @override
  final String? timeZone;

  factory _$ProfileUpdate([void Function(ProfileUpdateBuilder)? updates]) =>
      (ProfileUpdateBuilder()..update(updates))._build();

  _$ProfileUpdate._({this.displayName, this.locale, this.timeZone}) : super._();
  @override
  ProfileUpdate rebuild(void Function(ProfileUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ProfileUpdateBuilder toBuilder() => ProfileUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProfileUpdate &&
        displayName == other.displayName &&
        locale == other.locale &&
        timeZone == other.timeZone;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, displayName.hashCode);
    _$hash = $jc(_$hash, locale.hashCode);
    _$hash = $jc(_$hash, timeZone.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProfileUpdate')
          ..add('displayName', displayName)
          ..add('locale', locale)
          ..add('timeZone', timeZone))
        .toString();
  }
}

class ProfileUpdateBuilder
    implements Builder<ProfileUpdate, ProfileUpdateBuilder> {
  _$ProfileUpdate? _$v;

  String? _displayName;
  String? get displayName => _$this._displayName;
  set displayName(String? displayName) => _$this._displayName = displayName;

  String? _locale;
  String? get locale => _$this._locale;
  set locale(String? locale) => _$this._locale = locale;

  String? _timeZone;
  String? get timeZone => _$this._timeZone;
  set timeZone(String? timeZone) => _$this._timeZone = timeZone;

  ProfileUpdateBuilder() {
    ProfileUpdate._defaults(this);
  }

  ProfileUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _displayName = $v.displayName;
      _locale = $v.locale;
      _timeZone = $v.timeZone;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProfileUpdate other) {
    _$v = other as _$ProfileUpdate;
  }

  @override
  void update(void Function(ProfileUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProfileUpdate build() => _build();

  _$ProfileUpdate _build() {
    final _$result =
        _$v ??
        _$ProfileUpdate._(
          displayName: displayName,
          locale: locale,
          timeZone: timeZone,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
