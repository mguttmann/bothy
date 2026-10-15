//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/thread.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thread_moved_payload.g.dart';

/// ThreadMovedPayload
///
/// Properties:
/// * [thread]
/// * [fromChannelId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class ThreadMovedPayload
    implements Built<ThreadMovedPayload, ThreadMovedPayloadBuilder> {
  @BuiltValueField(wireName: r'thread')
  Thread get thread;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'from_channel_id')
  String get fromChannelId;

  ThreadMovedPayload._();

  factory ThreadMovedPayload([void updates(ThreadMovedPayloadBuilder b)]) =
      _$ThreadMovedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ThreadMovedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ThreadMovedPayload> get serializer =>
      _$ThreadMovedPayloadSerializer();
}

class _$ThreadMovedPayloadSerializer
    implements PrimitiveSerializer<ThreadMovedPayload> {
  @override
  final Iterable<Type> types = const [ThreadMovedPayload, _$ThreadMovedPayload];

  @override
  final String wireName = r'ThreadMovedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ThreadMovedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'thread';
    yield serializers.serialize(
      object.thread,
      specifiedType: const FullType(Thread),
    );
    yield r'from_channel_id';
    yield serializers.serialize(
      object.fromChannelId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ThreadMovedPayload object, {
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
    required ThreadMovedPayloadBuilder result,
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
        case r'from_channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.fromChannelId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ThreadMovedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ThreadMovedPayloadBuilder();
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
