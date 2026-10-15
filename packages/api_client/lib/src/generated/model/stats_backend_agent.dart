//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/stats_backend_denials.dart';
import 'package:api_client/src/generated/model/stats_backend_cost.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:api_client/src/generated/model/agent_rate_limit_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_backend_agent.g.dart';

/// StatsBackendAgent
///
/// Properties:
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [billingMode]
/// * [reportedBillingMode]
/// * [billingModesSupported]
/// * [switches] - Billing mode switches in the range.
/// * [invocationsDenied]
/// * [costs] - Only with `cost.read.aggregate`.
/// * [rateLimit]
@BuiltValue()
abstract class StatsBackendAgent
    implements Built<StatsBackendAgent, StatsBackendAgentBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  @BuiltValueField(wireName: r'billing_mode')
  BillingMode get billingMode;
  // enum billingModeEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'reported_billing_mode')
  BillingMode? get reportedBillingMode;
  // enum reportedBillingModeEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'billing_modes_supported')
  BuiltList<BillingMode> get billingModesSupported;

  /// Billing mode switches in the range.
  @BuiltValueField(wireName: r'switches')
  int get switches;

  @BuiltValueField(wireName: r'invocations_denied')
  StatsBackendDenials get invocationsDenied;

  /// Only with `cost.read.aggregate`.
  @BuiltValueField(wireName: r'costs')
  BuiltList<StatsBackendCost>? get costs;

  @BuiltValueField(wireName: r'rate_limit')
  AgentRateLimitState? get rateLimit;

  StatsBackendAgent._();

  factory StatsBackendAgent([void updates(StatsBackendAgentBuilder b)]) =
      _$StatsBackendAgent;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsBackendAgentBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsBackendAgent> get serializer =>
      _$StatsBackendAgentSerializer();
}

class _$StatsBackendAgentSerializer
    implements PrimitiveSerializer<StatsBackendAgent> {
  @override
  final Iterable<Type> types = const [StatsBackendAgent, _$StatsBackendAgent];

  @override
  final String wireName = r'StatsBackendAgent';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsBackendAgent object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
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
    yield r'billing_modes_supported';
    yield serializers.serialize(
      object.billingModesSupported,
      specifiedType: const FullType(BuiltList, [FullType(BillingMode)]),
    );
    yield r'switches';
    yield serializers.serialize(
      object.switches,
      specifiedType: const FullType(int),
    );
    yield r'invocations_denied';
    yield serializers.serialize(
      object.invocationsDenied,
      specifiedType: const FullType(StatsBackendDenials),
    );
    if (object.costs != null) {
      yield r'costs';
      yield serializers.serialize(
        object.costs,
        specifiedType: const FullType(BuiltList, [FullType(StatsBackendCost)]),
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
    StatsBackendAgent object, {
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
    required StatsBackendAgentBuilder result,
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
        case r'billing_modes_supported':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(BillingMode)]),
          ) as BuiltList<BillingMode>;
          result.billingModesSupported.replace(valueDes);
          break;
        case r'switches':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.switches = valueDes;
          break;
        case r'invocations_denied':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsBackendDenials),
          ) as StatsBackendDenials;
          result.invocationsDenied.replace(valueDes);
          break;
        case r'costs':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(StatsBackendCost),
            ]),
          ) as BuiltList<StatsBackendCost>?;
          if (valueDes == null) continue;
          result.costs.replace(valueDes);
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
  StatsBackendAgent deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsBackendAgentBuilder();
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
