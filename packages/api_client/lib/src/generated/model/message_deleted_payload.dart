//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'message_deleted_payload.g.dart';

/// MessageDeletedPayload
///
/// Properties:
/// * [messageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadSeq]
/// * [deletedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [deletedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class MessageDeletedPayload
    implements Built<MessageDeletedPayload, MessageDeletedPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'message_id')
  String get messageId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  @BuiltValueField(wireName: r'thread_seq')
  int get threadSeq;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'deleted_at')
  DateTime get deletedAt;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'deleted_by')
  String? get deletedBy;

  MessageDeletedPayload._();

  factory MessageDeletedPayload([
    void updates(MessageDeletedPayloadBuilder b),
  ]) = _$MessageDeletedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(MessageDeletedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<MessageDeletedPayload> get serializer =>
      _$MessageDeletedPayloadSerializer();
}

class _$MessageDeletedPayloadSerializer
    implements PrimitiveSerializer<MessageDeletedPayload> {
  @override
  final Iterable<Type> types = const [
    MessageDeletedPayload,
    _$MessageDeletedPayload,
  ];

  @override
  final String wireName = r'MessageDeletedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    MessageDeletedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'message_id';
    yield serializers.serialize(
      object.messageId,
      specifiedType: const FullType(String),
    );
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'thread_seq';
    yield serializers.serialize(
      object.threadSeq,
      specifiedType: const FullType(int),
    );
    yield r'deleted_at';
    yield serializers.serialize(
      object.deletedAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.deletedBy != null) {
      yield r'deleted_by';
      yield serializers.serialize(
        object.deletedBy,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    MessageDeletedPayload object, {
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
    required MessageDeletedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.messageId = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'thread_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.threadSeq = valueDes;
          break;
        case r'deleted_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.deletedAt = valueDes;
          break;
        case r'deleted_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.deletedBy = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  MessageDeletedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = MessageDeletedPayloadBuilder();
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
