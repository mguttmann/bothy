//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/stats_metric.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_live_sli.g.dart';

/// StatsLiveSli
///
/// Properties:
/// * [metric]
/// * [total]
/// * [good]
/// * [ratio] - `good / total`; absent when `total` is 0.
@BuiltValue()
abstract class StatsLiveSli
    implements Built<StatsLiveSli, StatsLiveSliBuilder> {
  @BuiltValueField(wireName: r'metric')
  StatsMetric get metric;
  // enum metricEnum {  messages_created,  threads_created,  runs_started,  runs_finished,  tool_calls,  approval_cards,  cost,  invocations_denied,  billing_mode_switches,  catchups,  resyncs,  slow_consumer_closes,  sli.api_availability,  sli.delivery_latency,  sli.reconnect,  sli.push_acceptance,  sli.run_start,  sli.turn_duration,  };

  @BuiltValueField(wireName: r'total')
  int get total;

  @BuiltValueField(wireName: r'good')
  int get good;

  /// `good / total`; absent when `total` is 0.
  @BuiltValueField(wireName: r'ratio')
  double? get ratio;

  StatsLiveSli._();

  factory StatsLiveSli([void updates(StatsLiveSliBuilder b)]) = _$StatsLiveSli;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsLiveSliBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsLiveSli> get serializer => _$StatsLiveSliSerializer();
}

class _$StatsLiveSliSerializer implements PrimitiveSerializer<StatsLiveSli> {
  @override
  final Iterable<Type> types = const [StatsLiveSli, _$StatsLiveSli];

  @override
  final String wireName = r'StatsLiveSli';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsLiveSli object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'metric';
    yield serializers.serialize(
      object.metric,
      specifiedType: const FullType(StatsMetric),
    );
    yield r'total';
    yield serializers.serialize(
      object.total,
      specifiedType: const FullType(int),
    );
    yield r'good';
    yield serializers.serialize(
      object.good,
      specifiedType: const FullType(int),
    );
    if (object.ratio != null) {
      yield r'ratio';
      yield serializers.serialize(
        object.ratio,
        specifiedType: const FullType(double),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsLiveSli object, {
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
    required StatsLiveSliBuilder result,
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
        case r'total':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.total = valueDes;
          break;
        case r'good':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.good = valueDes;
          break;
        case r'ratio':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(double),
          ) as double?;
          if (valueDes == null) continue;
          result.ratio = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsLiveSli deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsLiveSliBuilder();
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
