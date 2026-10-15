//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'audit_verify_request.g.dart';

/// AuditVerifyRequest
///
/// Properties:
/// * [fromSeq]
/// * [toSeq]
@BuiltValue()
abstract class AuditVerifyRequest
    implements Built<AuditVerifyRequest, AuditVerifyRequestBuilder> {
  @BuiltValueField(wireName: r'from_seq')
  int? get fromSeq;

  @BuiltValueField(wireName: r'to_seq')
  int? get toSeq;

  AuditVerifyRequest._();

  factory AuditVerifyRequest([void updates(AuditVerifyRequestBuilder b)]) =
      _$AuditVerifyRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AuditVerifyRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AuditVerifyRequest> get serializer =>
      _$AuditVerifyRequestSerializer();
}

class _$AuditVerifyRequestSerializer
    implements PrimitiveSerializer<AuditVerifyRequest> {
  @override
  final Iterable<Type> types = const [AuditVerifyRequest, _$AuditVerifyRequest];

  @override
  final String wireName = r'AuditVerifyRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AuditVerifyRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.fromSeq != null) {
      yield r'from_seq';
      yield serializers.serialize(
        object.fromSeq,
        specifiedType: const FullType(int),
      );
    }
    if (object.toSeq != null) {
      yield r'to_seq';
      yield serializers.serialize(
        object.toSeq,
        specifiedType: const FullType(int),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AuditVerifyRequest object, {
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
    required AuditVerifyRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'from_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.fromSeq = valueDes;
          break;
        case r'to_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.toSeq = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AuditVerifyRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AuditVerifyRequestBuilder();
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
