// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_metric.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatsMetric _$messagesCreated = const StatsMetric._('messagesCreated');
const StatsMetric _$threadsCreated = const StatsMetric._('threadsCreated');
const StatsMetric _$runsStarted = const StatsMetric._('runsStarted');
const StatsMetric _$runsFinished = const StatsMetric._('runsFinished');
const StatsMetric _$toolCalls = const StatsMetric._('toolCalls');
const StatsMetric _$approvalCards = const StatsMetric._('approvalCards');
const StatsMetric _$cost = const StatsMetric._('cost');
const StatsMetric _$invocationsDenied = const StatsMetric._(
  'invocationsDenied',
);
const StatsMetric _$billingModeSwitches = const StatsMetric._(
  'billingModeSwitches',
);
const StatsMetric _$catchups = const StatsMetric._('catchups');
const StatsMetric _$resyncs = const StatsMetric._('resyncs');
const StatsMetric _$slowConsumerCloses = const StatsMetric._(
  'slowConsumerCloses',
);
const StatsMetric _$sliPeriodApiAvailability = const StatsMetric._(
  'sliPeriodApiAvailability',
);
const StatsMetric _$sliPeriodDeliveryLatency = const StatsMetric._(
  'sliPeriodDeliveryLatency',
);
const StatsMetric _$sliPeriodReconnect = const StatsMetric._(
  'sliPeriodReconnect',
);
const StatsMetric _$sliPeriodPushAcceptance = const StatsMetric._(
  'sliPeriodPushAcceptance',
);
const StatsMetric _$sliPeriodRunStart = const StatsMetric._(
  'sliPeriodRunStart',
);
const StatsMetric _$sliPeriodTurnDuration = const StatsMetric._(
  'sliPeriodTurnDuration',
);
const StatsMetric _$unknownDefaultOpenApi = const StatsMetric._(
  'unknownDefaultOpenApi',
);

