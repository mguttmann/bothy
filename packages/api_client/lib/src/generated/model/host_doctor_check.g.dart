// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_doctor_check.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnum_hd1User =
    const HostDoctorCheckCheckEnum._('hd1User');
const HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnum_hd2Groups =
    const HostDoctorCheckCheckEnum._('hd2Groups');
const HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnum_hd3Sudo =
    const HostDoctorCheckCheckEnum._('hd3Sudo');
const HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnum_hd4StateDir =
    const HostDoctorCheckCheckEnum._('hd4StateDir');
const HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnum_hd5ServerHost =
    const HostDoctorCheckCheckEnum._('hd5ServerHost');
const HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnum_hd6LlmLogin =
    const HostDoctorCheckCheckEnum._('hd6LlmLogin');
const HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnum_hd7Profiles =
    const HostDoctorCheckCheckEnum._('hd7Profiles');
const HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnum_hd8Separation =
    const HostDoctorCheckCheckEnum._('hd8Separation');
const HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnum_hd9ForeignHomes =
    const HostDoctorCheckCheckEnum._('hd9ForeignHomes');
const HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnum_hd10HelperIntegrity =
    const HostDoctorCheckCheckEnum._('hd10HelperIntegrity');
const HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnum_hd11SharedLocations =
    const HostDoctorCheckCheckEnum._('hd11SharedLocations');
const HostDoctorCheckCheckEnum
_$hostDoctorCheckCheckEnum_unknownDefaultOpenApi =
    const HostDoctorCheckCheckEnum._('unknownDefaultOpenApi');

HostDoctorCheckCheckEnum _$hostDoctorCheckCheckEnumValueOf(String name) {
  switch (name) {
    case 'hd1User':
      return _$hostDoctorCheckCheckEnum_hd1User;
    case 'hd2Groups':
      return _$hostDoctorCheckCheckEnum_hd2Groups;
    case 'hd3Sudo':
      return _$hostDoctorCheckCheckEnum_hd3Sudo;
    case 'hd4StateDir':
      return _$hostDoctorCheckCheckEnum_hd4StateDir;
    case 'hd5ServerHost':
      return _$hostDoctorCheckCheckEnum_hd5ServerHost;
    case 'hd6LlmLogin':
      return _$hostDoctorCheckCheckEnum_hd6LlmLogin;
    case 'hd7Profiles':
      return _$hostDoctorCheckCheckEnum_hd7Profiles;
    case 'hd8Separation':
      return _$hostDoctorCheckCheckEnum_hd8Separation;
    case 'hd9ForeignHomes':
      return _$hostDoctorCheckCheckEnum_hd9ForeignHomes;
    case 'hd10HelperIntegrity':
      return _$hostDoctorCheckCheckEnum_hd10HelperIntegrity;
    case 'hd11SharedLocations':
      return _$hostDoctorCheckCheckEnum_hd11SharedLocations;
    case 'unknownDefaultOpenApi':
      return _$hostDoctorCheckCheckEnum_unknownDefaultOpenApi;
    default:
      return _$hostDoctorCheckCheckEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostDoctorCheckCheckEnum> _$hostDoctorCheckCheckEnumValues =
    BuiltSet<HostDoctorCheckCheckEnum>(const <HostDoctorCheckCheckEnum>[
      _$hostDoctorCheckCheckEnum_hd1User,
      _$hostDoctorCheckCheckEnum_hd2Groups,
      _$hostDoctorCheckCheckEnum_hd3Sudo,
      _$hostDoctorCheckCheckEnum_hd4StateDir,
      _$hostDoctorCheckCheckEnum_hd5ServerHost,
      _$hostDoctorCheckCheckEnum_hd6LlmLogin,
      _$hostDoctorCheckCheckEnum_hd7Profiles,
      _$hostDoctorCheckCheckEnum_hd8Separation,
      _$hostDoctorCheckCheckEnum_hd9ForeignHomes,
      _$hostDoctorCheckCheckEnum_hd10HelperIntegrity,
      _$hostDoctorCheckCheckEnum_hd11SharedLocations,
      _$hostDoctorCheckCheckEnum_unknownDefaultOpenApi,
    ]);

Serializer<HostDoctorCheckCheckEnum> _$hostDoctorCheckCheckEnumSerializer =
    _$HostDoctorCheckCheckEnumSerializer();

class _$HostDoctorCheckCheckEnumSerializer
    implements PrimitiveSerializer<HostDoctorCheckCheckEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'hd1User': 'hd1_user',
    'hd2Groups': 'hd2_groups',
    'hd3Sudo': 'hd3_sudo',
    'hd4StateDir': 'hd4_state_dir',
    'hd5ServerHost': 'hd5_server_host',
    'hd6LlmLogin': 'hd6_llm_login',
    'hd7Profiles': 'hd7_profiles',
    'hd8Separation': 'hd8_separation',
    'hd9ForeignHomes': 'hd9_foreign_homes',
    'hd10HelperIntegrity': 'hd10_helper_integrity',
    'hd11SharedLocations': 'hd11_shared_locations',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'hd1_user': 'hd1User',
    'hd2_groups': 'hd2Groups',
    'hd3_sudo': 'hd3Sudo',
    'hd4_state_dir': 'hd4StateDir',
    'hd5_server_host': 'hd5ServerHost',
    'hd6_llm_login': 'hd6LlmLogin',
    'hd7_profiles': 'hd7Profiles',
    'hd8_separation': 'hd8Separation',
    'hd9_foreign_homes': 'hd9ForeignHomes',
    'hd10_helper_integrity': 'hd10HelperIntegrity',
    'hd11_shared_locations': 'hd11SharedLocations',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostDoctorCheckCheckEnum];
  @override
  final String wireName = 'HostDoctorCheckCheckEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostDoctorCheckCheckEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostDoctorCheckCheckEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostDoctorCheckCheckEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$HostDoctorCheck extends HostDoctorCheck {
  @override
  final HostDoctorCheckCheckEnum check;
  @override
  final bool ok;
  @override
  final String? code;
  @override
  final String? sessionUser;

  factory _$HostDoctorCheck([void Function(HostDoctorCheckBuilder)? updates]) =>
      (HostDoctorCheckBuilder()..update(updates))._build();

  _$HostDoctorCheck._({
    required this.check,
    required this.ok,
    this.code,
    this.sessionUser,
  }) : super._();
  @override
  HostDoctorCheck rebuild(void Function(HostDoctorCheckBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostDoctorCheckBuilder toBuilder() => HostDoctorCheckBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostDoctorCheck &&
        check == other.check &&
        ok == other.ok &&
        code == other.code &&
        sessionUser == other.sessionUser;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, check.hashCode);
    _$hash = $jc(_$hash, ok.hashCode);
    _$hash = $jc(_$hash, code.hashCode);
    _$hash = $jc(_$hash, sessionUser.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostDoctorCheck')
          ..add('check', check)
          ..add('ok', ok)
          ..add('code', code)
          ..add('sessionUser', sessionUser))
        .toString();
  }
}

