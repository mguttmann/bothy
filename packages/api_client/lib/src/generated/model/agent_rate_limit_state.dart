//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_rate_limit_state.g.dart';

/// Rate-limit state of the agent runtime at its provider as reported by the adapter (`agent_heartbeat.v1`). Never quotas of an account, keys or subscription names.
///
/// Properties:
/// * [state]
/// * [resetsAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class AgentRateLimitState
    implements Built<AgentRateLimitState, AgentRateLimitStateBuilder> {
  @BuiltValueField(wireName: r'state')
  AgentRateLimitStateStateEnum get state;
  // enum stateEnum {  ok,  near_limit,  limited,  unknown,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'resets_at')
  DateTime? get resetsAt;

  AgentRateLimitState._();

  factory AgentRateLimitState([void updates(AgentRateLimitStateBuilder b)]) =
      _$AgentRateLimitState;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentRateLimitStateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentRateLimitState> get serializer =>
      _$AgentRateLimitStateSerializer();
}

class _$AgentRateLimitStateSerializer
    implements PrimitiveSerializer<AgentRateLimitState> {
  @override
  final Iterable<Type> types = const [
    AgentRateLimitState,
    _$AgentRateLimitState,
  ];

  @override
  final String wireName = r'AgentRateLimitState';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentRateLimitState object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(AgentRateLimitStateStateEnum),
    );
    if (object.resetsAt != null) {
      yield r'resets_at';
      yield serializers.serialize(
        object.resetsAt,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentRateLimitState object, {
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
    required AgentRateLimitStateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentRateLimitStateStateEnum),
          ) as AgentRateLimitStateStateEnum;
          result.state = valueDes;
          break;
        case r'resets_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.resetsAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentRateLimitState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentRateLimitStateBuilder();
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

class AgentRateLimitStateStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'ok')
  static const AgentRateLimitStateStateEnum ok =
      _$agentRateLimitStateStateEnum_ok;
  @BuiltValueEnumConst(wireName: r'near_limit')
  static const AgentRateLimitStateStateEnum nearLimit =
      _$agentRateLimitStateStateEnum_nearLimit;
  @BuiltValueEnumConst(wireName: r'limited')
  static const AgentRateLimitStateStateEnum limited =
      _$agentRateLimitStateStateEnum_limited;
  @BuiltValueEnumConst(wireName: r'unknown')
  static const AgentRateLimitStateStateEnum unknown =
      _$agentRateLimitStateStateEnum_unknown;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentRateLimitStateStateEnum unknownDefaultOpenApi =
      _$agentRateLimitStateStateEnum_unknownDefaultOpenApi;

  static Serializer<AgentRateLimitStateStateEnum> get serializer =>
      _$agentRateLimitStateStateEnumSerializer;

  const AgentRateLimitStateStateEnum._(String name) : super(name);

  static BuiltSet<AgentRateLimitStateStateEnum> get values =>
      _$agentRateLimitStateStateEnumValues;
  static AgentRateLimitStateStateEnum valueOf(String name) =>
      _$agentRateLimitStateStateEnumValueOf(name);
}
