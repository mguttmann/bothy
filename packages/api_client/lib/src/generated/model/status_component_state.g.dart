// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_component_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatusComponentState _$operational = const StatusComponentState._(
  'operational',
);
const StatusComponentState _$degraded = const StatusComponentState._(
  'degraded',
);
const StatusComponentState _$partialOutage = const StatusComponentState._(
  'partialOutage',
);
const StatusComponentState _$majorOutage = const StatusComponentState._(
  'majorOutage',
);
const StatusComponentState _$maintenance = const StatusComponentState._(
  'maintenance',
);
const StatusComponentState _$unknownDefaultOpenApi =
    const StatusComponentState._('unknownDefaultOpenApi');

StatusComponentState _$valueOf(String name) {
  switch (name) {
    case 'operational':
      return _$operational;
    case 'degraded':
      return _$degraded;
    case 'partialOutage':
      return _$partialOutage;
    case 'majorOutage':
      return _$majorOutage;
    case 'maintenance':
      return _$maintenance;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<StatusComponentState> _$values = BuiltSet<StatusComponentState>(
  const <StatusComponentState>[
    _$operational,
    _$degraded,
    _$partialOutage,
    _$majorOutage,
    _$maintenance,
    _$unknownDefaultOpenApi,
  ],
);

class _$StatusComponentStateMeta {
  const _$StatusComponentStateMeta();
  StatusComponentState get operational => _$operational;
  StatusComponentState get degraded => _$degraded;
  StatusComponentState get partialOutage => _$partialOutage;
  StatusComponentState get majorOutage => _$majorOutage;
  StatusComponentState get maintenance => _$maintenance;
  StatusComponentState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  StatusComponentState valueOf(String name) => _$valueOf(name);
  BuiltSet<StatusComponentState> get values => _$values;
}

mixin _$StatusComponentStateMixin {
  // ignore: non_constant_identifier_names
  _$StatusComponentStateMeta get StatusComponentState =>
      const _$StatusComponentStateMeta();
}

Serializer<StatusComponentState> _$statusComponentStateSerializer =
    _$StatusComponentStateSerializer();

class _$StatusComponentStateSerializer
    implements PrimitiveSerializer<StatusComponentState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'operational': 'operational',
    'degraded': 'degraded',
    'partialOutage': 'partial_outage',
    'majorOutage': 'major_outage',
    'maintenance': 'maintenance',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'operational': 'operational',
    'degraded': 'degraded',
    'partial_outage': 'partialOutage',
    'major_outage': 'majorOutage',
    'maintenance': 'maintenance',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatusComponentState];
  @override
  final String wireName = 'StatusComponentState';

  @override
  Object serialize(
    Serializers serializers,
    StatusComponentState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatusComponentState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatusComponentState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
