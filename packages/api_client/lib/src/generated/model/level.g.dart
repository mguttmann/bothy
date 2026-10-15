// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'level.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const Level _$read = const Level._('read');
const Level _$write = const Level._('write');
const Level _$trigger = const Level._('trigger');
const Level _$unknownDefaultOpenApi = const Level._('unknownDefaultOpenApi');

Level _$valueOf(String name) {
  switch (name) {
    case 'read':
      return _$read;
    case 'write':
      return _$write;
    case 'trigger':
      return _$trigger;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<Level> _$values = BuiltSet<Level>(const <Level>[
  _$read,
  _$write,
  _$trigger,
  _$unknownDefaultOpenApi,
]);

class _$LevelMeta {
  const _$LevelMeta();
  Level get read => _$read;
  Level get write => _$write;
  Level get trigger => _$trigger;
  Level get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  Level valueOf(String name) => _$valueOf(name);
  BuiltSet<Level> get values => _$values;
}

mixin _$LevelMixin {
  // ignore: non_constant_identifier_names
  _$LevelMeta get Level => const _$LevelMeta();
}

Serializer<Level> _$levelSerializer = _$LevelSerializer();

class _$LevelSerializer implements PrimitiveSerializer<Level> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'read': 'read',
    'write': 'write',
    'trigger': 'trigger',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'read': 'read',
    'write': 'write',
    'trigger': 'trigger',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[Level];
  @override
  final String wireName = 'Level';

  @override
  Object serialize(
    Serializers serializers,
    Level object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  Level deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => Level.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
