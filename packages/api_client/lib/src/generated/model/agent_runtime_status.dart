//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/tool_class.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:api_client/src/generated/model/agent_runtime_state.dart';
import 'package:api_client/src/generated/model/billing_evidence.dart';
import 'package:api_client/src/generated/model/agent_rate_limit_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_runtime_status.g.dart';

/// Reported state of the agent runtime (heartbeat), not the configured state.
///
/// Properties:
/// * [state]
/// * [reportedBillingMode]
/// * [runtimeVersion]
/// * [lastHeartbeatAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [billingModesSupported]
/// * [reportedToolClass]
/// * [billingModeMatches] - False when the reported mode differs from the configured `billing_mode`.
/// * [rateLimit]
/// * [billingEvidence]
/// * [warnings] - Known values `billing_mode_mismatch` (the reported mode has differed from the configured one for longer than `billing.mismatch_alert_after`), `billing_mode_unsupported` (no reported mode and the configured one is not in `billing_modes_supported`), `billing_evidence_unknown` (`billing_evidence.key_type` is `unknown`) and `tool_class_mismatch` (the reported tool class differs from the configured one). Only while the state is `online` or `degraded`.
@BuiltValue()
abstract class AgentRuntimeStatus
    implements Built<AgentRuntimeStatus, AgentRuntimeStatusBuilder> {
  @BuiltValueField(wireName: r'state')
  AgentRuntimeState get state;
  // enum stateEnum {  unknown,  online,  degraded,  offline,  };

  @BuiltValueField(wireName: r'reported_billing_mode')
  BillingMode? get reportedBillingMode;
  // enum reportedBillingModeEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'runtime_version')
  String? get runtimeVersion;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'last_heartbeat_at')
  DateTime? get lastHeartbeatAt;

  @BuiltValueField(wireName: r'billing_modes_supported')
  BuiltList<BillingMode>? get billingModesSupported;

  @BuiltValueField(wireName: r'reported_tool_class')
  ToolClass? get reportedToolClass;
  // enum reportedToolClassEnum {  chat,  scoped,  host,  };

  /// False when the reported mode differs from the configured `billing_mode`.
  @BuiltValueField(wireName: r'billing_mode_matches')
  bool? get billingModeMatches;

  @BuiltValueField(wireName: r'rate_limit')
  AgentRateLimitState? get rateLimit;

  @BuiltValueField(wireName: r'billing_evidence')
  BillingEvidence? get billingEvidence;

  /// Known values `billing_mode_mismatch` (the reported mode has differed from the configured one for longer than `billing.mismatch_alert_after`), `billing_mode_unsupported` (no reported mode and the configured one is not in `billing_modes_supported`), `billing_evidence_unknown` (`billing_evidence.key_type` is `unknown`) and `tool_class_mismatch` (the reported tool class differs from the configured one). Only while the state is `online` or `degraded`.
  @BuiltValueField(wireName: r'warnings')
  BuiltList<String>? get warnings;

  AgentRuntimeStatus._();

  factory AgentRuntimeStatus([void updates(AgentRuntimeStatusBuilder b)]) =
      _$AgentRuntimeStatus;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentRuntimeStatusBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentRuntimeStatus> get serializer =>
      _$AgentRuntimeStatusSerializer();
}

class _$AgentRuntimeStatusSerializer
    implements PrimitiveSerializer<AgentRuntimeStatus> {
  @override
  final Iterable<Type> types = const [AgentRuntimeStatus, _$AgentRuntimeStatus];

  @override
  final String wireName = r'AgentRuntimeStatus';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentRuntimeStatus object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
    if (object.runtimeVersion != null) {
      yield r'runtime_version';
      yield serializers.serialize(
        object.runtimeVersion,
        specifiedType: const FullType(String),
      );
    }
    if (object.lastHeartbeatAt != null) {
      yield r'last_heartbeat_at';
      yield serializers.serialize(
        object.lastHeartbeatAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.billingModesSupported != null) {
      yield r'billing_modes_supported';
      yield serializers.serialize(
        object.billingModesSupported,
        specifiedType: const FullType(BuiltList, [FullType(BillingMode)]),
      );
    }
    if (object.reportedToolClass != null) {
      yield r'reported_tool_class';
      yield serializers.serialize(
        object.reportedToolClass,
        specifiedType: const FullType(ToolClass),
      );
    }
    if (object.billingModeMatches != null) {
      yield r'billing_mode_matches';
      yield serializers.serialize(
        object.billingModeMatches,
        specifiedType: const FullType(bool),
      );
    }
    if (object.rateLimit != null) {
      yield r'rate_limit';
      yield serializers.serialize(
        object.rateLimit,
        specifiedType: const FullType(AgentRateLimitState),
      );
    }
    if (object.billingEvidence != null) {
      yield r'billing_evidence';
      yield serializers.serialize(
        object.billingEvidence,
        specifiedType: const FullType(BillingEvidence),
      );
    }
    if (object.warnings != null) {
      yield r'warnings';
      yield serializers.serialize(
        object.warnings,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentRuntimeStatus object, {
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
    required AgentRuntimeStatusBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
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
        case r'runtime_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.runtimeVersion = valueDes;
          break;
        case r'last_heartbeat_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.lastHeartbeatAt = valueDes;
          break;
        case r'billing_modes_supported':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(BillingMode),
            ]),
          ) as BuiltList<BillingMode>?;
          if (valueDes == null) continue;
          result.billingModesSupported.replace(valueDes);
          break;
        case r'reported_tool_class':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ToolClass),
          ) as ToolClass?;
          if (valueDes == null) continue;
          result.reportedToolClass = valueDes;
          break;
        case r'billing_mode_matches':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.billingModeMatches = valueDes;
          break;
        case r'rate_limit':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AgentRateLimitState),
          ) as AgentRateLimitState?;
          if (valueDes == null) continue;
          result.rateLimit.replace(valueDes);
          break;
        case r'billing_evidence':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BillingEvidence),
          ) as BillingEvidence?;
          if (valueDes == null) continue;
          result.billingEvidence.replace(valueDes);
          break;
        case r'warnings':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.warnings.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentRuntimeStatus deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentRuntimeStatusBuilder();
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
