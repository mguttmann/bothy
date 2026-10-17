//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/agent_session.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_session_changed_payload.g.dart';

/// AgentSessionChangedPayload
///
/// Properties:
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentSession]
@BuiltValue()
abstract class AgentSessionChangedPayload
    implements
        Built<AgentSessionChangedPayload, AgentSessionChangedPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  @BuiltValueField(wireName: r'agent_session')
  AgentSession get agentSession;

  AgentSessionChangedPayload._();

  factory AgentSessionChangedPayload([
    void updates(AgentSessionChangedPayloadBuilder b),
  ]) = _$AgentSessionChangedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentSessionChangedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentSessionChangedPayload> get serializer =>
      _$AgentSessionChangedPayloadSerializer();
}

class _$AgentSessionChangedPayloadSerializer
    implements PrimitiveSerializer<AgentSessionChangedPayload> {
  @override
  final Iterable<Type> types = const [
    AgentSessionChangedPayload,
    _$AgentSessionChangedPayload,
  ];

  @override
  final String wireName = r'AgentSessionChangedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentSessionChangedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'agent_session';
    yield serializers.serialize(
      object.agentSession,
      specifiedType: const FullType(AgentSession),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentSessionChangedPayload object, {
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
    required AgentSessionChangedPayloadBuilder result,
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
        case r'agent_session':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentSession),
          ) as AgentSession;
          result.agentSession.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentSessionChangedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentSessionChangedPayloadBuilder();
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
