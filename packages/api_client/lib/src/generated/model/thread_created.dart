//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/thread.dart';
import 'package:api_client/src/generated/model/message.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thread_created.g.dart';

/// ThreadCreated
///
/// Properties:
/// * [thread]
/// * [firstMessage]
@BuiltValue()
abstract class ThreadCreated
    implements Built<ThreadCreated, ThreadCreatedBuilder> {
  @BuiltValueField(wireName: r'thread')
  Thread get thread;

  @BuiltValueField(wireName: r'first_message')
  Message? get firstMessage;

  ThreadCreated._();

  factory ThreadCreated([void updates(ThreadCreatedBuilder b)]) =
      _$ThreadCreated;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ThreadCreatedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ThreadCreated> get serializer =>
      _$ThreadCreatedSerializer();
}

class _$ThreadCreatedSerializer implements PrimitiveSerializer<ThreadCreated> {
  @override
  final Iterable<Type> types = const [ThreadCreated, _$ThreadCreated];

  @override
  final String wireName = r'ThreadCreated';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ThreadCreated object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'thread';
    yield serializers.serialize(
      object.thread,
      specifiedType: const FullType(Thread),
    );
    if (object.firstMessage != null) {
      yield r'first_message';
      yield serializers.serialize(
        object.firstMessage,
        specifiedType: const FullType(Message),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ThreadCreated object, {
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
    required ThreadCreatedBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'thread':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Thread),
          ) as Thread;
          result.thread.replace(valueDes);
          break;
        case r'first_message':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Message),
          ) as Message?;
          if (valueDes == null) continue;
          result.firstMessage.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ThreadCreated deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ThreadCreatedBuilder();
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
