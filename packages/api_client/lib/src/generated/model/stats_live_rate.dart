//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/stats_metric.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_live_rate.g.dart';

/// Sum of `count` (and `sum` where meaningful) of one metric over the last full minute and the current minute up to `at`.
///
/// Properties:
/// * [metric]
/// * [dimension] - Dimension code; absent means all dimensions summed.
/// * [lastMinute]
/// * [currentMinute]
/// * [lastMinuteSum] - Sum of values (milliseconds or micro units) of the last full minute.
/// * [currency] - Only for metric `cost`.
@BuiltValue()
abstract class StatsLiveRate
    implements Built<StatsLiveRate, StatsLiveRateBuilder> {
  @BuiltValueField(wireName: r'metric')
  StatsMetric get metric;
  // enum metricEnum {  messages_created,  threads_created,  runs_started,  runs_finished,  tool_calls,  approval_cards,  cost,  invocations_denied,  billing_mode_switches,  catchups,  resyncs,  slow_consumer_closes,  sli.api_availability,  sli.delivery_latency,  sli.reconnect,  sli.push_acceptance,  sli.run_start,  sli.turn_duration,  };

  /// Dimension code; absent means all dimensions summed.
  @BuiltValueField(wireName: r'dimension')
  String? get dimension;

  @BuiltValueField(wireName: r'last_minute')
  int get lastMinute;

  @BuiltValueField(wireName: r'current_minute')
  int get currentMinute;

  /// Sum of values (milliseconds or micro units) of the last full minute.
  @BuiltValueField(wireName: r'last_minute_sum')
  int? get lastMinuteSum;

  /// Only for metric `cost`.
  @BuiltValueField(wireName: r'currency')
  String? get currency;

  StatsLiveRate._();

  factory StatsLiveRate([void updates(StatsLiveRateBuilder b)]) =
      _$StatsLiveRate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsLiveRateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsLiveRate> get serializer =>
      _$StatsLiveRateSerializer();
}

class _$StatsLiveRateSerializer implements PrimitiveSerializer<StatsLiveRate> {
  @override
  final Iterable<Type> types = const [StatsLiveRate, _$StatsLiveRate];

  @override
  final String wireName = r'StatsLiveRate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsLiveRate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'metric';
    yield serializers.serialize(
      object.metric,
      specifiedType: const FullType(StatsMetric),
    );
    if (object.dimension != null) {
      yield r'dimension';
      yield serializers.serialize(
        object.dimension,
        specifiedType: const FullType(String),
      );
    }
    yield r'last_minute';
    yield serializers.serialize(
      object.lastMinute,
      specifiedType: const FullType(int),
    );
    yield r'current_minute';
    yield serializers.serialize(
      object.currentMinute,
      specifiedType: const FullType(int),
    );
    if (object.lastMinuteSum != null) {
      yield r'last_minute_sum';
      yield serializers.serialize(
        object.lastMinuteSum,
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
    StatsLiveRate object, {
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
    required StatsLiveRateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'metric':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsMetric),
          ) as StatsMetric;
          result.metric = valueDes;
          break;
        case r'dimension':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.dimension = valueDes;
          break;
        case r'last_minute':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.lastMinute = valueDes;
          break;
        case r'current_minute':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.currentMinute = valueDes;
          break;
        case r'last_minute_sum':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.lastMinuteSum = valueDes;
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
  StatsLiveRate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsLiveRateBuilder();
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
