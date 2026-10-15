// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'device_key_registration.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const DeviceKeyRegistrationAlgEnum _$deviceKeyRegistrationAlgEnum_ES256 =
    const DeviceKeyRegistrationAlgEnum._('ES256');
const DeviceKeyRegistrationAlgEnum _$deviceKeyRegistrationAlgEnum_edDSA =
    const DeviceKeyRegistrationAlgEnum._('edDSA');
const DeviceKeyRegistrationAlgEnum
_$deviceKeyRegistrationAlgEnum_unknownDefaultOpenApi =
    const DeviceKeyRegistrationAlgEnum._('unknownDefaultOpenApi');

DeviceKeyRegistrationAlgEnum _$deviceKeyRegistrationAlgEnumValueOf(
  String name,
) {
  switch (name) {
    case 'ES256':
      return _$deviceKeyRegistrationAlgEnum_ES256;
    case 'edDSA':
      return _$deviceKeyRegistrationAlgEnum_edDSA;
    case 'unknownDefaultOpenApi':
      return _$deviceKeyRegistrationAlgEnum_unknownDefaultOpenApi;
    default:
      return _$deviceKeyRegistrationAlgEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<DeviceKeyRegistrationAlgEnum>
_$deviceKeyRegistrationAlgEnumValues = BuiltSet<DeviceKeyRegistrationAlgEnum>(
  const <DeviceKeyRegistrationAlgEnum>[
    _$deviceKeyRegistrationAlgEnum_ES256,
    _$deviceKeyRegistrationAlgEnum_edDSA,
    _$deviceKeyRegistrationAlgEnum_unknownDefaultOpenApi,
  ],
);

Serializer<DeviceKeyRegistrationAlgEnum>
_$deviceKeyRegistrationAlgEnumSerializer =
    _$DeviceKeyRegistrationAlgEnumSerializer();

class _$DeviceKeyRegistrationAlgEnumSerializer
    implements PrimitiveSerializer<DeviceKeyRegistrationAlgEnum> {
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
  final Iterable<Type> types = const <Type>[DeviceKeyRegistrationAlgEnum];
  @override
  final String wireName = 'DeviceKeyRegistrationAlgEnum';

  @override
  Object serialize(
    Serializers serializers,
    DeviceKeyRegistrationAlgEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  DeviceKeyRegistrationAlgEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => DeviceKeyRegistrationAlgEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$DeviceKeyRegistration extends DeviceKeyRegistration {
  @override
  final BuiltMap<String, JsonObject?> publicJwk;
  @override
  final DeviceKeyRegistrationAlgEnum alg;
  @override
  final BuiltMap<String, JsonObject?>? attestation;

  factory _$DeviceKeyRegistration([
    void Function(DeviceKeyRegistrationBuilder)? updates,
  ]) => (DeviceKeyRegistrationBuilder()..update(updates))._build();

  _$DeviceKeyRegistration._({
    required this.publicJwk,
    required this.alg,
    this.attestation,
  }) : super._();
  @override
  DeviceKeyRegistration rebuild(
    void Function(DeviceKeyRegistrationBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  DeviceKeyRegistrationBuilder toBuilder() =>
      DeviceKeyRegistrationBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DeviceKeyRegistration &&
        publicJwk == other.publicJwk &&
        alg == other.alg &&
        attestation == other.attestation;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, publicJwk.hashCode);
    _$hash = $jc(_$hash, alg.hashCode);
    _$hash = $jc(_$hash, attestation.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'DeviceKeyRegistration')
          ..add('publicJwk', publicJwk)
          ..add('alg', alg)
          ..add('attestation', attestation))
        .toString();
  }
}

class DeviceKeyRegistrationBuilder
    implements Builder<DeviceKeyRegistration, DeviceKeyRegistrationBuilder> {
  _$DeviceKeyRegistration? _$v;

  MapBuilder<String, JsonObject?>? _publicJwk;
  MapBuilder<String, JsonObject?> get publicJwk =>
      _$this._publicJwk ??= MapBuilder<String, JsonObject?>();
  set publicJwk(MapBuilder<String, JsonObject?>? publicJwk) =>
      _$this._publicJwk = publicJwk;

  DeviceKeyRegistrationAlgEnum? _alg;
  DeviceKeyRegistrationAlgEnum? get alg => _$this._alg;
  set alg(DeviceKeyRegistrationAlgEnum? alg) => _$this._alg = alg;

  MapBuilder<String, JsonObject?>? _attestation;
  MapBuilder<String, JsonObject?> get attestation =>
      _$this._attestation ??= MapBuilder<String, JsonObject?>();
  set attestation(MapBuilder<String, JsonObject?>? attestation) =>
      _$this._attestation = attestation;

  DeviceKeyRegistrationBuilder() {
    DeviceKeyRegistration._defaults(this);
  }

  DeviceKeyRegistrationBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _publicJwk = $v.publicJwk.toBuilder();
      _alg = $v.alg;
      _attestation = $v.attestation?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DeviceKeyRegistration other) {
    _$v = other as _$DeviceKeyRegistration;
  }

  @override
  void update(void Function(DeviceKeyRegistrationBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DeviceKeyRegistration build() => _build();

  _$DeviceKeyRegistration _build() {
    _$DeviceKeyRegistration _$result;
    try {
      _$result =
          _$v ??
          _$DeviceKeyRegistration._(
            publicJwk: publicJwk.build(),
            alg: BuiltValueNullFieldError.checkNotNull(
              alg,
              r'DeviceKeyRegistration',
              'alg',
            ),
            attestation: _attestation?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'publicJwk';
        publicJwk.build();

        _$failedField = 'attestation';
        _attestation?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'DeviceKeyRegistration',
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
