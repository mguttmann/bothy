// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'actor_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ActorKind _$user = const ActorKind._('user');
const ActorKind _$agent = const ActorKind._('agent');
const ActorKind _$app = const ActorKind._('app');
const ActorKind _$system = const ActorKind._('system');
const ActorKind _$unknownDefaultOpenApi = const ActorKind._(
  'unknownDefaultOpenApi',
);

ActorKind _$valueOf(String name) {
  switch (name) {
    case 'user':
      return _$user;
    case 'agent':
      return _$agent;
    case 'app':
      return _$app;
    case 'system':
      return _$system;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ActorKind> _$values = BuiltSet<ActorKind>(const <ActorKind>[
  _$user,
  _$agent,
  _$app,
  _$system,
  _$unknownDefaultOpenApi,
]);

class _$ActorKindMeta {
  const _$ActorKindMeta();
  ActorKind get user => _$user;
  ActorKind get agent => _$agent;
  ActorKind get app => _$app;
  ActorKind get system => _$system;
  ActorKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ActorKind valueOf(String name) => _$valueOf(name);
  BuiltSet<ActorKind> get values => _$values;
}

mixin _$ActorKindMixin {
  // ignore: non_constant_identifier_names
  _$ActorKindMeta get ActorKind => const _$ActorKindMeta();
}

Serializer<ActorKind> _$actorKindSerializer = _$ActorKindSerializer();

class _$ActorKindSerializer implements PrimitiveSerializer<ActorKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'user': 'user',
    'agent': 'agent',
    'app': 'app',
    'system': 'system',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'user': 'user',
    'agent': 'agent',
    'app': 'app',
    'system': 'system',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ActorKind];
  @override
  final String wireName = 'ActorKind';

  @override
  Object serialize(
    Serializers serializers,
    ActorKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ActorKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ActorKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
