// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_incident_impact.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatusIncidentImpact _$none = const StatusIncidentImpact._('none');
const StatusIncidentImpact _$minor = const StatusIncidentImpact._('minor');
const StatusIncidentImpact _$major = const StatusIncidentImpact._('major');
const StatusIncidentImpact _$critical = const StatusIncidentImpact._(
  'critical',
);
const StatusIncidentImpact _$unknownDefaultOpenApi =
    const StatusIncidentImpact._('unknownDefaultOpenApi');

StatusIncidentImpact _$valueOf(String name) {
  switch (name) {
    case 'none':
      return _$none;
    case 'minor':
      return _$minor;
    case 'major':
      return _$major;
    case 'critical':
      return _$critical;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<StatusIncidentImpact> _$values = BuiltSet<StatusIncidentImpact>(
  const <StatusIncidentImpact>[
    _$none,
    _$minor,
    _$major,
    _$critical,
    _$unknownDefaultOpenApi,
  ],
);

class _$StatusIncidentImpactMeta {
  const _$StatusIncidentImpactMeta();
  StatusIncidentImpact get none => _$none;
  StatusIncidentImpact get minor => _$minor;
  StatusIncidentImpact get major => _$major;
  StatusIncidentImpact get critical => _$critical;
  StatusIncidentImpact get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  StatusIncidentImpact valueOf(String name) => _$valueOf(name);
  BuiltSet<StatusIncidentImpact> get values => _$values;
}

mixin _$StatusIncidentImpactMixin {
  // ignore: non_constant_identifier_names
  _$StatusIncidentImpactMeta get StatusIncidentImpact =>
      const _$StatusIncidentImpactMeta();
}

Serializer<StatusIncidentImpact> _$statusIncidentImpactSerializer =
    _$StatusIncidentImpactSerializer();

class _$StatusIncidentImpactSerializer
    implements PrimitiveSerializer<StatusIncidentImpact> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'none': 'none',
    'minor': 'minor',
    'major': 'major',
    'critical': 'critical',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'none': 'none',
    'minor': 'minor',
    'major': 'major',
    'critical': 'critical',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatusIncidentImpact];
  @override
  final String wireName = 'StatusIncidentImpact';

  @override
  Object serialize(
    Serializers serializers,
    StatusIncidentImpact object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatusIncidentImpact deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatusIncidentImpact.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
