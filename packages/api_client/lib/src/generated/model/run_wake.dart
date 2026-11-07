//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/run_wake_condition.dart';
import 'package:api_client/src/generated/model/run_wake_reason.dart';
import 'package:api_client/src/generated/model/run_wake_state.dart';
import 'package:api_client/src/generated/model/run_wake_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake.g.dart';

/// A wake-up (03 `run_wakes`, 09 5.8.1): a server record that continues a thread later without a new message, on behalf of the person of its source run.
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [sourceRunId] - The run that asked for it, or the ended run of a recovery or usage-limit wake-up.
/// * [triggeredBy] - The person of the source run; every check uses this person.
/// * [kind]
/// * [reason]
/// * [label] - Short text of the agent, at most `run.wake_label_max_chars` characters; may be empty.
/// * [condition]
/// * [outage] - The run's outage number of a `usage_limit` wake-up of `createRunWake`.
/// * [notBefore] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [deadline] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [state]
/// * [stateReason] - Reason code of the last state change (09 5.8.3, 5.8.6), for example `autonomous_chain_max`, `deadline_passed` or `wake_input_skipped`.
/// * [resultText] - The result of a fired check, as the runner cut it.
/// * [firedRunId] - The wake run it started.
/// * [firedIntoRunId] - The active run it reached as a wake input.
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [endedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [version]
@BuiltValue()
abstract class RunWake implements Built<RunWake, RunWakeBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// The run that asked for it, or the ended run of a recovery or usage-limit wake-up.
  @BuiltValueField(wireName: r'source_run_id')
  String get sourceRunId;

  /// The person of the source run; every check uses this person.
  @BuiltValueField(wireName: r'triggered_by')
  String get triggeredBy;

  @BuiltValueField(wireName: r'kind')
  RunWakeKind get kind;
  // enum kindEnum {  at,  pr_checks,  command,  };

  @BuiltValueField(wireName: r'reason')
  RunWakeReason get reason;
  // enum reasonEnum {  agent,  recovery,  usage_limit,  };

  /// Short text of the agent, at most `run.wake_label_max_chars` characters; may be empty.
  @BuiltValueField(wireName: r'label')
  String get label;

  @BuiltValueField(wireName: r'condition')
  RunWakeCondition get condition;

  /// The run's outage number of a `usage_limit` wake-up of `createRunWake`.
  @BuiltValueField(wireName: r'outage')
  int? get outage;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'not_before')
  DateTime get notBefore;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'deadline')
  DateTime get deadline;

  @BuiltValueField(wireName: r'state')
  RunWakeState get state;
  // enum stateEnum {  pending,  fired,  cancelled,  expired,  refused,  failed,  };

  /// Reason code of the last state change (09 5.8.3, 5.8.6), for example `autonomous_chain_max`, `deadline_passed` or `wake_input_skipped`.
  @BuiltValueField(wireName: r'state_reason')
  String? get stateReason;

  /// The result of a fired check, as the runner cut it.
  @BuiltValueField(wireName: r'result_text')
  String? get resultText;

  /// The wake run it started.
  @BuiltValueField(wireName: r'fired_run_id')
  String? get firedRunId;

  /// The active run it reached as a wake input.
  @BuiltValueField(wireName: r'fired_into_run_id')
  String? get firedIntoRunId;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'ended_at')
  DateTime? get endedAt;

  @BuiltValueField(wireName: r'version')
  int get version;

  RunWake._();

  factory RunWake([void updates(RunWakeBuilder b)]) = _$RunWake;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunWakeBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunWake> get serializer => _$RunWakeSerializer();
}

class _$RunWakeSerializer implements PrimitiveSerializer<RunWake> {
  @override
  final Iterable<Type> types = const [RunWake, _$RunWake];

  @override
  final String wireName = r'RunWake';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunWake object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'source_run_id';
    yield serializers.serialize(
      object.sourceRunId,
      specifiedType: const FullType(String),
    );
    yield r'triggered_by';
    yield serializers.serialize(
      object.triggeredBy,
      specifiedType: const FullType(String),
    );
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(RunWakeKind),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(RunWakeReason),
    );
    yield r'label';
    yield serializers.serialize(
      object.label,
      specifiedType: const FullType(String),
    );
    yield r'condition';
    yield serializers.serialize(
      object.condition,
      specifiedType: const FullType(RunWakeCondition),
    );
    if (object.outage != null) {
      yield r'outage';
      yield serializers.serialize(
        object.outage,
        specifiedType: const FullType(int),
      );
    }
    yield r'not_before';
    yield serializers.serialize(
      object.notBefore,
      specifiedType: const FullType(DateTime),
    );
    yield r'deadline';
    yield serializers.serialize(
      object.deadline,
      specifiedType: const FullType(DateTime),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(RunWakeState),
    );
    if (object.stateReason != null) {
      yield r'state_reason';
      yield serializers.serialize(
        object.stateReason,
        specifiedType: const FullType(String),
      );
    }
    if (object.resultText != null) {
      yield r'result_text';
      yield serializers.serialize(
        object.resultText,
        specifiedType: const FullType(String),
      );
    }
    if (object.firedRunId != null) {
      yield r'fired_run_id';
      yield serializers.serialize(
        object.firedRunId,
        specifiedType: const FullType(String),
      );
    }
    if (object.firedIntoRunId != null) {
      yield r'fired_into_run_id';
      yield serializers.serialize(
        object.firedIntoRunId,
        specifiedType: const FullType(String),
      );
    }
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
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    RunWake object, {
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
    required RunWakeBuilder result,
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
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.agentId = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'source_run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sourceRunId = valueDes;
          break;
        case r'triggered_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.triggeredBy = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunWakeKind),
          ) as RunWakeKind;
          result.kind = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunWakeReason),
          ) as RunWakeReason;
          result.reason = valueDes;
          break;
        case r'label':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.label = valueDes;
          break;
        case r'condition':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunWakeCondition),
          ) as RunWakeCondition;
          result.condition.replace(valueDes);
          break;
        case r'outage':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.outage = valueDes;
          break;
        case r'not_before':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.notBefore = valueDes;
          break;
        case r'deadline':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.deadline = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunWakeState),
          ) as RunWakeState;
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
        case r'result_text':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.resultText = valueDes;
          break;
        case r'fired_run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.firedRunId = valueDes;
          break;
        case r'fired_into_run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.firedIntoRunId = valueDes;
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
  RunWake deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunWakeBuilder();
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
