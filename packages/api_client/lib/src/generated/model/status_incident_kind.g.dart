// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_incident_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatusIncidentKind _$incident = const StatusIncidentKind._('incident');
const StatusIncidentKind _$maintenance = const StatusIncidentKind._(
  'maintenance',
);
const StatusIncidentKind _$unknownDefaultOpenApi = const StatusIncidentKind._(
  'unknownDefaultOpenApi',
);

StatusIncidentKind _$valueOf(String name) {
  switch (name) {
    case 'incident':
      return _$incident;
    case 'maintenance':
      return _$maintenance;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<StatusIncidentKind> _$values = BuiltSet<StatusIncidentKind>(
  const <StatusIncidentKind>[
    _$incident,
    _$maintenance,
    _$unknownDefaultOpenApi,
  ],
);

class _$StatusIncidentKindMeta {
  const _$StatusIncidentKindMeta();
  StatusIncidentKind get incident => _$incident;
  StatusIncidentKind get maintenance => _$maintenance;
  StatusIncidentKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  StatusIncidentKind valueOf(String name) => _$valueOf(name);
  BuiltSet<StatusIncidentKind> get values => _$values;
}

mixin _$StatusIncidentKindMixin {
  // ignore: non_constant_identifier_names
  _$StatusIncidentKindMeta get StatusIncidentKind =>
      const _$StatusIncidentKindMeta();
}

Serializer<StatusIncidentKind> _$statusIncidentKindSerializer =
    _$StatusIncidentKindSerializer();

class _$StatusIncidentKindSerializer
    implements PrimitiveSerializer<StatusIncidentKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'incident': 'incident',
    'maintenance': 'maintenance',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'incident': 'incident',
    'maintenance': 'maintenance',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatusIncidentKind];
  @override
  final String wireName = 'StatusIncidentKind';

  @override
  Object serialize(
    Serializers serializers,
    StatusIncidentKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatusIncidentKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatusIncidentKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
