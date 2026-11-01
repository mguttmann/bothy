//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/stats_live_run.dart';
import 'package:api_client/src/generated/model/stats_live_connections.dart';
import 'package:api_client/src/generated/model/stats_live_agent.dart';
import 'package:api_client/src/generated/model/stats_live_sli.dart';
import 'package:api_client/src/generated/model/stats_scope.dart';
import 'package:api_client/src/generated/model/stats_live_rate.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_live_frame.g.dart';

/// Live frame (docs/spec/25-visualization.md, section 5.2). No person values and no content; the only names are the channel name and thread title of a run in `chat`, for a caller who may read the thread.
///
/// Properties:
/// * [at] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [intervalMs]
/// * [scope]
/// * [runs]
/// * [runsTruncated]
/// * [agents]
/// * [rates]
/// * [sli] - Only for the instance scope.
/// * [connections]
@BuiltValue()
abstract class StatsLiveFrame
    implements Built<StatsLiveFrame, StatsLiveFrameBuilder> {
  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'at')
  DateTime get at;

  @BuiltValueField(wireName: r'interval_ms')
  int get intervalMs;

  @BuiltValueField(wireName: r'scope')
  StatsScope get scope;

  @BuiltValueField(wireName: r'runs')
  BuiltList<StatsLiveRun> get runs;

  @BuiltValueField(wireName: r'runs_truncated')
  bool get runsTruncated;

  @BuiltValueField(wireName: r'agents')
  BuiltList<StatsLiveAgent> get agents;

  @BuiltValueField(wireName: r'rates')
  BuiltList<StatsLiveRate> get rates;

  /// Only for the instance scope.
  @BuiltValueField(wireName: r'sli')
  BuiltList<StatsLiveSli>? get sli;

  @BuiltValueField(wireName: r'connections')
  StatsLiveConnections? get connections;

  StatsLiveFrame._();

  factory StatsLiveFrame([void updates(StatsLiveFrameBuilder b)]) =
      _$StatsLiveFrame;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsLiveFrameBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsLiveFrame> get serializer =>
      _$StatsLiveFrameSerializer();
}

class _$StatsLiveFrameSerializer
    implements PrimitiveSerializer<StatsLiveFrame> {
  @override
  final Iterable<Type> types = const [StatsLiveFrame, _$StatsLiveFrame];

  @override
  final String wireName = r'StatsLiveFrame';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsLiveFrame object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'at';
    yield serializers.serialize(
      object.at,
      specifiedType: const FullType(DateTime),
    );
    yield r'interval_ms';
    yield serializers.serialize(
      object.intervalMs,
      specifiedType: const FullType(int),
    );
    yield r'scope';
    yield serializers.serialize(
      object.scope,
      specifiedType: const FullType(StatsScope),
    );
    yield r'runs';
    yield serializers.serialize(
      object.runs,
      specifiedType: const FullType(BuiltList, [FullType(StatsLiveRun)]),
    );
    yield r'runs_truncated';
    yield serializers.serialize(
      object.runsTruncated,
      specifiedType: const FullType(bool),
    );
    yield r'agents';
    yield serializers.serialize(
      object.agents,
      specifiedType: const FullType(BuiltList, [FullType(StatsLiveAgent)]),
    );
    yield r'rates';
    yield serializers.serialize(
      object.rates,
      specifiedType: const FullType(BuiltList, [FullType(StatsLiveRate)]),
    );
    if (object.sli != null) {
      yield r'sli';
      yield serializers.serialize(
        object.sli,
        specifiedType: const FullType(BuiltList, [FullType(StatsLiveSli)]),
      );
    }
    if (object.connections != null) {
      yield r'connections';
      yield serializers.serialize(
        object.connections,
        specifiedType: const FullType(StatsLiveConnections),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsLiveFrame object, {
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
    required StatsLiveFrameBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.at = valueDes;
          break;
        case r'interval_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.intervalMs = valueDes;
          break;
        case r'scope':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsScope),
          ) as StatsScope;
          result.scope.replace(valueDes);
          break;
        case r'runs':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(StatsLiveRun)]),
          ) as BuiltList<StatsLiveRun>;
          result.runs.replace(valueDes);
          break;
        case r'runs_truncated':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.runsTruncated = valueDes;
          break;
        case r'agents':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(StatsLiveAgent),
            ]),
          ) as BuiltList<StatsLiveAgent>;
          result.agents.replace(valueDes);
          break;
        case r'rates':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(StatsLiveRate)]),
          ) as BuiltList<StatsLiveRate>;
          result.rates.replace(valueDes);
          break;
        case r'sli':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(StatsLiveSli),
            ]),
          ) as BuiltList<StatsLiveSli>?;
          if (valueDes == null) continue;
          result.sli.replace(valueDes);
          break;
        case r'connections':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(StatsLiveConnections),
          ) as StatsLiveConnections?;
          if (valueDes == null) continue;
          result.connections.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsLiveFrame deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsLiveFrameBuilder();
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
