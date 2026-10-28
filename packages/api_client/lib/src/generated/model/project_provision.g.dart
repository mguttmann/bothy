// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_provision.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ProjectProvision _$none = const ProjectProvision._('none');
const ProjectProvision _$init = const ProjectProvision._('init');
const ProjectProvision _$clone = const ProjectProvision._('clone');
const ProjectProvision _$unknownDefaultOpenApi = const ProjectProvision._(
  'unknownDefaultOpenApi',
);

ProjectProvision _$valueOf(String name) {
  switch (name) {
    case 'none':
      return _$none;
    case 'init':
      return _$init;
    case 'clone':
      return _$clone;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ProjectProvision> _$values = BuiltSet<ProjectProvision>(
  const <ProjectProvision>[_$none, _$init, _$clone, _$unknownDefaultOpenApi],
);

class _$ProjectProvisionMeta {
  const _$ProjectProvisionMeta();
  ProjectProvision get none => _$none;
  ProjectProvision get init => _$init;
  ProjectProvision get clone => _$clone;
  ProjectProvision get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ProjectProvision valueOf(String name) => _$valueOf(name);
  BuiltSet<ProjectProvision> get values => _$values;
}

mixin _$ProjectProvisionMixin {
  // ignore: non_constant_identifier_names
  _$ProjectProvisionMeta get ProjectProvision => const _$ProjectProvisionMeta();
}

Serializer<ProjectProvision> _$projectProvisionSerializer =
    _$ProjectProvisionSerializer();

class _$ProjectProvisionSerializer
    implements PrimitiveSerializer<ProjectProvision> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'none': 'none',
    'init': 'init',
    'clone': 'clone',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'none': 'none',
    'init': 'init',
    'clone': 'clone',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ProjectProvision];
  @override
  final String wireName = 'ProjectProvision';

  @override
  Object serialize(
    Serializers serializers,
    ProjectProvision object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ProjectProvision deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ProjectProvision.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
