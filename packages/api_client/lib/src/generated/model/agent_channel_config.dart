//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/channel_behavior.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_channel_config.g.dart';

/// Configuration an operator agent loads for a channel it serves.
///
/// Properties:
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [spaceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [behavior]
@BuiltValue()
abstract class AgentChannelConfig
    implements Built<AgentChannelConfig, AgentChannelConfigBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'space_id')
  String? get spaceId;

  @BuiltValueField(wireName: r'behavior')
  ChannelBehavior get behavior;

  AgentChannelConfig._();

  factory AgentChannelConfig([void updates(AgentChannelConfigBuilder b)]) =
      _$AgentChannelConfig;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentChannelConfigBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentChannelConfig> get serializer =>
      _$AgentChannelConfigSerializer();
}

class _$AgentChannelConfigSerializer
    implements PrimitiveSerializer<AgentChannelConfig> {
  @override
  final Iterable<Type> types = const [AgentChannelConfig, _$AgentChannelConfig];

  @override
  final String wireName = r'AgentChannelConfig';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentChannelConfig object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    if (object.spaceId != null) {
      yield r'space_id';
      yield serializers.serialize(
        object.spaceId,
        specifiedType: const FullType(String),
      );
    }
    yield r'behavior';
    yield serializers.serialize(
      object.behavior,
      specifiedType: const FullType(ChannelBehavior),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentChannelConfig object, {
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
    required AgentChannelConfigBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.spaceId = valueDes;
          break;
        case r'behavior':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ChannelBehavior),
          ) as ChannelBehavior;
          result.behavior.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentChannelConfig deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentChannelConfigBuilder();
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
