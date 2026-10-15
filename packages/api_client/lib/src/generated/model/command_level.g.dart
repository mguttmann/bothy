// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command_level.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const CommandLevel _$write = const CommandLevel._('write');
const CommandLevel _$trigger = const CommandLevel._('trigger');
const CommandLevel _$unknownDefaultOpenApi = const CommandLevel._(
  'unknownDefaultOpenApi',
);

CommandLevel _$valueOf(String name) {
  switch (name) {
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

final BuiltSet<CommandLevel> _$values = BuiltSet<CommandLevel>(
  const <CommandLevel>[_$write, _$trigger, _$unknownDefaultOpenApi],
);

class _$CommandLevelMeta {
  const _$CommandLevelMeta();
  CommandLevel get write => _$write;
  CommandLevel get trigger => _$trigger;
  CommandLevel get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  CommandLevel valueOf(String name) => _$valueOf(name);
  BuiltSet<CommandLevel> get values => _$values;
}

mixin _$CommandLevelMixin {
  // ignore: non_constant_identifier_names
  _$CommandLevelMeta get CommandLevel => const _$CommandLevelMeta();
}

Serializer<CommandLevel> _$commandLevelSerializer = _$CommandLevelSerializer();

class _$CommandLevelSerializer implements PrimitiveSerializer<CommandLevel> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'write': 'write',
    'trigger': 'trigger',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'write': 'write',
    'trigger': 'trigger',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[CommandLevel];
  @override
  final String wireName = 'CommandLevel';

  @override
  Object serialize(
    Serializers serializers,
    CommandLevel object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  CommandLevel deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => CommandLevel.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
