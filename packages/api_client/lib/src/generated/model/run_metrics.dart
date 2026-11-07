//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/run_autonomy_last.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/run_api_retry.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_metrics.g.dart';

/// Schema `run_metrics.v1`.
///
/// Properties:
/// * [contextPct]
/// * [tokensIn]
/// * [tokensOut]
/// * [tokensCacheRead]
/// * [tokensCacheWrite] - Cache writes with 5 minutes (or without a stated duration).
/// * [tokensCacheWrite1h] - Cache writes with 1 hour, counted separately.
/// * [subagents]
/// * [backgroundJobs]
/// * [apiRetries]
/// * [progressEvents] - Count of progress messages of the runtime in the current turn that changed nothing visible (for example a long running tool); a patch of it counts as progress against `run.stall_after` (09 5.5, 18.4).
/// * [lastApiRetry]
/// * [autonomyLast]
/// * [endState] - The end state of the run's last answer (09 18.19.11, 03 `runs.metrics`): `done` nothing more to do, `waiting` more work after something happens, `blocked` needs a person. Set at the finish of a run of a runner with `runner.end_state`; absent otherwise. Clients show it on the collapsed line of the run (01 6.21 item 2).
/// * [wakePendingUntil] - The time the next pending wake-up of the thread fires at the earliest, at the end of the run (09 18.19.11); absent when no wake-up was pending. Clients show when the thread continues (01 6.21 item 2).
@BuiltValue()
abstract class RunMetrics implements Built<RunMetrics, RunMetricsBuilder> {
  @BuiltValueField(wireName: r'context_pct')
  double? get contextPct;

  @BuiltValueField(wireName: r'tokens_in')
  int? get tokensIn;

  @BuiltValueField(wireName: r'tokens_out')
  int? get tokensOut;

  @BuiltValueField(wireName: r'tokens_cache_read')
  int? get tokensCacheRead;

  /// Cache writes with 5 minutes (or without a stated duration).
  @BuiltValueField(wireName: r'tokens_cache_write')
  int? get tokensCacheWrite;

  /// Cache writes with 1 hour, counted separately.
  @BuiltValueField(wireName: r'tokens_cache_write_1h')
  int? get tokensCacheWrite1h;

  @BuiltValueField(wireName: r'subagents')
  int? get subagents;

  @BuiltValueField(wireName: r'background_jobs')
  int? get backgroundJobs;

  @BuiltValueField(wireName: r'api_retries')
  int? get apiRetries;

  /// Count of progress messages of the runtime in the current turn that changed nothing visible (for example a long running tool); a patch of it counts as progress against `run.stall_after` (09 5.5, 18.4).
  @BuiltValueField(wireName: r'progress_events')
  int? get progressEvents;

  @BuiltValueField(wireName: r'last_api_retry')
  RunApiRetry? get lastApiRetry;

  @BuiltValueField(wireName: r'autonomy_last')
  RunAutonomyLast? get autonomyLast;

  /// The end state of the run's last answer (09 18.19.11, 03 `runs.metrics`): `done` nothing more to do, `waiting` more work after something happens, `blocked` needs a person. Set at the finish of a run of a runner with `runner.end_state`; absent otherwise. Clients show it on the collapsed line of the run (01 6.21 item 2).
  @BuiltValueField(wireName: r'end_state')
  RunMetricsEndStateEnum? get endState;
  // enum endStateEnum {  done,  waiting,  blocked,  };

  /// The time the next pending wake-up of the thread fires at the earliest, at the end of the run (09 18.19.11); absent when no wake-up was pending. Clients show when the thread continues (01 6.21 item 2).
  @BuiltValueField(wireName: r'wake_pending_until')
  DateTime? get wakePendingUntil;

  RunMetrics._();

  factory RunMetrics([void updates(RunMetricsBuilder b)]) = _$RunMetrics;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunMetricsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunMetrics> get serializer => _$RunMetricsSerializer();
}

class _$RunMetricsSerializer implements PrimitiveSerializer<RunMetrics> {
  @override
  final Iterable<Type> types = const [RunMetrics, _$RunMetrics];

