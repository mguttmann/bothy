//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/money.dart';
import 'package:api_client/src/generated/model/run_state.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_summary.g.dart';

/// Summary for sidebars, \"running now\" and collapsed live boxes.
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [triggeredBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
/// * [stateReason]
/// * [version]
/// * [billingModeExpected]
/// * [billingModeUsed]
/// * [statusLine]
/// * [summary]
/// * [cost]
/// * [stepCount]
/// * [durationMs]
/// * [queuedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [startedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [finishedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [stallWarnedAt] - Set by `runs.stall_check` when a `running` run made no progress for `run.stall_after` (notice \"no movement since ...\"); the next stored patch clears it (09 5.5).
@BuiltValue()
abstract class RunSummary implements Built<RunSummary, RunSummaryBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'triggered_by')
  String get triggeredBy;

  @BuiltValueField(wireName: r'state')
  RunState get state;
  // enum stateEnum {  queued,  running,  waiting_input,  completed,  failed,  cancelled,  };

  @BuiltValueField(wireName: r'state_reason')
  String? get stateReason;

  @BuiltValueField(wireName: r'version')
  int get version;

  @BuiltValueField(wireName: r'billing_mode_expected')
  BillingMode get billingModeExpected;
  // enum billingModeExpectedEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'billing_mode_used')
  BillingMode? get billingModeUsed;
  // enum billingModeUsedEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'status_line')
  String? get statusLine;

  @BuiltValueField(wireName: r'summary')
  String? get summary;

  @BuiltValueField(wireName: r'cost')
  Money get cost;

  @BuiltValueField(wireName: r'step_count')
  int get stepCount;

  @BuiltValueField(wireName: r'duration_ms')
  int? get durationMs;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'queued_at')
  DateTime get queuedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'started_at')
  DateTime? get startedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'finished_at')
  DateTime? get finishedAt;

  /// Set by `runs.stall_check` when a `running` run made no progress for `run.stall_after` (notice \"no movement since ...\"); the next stored patch clears it (09 5.5).
  @BuiltValueField(wireName: r'stall_warned_at')
  DateTime? get stallWarnedAt;

  RunSummary._();

  factory RunSummary([void updates(RunSummaryBuilder b)]) = _$RunSummary;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunSummaryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunSummary> get serializer => _$RunSummarySerializer();
}

class _$RunSummarySerializer implements PrimitiveSerializer<RunSummary> {
  @override
  final Iterable<Type> types = const [RunSummary, _$RunSummary];

  @override
  final String wireName = r'RunSummary';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunSummary object, {
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
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'triggered_by';
    yield serializers.serialize(
      object.triggeredBy,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(RunState),
    );
    if (object.stateReason != null) {
      yield r'state_reason';
      yield serializers.serialize(
        object.stateReason,
        specifiedType: const FullType(String),
      );
    }
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
    yield r'billing_mode_expected';
    yield serializers.serialize(
      object.billingModeExpected,
      specifiedType: const FullType(BillingMode),
    );
    if (object.billingModeUsed != null) {
      yield r'billing_mode_used';
      yield serializers.serialize(
        object.billingModeUsed,
        specifiedType: const FullType(BillingMode),
      );
    }
    if (object.statusLine != null) {
      yield r'status_line';
      yield serializers.serialize(
        object.statusLine,
        specifiedType: const FullType(String),
      );
    }
    if (object.summary != null) {
      yield r'summary';
      yield serializers.serialize(
        object.summary,
        specifiedType: const FullType(String),
      );
    }
    yield r'cost';
    yield serializers.serialize(
      object.cost,
      specifiedType: const FullType(Money),
    );
    yield r'step_count';
    yield serializers.serialize(
      object.stepCount,
      specifiedType: const FullType(int),
    );
    if (object.durationMs != null) {
      yield r'duration_ms';
      yield serializers.serialize(
        object.durationMs,
        specifiedType: const FullType(int),
      );
    }
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
    if (object.finishedAt != null) {
      yield r'finished_at';
      yield serializers.serialize(
        object.finishedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.stallWarnedAt != null) {
      yield r'stall_warned_at';
      yield serializers.serialize(
        object.stallWarnedAt,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunSummary object, {
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
    required RunSummaryBuilder result,
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
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.agentId = valueDes;
          break;
        case r'triggered_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.triggeredBy = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunState),
          ) as RunState;
          result.state = valueDes;
          break;
        case r'state_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.stateReason = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        case r'billing_mode_expected':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BillingMode),
          ) as BillingMode;
          result.billingModeExpected = valueDes;
          break;
        case r'billing_mode_used':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BillingMode),
          ) as BillingMode?;
          if (valueDes == null) continue;
          result.billingModeUsed = valueDes;
          break;
        case r'status_line':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.statusLine = valueDes;
          break;
        case r'summary':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.summary = valueDes;
          break;
        case r'cost':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Money),
          ) as Money;
          result.cost.replace(valueDes);
          break;
        case r'step_count':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.stepCount = valueDes;
          break;
        case r'duration_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.durationMs = valueDes;
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
        case r'finished_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.finishedAt = valueDes;
          break;
        case r'stall_warned_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.stallWarnedAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunSummary deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunSummaryBuilder();
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
