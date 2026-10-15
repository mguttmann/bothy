//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:api_client/src/generated/model/agent_runtime_state.dart';
import 'package:api_client/src/generated/model/agent_rate_limit_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_live_agent.g.dart';

/// StatsLiveAgent
///
/// Properties:
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [presence]
/// * [billingMode]
/// * [reportedBillingMode] - Absent when the adapter follows the configured mode per run.
/// * [rateLimit]
@BuiltValue()
abstract class StatsLiveAgent
    implements Built<StatsLiveAgent, StatsLiveAgentBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  @BuiltValueField(wireName: r'presence')
  AgentRuntimeState get presence;
  // enum presenceEnum {  unknown,  online,  degraded,  offline,  };

  @BuiltValueField(wireName: r'billing_mode')
  BillingMode get billingMode;
  // enum billingModeEnum {  personal,  organization,  };

  /// Absent when the adapter follows the configured mode per run.
  @BuiltValueField(wireName: r'reported_billing_mode')
  BillingMode? get reportedBillingMode;
  // enum reportedBillingModeEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'rate_limit')
  AgentRateLimitState? get rateLimit;

  StatsLiveAgent._();

  factory StatsLiveAgent([void updates(StatsLiveAgentBuilder b)]) =
      _$StatsLiveAgent;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsLiveAgentBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsLiveAgent> get serializer =>
      _$StatsLiveAgentSerializer();
}

class _$StatsLiveAgentSerializer
    implements PrimitiveSerializer<StatsLiveAgent> {
  @override
  final Iterable<Type> types = const [StatsLiveAgent, _$StatsLiveAgent];

  @override
  final String wireName = r'StatsLiveAgent';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsLiveAgent object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'presence';
    yield serializers.serialize(
      object.presence,
      specifiedType: const FullType(AgentRuntimeState),
    );
    yield r'billing_mode';
    yield serializers.serialize(
      object.billingMode,
      specifiedType: const FullType(BillingMode),
    );
    if (object.reportedBillingMode != null) {
      yield r'reported_billing_mode';
      yield serializers.serialize(
        object.reportedBillingMode,
        specifiedType: const FullType(BillingMode),
      );
    }
    if (object.rateLimit != null) {
      yield r'rate_limit';
      yield serializers.serialize(
        object.rateLimit,
        specifiedType: const FullType(AgentRateLimitState),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsLiveAgent object, {
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
    required StatsLiveAgentBuilder result,
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
        case r'presence':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentRuntimeState),
          ) as AgentRuntimeState;
          result.presence = valueDes;
          break;
        case r'billing_mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BillingMode),
          ) as BillingMode;
          result.billingMode = valueDes;
          break;
        case r'reported_billing_mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BillingMode),
          ) as BillingMode?;
          if (valueDes == null) continue;
          result.reportedBillingMode = valueDes;
          break;
        case r'rate_limit':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AgentRateLimitState),
          ) as AgentRateLimitState?;
          if (valueDes == null) continue;
          result.rateLimit.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsLiveAgent deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsLiveAgentBuilder();
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
