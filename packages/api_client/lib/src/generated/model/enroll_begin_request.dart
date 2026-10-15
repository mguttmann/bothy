//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'enroll_begin_request.g.dart';

/// EnrollBeginRequest
///
/// Properties:
/// * [enrollmentCode]
@BuiltValue()
abstract class EnrollBeginRequest
    implements Built<EnrollBeginRequest, EnrollBeginRequestBuilder> {
  @BuiltValueField(wireName: r'enrollment_code')
  String get enrollmentCode;

  EnrollBeginRequest._();

  factory EnrollBeginRequest([void updates(EnrollBeginRequestBuilder b)]) =
      _$EnrollBeginRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(EnrollBeginRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<EnrollBeginRequest> get serializer =>
      _$EnrollBeginRequestSerializer();
}

class _$EnrollBeginRequestSerializer
    implements PrimitiveSerializer<EnrollBeginRequest> {
  @override
  final Iterable<Type> types = const [EnrollBeginRequest, _$EnrollBeginRequest];

  @override
  final String wireName = r'EnrollBeginRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    EnrollBeginRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'enrollment_code';
    yield serializers.serialize(
      object.enrollmentCode,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    EnrollBeginRequest object, {
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
    required EnrollBeginRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'enrollment_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.enrollmentCode = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  EnrollBeginRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = EnrollBeginRequestBuilder();
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
