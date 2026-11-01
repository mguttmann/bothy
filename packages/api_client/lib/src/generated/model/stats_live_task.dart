//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_live_task.g.dart';

/// One child node of an active run in the live topology (25 5.2). Never content except `label`, which only a receiver with `run.read` on the run gets.
///
/// Properties:
/// * [id] - The step ID.
/// * [parentId] - The task node this node hangs under; absent when it hangs under the run.
/// * [kind]
/// * [name] - Step name, the adapter's type of the task, never content.
/// * [state]
/// * [startedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [finishedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [tool] - Name of the current tool of a running node.
/// * [label] - First line of the step title; only with `run.read` on the run.
@BuiltValue()
abstract class StatsLiveTask
    implements Built<StatsLiveTask, StatsLiveTaskBuilder> {
  /// The step ID.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// The task node this node hangs under; absent when it hangs under the run.
  @BuiltValueField(wireName: r'parent_id')
  String? get parentId;

  @BuiltValueField(wireName: r'kind')
  StatsLiveTaskKindEnum get kind;
  // enum kindEnum {  subagent,  workflow,  background_job,  };

  /// Step name, the adapter's type of the task, never content.
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'state')
  StatsLiveTaskStateEnum get state;
  // enum stateEnum {  running,  completed,  failed,  cancelled,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'started_at')
  DateTime get startedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'finished_at')
  DateTime? get finishedAt;

  /// Name of the current tool of a running node.
  @BuiltValueField(wireName: r'tool')
  String? get tool;

  /// First line of the step title; only with `run.read` on the run.
  @BuiltValueField(wireName: r'label')
  String? get label;

  StatsLiveTask._();

  factory StatsLiveTask([void updates(StatsLiveTaskBuilder b)]) =
      _$StatsLiveTask;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsLiveTaskBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsLiveTask> get serializer =>
      _$StatsLiveTaskSerializer();
}

class _$StatsLiveTaskSerializer implements PrimitiveSerializer<StatsLiveTask> {
  @override
  final Iterable<Type> types = const [StatsLiveTask, _$StatsLiveTask];

  @override
  final String wireName = r'StatsLiveTask';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsLiveTask object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    if (object.parentId != null) {
      yield r'parent_id';
      yield serializers.serialize(
        object.parentId,
        specifiedType: const FullType(String),
      );
    }
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(StatsLiveTaskKindEnum),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(StatsLiveTaskStateEnum),
    );
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
    if (object.tool != null) {
      yield r'tool';
      yield serializers.serialize(
        object.tool,
        specifiedType: const FullType(String),
      );
    }
    if (object.label != null) {
      yield r'label';
      yield serializers.serialize(
        object.label,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsLiveTask object, {
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
    required StatsLiveTaskBuilder result,
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
        case r'parent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.parentId = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsLiveTaskKindEnum),
          ) as StatsLiveTaskKindEnum;
          result.kind = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsLiveTaskStateEnum),
          ) as StatsLiveTaskStateEnum;
          result.state = valueDes;
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
        case r'tool':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.tool = valueDes;
          break;
        case r'label':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.label = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsLiveTask deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsLiveTaskBuilder();
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

class StatsLiveTaskKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'subagent')
  static const StatsLiveTaskKindEnum subagent =
      _$statsLiveTaskKindEnum_subagent;
  @BuiltValueEnumConst(wireName: r'workflow')
  static const StatsLiveTaskKindEnum workflow =
      _$statsLiveTaskKindEnum_workflow;
  @BuiltValueEnumConst(wireName: r'background_job')
  static const StatsLiveTaskKindEnum backgroundJob =
      _$statsLiveTaskKindEnum_backgroundJob;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatsLiveTaskKindEnum unknownDefaultOpenApi =
      _$statsLiveTaskKindEnum_unknownDefaultOpenApi;

  static Serializer<StatsLiveTaskKindEnum> get serializer =>
      _$statsLiveTaskKindEnumSerializer;

  const StatsLiveTaskKindEnum._(String name) : super(name);

  static BuiltSet<StatsLiveTaskKindEnum> get values =>
      _$statsLiveTaskKindEnumValues;
  static StatsLiveTaskKindEnum valueOf(String name) =>
      _$statsLiveTaskKindEnumValueOf(name);
}

class StatsLiveTaskStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'running')
  static const StatsLiveTaskStateEnum running =
      _$statsLiveTaskStateEnum_running;
  @BuiltValueEnumConst(wireName: r'completed')
  static const StatsLiveTaskStateEnum completed =
      _$statsLiveTaskStateEnum_completed;
  @BuiltValueEnumConst(wireName: r'failed')
  static const StatsLiveTaskStateEnum failed = _$statsLiveTaskStateEnum_failed;
  @BuiltValueEnumConst(wireName: r'cancelled')
  static const StatsLiveTaskStateEnum cancelled =
      _$statsLiveTaskStateEnum_cancelled;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatsLiveTaskStateEnum unknownDefaultOpenApi =
      _$statsLiveTaskStateEnum_unknownDefaultOpenApi;

  static Serializer<StatsLiveTaskStateEnum> get serializer =>
      _$statsLiveTaskStateEnumSerializer;

  const StatsLiveTaskStateEnum._(String name) : super(name);

  static BuiltSet<StatsLiveTaskStateEnum> get values =>
      _$statsLiveTaskStateEnumValues;
  static StatsLiveTaskStateEnum valueOf(String name) =>
      _$statsLiveTaskStateEnumValueOf(name);
}
