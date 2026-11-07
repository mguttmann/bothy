//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/agent_wake_facts.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_wake_input_payload.g.dart';

/// Payload of `agent.wake_input` (04 15.9); the envelope carries a fresh `invocation_token` with `trigger_kind = wake`, `run_id`, `thread_id`, `wake_id` and `sub` per delivery.
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [wakeId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [wake]
@BuiltValue()
abstract class AgentWakeInputPayload
    implements Built<AgentWakeInputPayload, AgentWakeInputPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'wake_id')
  String get wakeId;

  @BuiltValueField(wireName: r'wake')
  AgentWakeFacts get wake;

  AgentWakeInputPayload._();

  factory AgentWakeInputPayload([
    void updates(AgentWakeInputPayloadBuilder b),
  ]) = _$AgentWakeInputPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentWakeInputPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentWakeInputPayload> get serializer =>
      _$AgentWakeInputPayloadSerializer();
}

class _$AgentWakeInputPayloadSerializer
    implements PrimitiveSerializer<AgentWakeInputPayload> {
  @override
  final Iterable<Type> types = const [
    AgentWakeInputPayload,
    _$AgentWakeInputPayload,
  ];

  @override
  final String wireName = r'AgentWakeInputPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentWakeInputPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'wake_id';
    yield serializers.serialize(
      object.wakeId,
      specifiedType: const FullType(String),
    );
    yield r'wake';
    yield serializers.serialize(
      object.wake,
      specifiedType: const FullType(AgentWakeFacts),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentWakeInputPayload object, {
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
    required AgentWakeInputPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.runId = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'wake_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.wakeId = valueDes;
          break;
        case r'wake':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentWakeFacts),
          ) as AgentWakeFacts;
          result.wake.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentWakeInputPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentWakeInputPayloadBuilder();
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
