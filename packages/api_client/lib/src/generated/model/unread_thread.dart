//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'unread_thread.g.dart';

/// UnreadThread
///
/// Properties:
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [unreadCount]
/// * [mentionCount]
/// * [lastMessageSeq]
/// * [lastReadSeq]
@BuiltValue()
abstract class UnreadThread
    implements Built<UnreadThread, UnreadThreadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  @BuiltValueField(wireName: r'unread_count')
  int get unreadCount;

  @BuiltValueField(wireName: r'mention_count')
  int get mentionCount;

  @BuiltValueField(wireName: r'last_message_seq')
  int get lastMessageSeq;

  @BuiltValueField(wireName: r'last_read_seq')
  int get lastReadSeq;

  UnreadThread._();

  factory UnreadThread([void updates(UnreadThreadBuilder b)]) = _$UnreadThread;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(UnreadThreadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<UnreadThread> get serializer => _$UnreadThreadSerializer();
}

class _$UnreadThreadSerializer implements PrimitiveSerializer<UnreadThread> {
  @override
  final Iterable<Type> types = const [UnreadThread, _$UnreadThread];

  @override
  final String wireName = r'UnreadThread';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    UnreadThread object, {
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
    yield r'unread_count';
    yield serializers.serialize(
      object.unreadCount,
      specifiedType: const FullType(int),
    );
    yield r'mention_count';
    yield serializers.serialize(
      object.mentionCount,
      specifiedType: const FullType(int),
    );
    yield r'last_message_seq';
    yield serializers.serialize(
      object.lastMessageSeq,
      specifiedType: const FullType(int),
    );
    yield r'last_read_seq';
    yield serializers.serialize(
      object.lastReadSeq,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    UnreadThread object, {
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
    required UnreadThreadBuilder result,
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
        case r'unread_count':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.unreadCount = valueDes;
          break;
        case r'mention_count':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.mentionCount = valueDes;
          break;
        case r'last_message_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.lastMessageSeq = valueDes;
          break;
        case r'last_read_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.lastReadSeq = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  UnreadThread deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = UnreadThreadBuilder();
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
