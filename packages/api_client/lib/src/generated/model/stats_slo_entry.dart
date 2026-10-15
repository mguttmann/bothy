//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/stats_slo_state.dart';
import 'package:api_client/src/generated/model/stats_metric.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_slo_entry.g.dart';

/// StatsSloEntry
///
/// Properties:
/// * [slo] - `service/name` of the Sloth file.
/// * [metric]
/// * [objective] - Objective in percent from the Sloth file.
/// * [thresholdMs] - Threshold of a latency SLO; absent for ratio SLOs.
/// * [periodDays]
/// * [total]
/// * [good]
/// * [ratio]
/// * [budgetRemaining] - Remaining error budget, `1 - (1 - ratio) / (1 - objective/100)`; may be negative.
/// * [state]
@BuiltValue()
abstract class StatsSloEntry
    implements Built<StatsSloEntry, StatsSloEntryBuilder> {
  /// `service/name` of the Sloth file.
  @BuiltValueField(wireName: r'slo')
  String get slo;

  @BuiltValueField(wireName: r'metric')
  StatsMetric get metric;
  // enum metricEnum {  messages_created,  threads_created,  runs_started,  runs_finished,  tool_calls,  approval_cards,  cost,  invocations_denied,  billing_mode_switches,  catchups,  resyncs,  slow_consumer_closes,  sli.api_availability,  sli.delivery_latency,  sli.reconnect,  sli.push_acceptance,  sli.run_start,  sli.turn_duration,  };

  /// Objective in percent from the Sloth file.
  @BuiltValueField(wireName: r'objective')
  double get objective;

  /// Threshold of a latency SLO; absent for ratio SLOs.
  @BuiltValueField(wireName: r'threshold_ms')
  int? get thresholdMs;

  @BuiltValueField(wireName: r'period_days')
  int get periodDays;

  @BuiltValueField(wireName: r'total')
  int get total;

  @BuiltValueField(wireName: r'good')
  int get good;

  @BuiltValueField(wireName: r'ratio')
  double? get ratio;

  /// Remaining error budget, `1 - (1 - ratio) / (1 - objective/100)`; may be negative.
  @BuiltValueField(wireName: r'budget_remaining')
  double? get budgetRemaining;

  @BuiltValueField(wireName: r'state')
  StatsSloState get state;
  // enum stateEnum {  green,  amber,  red,  insufficient_data,  };

  StatsSloEntry._();

  factory StatsSloEntry([void updates(StatsSloEntryBuilder b)]) =
      _$StatsSloEntry;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsSloEntryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsSloEntry> get serializer =>
      _$StatsSloEntrySerializer();
}

class _$StatsSloEntrySerializer implements PrimitiveSerializer<StatsSloEntry> {
  @override
  final Iterable<Type> types = const [StatsSloEntry, _$StatsSloEntry];

  @override
  final String wireName = r'StatsSloEntry';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsSloEntry object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'slo';
    yield serializers.serialize(
      object.slo,
      specifiedType: const FullType(String),
    );
    yield r'metric';
    yield serializers.serialize(
      object.metric,
      specifiedType: const FullType(StatsMetric),
    );
    yield r'objective';
    yield serializers.serialize(
      object.objective,
      specifiedType: const FullType(double),
    );
    if (object.thresholdMs != null) {
      yield r'threshold_ms';
      yield serializers.serialize(
        object.thresholdMs,
        specifiedType: const FullType(int),
      );
    }
    yield r'period_days';
    yield serializers.serialize(
      object.periodDays,
      specifiedType: const FullType(int),
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
    if (object.budgetRemaining != null) {
      yield r'budget_remaining';
      yield serializers.serialize(
        object.budgetRemaining,
        specifiedType: const FullType(double),
      );
    }
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(StatsSloState),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsSloEntry object, {
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
    required StatsSloEntryBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'slo':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.slo = valueDes;
          break;
        case r'metric':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsMetric),
          ) as StatsMetric;
          result.metric = valueDes;
          break;
        case r'objective':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(double),
          ) as double;
          result.objective = valueDes;
          break;
        case r'threshold_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.thresholdMs = valueDes;
          break;
        case r'period_days':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.periodDays = valueDes;
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
        case r'budget_remaining':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(double),
          ) as double?;
          if (valueDes == null) continue;
          result.budgetRemaining = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsSloState),
          ) as StatsSloState;
          result.state = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsSloEntry deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsSloEntryBuilder();
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
