//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/message_revision.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'message_revision_list.g.dart';

/// MessageRevisionList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class MessageRevisionList
    implements Built<MessageRevisionList, MessageRevisionListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<MessageRevision> get items;

  MessageRevisionList._();

  factory MessageRevisionList([void updates(MessageRevisionListBuilder b)]) =
      _$MessageRevisionList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(MessageRevisionListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<MessageRevisionList> get serializer =>
      _$MessageRevisionListSerializer();
}

class _$MessageRevisionListSerializer
    implements PrimitiveSerializer<MessageRevisionList> {
  @override
  final Iterable<Type> types = const [
    MessageRevisionList,
    _$MessageRevisionList,
  ];

  @override
  final String wireName = r'MessageRevisionList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    MessageRevisionList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(MessageRevision)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    MessageRevisionList object, {
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
    required MessageRevisionListBuilder result,
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
              FullType(MessageRevision),
            ]),
          ) as BuiltList<MessageRevision>;
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
  MessageRevisionList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = MessageRevisionListBuilder();
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