class HostDoctorCheckBuilder
    implements Builder<HostDoctorCheck, HostDoctorCheckBuilder> {
  _$HostDoctorCheck? _$v;

  HostDoctorCheckCheckEnum? _check;
  HostDoctorCheckCheckEnum? get check => _$this._check;
  set check(HostDoctorCheckCheckEnum? check) => _$this._check = check;

  bool? _ok;
  bool? get ok => _$this._ok;
  set ok(bool? ok) => _$this._ok = ok;

  String? _code;
  String? get code => _$this._code;
  set code(String? code) => _$this._code = code;

  String? _sessionUser;
  String? get sessionUser => _$this._sessionUser;
  set sessionUser(String? sessionUser) => _$this._sessionUser = sessionUser;

  HostDoctorCheckBuilder() {
    HostDoctorCheck._defaults(this);
  }

  HostDoctorCheckBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _check = $v.check;
      _ok = $v.ok;
      _code = $v.code;
      _sessionUser = $v.sessionUser;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostDoctorCheck other) {
    _$v = other as _$HostDoctorCheck;
  }

  @override
  void update(void Function(HostDoctorCheckBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostDoctorCheck build() => _build();

  _$HostDoctorCheck _build() {
    final _$result =
        _$v ??
        _$HostDoctorCheck._(
          check: BuiltValueNullFieldError.checkNotNull(
            check,
            r'HostDoctorCheck',
            'check',
          ),
          ok: BuiltValueNullFieldError.checkNotNull(
            ok,
            r'HostDoctorCheck',
            'ok',
          ),
          code: code,
          sessionUser: sessionUser,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
