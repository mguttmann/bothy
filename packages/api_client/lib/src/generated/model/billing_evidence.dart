//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'billing_evidence.g.dart';

/// How the runtime proves its actual billing mode. Never a key value or prefix (schema `billing_evidence.v1`).
///
/// Properties:
/// * [provider]
/// * [keyType]
/// * [method]
@BuiltValue()
abstract class BillingEvidence
    implements Built<BillingEvidence, BillingEvidenceBuilder> {
  @BuiltValueField(wireName: r'provider')
  String? get provider;

  @BuiltValueField(wireName: r'key_type')
  BillingEvidenceKeyTypeEnum get keyType;
  // enum keyTypeEnum {  api_key,  oauth,  cloud,  none,  unknown,  };

  @BuiltValueField(wireName: r'method')
  String? get method;

  BillingEvidence._();

  factory BillingEvidence([void updates(BillingEvidenceBuilder b)]) =
      _$BillingEvidence;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(BillingEvidenceBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<BillingEvidence> get serializer =>
      _$BillingEvidenceSerializer();
}

class _$BillingEvidenceSerializer
    implements PrimitiveSerializer<BillingEvidence> {
  @override
  final Iterable<Type> types = const [BillingEvidence, _$BillingEvidence];

  @override
  final String wireName = r'BillingEvidence';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    BillingEvidence object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.provider != null) {
      yield r'provider';
      yield serializers.serialize(
        object.provider,
        specifiedType: const FullType(String),
      );
    }
    yield r'key_type';
    yield serializers.serialize(
      object.keyType,
      specifiedType: const FullType(BillingEvidenceKeyTypeEnum),
    );
    if (object.method != null) {
      yield r'method';
      yield serializers.serialize(
        object.method,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    BillingEvidence object, {
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
    required BillingEvidenceBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'provider':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.provider = valueDes;
          break;
        case r'key_type':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BillingEvidenceKeyTypeEnum),
          ) as BillingEvidenceKeyTypeEnum;
          result.keyType = valueDes;
          break;
        case r'method':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.method = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  BillingEvidence deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = BillingEvidenceBuilder();
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

class BillingEvidenceKeyTypeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'api_key')
  static const BillingEvidenceKeyTypeEnum apiKey =
      _$billingEvidenceKeyTypeEnum_apiKey;
  @BuiltValueEnumConst(wireName: r'oauth')
  static const BillingEvidenceKeyTypeEnum oauth =
      _$billingEvidenceKeyTypeEnum_oauth;
  @BuiltValueEnumConst(wireName: r'cloud')
  static const BillingEvidenceKeyTypeEnum cloud =
      _$billingEvidenceKeyTypeEnum_cloud;
  @BuiltValueEnumConst(wireName: r'none')
  static const BillingEvidenceKeyTypeEnum none =
      _$billingEvidenceKeyTypeEnum_none;
  @BuiltValueEnumConst(wireName: r'unknown')
  static const BillingEvidenceKeyTypeEnum unknown =
      _$billingEvidenceKeyTypeEnum_unknown;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const BillingEvidenceKeyTypeEnum unknownDefaultOpenApi =
      _$billingEvidenceKeyTypeEnum_unknownDefaultOpenApi;

  static Serializer<BillingEvidenceKeyTypeEnum> get serializer =>
      _$billingEvidenceKeyTypeEnumSerializer;

  const BillingEvidenceKeyTypeEnum._(String name) : super(name);

  static BuiltSet<BillingEvidenceKeyTypeEnum> get values =>
      _$billingEvidenceKeyTypeEnumValues;
  static BillingEvidenceKeyTypeEnum valueOf(String name) =>
      _$billingEvidenceKeyTypeEnumValueOf(name);
}
