//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/thread_title_hit_list_items_inner.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thread_title_hit_list.g.dart';

/// ThreadTitleHitList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class ThreadTitleHitList
    implements Built<ThreadTitleHitList, ThreadTitleHitListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<ThreadTitleHitListItemsInner> get items;

  ThreadTitleHitList._();

  factory ThreadTitleHitList([void updates(ThreadTitleHitListBuilder b)]) =
      _$ThreadTitleHitList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ThreadTitleHitListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ThreadTitleHitList> get serializer =>
      _$ThreadTitleHitListSerializer();
}

class _$ThreadTitleHitListSerializer
    implements PrimitiveSerializer<ThreadTitleHitList> {
  @override
  final Iterable<Type> types = const [ThreadTitleHitList, _$ThreadTitleHitList];

  @override
  final String wireName = r'ThreadTitleHitList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ThreadTitleHitList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [
        FullType(ThreadTitleHitListItemsInner),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ThreadTitleHitList object, {
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
    required ThreadTitleHitListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(ThreadTitleHitListItemsInner),
            ]),
          ) as BuiltList<ThreadTitleHitListItemsInner>;
          result.items.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ThreadTitleHitList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ThreadTitleHitListBuilder();
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
