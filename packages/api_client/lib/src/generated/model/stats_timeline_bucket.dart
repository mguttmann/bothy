//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/stats_metric.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_timeline_bucket.g.dart';

/// One row of `stats_minutes`.
///
/// Properties:
/// * [start] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [metric]
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [dimension] - Code from the closed list per metric; empty string when the metric has none.
/// * [currency]
/// * [count]
/// * [sum]
/// * [good]
@BuiltValue()
abstract class StatsTimelineBucket
    implements Built<StatsTimelineBucket, StatsTimelineBucketBuilder> {
  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'start')
  DateTime get start;

  @BuiltValueField(wireName: r'metric')
  StatsMetric get metric;
  // enum metricEnum {  messages_created,  threads_created,  runs_started,  runs_finished,  tool_calls,  approval_cards,  cost,  invocations_denied,  billing_mode_switches,  catchups,  resyncs,  slow_consumer_closes,  sli.api_availability,  sli.delivery_latency,  sli.reconnect,  sli.push_acceptance,  sli.run_start,  sli.turn_duration,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String? get channelId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String? get agentId;

  /// Code from the closed list per metric; empty string when the metric has none.
  @BuiltValueField(wireName: r'dimension')
  String get dimension;

  @BuiltValueField(wireName: r'currency')
  String? get currency;

  @BuiltValueField(wireName: r'count')
  int get count;

  @BuiltValueField(wireName: r'sum')
  int get sum;

  @BuiltValueField(wireName: r'good')
  int get good;

  StatsTimelineBucket._();

  factory StatsTimelineBucket([void updates(StatsTimelineBucketBuilder b)]) =
      _$StatsTimelineBucket;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsTimelineBucketBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsTimelineBucket> get serializer =>
      _$StatsTimelineBucketSerializer();
}

class _$StatsTimelineBucketSerializer
    implements PrimitiveSerializer<StatsTimelineBucket> {
  @override
  final Iterable<Type> types = const [
    StatsTimelineBucket,
    _$StatsTimelineBucket,
  ];

  @override
  final String wireName = r'StatsTimelineBucket';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsTimelineBucket object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'start';
    yield serializers.serialize(
      object.start,
      specifiedType: const FullType(DateTime),
    );
    yield r'metric';
    yield serializers.serialize(
      object.metric,
      specifiedType: const FullType(StatsMetric),
    );
    if (object.channelId != null) {
      yield r'channel_id';
      yield serializers.serialize(
        object.channelId,
        specifiedType: const FullType(String),
      );
    }
    if (object.agentId != null) {
      yield r'agent_id';
      yield serializers.serialize(
        object.agentId,
        specifiedType: const FullType(String),
      );
    }
    yield r'dimension';
    yield serializers.serialize(
      object.dimension,
      specifiedType: const FullType(String),
    );
    if (object.currency != null) {
      yield r'currency';
      yield serializers.serialize(
        object.currency,
        specifiedType: const FullType(String),
      );
    }
    yield r'count';
    yield serializers.serialize(
      object.count,
      specifiedType: const FullType(int),
    );
    yield r'sum';
    yield serializers.serialize(object.sum, specifiedType: const FullType(int));
    yield r'good';
    yield serializers.serialize(
      object.good,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsTimelineBucket object, {
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
    required StatsTimelineBucketBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'start':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.start = valueDes;
          break;
        case r'metric':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsMetric),
          ) as StatsMetric;
          result.metric = valueDes;
          break;
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.channelId = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.agentId = valueDes;
          break;
        case r'dimension':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.dimension = valueDes;
          break;
        case r'currency':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.currency = valueDes;
          break;
        case r'count':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.count = valueDes;
          break;
        case r'sum':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.sum = valueDes;
          break;
        case r'good':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.good = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsTimelineBucket deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsTimelineBucketBuilder();
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
