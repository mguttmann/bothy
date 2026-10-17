// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'push_environment.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const PushEnvironment _$production = const PushEnvironment._('production');
const PushEnvironment _$sandbox = const PushEnvironment._('sandbox');
const PushEnvironment _$unknownDefaultOpenApi = const PushEnvironment._(
  'unknownDefaultOpenApi',
);

PushEnvironment _$valueOf(String name) {
  switch (name) {
    case 'production':
      return _$production;
    case 'sandbox':
      return _$sandbox;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<PushEnvironment> _$values = BuiltSet<PushEnvironment>(
  const <PushEnvironment>[_$production, _$sandbox, _$unknownDefaultOpenApi],
);

class _$PushEnvironmentMeta {
  const _$PushEnvironmentMeta();
  PushEnvironment get production => _$production;
  PushEnvironment get sandbox => _$sandbox;
  PushEnvironment get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  PushEnvironment valueOf(String name) => _$valueOf(name);
  BuiltSet<PushEnvironment> get values => _$values;
}

mixin _$PushEnvironmentMixin {
  // ignore: non_constant_identifier_names
  _$PushEnvironmentMeta get PushEnvironment => const _$PushEnvironmentMeta();
}

Serializer<PushEnvironment> _$pushEnvironmentSerializer =
    _$PushEnvironmentSerializer();

class _$PushEnvironmentSerializer
    implements PrimitiveSerializer<PushEnvironment> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'production': 'production',
    'sandbox': 'sandbox',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'production': 'production',
    'sandbox': 'sandbox',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[PushEnvironment];
  @override
  final String wireName = 'PushEnvironment';

  @override
  Object serialize(
    Serializers serializers,
    PushEnvironment object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  PushEnvironment deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => PushEnvironment.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
