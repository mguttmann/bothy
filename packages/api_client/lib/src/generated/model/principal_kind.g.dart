// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'principal_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const PrincipalKind _$user = const PrincipalKind._('user');
const PrincipalKind _$agent = const PrincipalKind._('agent');
const PrincipalKind _$app = const PrincipalKind._('app');
const PrincipalKind _$unknownDefaultOpenApi = const PrincipalKind._(
  'unknownDefaultOpenApi',
);

PrincipalKind _$valueOf(String name) {
  switch (name) {
    case 'user':
      return _$user;
    case 'agent':
      return _$agent;
    case 'app':
      return _$app;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<PrincipalKind> _$values = BuiltSet<PrincipalKind>(
  const <PrincipalKind>[_$user, _$agent, _$app, _$unknownDefaultOpenApi],
);

class _$PrincipalKindMeta {
  const _$PrincipalKindMeta();
  PrincipalKind get user => _$user;
  PrincipalKind get agent => _$agent;
  PrincipalKind get app => _$app;
  PrincipalKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  PrincipalKind valueOf(String name) => _$valueOf(name);
  BuiltSet<PrincipalKind> get values => _$values;
}

mixin _$PrincipalKindMixin {
  // ignore: non_constant_identifier_names
  _$PrincipalKindMeta get PrincipalKind => const _$PrincipalKindMeta();
}

Serializer<PrincipalKind> _$principalKindSerializer =
    _$PrincipalKindSerializer();

class _$PrincipalKindSerializer implements PrimitiveSerializer<PrincipalKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'user': 'user',
    'agent': 'agent',
    'app': 'app',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'user': 'user',
    'agent': 'agent',
    'app': 'app',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[PrincipalKind];
  @override
  final String wireName = 'PrincipalKind';

  @override
  Object serialize(
    Serializers serializers,
    PrincipalKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  PrincipalKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => PrincipalKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
