//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_earlier_session.g.dart';

/// The agent session the thread had before the migration (22, 9 step 1; `--sessions` of the tool). The server writes one earlier `agent_sessions` row (`is_current = false`, `started = true`, `ended_at` = the time of the batch, `end_reason = import`, the cost baseline) exactly once per thread, agent and reference, also for a thread posted by an earlier run, and never after the thread has had a current session; the item member `session` names the outcome. `agent_id` must be the operator of the thread (`threads.operator_agent_id`), else `400 validation_failed` at `/items/<i>/data/earlier_session/agent_id`. The reference is opaque, at most `agent.session_ref_max_bytes` bytes, and is never logged.
///
/// Properties:
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [externalSessionRef]
/// * [costBaselineMicros] - The cumulative amount of the external session before the binding, in micro units of `currency` (22, 9 step 7); absent means 0.
/// * [currency] - ISO 4217 code of `cost_baseline_micros`; required when it is above 0.
@BuiltValue()
abstract class ImportEarlierSession
    implements Built<ImportEarlierSession, ImportEarlierSessionBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  @BuiltValueField(wireName: r'external_session_ref')
  String get externalSessionRef;

  /// The cumulative amount of the external session before the binding, in micro units of `currency` (22, 9 step 7); absent means 0.
  @BuiltValueField(wireName: r'cost_baseline_micros')
  int? get costBaselineMicros;

  /// ISO 4217 code of `cost_baseline_micros`; required when it is above 0.
  @BuiltValueField(wireName: r'currency')
  String? get currency;

  ImportEarlierSession._();

  factory ImportEarlierSession([void updates(ImportEarlierSessionBuilder b)]) =
      _$ImportEarlierSession;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportEarlierSessionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportEarlierSession> get serializer =>
      _$ImportEarlierSessionSerializer();
}

class _$ImportEarlierSessionSerializer
    implements PrimitiveSerializer<ImportEarlierSession> {
  @override
  final Iterable<Type> types = const [
    ImportEarlierSession,
    _$ImportEarlierSession,
  ];

  @override
  final String wireName = r'ImportEarlierSession';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportEarlierSession object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'external_session_ref';
    yield serializers.serialize(
      object.externalSessionRef,
      specifiedType: const FullType(String),
    );
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
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportEarlierSession object, {
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
    required ImportEarlierSessionBuilder result,
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
        case r'external_session_ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.externalSessionRef = valueDes;
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
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportEarlierSession deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportEarlierSessionBuilder();
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
