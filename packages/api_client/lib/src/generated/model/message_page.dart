//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/message.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'message_page.g.dart';

/// Messages in ascending `thread_seq`. Tombstones are included.
///
/// Properties:
/// * [items]
/// * [hasMoreBefore]
/// * [hasMoreAfter]
@BuiltValue()
abstract class MessagePage implements Built<MessagePage, MessagePageBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<Message> get items;

  @BuiltValueField(wireName: r'has_more_before')
  bool get hasMoreBefore;

  @BuiltValueField(wireName: r'has_more_after')
  bool get hasMoreAfter;

  MessagePage._();

  factory MessagePage([void updates(MessagePageBuilder b)]) = _$MessagePage;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(MessagePageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<MessagePage> get serializer => _$MessagePageSerializer();
}

class _$MessagePageSerializer implements PrimitiveSerializer<MessagePage> {
  @override
  final Iterable<Type> types = const [MessagePage, _$MessagePage];

  @override
  final String wireName = r'MessagePage';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    MessagePage object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(Message)]),
    );
    yield r'has_more_before';
    yield serializers.serialize(
      object.hasMoreBefore,
      specifiedType: const FullType(bool),
    );
    yield r'has_more_after';
    yield serializers.serialize(
      object.hasMoreAfter,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    MessagePage object, {
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
    required MessagePageBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Message)]),
          ) as BuiltList<Message>;
          result.items.replace(valueDes);
          break;
        case r'has_more_before':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.hasMoreBefore = valueDes;
          break;
        case r'has_more_after':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.hasMoreAfter = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  MessagePage deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = MessagePageBuilder();
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
