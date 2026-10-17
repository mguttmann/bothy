//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'device_key_registration.g.dart';

/// DeviceKeyRegistration
///
/// Properties:
/// * [publicJwk] - Public JWK (RFC 7517) without private members.
/// * [alg]
/// * [attestation] - Free-form JSON object whose schema is named in the field description.
@BuiltValue()
abstract class DeviceKeyRegistration
    implements Built<DeviceKeyRegistration, DeviceKeyRegistrationBuilder> {
  /// Public JWK (RFC 7517) without private members.
  @BuiltValueField(wireName: r'public_jwk')
  BuiltMap<String, JsonObject?> get publicJwk;

  @BuiltValueField(wireName: r'alg')
  DeviceKeyRegistrationAlgEnum get alg;
  // enum algEnum {  ES256,  EdDSA,  };

  /// Free-form JSON object whose schema is named in the field description.
  @BuiltValueField(wireName: r'attestation')
  BuiltMap<String, JsonObject?>? get attestation;

  DeviceKeyRegistration._();

  factory DeviceKeyRegistration([
    void updates(DeviceKeyRegistrationBuilder b),
  ]) = _$DeviceKeyRegistration;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DeviceKeyRegistrationBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<DeviceKeyRegistration> get serializer =>
      _$DeviceKeyRegistrationSerializer();
}

class _$DeviceKeyRegistrationSerializer
    implements PrimitiveSerializer<DeviceKeyRegistration> {
  @override
  final Iterable<Type> types = const [
    DeviceKeyRegistration,
    _$DeviceKeyRegistration,
  ];

  @override
  final String wireName = r'DeviceKeyRegistration';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    DeviceKeyRegistration object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
      specifiedType: const FullType(DeviceKeyRegistrationAlgEnum),
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
  }

  @override
  Object serialize(
    Serializers serializers,
    DeviceKeyRegistration object, {
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
    required DeviceKeyRegistrationBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
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
            specifiedType: const FullType(DeviceKeyRegistrationAlgEnum),
          ) as DeviceKeyRegistrationAlgEnum;
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
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  DeviceKeyRegistration deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DeviceKeyRegistrationBuilder();
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

class DeviceKeyRegistrationAlgEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'ES256')
  static const DeviceKeyRegistrationAlgEnum ES256 =
      _$deviceKeyRegistrationAlgEnum_ES256;
  @BuiltValueEnumConst(wireName: r'EdDSA')
  static const DeviceKeyRegistrationAlgEnum edDSA =
      _$deviceKeyRegistrationAlgEnum_edDSA;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const DeviceKeyRegistrationAlgEnum unknownDefaultOpenApi =
      _$deviceKeyRegistrationAlgEnum_unknownDefaultOpenApi;

  static Serializer<DeviceKeyRegistrationAlgEnum> get serializer =>
      _$deviceKeyRegistrationAlgEnumSerializer;

  const DeviceKeyRegistrationAlgEnum._(String name) : super(name);

  static BuiltSet<DeviceKeyRegistrationAlgEnum> get values =>
      _$deviceKeyRegistrationAlgEnumValues;
  static DeviceKeyRegistrationAlgEnum valueOf(String name) =>
      _$deviceKeyRegistrationAlgEnumValueOf(name);
}
