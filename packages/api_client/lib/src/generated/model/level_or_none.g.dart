// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'level_or_none.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const LevelOrNone _$none = const LevelOrNone._('none');
const LevelOrNone _$read = const LevelOrNone._('read');
const LevelOrNone _$write = const LevelOrNone._('write');
const LevelOrNone _$trigger = const LevelOrNone._('trigger');
const LevelOrNone _$unknownDefaultOpenApi = const LevelOrNone._(
  'unknownDefaultOpenApi',
);

LevelOrNone _$valueOf(String name) {
  switch (name) {
    case 'none':
      return _$none;
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

final BuiltSet<LevelOrNone> _$values = BuiltSet<LevelOrNone>(
  const <LevelOrNone>[
    _$none,
    _$read,
    _$write,
    _$trigger,
    _$unknownDefaultOpenApi,
  ],
);

class _$LevelOrNoneMeta {
  const _$LevelOrNoneMeta();
  LevelOrNone get none => _$none;
  LevelOrNone get read => _$read;
  LevelOrNone get write => _$write;
  LevelOrNone get trigger => _$trigger;
  LevelOrNone get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  LevelOrNone valueOf(String name) => _$valueOf(name);
  BuiltSet<LevelOrNone> get values => _$values;
}

mixin _$LevelOrNoneMixin {
  // ignore: non_constant_identifier_names
  _$LevelOrNoneMeta get LevelOrNone => const _$LevelOrNoneMeta();
}

Serializer<LevelOrNone> _$levelOrNoneSerializer = _$LevelOrNoneSerializer();

class _$LevelOrNoneSerializer implements PrimitiveSerializer<LevelOrNone> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'none': 'none',
    'read': 'read',
    'write': 'write',
    'trigger': 'trigger',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'none': 'none',
    'read': 'read',
    'write': 'write',
    'trigger': 'trigger',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[LevelOrNone];
  @override
  final String wireName = 'LevelOrNone';

  @override
  Object serialize(
    Serializers serializers,
    LevelOrNone object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  LevelOrNone deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => LevelOrNone.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
