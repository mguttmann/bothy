// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_incident_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatusIncidentState _$scheduled = const StatusIncidentState._(
  'scheduled',
);
const StatusIncidentState _$investigating = const StatusIncidentState._(
  'investigating',
);
const StatusIncidentState _$identified = const StatusIncidentState._(
  'identified',
);
const StatusIncidentState _$monitoring = const StatusIncidentState._(
  'monitoring',
);
const StatusIncidentState _$resolved = const StatusIncidentState._('resolved');
const StatusIncidentState _$unknownDefaultOpenApi = const StatusIncidentState._(
  'unknownDefaultOpenApi',
);

StatusIncidentState _$valueOf(String name) {
  switch (name) {
    case 'scheduled':
      return _$scheduled;
    case 'investigating':
      return _$investigating;
    case 'identified':
      return _$identified;
    case 'monitoring':
      return _$monitoring;
    case 'resolved':
      return _$resolved;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<StatusIncidentState> _$values = BuiltSet<StatusIncidentState>(
  const <StatusIncidentState>[
    _$scheduled,
    _$investigating,
    _$identified,
    _$monitoring,
    _$resolved,
    _$unknownDefaultOpenApi,
  ],
);

class _$StatusIncidentStateMeta {
  const _$StatusIncidentStateMeta();
  StatusIncidentState get scheduled => _$scheduled;
  StatusIncidentState get investigating => _$investigating;
  StatusIncidentState get identified => _$identified;
  StatusIncidentState get monitoring => _$monitoring;
  StatusIncidentState get resolved => _$resolved;
  StatusIncidentState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  StatusIncidentState valueOf(String name) => _$valueOf(name);
  BuiltSet<StatusIncidentState> get values => _$values;
}

mixin _$StatusIncidentStateMixin {
  // ignore: non_constant_identifier_names
  _$StatusIncidentStateMeta get StatusIncidentState =>
      const _$StatusIncidentStateMeta();
}

Serializer<StatusIncidentState> _$statusIncidentStateSerializer =
    _$StatusIncidentStateSerializer();

class _$StatusIncidentStateSerializer
    implements PrimitiveSerializer<StatusIncidentState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'scheduled': 'scheduled',
    'investigating': 'investigating',
    'identified': 'identified',
    'monitoring': 'monitoring',
    'resolved': 'resolved',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'scheduled': 'scheduled',
    'investigating': 'investigating',
    'identified': 'identified',
    'monitoring': 'monitoring',
    'resolved': 'resolved',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatusIncidentState];
  @override
  final String wireName = 'StatusIncidentState';

  @override
  Object serialize(
    Serializers serializers,
    StatusIncidentState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatusIncidentState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatusIncidentState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
