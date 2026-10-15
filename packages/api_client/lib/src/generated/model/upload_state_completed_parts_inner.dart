//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'upload_state_completed_parts_inner.g.dart';

/// UploadStateCompletedPartsInner
///
/// Properties:
/// * [partNumber]
/// * [etag]
@BuiltValue()
abstract class UploadStateCompletedPartsInner
    implements
        Built<
          UploadStateCompletedPartsInner,
          UploadStateCompletedPartsInnerBuilder
        > {
  @BuiltValueField(wireName: r'part_number')
  int get partNumber;

  @BuiltValueField(wireName: r'etag')
  String get etag;

  UploadStateCompletedPartsInner._();

  factory UploadStateCompletedPartsInner([
    void updates(UploadStateCompletedPartsInnerBuilder b),
  ]) = _$UploadStateCompletedPartsInner;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(UploadStateCompletedPartsInnerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<UploadStateCompletedPartsInner> get serializer =>
      _$UploadStateCompletedPartsInnerSerializer();
}

class _$UploadStateCompletedPartsInnerSerializer
    implements PrimitiveSerializer<UploadStateCompletedPartsInner> {
  @override
  final Iterable<Type> types = const [
    UploadStateCompletedPartsInner,
    _$UploadStateCompletedPartsInner,
  ];

  @override
  final String wireName = r'UploadStateCompletedPartsInner';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    UploadStateCompletedPartsInner object, {
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
    UploadStateCompletedPartsInner object, {
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
    required UploadStateCompletedPartsInnerBuilder result,
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
  UploadStateCompletedPartsInner deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = UploadStateCompletedPartsInnerBuilder();
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
