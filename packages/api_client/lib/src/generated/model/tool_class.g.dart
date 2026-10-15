// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'tool_class.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ToolClass _$chat = const ToolClass._('chat');
const ToolClass _$scoped = const ToolClass._('scoped');
const ToolClass _$host = const ToolClass._('host');
const ToolClass _$unknownDefaultOpenApi = const ToolClass._(
  'unknownDefaultOpenApi',
);

ToolClass _$valueOf(String name) {
  switch (name) {
    case 'chat':
      return _$chat;
    case 'scoped':
      return _$scoped;
    case 'host':
      return _$host;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ToolClass> _$values = BuiltSet<ToolClass>(const <ToolClass>[
  _$chat,
  _$scoped,
  _$host,
  _$unknownDefaultOpenApi,
]);

class _$ToolClassMeta {
  const _$ToolClassMeta();
  ToolClass get chat => _$chat;
  ToolClass get scoped => _$scoped;
  ToolClass get host => _$host;
  ToolClass get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ToolClass valueOf(String name) => _$valueOf(name);
  BuiltSet<ToolClass> get values => _$values;
}

mixin _$ToolClassMixin {
  // ignore: non_constant_identifier_names
  _$ToolClassMeta get ToolClass => const _$ToolClassMeta();
}

Serializer<ToolClass> _$toolClassSerializer = _$ToolClassSerializer();

class _$ToolClassSerializer implements PrimitiveSerializer<ToolClass> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'chat': 'chat',
    'scoped': 'scoped',
    'host': 'host',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'chat': 'chat',
    'scoped': 'scoped',
    'host': 'host',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ToolClass];
  @override
  final String wireName = 'ToolClass';

  @override
  Object serialize(
    Serializers serializers,
    ToolClass object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ToolClass deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ToolClass.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
