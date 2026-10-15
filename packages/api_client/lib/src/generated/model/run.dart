//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/money.dart';
import 'package:api_client/src/generated/model/run_state.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/run_step.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:api_client/src/generated/model/run_followup.dart';
import 'package:api_client/src/generated/model/run_metrics.dart';
import 'package:api_client/src/generated/model/run_plan.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run.g.dart';

/// Run document (`run_document.v1`), the content of the live box.
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentSessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [triggerKind]
/// * [triggeringMessageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [triggeringCommandId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [triggeringInteractionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [triggeredBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
/// * [stateReason]
/// * [version]
/// * [billingModeExpected]
/// * [billingModeUsed]
/// * [model]
/// * [statusLine]
/// * [lastTextExcerpt]
/// * [metrics]
/// * [cost]
/// * [summary]
/// * [steps] - Steps keyed by step ID; order comes from `ordinal`.
/// * [followups] - Follow-ups keyed by message ID.
/// * [queuedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [startedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [finishedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [durationMs]
/// * [plan]
@BuiltValue()
abstract class Run implements Built<Run, RunBuilder> {
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
  @BuiltValueField(wireName: r'agent_session_id')
  String? get agentSessionId;

  @BuiltValueField(wireName: r'trigger_kind')
  RunTriggerKindEnum? get triggerKind;
  // enum triggerKindEnum {  message,  command,  interaction,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'triggering_message_id')
  String? get triggeringMessageId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'triggering_command_id')
  String? get triggeringCommandId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'triggering_interaction_id')
  String? get triggeringInteractionId;

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

  @BuiltValueField(wireName: r'model')
  String? get model;

  @BuiltValueField(wireName: r'status_line')
  String? get statusLine;

  @BuiltValueField(wireName: r'last_text_excerpt')
  String? get lastTextExcerpt;

  @BuiltValueField(wireName: r'metrics')
  RunMetrics get metrics;

  @BuiltValueField(wireName: r'cost')
  Money get cost;

  @BuiltValueField(wireName: r'summary')
  String? get summary;

  /// Steps keyed by step ID; order comes from `ordinal`.
  @BuiltValueField(wireName: r'steps')
  BuiltMap<String, RunStep> get steps;

  /// Follow-ups keyed by message ID.
  @BuiltValueField(wireName: r'followups')
  BuiltMap<String, RunFollowup> get followups;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'queued_at')
  DateTime get queuedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'started_at')
  DateTime? get startedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'finished_at')
  DateTime? get finishedAt;

  @BuiltValueField(wireName: r'duration_ms')
  int? get durationMs;

  @BuiltValueField(wireName: r'plan')
  RunPlan? get plan;

  Run._();

  factory Run([void updates(RunBuilder b)]) = _$Run;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Run> get serializer => _$RunSerializer();
}

class _$RunSerializer implements PrimitiveSerializer<Run> {
  @override
  final Iterable<Type> types = const [Run, _$Run];

