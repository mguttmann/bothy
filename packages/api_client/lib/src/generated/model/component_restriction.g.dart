// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'component_restriction.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ComponentRestriction _$any = const ComponentRestriction._('any');
const ComponentRestriction _$runStarter = const ComponentRestriction._(
  'runStarter',
);
const ComponentRestriction _$owner = const ComponentRestriction._('owner');
const ComponentRestriction _$unknownDefaultOpenApi =
    const ComponentRestriction._('unknownDefaultOpenApi');

ComponentRestriction _$valueOf(String name) {
  switch (name) {
    case 'any':
      return _$any;
    case 'runStarter':
      return _$runStarter;
    case 'owner':
      return _$owner;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ComponentRestriction> _$values = BuiltSet<ComponentRestriction>(
  const <ComponentRestriction>[
    _$any,
    _$runStarter,
    _$owner,
    _$unknownDefaultOpenApi,
  ],
);

class _$ComponentRestrictionMeta {
  const _$ComponentRestrictionMeta();
  ComponentRestriction get any => _$any;
  ComponentRestriction get runStarter => _$runStarter;
  ComponentRestriction get owner => _$owner;
  ComponentRestriction get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ComponentRestriction valueOf(String name) => _$valueOf(name);
  BuiltSet<ComponentRestriction> get values => _$values;
}

mixin _$ComponentRestrictionMixin {
  // ignore: non_constant_identifier_names
  _$ComponentRestrictionMeta get ComponentRestriction =>
      const _$ComponentRestrictionMeta();
}

Serializer<ComponentRestriction> _$componentRestrictionSerializer =
    _$ComponentRestrictionSerializer();

class _$ComponentRestrictionSerializer
    implements PrimitiveSerializer<ComponentRestriction> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'any': 'any',
    'runStarter': 'run_starter',
    'owner': 'owner',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'any': 'any',
    'run_starter': 'runStarter',
    'owner': 'owner',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ComponentRestriction];
  @override
  final String wireName = 'ComponentRestriction';

  @override
  Object serialize(
    Serializers serializers,
    ComponentRestriction object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ComponentRestriction deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ComponentRestriction.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
