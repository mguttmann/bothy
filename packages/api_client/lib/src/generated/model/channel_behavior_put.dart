//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_behavior_put.g.dart';

/// ChannelBehaviorPut
///
/// Properties:
/// * [requireMention]
/// * [threadRequireMention]
/// * [freeResponse]
/// * [autoThread]
/// * [allowMentionedAgents]
/// * [historyBackfillLimit]
/// * [prompt]
/// * [skills]
@BuiltValue()
abstract class ChannelBehaviorPut
    implements Built<ChannelBehaviorPut, ChannelBehaviorPutBuilder> {
  @BuiltValueField(wireName: r'require_mention')
  bool get requireMention;

  @BuiltValueField(wireName: r'thread_require_mention')
  bool get threadRequireMention;

  @BuiltValueField(wireName: r'free_response')
  bool get freeResponse;

  @BuiltValueField(wireName: r'auto_thread')
  bool get autoThread;

  @BuiltValueField(wireName: r'allow_mentioned_agents')
  bool get allowMentionedAgents;

  @BuiltValueField(wireName: r'history_backfill_limit')
  int get historyBackfillLimit;

  @BuiltValueField(wireName: r'prompt')
  String get prompt;

  @BuiltValueField(wireName: r'skills')
  BuiltList<String> get skills;

  ChannelBehaviorPut._();

  factory ChannelBehaviorPut([void updates(ChannelBehaviorPutBuilder b)]) =
      _$ChannelBehaviorPut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelBehaviorPutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ChannelBehaviorPut> get serializer =>
      _$ChannelBehaviorPutSerializer();
}

class _$ChannelBehaviorPutSerializer
    implements PrimitiveSerializer<ChannelBehaviorPut> {
  @override
  final Iterable<Type> types = const [ChannelBehaviorPut, _$ChannelBehaviorPut];

  @override
  final String wireName = r'ChannelBehaviorPut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ChannelBehaviorPut object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'require_mention';
    yield serializers.serialize(
      object.requireMention,
      specifiedType: const FullType(bool),
    );
    yield r'thread_require_mention';
    yield serializers.serialize(
      object.threadRequireMention,
      specifiedType: const FullType(bool),
    );
    yield r'free_response';
    yield serializers.serialize(
      object.freeResponse,
      specifiedType: const FullType(bool),
    );
    yield r'auto_thread';
    yield serializers.serialize(
      object.autoThread,
      specifiedType: const FullType(bool),
    );
    yield r'allow_mentioned_agents';
    yield serializers.serialize(
      object.allowMentionedAgents,
      specifiedType: const FullType(bool),
    );
    yield r'history_backfill_limit';
    yield serializers.serialize(
      object.historyBackfillLimit,
      specifiedType: const FullType(int),
    );
    yield r'prompt';
    yield serializers.serialize(
      object.prompt,
      specifiedType: const FullType(String),
    );
    yield r'skills';
    yield serializers.serialize(
      object.skills,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ChannelBehaviorPut object, {
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
    required ChannelBehaviorPutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'require_mention':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.requireMention = valueDes;
          break;
        case r'thread_require_mention':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.threadRequireMention = valueDes;
          break;
        case r'free_response':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.freeResponse = valueDes;
          break;
        case r'auto_thread':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.autoThread = valueDes;
          break;
        case r'allow_mentioned_agents':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.allowMentionedAgents = valueDes;
          break;
        case r'history_backfill_limit':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.historyBackfillLimit = valueDes;
          break;
        case r'prompt':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.prompt = valueDes;
          break;
        case r'skills':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.skills.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ChannelBehaviorPut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelBehaviorPutBuilder();
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
