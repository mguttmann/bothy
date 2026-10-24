//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/money.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_session.g.dart';

/// AgentSession
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [externalSessionRef]
/// * [isCurrent]
/// * [started]
/// * [model]
/// * [cost]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [endedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [endReason]
/// * [costBaselineMicros] - The cumulative amount of the external session before it was bound to this row (`agent_sessions.cost_baseline_micros`, 22 9 step 7), in micro units of `currency`; present only when above 0. An adapter whose runtime reports cumulative session values books per run only the part above it (09, 18.7; RK-75).
/// * [currency] - ISO 4217 code of `cost_baseline_micros` (`agent_sessions.currency`); present exactly when `cost_baseline_micros` is.
/// * [version]
@BuiltValue()
abstract class AgentSession
    implements Built<AgentSession, AgentSessionBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  @BuiltValueField(wireName: r'external_session_ref')
  String? get externalSessionRef;

  @BuiltValueField(wireName: r'is_current')
  bool get isCurrent;

  @BuiltValueField(wireName: r'started')
  bool get started;

  @BuiltValueField(wireName: r'model')
  String? get model;

  @BuiltValueField(wireName: r'cost')
  Money get cost;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'ended_at')
  DateTime? get endedAt;

  @BuiltValueField(wireName: r'end_reason')
  AgentSessionEndReasonEnum? get endReason;
  // enum endReasonEnum {  reset,  rebind,  import,  agent_changed,  };

  /// The cumulative amount of the external session before it was bound to this row (`agent_sessions.cost_baseline_micros`, 22 9 step 7), in micro units of `currency`; present only when above 0. An adapter whose runtime reports cumulative session values books per run only the part above it (09, 18.7; RK-75).
  @BuiltValueField(wireName: r'cost_baseline_micros')
  int? get costBaselineMicros;

  /// ISO 4217 code of `cost_baseline_micros` (`agent_sessions.currency`); present exactly when `cost_baseline_micros` is.
  @BuiltValueField(wireName: r'currency')
  String? get currency;

  @BuiltValueField(wireName: r'version')
  int get version;

  AgentSession._();

  factory AgentSession([void updates(AgentSessionBuilder b)]) = _$AgentSession;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentSessionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentSession> get serializer => _$AgentSessionSerializer();
}

class _$AgentSessionSerializer implements PrimitiveSerializer<AgentSession> {
  @override
  final Iterable<Type> types = const [AgentSession, _$AgentSession];

  @override
  final String wireName = r'AgentSession';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentSession object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    if (object.externalSessionRef != null) {
      yield r'external_session_ref';
      yield serializers.serialize(
        object.externalSessionRef,
        specifiedType: const FullType(String),
      );
    }
    yield r'is_current';
    yield serializers.serialize(
      object.isCurrent,
      specifiedType: const FullType(bool),
    );
    yield r'started';
    yield serializers.serialize(
      object.started,
      specifiedType: const FullType(bool),
    );
    if (object.model != null) {
      yield r'model';
      yield serializers.serialize(
        object.model,
        specifiedType: const FullType(String),
      );
    }
    yield r'cost';
    yield serializers.serialize(
      object.cost,
      specifiedType: const FullType(Money),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.endedAt != null) {
      yield r'ended_at';
      yield serializers.serialize(
        object.endedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.endReason != null) {
      yield r'end_reason';
      yield serializers.serialize(
        object.endReason,
        specifiedType: const FullType(AgentSessionEndReasonEnum),
      );
    }
    if (object.costBaselineMicros != null) {
      yield r'cost_baseline_micros';
      yield serializers.serialize(
        object.costBaselineMicros,
        specifiedType: const FullType(int),
      );
    }
    if (object.currency != null) {
      yield r'currency';
      yield serializers.serialize(
        object.currency,
        specifiedType: const FullType(String),
      );
    }
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentSession object, {
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
    required AgentSessionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.agentId = valueDes;
          break;
        case r'external_session_ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.externalSessionRef = valueDes;
          break;
        case r'is_current':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.isCurrent = valueDes;
          break;
        case r'started':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.started = valueDes;
          break;
        case r'model':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.model = valueDes;
          break;
        case r'cost':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Money),
          ) as Money;
          result.cost.replace(valueDes);
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'ended_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.endedAt = valueDes;
          break;
        case r'end_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AgentSessionEndReasonEnum),
          ) as AgentSessionEndReasonEnum?;
          if (valueDes == null) continue;
          result.endReason = valueDes;
          break;
        case r'cost_baseline_micros':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.costBaselineMicros = valueDes;
          break;
        case r'currency':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.currency = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentSession deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentSessionBuilder();
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

class AgentSessionEndReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'reset')
  static const AgentSessionEndReasonEnum reset =
      _$agentSessionEndReasonEnum_reset;
  @BuiltValueEnumConst(wireName: r'rebind')
  static const AgentSessionEndReasonEnum rebind =
      _$agentSessionEndReasonEnum_rebind;
  @BuiltValueEnumConst(wireName: r'import')
  static const AgentSessionEndReasonEnum import_ =
      _$agentSessionEndReasonEnum_import_;
  @BuiltValueEnumConst(wireName: r'agent_changed')
  static const AgentSessionEndReasonEnum agentChanged =
      _$agentSessionEndReasonEnum_agentChanged;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentSessionEndReasonEnum unknownDefaultOpenApi =
      _$agentSessionEndReasonEnum_unknownDefaultOpenApi;

  static Serializer<AgentSessionEndReasonEnum> get serializer =>
      _$agentSessionEndReasonEnumSerializer;

  const AgentSessionEndReasonEnum._(String name) : super(name);

  static BuiltSet<AgentSessionEndReasonEnum> get values =>
      _$agentSessionEndReasonEnumValues;
  static AgentSessionEndReasonEnum valueOf(String name) =>
      _$agentSessionEndReasonEnumValueOf(name);
}
