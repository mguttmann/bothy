//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thread_tags_put.g.dart';

/// ThreadTagsPut
///
/// Properties:
/// * [tagIds] - Complete set, at most `thread.max_tags`, without duplicates.
@BuiltValue()
abstract class ThreadTagsPut
    implements Built<ThreadTagsPut, ThreadTagsPutBuilder> {
  /// Complete set, at most `thread.max_tags`, without duplicates.
  @BuiltValueField(wireName: r'tag_ids')
  BuiltList<String> get tagIds;

  ThreadTagsPut._();

  factory ThreadTagsPut([void updates(ThreadTagsPutBuilder b)]) =
      _$ThreadTagsPut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ThreadTagsPutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ThreadTagsPut> get serializer =>
      _$ThreadTagsPutSerializer();
}

class _$ThreadTagsPutSerializer implements PrimitiveSerializer<ThreadTagsPut> {
  @override
  final Iterable<Type> types = const [ThreadTagsPut, _$ThreadTagsPut];

  @override
  final String wireName = r'ThreadTagsPut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ThreadTagsPut object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'tag_ids';
    yield serializers.serialize(
      object.tagIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ThreadTagsPut object, {
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
    required ThreadTagsPutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'tag_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.tagIds.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ThreadTagsPut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ThreadTagsPutBuilder();
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
