// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_slo_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatsSloState _$green = const StatsSloState._('green');
const StatsSloState _$amber = const StatsSloState._('amber');
const StatsSloState _$red = const StatsSloState._('red');
const StatsSloState _$insufficientData = const StatsSloState._(
  'insufficientData',
);
const StatsSloState _$unknownDefaultOpenApi = const StatsSloState._(
  'unknownDefaultOpenApi',
);

StatsSloState _$valueOf(String name) {
  switch (name) {
    case 'green':
      return _$green;
    case 'amber':
      return _$amber;
    case 'red':
      return _$red;
    case 'insufficientData':
      return _$insufficientData;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<StatsSloState> _$values = BuiltSet<StatsSloState>(
  const <StatsSloState>[
    _$green,
    _$amber,
    _$red,
    _$insufficientData,
    _$unknownDefaultOpenApi,
  ],
);

class _$StatsSloStateMeta {
  const _$StatsSloStateMeta();
  StatsSloState get green => _$green;
  StatsSloState get amber => _$amber;
  StatsSloState get red => _$red;
  StatsSloState get insufficientData => _$insufficientData;
  StatsSloState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  StatsSloState valueOf(String name) => _$valueOf(name);
  BuiltSet<StatsSloState> get values => _$values;
}

mixin _$StatsSloStateMixin {
  // ignore: non_constant_identifier_names
  _$StatsSloStateMeta get StatsSloState => const _$StatsSloStateMeta();
}

Serializer<StatsSloState> _$statsSloStateSerializer =
    _$StatsSloStateSerializer();

class _$StatsSloStateSerializer implements PrimitiveSerializer<StatsSloState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'green': 'green',
    'amber': 'amber',
    'red': 'red',
    'insufficientData': 'insufficient_data',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'green': 'green',
    'amber': 'amber',
    'red': 'red',
    'insufficient_data': 'insufficientData',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatsSloState];
  @override
  final String wireName = 'StatsSloState';

  @override
  Object serialize(
    Serializers serializers,
    StatsSloState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatsSloState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatsSloState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
