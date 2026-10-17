//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/tool_class.dart';
import 'package:api_client/src/generated/model/project_readiness.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:api_client/src/generated/model/billing_evidence.dart';
import 'package:api_client/src/generated/model/agent_rate_limit_state.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'heartbeat.g.dart';

/// Heartbeat
///
/// Properties:
/// * [state]
/// * [runtimeVersion]
/// * [reportedBillingMode] - Absent when the adapter follows the configured mode per run; then `billing_modes_supported` names both modes and the per-run check binds (ruling L8).
/// * [billingModesSupported]
/// * [billingEvidence]
/// * [reportedToolClass]
/// * [rateLimit]
/// * [details] - Further fields of schema `agent_heartbeat.v1` (capabilities, active runs). No secrets, no content.
/// * [projectReadiness]
@BuiltValue()
abstract class Heartbeat implements Built<Heartbeat, HeartbeatBuilder> {
  @BuiltValueField(wireName: r'state')
  HeartbeatStateEnum get state;
  // enum stateEnum {  online,  degraded,  };

  @BuiltValueField(wireName: r'runtime_version')
  String? get runtimeVersion;

  /// Absent when the adapter follows the configured mode per run; then `billing_modes_supported` names both modes and the per-run check binds (ruling L8).
  @BuiltValueField(wireName: r'reported_billing_mode')
  BillingMode? get reportedBillingMode;
  // enum reportedBillingModeEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'billing_modes_supported')
  BuiltList<BillingMode>? get billingModesSupported;

  @BuiltValueField(wireName: r'billing_evidence')
  BillingEvidence? get billingEvidence;

  @BuiltValueField(wireName: r'reported_tool_class')
  ToolClass? get reportedToolClass;
  // enum reportedToolClassEnum {  chat,  scoped,  host,  };

  @BuiltValueField(wireName: r'rate_limit')
  AgentRateLimitState? get rateLimit;

  /// Further fields of schema `agent_heartbeat.v1` (capabilities, active runs). No secrets, no content.
  @BuiltValueField(wireName: r'details')
  BuiltMap<String, JsonObject?>? get details;

  @BuiltValueField(wireName: r'project_readiness')
  BuiltList<ProjectReadiness>? get projectReadiness;

  Heartbeat._();

  factory Heartbeat([void updates(HeartbeatBuilder b)]) = _$Heartbeat;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HeartbeatBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Heartbeat> get serializer => _$HeartbeatSerializer();
}

class _$HeartbeatSerializer implements PrimitiveSerializer<Heartbeat> {
  @override
  final Iterable<Type> types = const [Heartbeat, _$Heartbeat];

  @override
  final String wireName = r'Heartbeat';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Heartbeat object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(HeartbeatStateEnum),
    );
    if (object.runtimeVersion != null) {
      yield r'runtime_version';
      yield serializers.serialize(
        object.runtimeVersion,
        specifiedType: const FullType(String),
      );
    }
    if (object.reportedBillingMode != null) {
      yield r'reported_billing_mode';
      yield serializers.serialize(
        object.reportedBillingMode,
        specifiedType: const FullType(BillingMode),
      );
    }
    if (object.billingModesSupported != null) {
      yield r'billing_modes_supported';
      yield serializers.serialize(
        object.billingModesSupported,
        specifiedType: const FullType(BuiltList, [FullType(BillingMode)]),
      );
    }
    if (object.billingEvidence != null) {
      yield r'billing_evidence';
      yield serializers.serialize(
        object.billingEvidence,
        specifiedType: const FullType(BillingEvidence),
      );
    }
    if (object.reportedToolClass != null) {
      yield r'reported_tool_class';
      yield serializers.serialize(
        object.reportedToolClass,
        specifiedType: const FullType(ToolClass),
      );
    }
    if (object.rateLimit != null) {
      yield r'rate_limit';
      yield serializers.serialize(
        object.rateLimit,
        specifiedType: const FullType(AgentRateLimitState),
      );
    }
    if (object.details != null) {
      yield r'details';
      yield serializers.serialize(
        object.details,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
    if (object.projectReadiness != null) {
      yield r'project_readiness';
      yield serializers.serialize(
        object.projectReadiness,
        specifiedType: const FullType(BuiltList, [FullType(ProjectReadiness)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    Heartbeat object, {
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
    required HeartbeatBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HeartbeatStateEnum),
          ) as HeartbeatStateEnum;
          result.state = valueDes;
          break;
        case r'runtime_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.runtimeVersion = valueDes;
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
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(BillingMode),
            ]),
          ) as BuiltList<BillingMode>?;
          if (valueDes == null) continue;
          result.billingModesSupported.replace(valueDes);
          break;
        case r'billing_evidence':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BillingEvidence),
          ) as BillingEvidence?;
          if (valueDes == null) continue;
          result.billingEvidence.replace(valueDes);
          break;
        case r'reported_tool_class':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ToolClass),
          ) as ToolClass?;
          if (valueDes == null) continue;
          result.reportedToolClass = valueDes;
          break;
        case r'rate_limit':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AgentRateLimitState),
          ) as AgentRateLimitState?;
          if (valueDes == null) continue;
          result.rateLimit.replace(valueDes);
          break;
        case r'details':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.details.replace(valueDes);
          break;
        case r'project_readiness':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(ProjectReadiness),
            ]),
          ) as BuiltList<ProjectReadiness>?;
          if (valueDes == null) continue;
          result.projectReadiness.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Heartbeat deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HeartbeatBuilder();
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

class HeartbeatStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'online')
  static const HeartbeatStateEnum online = _$heartbeatStateEnum_online;
  @BuiltValueEnumConst(wireName: r'degraded')
  static const HeartbeatStateEnum degraded = _$heartbeatStateEnum_degraded;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HeartbeatStateEnum unknownDefaultOpenApi =
      _$heartbeatStateEnum_unknownDefaultOpenApi;

  static Serializer<HeartbeatStateEnum> get serializer =>
      _$heartbeatStateEnumSerializer;

  const HeartbeatStateEnum._(String name) : super(name);

  static BuiltSet<HeartbeatStateEnum> get values => _$heartbeatStateEnumValues;
  static HeartbeatStateEnum valueOf(String name) =>
      _$heartbeatStateEnumValueOf(name);
}
