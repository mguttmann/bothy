//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_channel_config.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_channel_config_list.g.dart';

/// AgentChannelConfigList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class AgentChannelConfigList
    implements Built<AgentChannelConfigList, AgentChannelConfigListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<AgentChannelConfig> get items;

  AgentChannelConfigList._();

  factory AgentChannelConfigList([
    void updates(AgentChannelConfigListBuilder b),
  ]) = _$AgentChannelConfigList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentChannelConfigListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentChannelConfigList> get serializer =>
      _$AgentChannelConfigListSerializer();
}

class _$AgentChannelConfigListSerializer
    implements PrimitiveSerializer<AgentChannelConfigList> {
  @override
  final Iterable<Type> types = const [
    AgentChannelConfigList,
    _$AgentChannelConfigList,
  ];

  @override
  final String wireName = r'AgentChannelConfigList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentChannelConfigList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(AgentChannelConfig)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentChannelConfigList object, {
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
    required AgentChannelConfigListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(AgentChannelConfig),
            ]),
          ) as BuiltList<AgentChannelConfig>;
          result.items.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentChannelConfigList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentChannelConfigListBuilder();
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
