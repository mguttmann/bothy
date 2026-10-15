//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'upload_part_result.g.dart';

/// UploadPartResult
///
/// Properties:
/// * [partNumber]
/// * [etag]
@BuiltValue()
abstract class UploadPartResult
    implements Built<UploadPartResult, UploadPartResultBuilder> {
  @BuiltValueField(wireName: r'part_number')
  int get partNumber;

  @BuiltValueField(wireName: r'etag')
  String get etag;

  UploadPartResult._();

  factory UploadPartResult([void updates(UploadPartResultBuilder b)]) =
      _$UploadPartResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(UploadPartResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<UploadPartResult> get serializer =>
      _$UploadPartResultSerializer();
}

class _$UploadPartResultSerializer
    implements PrimitiveSerializer<UploadPartResult> {
  @override
  final Iterable<Type> types = const [UploadPartResult, _$UploadPartResult];

  @override
  final String wireName = r'UploadPartResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    UploadPartResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'part_number';
    yield serializers.serialize(
      object.partNumber,
      specifiedType: const FullType(int),
    );
    yield r'etag';
    yield serializers.serialize(
      object.etag,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    UploadPartResult object, {
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
    required UploadPartResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'part_number':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.partNumber = valueDes;
          break;
        case r'etag':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.etag = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  UploadPartResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = UploadPartResultBuilder();
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
