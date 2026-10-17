// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'device_registration.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const DeviceRegistrationAlgEnum _$deviceRegistrationAlgEnum_ES256 =
    const DeviceRegistrationAlgEnum._('ES256');
const DeviceRegistrationAlgEnum _$deviceRegistrationAlgEnum_edDSA =
    const DeviceRegistrationAlgEnum._('edDSA');
const DeviceRegistrationAlgEnum
_$deviceRegistrationAlgEnum_unknownDefaultOpenApi =
    const DeviceRegistrationAlgEnum._('unknownDefaultOpenApi');

DeviceRegistrationAlgEnum _$deviceRegistrationAlgEnumValueOf(String name) {
  switch (name) {
    case 'ES256':
      return _$deviceRegistrationAlgEnum_ES256;
    case 'edDSA':
      return _$deviceRegistrationAlgEnum_edDSA;
    case 'unknownDefaultOpenApi':
      return _$deviceRegistrationAlgEnum_unknownDefaultOpenApi;
    default:
      return _$deviceRegistrationAlgEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<DeviceRegistrationAlgEnum> _$deviceRegistrationAlgEnumValues =
    BuiltSet<DeviceRegistrationAlgEnum>(const <DeviceRegistrationAlgEnum>[
      _$deviceRegistrationAlgEnum_ES256,
      _$deviceRegistrationAlgEnum_edDSA,
      _$deviceRegistrationAlgEnum_unknownDefaultOpenApi,
    ]);

Serializer<DeviceRegistrationAlgEnum> _$deviceRegistrationAlgEnumSerializer =
    _$DeviceRegistrationAlgEnumSerializer();

class _$DeviceRegistrationAlgEnumSerializer
    implements PrimitiveSerializer<DeviceRegistrationAlgEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'ES256': 'ES256',
    'edDSA': 'EdDSA',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'ES256': 'ES256',
    'EdDSA': 'edDSA',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[DeviceRegistrationAlgEnum];
  @override
  final String wireName = 'DeviceRegistrationAlgEnum';

  @override
  Object serialize(
    Serializers serializers,
    DeviceRegistrationAlgEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  DeviceRegistrationAlgEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => DeviceRegistrationAlgEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$DeviceRegistration extends DeviceRegistration {
  @override
  final String deviceId;
  @override
  final String name;
  @override
  final DevicePlatform platform;
  @override
  final String? appVersion;
  @override
  final BuiltMap<String, JsonObject?> publicJwk;
  @override
  final DeviceRegistrationAlgEnum alg;
  @override
  final BuiltMap<String, JsonObject?>? attestation;
  @override
  final DeviceKeyRegistration? stepUpKey;

  factory _$DeviceRegistration([
    void Function(DeviceRegistrationBuilder)? updates,
  ]) => (DeviceRegistrationBuilder()..update(updates))._build();

  _$DeviceRegistration._({
    required this.deviceId,
    required this.name,
    required this.platform,
    this.appVersion,
    required this.publicJwk,
    required this.alg,
    this.attestation,
    this.stepUpKey,
  }) : super._();
  @override
  DeviceRegistration rebuild(
    void Function(DeviceRegistrationBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  DeviceRegistrationBuilder toBuilder() =>
      DeviceRegistrationBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DeviceRegistration &&
        deviceId == other.deviceId &&
        name == other.name &&
        platform == other.platform &&
        appVersion == other.appVersion &&
        publicJwk == other.publicJwk &&
        alg == other.alg &&
        attestation == other.attestation &&
        stepUpKey == other.stepUpKey;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, deviceId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, platform.hashCode);
    _$hash = $jc(_$hash, appVersion.hashCode);
    _$hash = $jc(_$hash, publicJwk.hashCode);
    _$hash = $jc(_$hash, alg.hashCode);
    _$hash = $jc(_$hash, attestation.hashCode);
    _$hash = $jc(_$hash, stepUpKey.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'DeviceRegistration')
          ..add('deviceId', deviceId)
          ..add('name', name)
          ..add('platform', platform)
          ..add('appVersion', appVersion)
          ..add('publicJwk', publicJwk)
          ..add('alg', alg)
          ..add('attestation', attestation)
          ..add('stepUpKey', stepUpKey))
        .toString();
  }
}

class DeviceRegistrationBuilder
    implements Builder<DeviceRegistration, DeviceRegistrationBuilder> {
  _$DeviceRegistration? _$v;

  String? _deviceId;
  String? get deviceId => _$this._deviceId;
  set deviceId(String? deviceId) => _$this._deviceId = deviceId;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  DevicePlatform? _platform;
  DevicePlatform? get platform => _$this._platform;
  set platform(DevicePlatform? platform) => _$this._platform = platform;

  String? _appVersion;
  String? get appVersion => _$this._appVersion;
  set appVersion(String? appVersion) => _$this._appVersion = appVersion;

  MapBuilder<String, JsonObject?>? _publicJwk;
  MapBuilder<String, JsonObject?> get publicJwk =>
      _$this._publicJwk ??= MapBuilder<String, JsonObject?>();
  set publicJwk(MapBuilder<String, JsonObject?>? publicJwk) =>
      _$this._publicJwk = publicJwk;

  DeviceRegistrationAlgEnum? _alg;
  DeviceRegistrationAlgEnum? get alg => _$this._alg;
  set alg(DeviceRegistrationAlgEnum? alg) => _$this._alg = alg;

  MapBuilder<String, JsonObject?>? _attestation;
  MapBuilder<String, JsonObject?> get attestation =>
      _$this._attestation ??= MapBuilder<String, JsonObject?>();
  set attestation(MapBuilder<String, JsonObject?>? attestation) =>
      _$this._attestation = attestation;

  DeviceKeyRegistrationBuilder? _stepUpKey;
  DeviceKeyRegistrationBuilder get stepUpKey =>
      _$this._stepUpKey ??= DeviceKeyRegistrationBuilder();
  set stepUpKey(DeviceKeyRegistrationBuilder? stepUpKey) =>
      _$this._stepUpKey = stepUpKey;

  DeviceRegistrationBuilder() {
    DeviceRegistration._defaults(this);
  }

  DeviceRegistrationBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _deviceId = $v.deviceId;
      _name = $v.name;
      _platform = $v.platform;
      _appVersion = $v.appVersion;
      _publicJwk = $v.publicJwk.toBuilder();
      _alg = $v.alg;
      _attestation = $v.attestation?.toBuilder();
      _stepUpKey = $v.stepUpKey?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DeviceRegistration other) {
    _$v = other as _$DeviceRegistration;
  }

  @override
  void update(void Function(DeviceRegistrationBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DeviceRegistration build() => _build();

  _$DeviceRegistration _build() {
    _$DeviceRegistration _$result;
    try {
      _$result =
          _$v ??
          _$DeviceRegistration._(
            deviceId: BuiltValueNullFieldError.checkNotNull(
              deviceId,
              r'DeviceRegistration',
              'deviceId',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'DeviceRegistration',
              'name',
            ),
            platform: BuiltValueNullFieldError.checkNotNull(
              platform,
              r'DeviceRegistration',
              'platform',
            ),
            appVersion: appVersion,
            publicJwk: publicJwk.build(),
            alg: BuiltValueNullFieldError.checkNotNull(
              alg,
              r'DeviceRegistration',
              'alg',
            ),
            attestation: _attestation?.build(),
            stepUpKey: _stepUpKey?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'publicJwk';
        publicJwk.build();

        _$failedField = 'attestation';
        _attestation?.build();
        _$failedField = 'stepUpKey';
        _stepUpKey?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'DeviceRegistration',
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
