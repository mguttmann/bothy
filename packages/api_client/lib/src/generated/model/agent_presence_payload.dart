//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:api_client/src/generated/model/agent_runtime_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_presence_payload.g.dart';

/// AgentPresencePayload
///
/// Properties:
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
/// * [reportedBillingMode]
@BuiltValue()
abstract class AgentPresencePayload
    implements Built<AgentPresencePayload, AgentPresencePayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  @BuiltValueField(wireName: r'state')
  AgentRuntimeState get state;
  // enum stateEnum {  unknown,  online,  degraded,  offline,  };

  @BuiltValueField(wireName: r'reported_billing_mode')
  BillingMode? get reportedBillingMode;
  // enum reportedBillingModeEnum {  personal,  organization,  };

  AgentPresencePayload._();

  factory AgentPresencePayload([void updates(AgentPresencePayloadBuilder b)]) =
      _$AgentPresencePayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentPresencePayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentPresencePayload> get serializer =>
      _$AgentPresencePayloadSerializer();
}

class _$AgentPresencePayloadSerializer
    implements PrimitiveSerializer<AgentPresencePayload> {
  @override
  final Iterable<Type> types = const [
    AgentPresencePayload,
    _$AgentPresencePayload,
  ];

  @override
  final String wireName = r'AgentPresencePayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentPresencePayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(AgentRuntimeState),
    );
    if (object.reportedBillingMode != null) {
      yield r'reported_billing_mode';
      yield serializers.serialize(
        object.reportedBillingMode,
        specifiedType: const FullType(BillingMode),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentPresencePayload object, {
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
    required AgentPresencePayloadBuilder result,
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
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentRuntimeState),
          ) as AgentRuntimeState;
          result.state = valueDes;
          break;
        case r'reported_billing_mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BillingMode),
          ) as BillingMode?;
          if (valueDes == null) continue;
          result.reportedBillingMode = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentPresencePayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentPresencePayloadBuilder();
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