  @override
  final String wireName = r'RunMetrics';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunMetrics object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.contextPct != null) {
      yield r'context_pct';
      yield serializers.serialize(
        object.contextPct,
        specifiedType: const FullType(double),
      );
    }
    if (object.tokensIn != null) {
      yield r'tokens_in';
      yield serializers.serialize(
        object.tokensIn,
        specifiedType: const FullType(int),
      );
    }
    if (object.tokensOut != null) {
      yield r'tokens_out';
      yield serializers.serialize(
        object.tokensOut,
        specifiedType: const FullType(int),
      );
    }
    if (object.tokensCacheRead != null) {
      yield r'tokens_cache_read';
      yield serializers.serialize(
        object.tokensCacheRead,
        specifiedType: const FullType(int),
      );
    }
    if (object.tokensCacheWrite != null) {
      yield r'tokens_cache_write';
      yield serializers.serialize(
        object.tokensCacheWrite,
        specifiedType: const FullType(int),
      );
    }
    if (object.tokensCacheWrite1h != null) {
      yield r'tokens_cache_write_1h';
      yield serializers.serialize(
        object.tokensCacheWrite1h,
        specifiedType: const FullType(int),
      );
    }
    if (object.subagents != null) {
      yield r'subagents';
      yield serializers.serialize(
        object.subagents,
        specifiedType: const FullType(int),
      );
    }
    if (object.backgroundJobs != null) {
      yield r'background_jobs';
      yield serializers.serialize(
        object.backgroundJobs,
        specifiedType: const FullType(int),
      );
    }
    if (object.apiRetries != null) {
      yield r'api_retries';
      yield serializers.serialize(
        object.apiRetries,
        specifiedType: const FullType(int),
      );
    }
    if (object.progressEvents != null) {
      yield r'progress_events';
      yield serializers.serialize(
        object.progressEvents,
        specifiedType: const FullType(int),
      );
    }
    if (object.lastApiRetry != null) {
      yield r'last_api_retry';
      yield serializers.serialize(
        object.lastApiRetry,
        specifiedType: const FullType(RunApiRetry),
      );
    }
    if (object.autonomyLast != null) {
      yield r'autonomy_last';
      yield serializers.serialize(
        object.autonomyLast,
        specifiedType: const FullType(RunAutonomyLast),
      );
    }
    if (object.endState != null) {
      yield r'end_state';
      yield serializers.serialize(
        object.endState,
        specifiedType: const FullType(RunMetricsEndStateEnum),
      );
    }
    if (object.wakePendingUntil != null) {
      yield r'wake_pending_until';
      yield serializers.serialize(
        object.wakePendingUntil,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunMetrics object, {
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
    required RunMetricsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'context_pct':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(double),
          ) as double?;
          if (valueDes == null) continue;
          result.contextPct = valueDes;
          break;
        case r'tokens_in':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.tokensIn = valueDes;
          break;
        case r'tokens_out':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.tokensOut = valueDes;
          break;
        case r'tokens_cache_read':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.tokensCacheRead = valueDes;
          break;
        case r'tokens_cache_write':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.tokensCacheWrite = valueDes;
          break;
        case r'tokens_cache_write_1h':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.tokensCacheWrite1h = valueDes;
          break;
        case r'subagents':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.subagents = valueDes;
          break;
        case r'background_jobs':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.backgroundJobs = valueDes;
          break;
        case r'api_retries':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.apiRetries = valueDes;
          break;
        case r'progress_events':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.progressEvents = valueDes;
          break;
        case r'last_api_retry':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(RunApiRetry),
          ) as RunApiRetry?;
          if (valueDes == null) continue;
          result.lastApiRetry.replace(valueDes);
          break;
        case r'autonomy_last':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(RunAutonomyLast),
          ) as RunAutonomyLast?;
          if (valueDes == null) continue;
          result.autonomyLast.replace(valueDes);
          break;
        case r'end_state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(RunMetricsEndStateEnum),
          ) as RunMetricsEndStateEnum?;
          if (valueDes == null) continue;
          result.endState = valueDes;
          break;
        case r'wake_pending_until':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.wakePendingUntil = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunMetrics deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunMetricsBuilder();
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

/// The end state of the run's last answer (09 18.19.11, 03 `runs.metrics`): `done` nothing more to do, `waiting` more work after something happens, `blocked` needs a person. Set at the finish of a run of a runner with `runner.end_state`; absent otherwise. Clients show it on the collapsed line of the run (01 6.21 item 2).
class RunMetricsEndStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'done')
  static const RunMetricsEndStateEnum done = _$runMetricsEndStateEnum_done;
  @BuiltValueEnumConst(wireName: r'waiting')
  static const RunMetricsEndStateEnum waiting =
      _$runMetricsEndStateEnum_waiting;
  @BuiltValueEnumConst(wireName: r'blocked')
  static const RunMetricsEndStateEnum blocked =
      _$runMetricsEndStateEnum_blocked;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunMetricsEndStateEnum unknownDefaultOpenApi =
      _$runMetricsEndStateEnum_unknownDefaultOpenApi;

  static Serializer<RunMetricsEndStateEnum> get serializer =>
      _$runMetricsEndStateEnumSerializer;

  const RunMetricsEndStateEnum._(String name) : super(name);

  static BuiltSet<RunMetricsEndStateEnum> get values =>
      _$runMetricsEndStateEnumValues;
  static RunMetricsEndStateEnum valueOf(String name) =>
      _$runMetricsEndStateEnumValueOf(name);
}
