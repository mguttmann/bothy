//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_run_chat.g.dart';

/// The chat a run belongs to, as the sidebar names it: the name of its channel (the project) and the title of its thread. Present only when the caller may read the thread (`thread.read`, the same rule as the sidebar); absent otherwise, and then a client shows neither a name nor a link (docs/spec/25-visualization.md, section 4, rule 6).
///
/// Properties:
/// * [threadId] - Target of the deep link that opens the chat.
/// * [channelName]
/// * [threadTitle] - Title of the thread; absent for the main thread of a channel and for an empty title.
@BuiltValue()
abstract class StatsRunChat
    implements Built<StatsRunChat, StatsRunChatBuilder> {
  /// Target of the deep link that opens the chat.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  @BuiltValueField(wireName: r'channel_name')
  String get channelName;

  /// Title of the thread; absent for the main thread of a channel and for an empty title.
  @BuiltValueField(wireName: r'thread_title')
  String? get threadTitle;

  StatsRunChat._();

  factory StatsRunChat([void updates(StatsRunChatBuilder b)]) = _$StatsRunChat;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsRunChatBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsRunChat> get serializer => _$StatsRunChatSerializer();
}

class _$StatsRunChatSerializer implements PrimitiveSerializer<StatsRunChat> {
  @override
  final Iterable<Type> types = const [StatsRunChat, _$StatsRunChat];

  @override
  final String wireName = r'StatsRunChat';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsRunChat object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'channel_name';
    yield serializers.serialize(
      object.channelName,
      specifiedType: const FullType(String),
    );
    if (object.threadTitle != null) {
      yield r'thread_title';
      yield serializers.serialize(
        object.threadTitle,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsRunChat object, {
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
    required StatsRunChatBuilder result,
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
        case r'channel_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelName = valueDes;
          break;
        case r'thread_title':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.threadTitle = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsRunChat deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsRunChatBuilder();
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
