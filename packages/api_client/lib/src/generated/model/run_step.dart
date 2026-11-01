//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_step.g.dart';

/// RunStep
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [ordinal]
/// * [parentStepId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [kind]
/// * [name]
/// * [title]
/// * [status]
/// * [detail] - Schema `run_step_detail.v1`, cut to `run.step_detail_max_bytes`. A step of kind `subagent`, `workflow` or `background_job` may carry the member `usage` (object with the optional integers `tokens`, `tool_uses`, `duration_ms`, `cost_micros` and the text `currency`, required with `cost_micros`), reported by the adapter for display only, never a booking (09 5.6). A workflow agent may carry the texts `last_tool` (the tool it ran last, its current tool while it runs) and `phase` (09 5.6).
/// * [startedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [finishedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class RunStep implements Built<RunStep, RunStepBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'ordinal')
  int get ordinal;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'parent_step_id')
  String? get parentStepId;

  @BuiltValueField(wireName: r'kind')
  RunStepKindEnum get kind;
  // enum kindEnum {  tool_call,  status,  subagent,  workflow,  background_job,  };

  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'title')
  String get title;

  @BuiltValueField(wireName: r'status')
  RunStepStatusEnum get status;
  // enum statusEnum {  pending,  running,  completed,  failed,  cancelled,  };

  /// Schema `run_step_detail.v1`, cut to `run.step_detail_max_bytes`. A step of kind `subagent`, `workflow` or `background_job` may carry the member `usage` (object with the optional integers `tokens`, `tool_uses`, `duration_ms`, `cost_micros` and the text `currency`, required with `cost_micros`), reported by the adapter for display only, never a booking (09 5.6). A workflow agent may carry the texts `last_tool` (the tool it ran last, its current tool while it runs) and `phase` (09 5.6).
  @BuiltValueField(wireName: r'detail')
  BuiltMap<String, JsonObject?>? get detail;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'started_at')
  DateTime get startedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'finished_at')
  DateTime? get finishedAt;

  RunStep._();

  factory RunStep([void updates(RunStepBuilder b)]) = _$RunStep;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunStepBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunStep> get serializer => _$RunStepSerializer();
}

class _$RunStepSerializer implements PrimitiveSerializer<RunStep> {
  @override
  final Iterable<Type> types = const [RunStep, _$RunStep];

  @override
  final String wireName = r'RunStep';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunStep object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'ordinal';
    yield serializers.serialize(
      object.ordinal,
      specifiedType: const FullType(int),
    );
    if (object.parentStepId != null) {
      yield r'parent_step_id';
      yield serializers.serialize(
        object.parentStepId,
        specifiedType: const FullType(String),
      );
    }
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(RunStepKindEnum),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'title';
    yield serializers.serialize(
      object.title,
      specifiedType: const FullType(String),
    );
    yield r'status';
    yield serializers.serialize(
      object.status,
      specifiedType: const FullType(RunStepStatusEnum),
    );
    if (object.detail != null) {
      yield r'detail';
      yield serializers.serialize(
        object.detail,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
    yield r'started_at';
    yield serializers.serialize(
      object.startedAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.finishedAt != null) {
      yield r'finished_at';
      yield serializers.serialize(
        object.finishedAt,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunStep object, {
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
    required RunStepBuilder result,
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
        case r'ordinal':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.ordinal = valueDes;
          break;
        case r'parent_step_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.parentStepId = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunStepKindEnum),
          ) as RunStepKindEnum;
          result.kind = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'title':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.title = valueDes;
          break;
        case r'status':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunStepStatusEnum),
          ) as RunStepStatusEnum;
          result.status = valueDes;
          break;
        case r'detail':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.detail.replace(valueDes);
          break;
        case r'started_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
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
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunStep deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunStepBuilder();
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

class RunStepKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'tool_call')
  static const RunStepKindEnum toolCall = _$runStepKindEnum_toolCall;
  @BuiltValueEnumConst(wireName: r'status')
  static const RunStepKindEnum status = _$runStepKindEnum_status;
  @BuiltValueEnumConst(wireName: r'subagent')
  static const RunStepKindEnum subagent = _$runStepKindEnum_subagent;
  @BuiltValueEnumConst(wireName: r'workflow')
  static const RunStepKindEnum workflow = _$runStepKindEnum_workflow;
  @BuiltValueEnumConst(wireName: r'background_job')
  static const RunStepKindEnum backgroundJob = _$runStepKindEnum_backgroundJob;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunStepKindEnum unknownDefaultOpenApi =
      _$runStepKindEnum_unknownDefaultOpenApi;

  static Serializer<RunStepKindEnum> get serializer =>
      _$runStepKindEnumSerializer;

  const RunStepKindEnum._(String name) : super(name);

  static BuiltSet<RunStepKindEnum> get values => _$runStepKindEnumValues;
  static RunStepKindEnum valueOf(String name) => _$runStepKindEnumValueOf(name);
}

class RunStepStatusEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'pending')
  static const RunStepStatusEnum pending = _$runStepStatusEnum_pending;
  @BuiltValueEnumConst(wireName: r'running')
  static const RunStepStatusEnum running = _$runStepStatusEnum_running;
  @BuiltValueEnumConst(wireName: r'completed')
  static const RunStepStatusEnum completed = _$runStepStatusEnum_completed;
  @BuiltValueEnumConst(wireName: r'failed')
  static const RunStepStatusEnum failed = _$runStepStatusEnum_failed;
  @BuiltValueEnumConst(wireName: r'cancelled')
  static const RunStepStatusEnum cancelled = _$runStepStatusEnum_cancelled;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunStepStatusEnum unknownDefaultOpenApi =
      _$runStepStatusEnum_unknownDefaultOpenApi;

  static Serializer<RunStepStatusEnum> get serializer =>
      _$runStepStatusEnumSerializer;

  const RunStepStatusEnum._(String name) : super(name);

  static BuiltSet<RunStepStatusEnum> get values => _$runStepStatusEnumValues;
  static RunStepStatusEnum valueOf(String name) =>
      _$runStepStatusEnumValueOf(name);
}
