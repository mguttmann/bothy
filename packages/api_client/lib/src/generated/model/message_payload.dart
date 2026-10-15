//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/message.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'message_payload.g.dart';

/// MessagePayload
///
/// Properties:
/// * [message]
@BuiltValue()
abstract class MessagePayload
    implements Built<MessagePayload, MessagePayloadBuilder> {
  @BuiltValueField(wireName: r'message')
  Message get message;

  MessagePayload._();

  factory MessagePayload([void updates(MessagePayloadBuilder b)]) =
      _$MessagePayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(MessagePayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<MessagePayload> get serializer =>
      _$MessagePayloadSerializer();
}

class _$MessagePayloadSerializer
    implements PrimitiveSerializer<MessagePayload> {
  @override
  final Iterable<Type> types = const [MessagePayload, _$MessagePayload];

  @override
  final String wireName = r'MessagePayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    MessagePayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'message';
    yield serializers.serialize(
      object.message,
      specifiedType: const FullType(Message),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    MessagePayload object, {
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
    required MessagePayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'message':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Message),
          ) as Message;
          result.message.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  MessagePayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = MessagePayloadBuilder();
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
