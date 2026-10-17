// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message_origin.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const MessageOrigin _$human = const MessageOrigin._('human');
const MessageOrigin _$agent = const MessageOrigin._('agent');
const MessageOrigin _$app = const MessageOrigin._('app');
const MessageOrigin _$import_ = const MessageOrigin._('import_');
const MessageOrigin _$system = const MessageOrigin._('system');
const MessageOrigin _$unknownDefaultOpenApi = const MessageOrigin._(
  'unknownDefaultOpenApi',
);

MessageOrigin _$valueOf(String name) {
  switch (name) {
    case 'human':
      return _$human;
    case 'agent':
      return _$agent;
    case 'app':
      return _$app;
    case 'import_':
      return _$import_;
    case 'system':
      return _$system;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<MessageOrigin> _$values = BuiltSet<MessageOrigin>(
  const <MessageOrigin>[
    _$human,
    _$agent,
    _$app,
    _$import_,
    _$system,
    _$unknownDefaultOpenApi,
  ],
);

class _$MessageOriginMeta {
  const _$MessageOriginMeta();
  MessageOrigin get human => _$human;
  MessageOrigin get agent => _$agent;
  MessageOrigin get app => _$app;
  MessageOrigin get import_ => _$import_;
  MessageOrigin get system => _$system;
  MessageOrigin get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  MessageOrigin valueOf(String name) => _$valueOf(name);
  BuiltSet<MessageOrigin> get values => _$values;
}

mixin _$MessageOriginMixin {
  // ignore: non_constant_identifier_names
  _$MessageOriginMeta get MessageOrigin => const _$MessageOriginMeta();
}

Serializer<MessageOrigin> _$messageOriginSerializer =
    _$MessageOriginSerializer();

class _$MessageOriginSerializer implements PrimitiveSerializer<MessageOrigin> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'human': 'human',
    'agent': 'agent',
    'app': 'app',
    'import_': 'import',
    'system': 'system',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'human': 'human',
    'agent': 'agent',
    'app': 'app',
    'import': 'import_',
    'system': 'system',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[MessageOrigin];
  @override
  final String wireName = 'MessageOrigin';

  @override
  Object serialize(
    Serializers serializers,
    MessageOrigin object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  MessageOrigin deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => MessageOrigin.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
