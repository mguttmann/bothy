//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_draft_channel.g.dart';

/// Where people talk to the new agent; the creation flow offers to create this channel (26 8.2).
///
/// Properties:
/// * [spaceName]
/// * [channelName]
@BuiltValue()
abstract class AgentDraftChannel
    implements Built<AgentDraftChannel, AgentDraftChannelBuilder> {
  @BuiltValueField(wireName: r'space_name')
  String get spaceName;

  @BuiltValueField(wireName: r'channel_name')
  String get channelName;

  AgentDraftChannel._();

  factory AgentDraftChannel([void updates(AgentDraftChannelBuilder b)]) =
      _$AgentDraftChannel;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDraftChannelBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDraftChannel> get serializer =>
      _$AgentDraftChannelSerializer();
}

class _$AgentDraftChannelSerializer
    implements PrimitiveSerializer<AgentDraftChannel> {
  @override
  final Iterable<Type> types = const [AgentDraftChannel, _$AgentDraftChannel];

  @override
  final String wireName = r'AgentDraftChannel';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDraftChannel object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'space_name';
    yield serializers.serialize(
      object.spaceName,
      specifiedType: const FullType(String),
    );
    yield r'channel_name';
    yield serializers.serialize(
      object.channelName,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDraftChannel object, {
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
    required AgentDraftChannelBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'space_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.spaceName = valueDes;
          break;
        case r'channel_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelName = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentDraftChannel deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDraftChannelBuilder();
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
