//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/thread.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thread_payload.g.dart';

/// ThreadPayload
///
/// Properties:
/// * [thread]
@BuiltValue()
abstract class ThreadPayload
    implements Built<ThreadPayload, ThreadPayloadBuilder> {
  @BuiltValueField(wireName: r'thread')
  Thread get thread;

  ThreadPayload._();

  factory ThreadPayload([void updates(ThreadPayloadBuilder b)]) =
      _$ThreadPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ThreadPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ThreadPayload> get serializer =>
      _$ThreadPayloadSerializer();
}

class _$ThreadPayloadSerializer implements PrimitiveSerializer<ThreadPayload> {
  @override
  final Iterable<Type> types = const [ThreadPayload, _$ThreadPayload];

  @override
  final String wireName = r'ThreadPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ThreadPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'thread';
    yield serializers.serialize(
      object.thread,
      specifiedType: const FullType(Thread),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ThreadPayload object, {
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
    required ThreadPayloadBuilder result,
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
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ThreadPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ThreadPayloadBuilder();
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
