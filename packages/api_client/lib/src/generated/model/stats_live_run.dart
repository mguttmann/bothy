//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/stats_live_step.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_live_run.g.dart';

/// StatsLiveRun
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [spaceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
/// * [queuedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [startedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [billingMode] - Mode used by the run, else the configured mode.
/// * [step]
/// * [approvalPending] - An approval card of this run waits for a decision.
/// * [costMicros] - Cost so far; only with `cost.read.aggregate`.
/// * [currency]
/// * [costIsEstimate]
@BuiltValue()
abstract class StatsLiveRun
    implements Built<StatsLiveRun, StatsLiveRunBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'space_id')
  String get spaceId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  @BuiltValueField(wireName: r'state')
  StatsLiveRunStateEnum get state;
  // enum stateEnum {  queued,  running,  waiting_input,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'queued_at')
  DateTime get queuedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'started_at')
  DateTime? get startedAt;

  /// Mode used by the run, else the configured mode.
  @BuiltValueField(wireName: r'billing_mode')
  BillingMode get billingMode;
  // enum billingModeEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'step')
  StatsLiveStep? get step;

  /// An approval card of this run waits for a decision.
  @BuiltValueField(wireName: r'approval_pending')
  bool get approvalPending;

  /// Cost so far; only with `cost.read.aggregate`.
  @BuiltValueField(wireName: r'cost_micros')
  int? get costMicros;

  @BuiltValueField(wireName: r'currency')
  String? get currency;

  @BuiltValueField(wireName: r'cost_is_estimate')
  bool? get costIsEstimate;

  StatsLiveRun._();

  factory StatsLiveRun([void updates(StatsLiveRunBuilder b)]) = _$StatsLiveRun;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsLiveRunBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsLiveRun> get serializer => _$StatsLiveRunSerializer();
}

class _$StatsLiveRunSerializer implements PrimitiveSerializer<StatsLiveRun> {
  @override
  final Iterable<Type> types = const [StatsLiveRun, _$StatsLiveRun];

  @override
  final String wireName = r'StatsLiveRun';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsLiveRun object, {
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
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    yield r'space_id';
    yield serializers.serialize(
      object.spaceId,
      specifiedType: const FullType(String),
    );
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(StatsLiveRunStateEnum),
    );
    yield r'queued_at';
    yield serializers.serialize(
      object.queuedAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.startedAt != null) {
      yield r'started_at';
      yield serializers.serialize(
        object.startedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'billing_mode';
    yield serializers.serialize(
      object.billingMode,
      specifiedType: const FullType(BillingMode),
    );
    if (object.step != null) {
      yield r'step';
      yield serializers.serialize(
        object.step,
        specifiedType: const FullType(StatsLiveStep),
      );
    }
    yield r'approval_pending';
    yield serializers.serialize(
      object.approvalPending,
      specifiedType: const FullType(bool),
    );
    if (object.costMicros != null) {
      yield r'cost_micros';
      yield serializers.serialize(
        object.costMicros,
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
    if (object.costIsEstimate != null) {
      yield r'cost_is_estimate';
      yield serializers.serialize(
        object.costIsEstimate,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsLiveRun object, {
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
    required StatsLiveRunBuilder result,
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
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.spaceId = valueDes;
          break;
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
            specifiedType: const FullType(StatsLiveRunStateEnum),
          ) as StatsLiveRunStateEnum;
          result.state = valueDes;
          break;
        case r'queued_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.queuedAt = valueDes;
          break;
        case r'started_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.startedAt = valueDes;
          break;
        case r'billing_mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BillingMode),
          ) as BillingMode;
          result.billingMode = valueDes;
          break;
        case r'step':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(StatsLiveStep),
          ) as StatsLiveStep?;
          if (valueDes == null) continue;
          result.step.replace(valueDes);
          break;
        case r'approval_pending':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.approvalPending = valueDes;
          break;
        case r'cost_micros':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.costMicros = valueDes;
          break;
        case r'currency':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.currency = valueDes;
          break;
        case r'cost_is_estimate':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.costIsEstimate = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsLiveRun deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsLiveRunBuilder();
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

class StatsLiveRunStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'queued')
  static const StatsLiveRunStateEnum queued = _$statsLiveRunStateEnum_queued;
  @BuiltValueEnumConst(wireName: r'running')
  static const StatsLiveRunStateEnum running = _$statsLiveRunStateEnum_running;
  @BuiltValueEnumConst(wireName: r'waiting_input')
  static const StatsLiveRunStateEnum waitingInput =
      _$statsLiveRunStateEnum_waitingInput;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatsLiveRunStateEnum unknownDefaultOpenApi =
      _$statsLiveRunStateEnum_unknownDefaultOpenApi;

  static Serializer<StatsLiveRunStateEnum> get serializer =>
      _$statsLiveRunStateEnumSerializer;

  const StatsLiveRunStateEnum._(String name) : super(name);

  static BuiltSet<StatsLiveRunStateEnum> get values =>
      _$statsLiveRunStateEnumValues;
  static StatsLiveRunStateEnum valueOf(String name) =>
      _$statsLiveRunStateEnumValueOf(name);
}
