//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/upload_part_result.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'upload_complete.g.dart';

/// UploadComplete
///
/// Properties:
/// * [parts]
@BuiltValue()
abstract class UploadComplete
    implements Built<UploadComplete, UploadCompleteBuilder> {
  @BuiltValueField(wireName: r'parts')
  BuiltList<UploadPartResult> get parts;

  UploadComplete._();

  factory UploadComplete([void updates(UploadCompleteBuilder b)]) =
      _$UploadComplete;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(UploadCompleteBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<UploadComplete> get serializer =>
      _$UploadCompleteSerializer();
}

class _$UploadCompleteSerializer
    implements PrimitiveSerializer<UploadComplete> {
  @override
  final Iterable<Type> types = const [UploadComplete, _$UploadComplete];

  @override
  final String wireName = r'UploadComplete';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    UploadComplete object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'parts';
    yield serializers.serialize(
      object.parts,
      specifiedType: const FullType(BuiltList, [FullType(UploadPartResult)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    UploadComplete object, {
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
    required UploadCompleteBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'parts':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(UploadPartResult),
            ]),
          ) as BuiltList<UploadPartResult>;
          result.parts.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  UploadComplete deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = UploadCompleteBuilder();
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
