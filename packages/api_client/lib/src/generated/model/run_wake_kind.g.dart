// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunWakeKind _$at = const RunWakeKind._('at');
const RunWakeKind _$prChecks = const RunWakeKind._('prChecks');
const RunWakeKind _$command = const RunWakeKind._('command');
const RunWakeKind _$unknownDefaultOpenApi = const RunWakeKind._(
  'unknownDefaultOpenApi',
);

RunWakeKind _$valueOf(String name) {
  switch (name) {
    case 'at':
      return _$at;
    case 'prChecks':
      return _$prChecks;
    case 'command':
      return _$command;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<RunWakeKind> _$values = BuiltSet<RunWakeKind>(
  const <RunWakeKind>[_$at, _$prChecks, _$command, _$unknownDefaultOpenApi],
);

class _$RunWakeKindMeta {
  const _$RunWakeKindMeta();
  RunWakeKind get at => _$at;
  RunWakeKind get prChecks => _$prChecks;
  RunWakeKind get command => _$command;
  RunWakeKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  RunWakeKind valueOf(String name) => _$valueOf(name);
  BuiltSet<RunWakeKind> get values => _$values;
}

mixin _$RunWakeKindMixin {
  // ignore: non_constant_identifier_names
  _$RunWakeKindMeta get RunWakeKind => const _$RunWakeKindMeta();
}

Serializer<RunWakeKind> _$runWakeKindSerializer = _$RunWakeKindSerializer();

class _$RunWakeKindSerializer implements PrimitiveSerializer<RunWakeKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'at': 'at',
    'prChecks': 'pr_checks',
    'command': 'command',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'at': 'at',
    'pr_checks': 'prChecks',
    'command': 'command',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunWakeKind];
  @override
  final String wireName = 'RunWakeKind';

  @override
  Object serialize(
    Serializers serializers,
    RunWakeKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunWakeKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunWakeKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
