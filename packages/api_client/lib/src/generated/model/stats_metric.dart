//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_metric.g.dart';

/// Metric of the own measurement in `stats_minutes` (docs/spec/25-visualization.md, section 3.2).
class StatsMetric extends EnumClass {
  @BuiltValueEnumConst(wireName: r'messages_created')
  static const StatsMetric messagesCreated = _$messagesCreated;
  @BuiltValueEnumConst(wireName: r'threads_created')
  static const StatsMetric threadsCreated = _$threadsCreated;
  @BuiltValueEnumConst(wireName: r'runs_started')
  static const StatsMetric runsStarted = _$runsStarted;
  @BuiltValueEnumConst(wireName: r'runs_finished')
  static const StatsMetric runsFinished = _$runsFinished;
  @BuiltValueEnumConst(wireName: r'tool_calls')
  static const StatsMetric toolCalls = _$toolCalls;
  @BuiltValueEnumConst(wireName: r'approval_cards')
  static const StatsMetric approvalCards = _$approvalCards;
  @BuiltValueEnumConst(wireName: r'cost')
  static const StatsMetric cost = _$cost;
  @BuiltValueEnumConst(wireName: r'invocations_denied')
  static const StatsMetric invocationsDenied = _$invocationsDenied;
  @BuiltValueEnumConst(wireName: r'billing_mode_switches')
  static const StatsMetric billingModeSwitches = _$billingModeSwitches;
  @BuiltValueEnumConst(wireName: r'catchups')
  static const StatsMetric catchups = _$catchups;
  @BuiltValueEnumConst(wireName: r'resyncs')
  static const StatsMetric resyncs = _$resyncs;
  @BuiltValueEnumConst(wireName: r'slow_consumer_closes')
  static const StatsMetric slowConsumerCloses = _$slowConsumerCloses;
  @BuiltValueEnumConst(wireName: r'sli.api_availability')
  static const StatsMetric sliPeriodApiAvailability =
      _$sliPeriodApiAvailability;
  @BuiltValueEnumConst(wireName: r'sli.delivery_latency')
  static const StatsMetric sliPeriodDeliveryLatency =
      _$sliPeriodDeliveryLatency;
  @BuiltValueEnumConst(wireName: r'sli.reconnect')
  static const StatsMetric sliPeriodReconnect = _$sliPeriodReconnect;
  @BuiltValueEnumConst(wireName: r'sli.push_acceptance')
  static const StatsMetric sliPeriodPushAcceptance = _$sliPeriodPushAcceptance;
  @BuiltValueEnumConst(wireName: r'sli.run_start')
  static const StatsMetric sliPeriodRunStart = _$sliPeriodRunStart;
  @BuiltValueEnumConst(wireName: r'sli.turn_duration')
  static const StatsMetric sliPeriodTurnDuration = _$sliPeriodTurnDuration;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatsMetric unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<StatsMetric> get serializer => _$statsMetricSerializer;

  const StatsMetric._(String name) : super(name);

  static BuiltSet<StatsMetric> get values => _$values;
  static StatsMetric valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class StatsMetricMixin = Object with _$StatsMetricMixin;
