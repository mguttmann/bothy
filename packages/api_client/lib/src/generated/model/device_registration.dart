//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/device_platform.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/device_key_registration.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'device_registration.g.dart';

/// Device and device keys sent with every login completion. The DPoP proof of the request must be signed with the `dpop` key (`public_jwk`); `step_up_key` is the second key with user verification (08, 6.1), absent where no secure key store exists.
///
/// Properties:
/// * [deviceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [name]
/// * [platform]
/// * [appVersion]
/// * [publicJwk] - Public JWK (RFC 7517) of the device key, without private members.
/// * [alg]
/// * [attestation] - Platform attestation, if the platform provides one.
/// * [stepUpKey]
@BuiltValue()
abstract class DeviceRegistration
    implements Built<DeviceRegistration, DeviceRegistrationBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'device_id')
  String get deviceId;

  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'platform')
  DevicePlatform get platform;
  // enum platformEnum {  ios,  android,  macos,  windows,  web,  cli,  };

  @BuiltValueField(wireName: r'app_version')
  String? get appVersion;

  /// Public JWK (RFC 7517) of the device key, without private members.
  @BuiltValueField(wireName: r'public_jwk')
  BuiltMap<String, JsonObject?> get publicJwk;

  @BuiltValueField(wireName: r'alg')
  DeviceRegistrationAlgEnum get alg;
  // enum algEnum {  ES256,  EdDSA,  };

  /// Platform attestation, if the platform provides one.
  @BuiltValueField(wireName: r'attestation')
  BuiltMap<String, JsonObject?>? get attestation;

  @BuiltValueField(wireName: r'step_up_key')
  DeviceKeyRegistration? get stepUpKey;

  DeviceRegistration._();

  factory DeviceRegistration([void updates(DeviceRegistrationBuilder b)]) =
      _$DeviceRegistration;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DeviceRegistrationBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<DeviceRegistration> get serializer =>
      _$DeviceRegistrationSerializer();
}

class _$DeviceRegistrationSerializer
    implements PrimitiveSerializer<DeviceRegistration> {
  @override
  final Iterable<Type> types = const [DeviceRegistration, _$DeviceRegistration];

  @override
  final String wireName = r'DeviceRegistration';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    DeviceRegistration object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'device_id';
    yield serializers.serialize(
      object.deviceId,
      specifiedType: const FullType(String),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'platform';
    yield serializers.serialize(
      object.platform,
      specifiedType: const FullType(DevicePlatform),
    );
    if (object.appVersion != null) {
      yield r'app_version';
      yield serializers.serialize(
        object.appVersion,
        specifiedType: const FullType(String),
      );
    }
    yield r'public_jwk';
    yield serializers.serialize(
      object.publicJwk,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
    yield r'alg';
    yield serializers.serialize(
      object.alg,
      specifiedType: const FullType(DeviceRegistrationAlgEnum),
    );
    if (object.attestation != null) {
      yield r'attestation';
      yield serializers.serialize(
        object.attestation,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
    if (object.stepUpKey != null) {
      yield r'step_up_key';
      yield serializers.serialize(
        object.stepUpKey,
        specifiedType: const FullType(DeviceKeyRegistration),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    DeviceRegistration object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return _serializeProperties(
      serializers,
      object,
      specifiedType: specifiedType,
    ).toList();
  }

  void _deserializeProperties(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
    required List<Object?> serializedList,
    required DeviceRegistrationBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'device_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.deviceId = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'platform':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DevicePlatform),
          ) as DevicePlatform;
          result.platform = valueDes;
          break;
        case r'app_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.appVersion = valueDes;
          break;
        case r'public_jwk':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.publicJwk.replace(valueDes);
          break;
        case r'alg':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DeviceRegistrationAlgEnum),
          ) as DeviceRegistrationAlgEnum;
          result.alg = valueDes;
          break;
        case r'attestation':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.attestation.replace(valueDes);
          break;
        case r'step_up_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DeviceKeyRegistration),
          ) as DeviceKeyRegistration?;
          if (valueDes == null) continue;
          result.stepUpKey.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  DeviceRegistration deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DeviceRegistrationBuilder();
    final serializedList = (serialized as Iterable<Object?>).toList();
    final unhandled = <Object?>[];
    _deserializeProperties(
      serializers,
      serialized,
      specifiedType: specifiedType,
      serializedList: serializedList,
      unhandled: unhandled,
      result: result,
    );
    return result.build();
  }
}

class DeviceRegistrationAlgEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'ES256')
  static const DeviceRegistrationAlgEnum ES256 =
      _$deviceRegistrationAlgEnum_ES256;
  @BuiltValueEnumConst(wireName: r'EdDSA')
  static const DeviceRegistrationAlgEnum edDSA =
      _$deviceRegistrationAlgEnum_edDSA;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const DeviceRegistrationAlgEnum unknownDefaultOpenApi =
      _$deviceRegistrationAlgEnum_unknownDefaultOpenApi;

  static Serializer<DeviceRegistrationAlgEnum> get serializer =>
      _$deviceRegistrationAlgEnumSerializer;

  const DeviceRegistrationAlgEnum._(String name) : super(name);

  static BuiltSet<DeviceRegistrationAlgEnum> get values =>
      _$deviceRegistrationAlgEnumValues;
  static DeviceRegistrationAlgEnum valueOf(String name) =>
      _$deviceRegistrationAlgEnumValueOf(name);
}