StatsMetric _$valueOf(String name) {
  switch (name) {
    case 'messagesCreated':
      return _$messagesCreated;
    case 'threadsCreated':
      return _$threadsCreated;
    case 'runsStarted':
      return _$runsStarted;
    case 'runsFinished':
      return _$runsFinished;
    case 'toolCalls':
      return _$toolCalls;
    case 'approvalCards':
      return _$approvalCards;
    case 'cost':
      return _$cost;
    case 'invocationsDenied':
      return _$invocationsDenied;
    case 'billingModeSwitches':
      return _$billingModeSwitches;
    case 'catchups':
      return _$catchups;
    case 'resyncs':
      return _$resyncs;
    case 'slowConsumerCloses':
      return _$slowConsumerCloses;
    case 'sliPeriodApiAvailability':
      return _$sliPeriodApiAvailability;
    case 'sliPeriodDeliveryLatency':
      return _$sliPeriodDeliveryLatency;
    case 'sliPeriodReconnect':
      return _$sliPeriodReconnect;
    case 'sliPeriodPushAcceptance':
      return _$sliPeriodPushAcceptance;
    case 'sliPeriodRunStart':
      return _$sliPeriodRunStart;
    case 'sliPeriodTurnDuration':
      return _$sliPeriodTurnDuration;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<StatsMetric> _$values = BuiltSet<StatsMetric>(
  const <StatsMetric>[
    _$messagesCreated,
    _$threadsCreated,
    _$runsStarted,
    _$runsFinished,
    _$toolCalls,
    _$approvalCards,
    _$cost,
    _$invocationsDenied,
    _$billingModeSwitches,
    _$catchups,
    _$resyncs,
    _$slowConsumerCloses,
    _$sliPeriodApiAvailability,
    _$sliPeriodDeliveryLatency,
    _$sliPeriodReconnect,
    _$sliPeriodPushAcceptance,
    _$sliPeriodRunStart,
    _$sliPeriodTurnDuration,
    _$unknownDefaultOpenApi,
  ],
);

class _$StatsMetricMeta {
  const _$StatsMetricMeta();
  StatsMetric get messagesCreated => _$messagesCreated;
  StatsMetric get threadsCreated => _$threadsCreated;
  StatsMetric get runsStarted => _$runsStarted;
  StatsMetric get runsFinished => _$runsFinished;
  StatsMetric get toolCalls => _$toolCalls;
  StatsMetric get approvalCards => _$approvalCards;
  StatsMetric get cost => _$cost;
  StatsMetric get invocationsDenied => _$invocationsDenied;
  StatsMetric get billingModeSwitches => _$billingModeSwitches;
  StatsMetric get catchups => _$catchups;
  StatsMetric get resyncs => _$resyncs;
  StatsMetric get slowConsumerCloses => _$slowConsumerCloses;
  StatsMetric get sliPeriodApiAvailability => _$sliPeriodApiAvailability;
  StatsMetric get sliPeriodDeliveryLatency => _$sliPeriodDeliveryLatency;
  StatsMetric get sliPeriodReconnect => _$sliPeriodReconnect;
  StatsMetric get sliPeriodPushAcceptance => _$sliPeriodPushAcceptance;
  StatsMetric get sliPeriodRunStart => _$sliPeriodRunStart;
  StatsMetric get sliPeriodTurnDuration => _$sliPeriodTurnDuration;
  StatsMetric get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  StatsMetric valueOf(String name) => _$valueOf(name);
  BuiltSet<StatsMetric> get values => _$values;
}

mixin _$StatsMetricMixin {
  // ignore: non_constant_identifier_names
  _$StatsMetricMeta get StatsMetric => const _$StatsMetricMeta();
}

Serializer<StatsMetric> _$statsMetricSerializer = _$StatsMetricSerializer();

class _$StatsMetricSerializer implements PrimitiveSerializer<StatsMetric> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'messagesCreated': 'messages_created',
    'threadsCreated': 'threads_created',
    'runsStarted': 'runs_started',
    'runsFinished': 'runs_finished',
    'toolCalls': 'tool_calls',
    'approvalCards': 'approval_cards',
    'cost': 'cost',
    'invocationsDenied': 'invocations_denied',
    'billingModeSwitches': 'billing_mode_switches',
    'catchups': 'catchups',
    'resyncs': 'resyncs',
    'slowConsumerCloses': 'slow_consumer_closes',
    'sliPeriodApiAvailability': 'sli.api_availability',
    'sliPeriodDeliveryLatency': 'sli.delivery_latency',
    'sliPeriodReconnect': 'sli.reconnect',
    'sliPeriodPushAcceptance': 'sli.push_acceptance',
    'sliPeriodRunStart': 'sli.run_start',
    'sliPeriodTurnDuration': 'sli.turn_duration',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'messages_created': 'messagesCreated',
    'threads_created': 'threadsCreated',
    'runs_started': 'runsStarted',
    'runs_finished': 'runsFinished',
    'tool_calls': 'toolCalls',
    'approval_cards': 'approvalCards',
    'cost': 'cost',
    'invocations_denied': 'invocationsDenied',
    'billing_mode_switches': 'billingModeSwitches',
    'catchups': 'catchups',
    'resyncs': 'resyncs',
    'slow_consumer_closes': 'slowConsumerCloses',
    'sli.api_availability': 'sliPeriodApiAvailability',
    'sli.delivery_latency': 'sliPeriodDeliveryLatency',
    'sli.reconnect': 'sliPeriodReconnect',
    'sli.push_acceptance': 'sliPeriodPushAcceptance',
    'sli.run_start': 'sliPeriodRunStart',
    'sli.turn_duration': 'sliPeriodTurnDuration',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatsMetric];
  @override
  final String wireName = 'StatsMetric';

  @override
  Object serialize(
    Serializers serializers,
    StatsMetric object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatsMetric deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatsMetric.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
