// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'me.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Me extends Me {
  @override
  final PrincipalRef principal;
  @override
  final bool isOwner;
  @override
  final String? email;
  @override
  final String? locale;
  @override
  final String? timeZone;
  @override
  final int permissionVersion;
  @override
  final String? sessionId;
  @override
  final String? deviceId;
  @override
  final DateTime? stepUpValidUntil;

  factory _$Me([void Function(MeBuilder)? updates]) =>
      (MeBuilder()..update(updates))._build();

  _$Me._({
    required this.principal,
    required this.isOwner,
    this.email,
    this.locale,
    this.timeZone,
    required this.permissionVersion,
    this.sessionId,
    this.deviceId,
    this.stepUpValidUntil,
  }) : super._();
  @override
  Me rebuild(void Function(MeBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  MeBuilder toBuilder() => MeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Me &&
        principal == other.principal &&
        isOwner == other.isOwner &&
        email == other.email &&
        locale == other.locale &&
        timeZone == other.timeZone &&
        permissionVersion == other.permissionVersion &&
        sessionId == other.sessionId &&
        deviceId == other.deviceId &&
        stepUpValidUntil == other.stepUpValidUntil;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, principal.hashCode);
    _$hash = $jc(_$hash, isOwner.hashCode);
    _$hash = $jc(_$hash, email.hashCode);
    _$hash = $jc(_$hash, locale.hashCode);
    _$hash = $jc(_$hash, timeZone.hashCode);
    _$hash = $jc(_$hash, permissionVersion.hashCode);
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, deviceId.hashCode);
    _$hash = $jc(_$hash, stepUpValidUntil.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Me')
          ..add('principal', principal)
          ..add('isOwner', isOwner)
          ..add('email', email)
          ..add('locale', locale)
          ..add('timeZone', timeZone)
          ..add('permissionVersion', permissionVersion)
          ..add('sessionId', sessionId)
          ..add('deviceId', deviceId)
          ..add('stepUpValidUntil', stepUpValidUntil))
        .toString();
  }
}

class MeBuilder implements Builder<Me, MeBuilder> {
  _$Me? _$v;

  PrincipalRefBuilder? _principal;
  PrincipalRefBuilder get principal =>
      _$this._principal ??= PrincipalRefBuilder();
  set principal(PrincipalRefBuilder? principal) =>
      _$this._principal = principal;

  bool? _isOwner;
  bool? get isOwner => _$this._isOwner;
  set isOwner(bool? isOwner) => _$this._isOwner = isOwner;

  String? _email;
  String? get email => _$this._email;
  set email(String? email) => _$this._email = email;

  String? _locale;
  String? get locale => _$this._locale;
  set locale(String? locale) => _$this._locale = locale;

  String? _timeZone;
  String? get timeZone => _$this._timeZone;
  set timeZone(String? timeZone) => _$this._timeZone = timeZone;

  int? _permissionVersion;
  int? get permissionVersion => _$this._permissionVersion;
  set permissionVersion(int? permissionVersion) =>
      _$this._permissionVersion = permissionVersion;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  String? _deviceId;
  String? get deviceId => _$this._deviceId;
  set deviceId(String? deviceId) => _$this._deviceId = deviceId;

  DateTime? _stepUpValidUntil;
  DateTime? get stepUpValidUntil => _$this._stepUpValidUntil;
  set stepUpValidUntil(DateTime? stepUpValidUntil) =>
      _$this._stepUpValidUntil = stepUpValidUntil;

  MeBuilder() {
    Me._defaults(this);
  }

  MeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _principal = $v.principal.toBuilder();
      _isOwner = $v.isOwner;
      _email = $v.email;
      _locale = $v.locale;
      _timeZone = $v.timeZone;
      _permissionVersion = $v.permissionVersion;
      _sessionId = $v.sessionId;
      _deviceId = $v.deviceId;
      _stepUpValidUntil = $v.stepUpValidUntil;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Me other) {
    _$v = other as _$Me;
  }

  @override
  void update(void Function(MeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Me build() => _build();

  _$Me _build() {
    _$Me _$result;
    try {
      _$result =
          _$v ??
          _$Me._(
            principal: principal.build(),
            isOwner: BuiltValueNullFieldError.checkNotNull(
              isOwner,
              r'Me',
              'isOwner',
            ),
            email: email,
            locale: locale,
            timeZone: timeZone,
            permissionVersion: BuiltValueNullFieldError.checkNotNull(
              permissionVersion,
              r'Me',
              'permissionVersion',
            ),
            sessionId: sessionId,
            deviceId: deviceId,
            stepUpValidUntil: stepUpValidUntil,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'principal';
        principal.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(r'Me', _$failedField, e.toString());
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
