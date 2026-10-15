//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/forum_tag.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'forum_tag_list.g.dart';

/// ForumTagList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class ForumTagList
    implements Built<ForumTagList, ForumTagListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<ForumTag> get items;

  ForumTagList._();

  factory ForumTagList([void updates(ForumTagListBuilder b)]) = _$ForumTagList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ForumTagListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ForumTagList> get serializer => _$ForumTagListSerializer();
}

class _$ForumTagListSerializer implements PrimitiveSerializer<ForumTagList> {
  @override
  final Iterable<Type> types = const [ForumTagList, _$ForumTagList];

  @override
  final String wireName = r'ForumTagList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ForumTagList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(ForumTag)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ForumTagList object, {
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
    required ForumTagListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(ForumTag)]),
          ) as BuiltList<ForumTag>;
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
  ForumTagList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ForumTagListBuilder();
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
