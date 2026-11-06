//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_hosting_changed_payload.g.dart';

/// Payload of `agent.hosting_changed` (26 9); only the id of the built-in agent, the runner reads the rest with `listHostedAgents`.
///
/// Properties:
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class AgentHostingChangedPayload
    implements
        Built<AgentHostingChangedPayload, AgentHostingChangedPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  AgentHostingChangedPayload._();

  factory AgentHostingChangedPayload([
    void updates(AgentHostingChangedPayloadBuilder b),
  ]) = _$AgentHostingChangedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentHostingChangedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentHostingChangedPayload> get serializer =>
      _$AgentHostingChangedPayloadSerializer();
}

class _$AgentHostingChangedPayloadSerializer
    implements PrimitiveSerializer<AgentHostingChangedPayload> {
  @override
  final Iterable<Type> types = const [
    AgentHostingChangedPayload,
    _$AgentHostingChangedPayload,
  ];

  @override
  final String wireName = r'AgentHostingChangedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentHostingChangedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentHostingChangedPayload object, {
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
    required AgentHostingChangedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.agentId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentHostingChangedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentHostingChangedPayloadBuilder();
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