  @override
  final String wireName = r'Run';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Run object, {
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
    if (object.agentSessionId != null) {
      yield r'agent_session_id';
      yield serializers.serialize(
        object.agentSessionId,
        specifiedType: const FullType(String),
      );
    }
    if (object.triggerKind != null) {
      yield r'trigger_kind';
      yield serializers.serialize(
        object.triggerKind,
        specifiedType: const FullType(RunTriggerKindEnum),
      );
    }
    if (object.triggeringMessageId != null) {
      yield r'triggering_message_id';
      yield serializers.serialize(
        object.triggeringMessageId,
        specifiedType: const FullType(String),
      );
    }
    if (object.triggeringCommandId != null) {
      yield r'triggering_command_id';
      yield serializers.serialize(
        object.triggeringCommandId,
        specifiedType: const FullType(String),
      );
    }
    if (object.triggeringInteractionId != null) {
      yield r'triggering_interaction_id';
      yield serializers.serialize(
        object.triggeringInteractionId,
        specifiedType: const FullType(String),
      );
    }
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
    if (object.model != null) {
      yield r'model';
      yield serializers.serialize(
        object.model,
        specifiedType: const FullType(String),
      );
    }
    if (object.statusLine != null) {
      yield r'status_line';
      yield serializers.serialize(
        object.statusLine,
        specifiedType: const FullType(String),
      );
    }
    if (object.lastTextExcerpt != null) {
      yield r'last_text_excerpt';
      yield serializers.serialize(
        object.lastTextExcerpt,
        specifiedType: const FullType(String),
      );
    }
    yield r'metrics';
    yield serializers.serialize(
      object.metrics,
      specifiedType: const FullType(RunMetrics),
    );
    yield r'cost';
    yield serializers.serialize(
      object.cost,
      specifiedType: const FullType(Money),
    );
    if (object.summary != null) {
      yield r'summary';
      yield serializers.serialize(
        object.summary,
        specifiedType: const FullType(String),
      );
    }
    yield r'steps';
    yield serializers.serialize(
      object.steps,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType(RunStep),
      ]),
    );
    yield r'followups';
    yield serializers.serialize(
      object.followups,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType(RunFollowup),
      ]),
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
    if (object.finishedAt != null) {
      yield r'finished_at';
      yield serializers.serialize(
        object.finishedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.durationMs != null) {
      yield r'duration_ms';
      yield serializers.serialize(
        object.durationMs,
        specifiedType: const FullType(int),
      );
    }
    if (object.plan != null) {
      yield r'plan';
      yield serializers.serialize(
        object.plan,
        specifiedType: const FullType(RunPlan),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    Run object, {
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
    required RunBuilder result,
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
        case r'agent_session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.agentSessionId = valueDes;
          break;
        case r'trigger_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(RunTriggerKindEnum),
          ) as RunTriggerKindEnum?;
          if (valueDes == null) continue;
          result.triggerKind = valueDes;
          break;
        case r'triggering_message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.triggeringMessageId = valueDes;
          break;
        case r'triggering_command_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.triggeringCommandId = valueDes;
          break;
        case r'triggering_interaction_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.triggeringInteractionId = valueDes;
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
        case r'model':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.model = valueDes;
          break;
        case r'status_line':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.statusLine = valueDes;
          break;
        case r'last_text_excerpt':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.lastTextExcerpt = valueDes;
          break;
        case r'metrics':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunMetrics),
          ) as RunMetrics;
          result.metrics.replace(valueDes);
          break;
        case r'cost':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Money),
          ) as Money;
          result.cost.replace(valueDes);
          break;
        case r'summary':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.summary = valueDes;
          break;
        case r'steps':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType(RunStep),
            ]),
          ) as BuiltMap<String, RunStep>;
          result.steps.replace(valueDes);
          break;
        case r'followups':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType(RunFollowup),
            ]),
          ) as BuiltMap<String, RunFollowup>;
          result.followups.replace(valueDes);
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
        case r'duration_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.durationMs = valueDes;
          break;
        case r'plan':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(RunPlan),
          ) as RunPlan?;
          if (valueDes == null) continue;
          result.plan.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Run deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunBuilder();
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

class RunTriggerKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'message')
  static const RunTriggerKindEnum message = _$runTriggerKindEnum_message;
  @BuiltValueEnumConst(wireName: r'command')
  static const RunTriggerKindEnum command = _$runTriggerKindEnum_command;
  @BuiltValueEnumConst(wireName: r'interaction')
  static const RunTriggerKindEnum interaction =
      _$runTriggerKindEnum_interaction;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunTriggerKindEnum unknownDefaultOpenApi =
      _$runTriggerKindEnum_unknownDefaultOpenApi;

  static Serializer<RunTriggerKindEnum> get serializer =>
      _$runTriggerKindEnumSerializer;

  const RunTriggerKindEnum._(String name) : super(name);

  static BuiltSet<RunTriggerKindEnum> get values => _$runTriggerKindEnumValues;
  static RunTriggerKindEnum valueOf(String name) =>
      _$runTriggerKindEnumValueOf(name);
}
