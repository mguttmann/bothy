//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/grant.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'grant_changed_payload.g.dart';

/// GrantChangedPayload
///
/// Properties:
/// * [grant]
/// * [change]
@BuiltValue()
abstract class GrantChangedPayload
    implements Built<GrantChangedPayload, GrantChangedPayloadBuilder> {
  @BuiltValueField(wireName: r'grant')
  Grant get grant;

  @BuiltValueField(wireName: r'change')
  GrantChangedPayloadChangeEnum get change;
  // enum changeEnum {  created,  revoked,  expired,  };

  GrantChangedPayload._();

  factory GrantChangedPayload([void updates(GrantChangedPayloadBuilder b)]) =
      _$GrantChangedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GrantChangedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GrantChangedPayload> get serializer =>
      _$GrantChangedPayloadSerializer();
}

class _$GrantChangedPayloadSerializer
    implements PrimitiveSerializer<GrantChangedPayload> {
  @override
  final Iterable<Type> types = const [
    GrantChangedPayload,
    _$GrantChangedPayload,
  ];

  @override
  final String wireName = r'GrantChangedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GrantChangedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'grant';
    yield serializers.serialize(
      object.grant,
      specifiedType: const FullType(Grant),
    );
    yield r'change';
    yield serializers.serialize(
      object.change,
      specifiedType: const FullType(GrantChangedPayloadChangeEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GrantChangedPayload object, {
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
    required GrantChangedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'grant':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Grant),
          ) as Grant;
          result.grant.replace(valueDes);
          break;
        case r'change':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(GrantChangedPayloadChangeEnum),
          ) as GrantChangedPayloadChangeEnum;
          result.change = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GrantChangedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GrantChangedPayloadBuilder();
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

class GrantChangedPayloadChangeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'created')
  static const GrantChangedPayloadChangeEnum created =
      _$grantChangedPayloadChangeEnum_created;
  @BuiltValueEnumConst(wireName: r'revoked')
  static const GrantChangedPayloadChangeEnum revoked =
      _$grantChangedPayloadChangeEnum_revoked;
  @BuiltValueEnumConst(wireName: r'expired')
  static const GrantChangedPayloadChangeEnum expired =
      _$grantChangedPayloadChangeEnum_expired;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const GrantChangedPayloadChangeEnum unknownDefaultOpenApi =
      _$grantChangedPayloadChangeEnum_unknownDefaultOpenApi;

  static Serializer<GrantChangedPayloadChangeEnum> get serializer =>
      _$grantChangedPayloadChangeEnumSerializer;

  const GrantChangedPayloadChangeEnum._(String name) : super(name);

  static BuiltSet<GrantChangedPayloadChangeEnum> get values =>
      _$grantChangedPayloadChangeEnumValues;
  static GrantChangedPayloadChangeEnum valueOf(String name) =>
      _$grantChangedPayloadChangeEnumValueOf(name);
}
