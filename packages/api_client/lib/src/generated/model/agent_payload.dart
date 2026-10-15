//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/agent.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_payload.g.dart';

/// AgentPayload
///
/// Properties:
/// * [agent]
@BuiltValue()
abstract class AgentPayload
    implements Built<AgentPayload, AgentPayloadBuilder> {
  @BuiltValueField(wireName: r'agent')
  Agent get agent;

  AgentPayload._();

  factory AgentPayload([void updates(AgentPayloadBuilder b)]) = _$AgentPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentPayload> get serializer => _$AgentPayloadSerializer();
}

class _$AgentPayloadSerializer implements PrimitiveSerializer<AgentPayload> {
  @override
  final Iterable<Type> types = const [AgentPayload, _$AgentPayload];

  @override
  final String wireName = r'AgentPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'agent';
    yield serializers.serialize(
      object.agent,
      specifiedType: const FullType(Agent),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentPayload object, {
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
    required AgentPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'agent':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Agent),
          ) as Agent;
          result.agent.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentPayloadBuilder();
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
