// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_granularity.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatsGranularity _$hour = const StatsGranularity._('hour');
const StatsGranularity _$day = const StatsGranularity._('day');
const StatsGranularity _$week = const StatsGranularity._('week');
const StatsGranularity _$month = const StatsGranularity._('month');
const StatsGranularity _$unknownDefaultOpenApi = const StatsGranularity._(
  'unknownDefaultOpenApi',
);

StatsGranularity _$valueOf(String name) {
  switch (name) {
    case 'hour':
      return _$hour;
    case 'day':
      return _$day;
    case 'week':
      return _$week;
    case 'month':
      return _$month;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<StatsGranularity> _$values = BuiltSet<StatsGranularity>(
  const <StatsGranularity>[
    _$hour,
    _$day,
    _$week,
    _$month,
    _$unknownDefaultOpenApi,
  ],
);

class _$StatsGranularityMeta {
  const _$StatsGranularityMeta();
  StatsGranularity get hour => _$hour;
  StatsGranularity get day => _$day;
  StatsGranularity get week => _$week;
  StatsGranularity get month => _$month;
  StatsGranularity get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  StatsGranularity valueOf(String name) => _$valueOf(name);
  BuiltSet<StatsGranularity> get values => _$values;
}

mixin _$StatsGranularityMixin {
  // ignore: non_constant_identifier_names
  _$StatsGranularityMeta get StatsGranularity => const _$StatsGranularityMeta();
}

Serializer<StatsGranularity> _$statsGranularitySerializer =
    _$StatsGranularitySerializer();

class _$StatsGranularitySerializer
    implements PrimitiveSerializer<StatsGranularity> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'hour': 'hour',
    'day': 'day',
    'week': 'week',
    'month': 'month',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'hour': 'hour',
    'day': 'day',
    'week': 'week',
    'month': 'month',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatsGranularity];
  @override
  final String wireName = 'StatsGranularity';

  @override
  Object serialize(
    Serializers serializers,
    StatsGranularity object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatsGranularity deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatsGranularity.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
