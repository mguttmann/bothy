//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thread_moved_out_payload.g.dart';

/// Sent in the source channel. Does not name the target channel.
///
/// Properties:
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class ThreadMovedOutPayload
    implements Built<ThreadMovedOutPayload, ThreadMovedOutPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  ThreadMovedOutPayload._();

  factory ThreadMovedOutPayload([
    void updates(ThreadMovedOutPayloadBuilder b),
  ]) = _$ThreadMovedOutPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ThreadMovedOutPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ThreadMovedOutPayload> get serializer =>
      _$ThreadMovedOutPayloadSerializer();
}

class _$ThreadMovedOutPayloadSerializer
    implements PrimitiveSerializer<ThreadMovedOutPayload> {
  @override
  final Iterable<Type> types = const [
    ThreadMovedOutPayload,
    _$ThreadMovedOutPayload,
  ];

  @override
  final String wireName = r'ThreadMovedOutPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ThreadMovedOutPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ThreadMovedOutPayload object, {
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
    required ThreadMovedOutPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ThreadMovedOutPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ThreadMovedOutPayloadBuilder();
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
