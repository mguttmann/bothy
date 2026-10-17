// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const MessageKind _$text = const MessageKind._('text');
const MessageKind _$voice = const MessageKind._('voice');
const MessageKind _$run = const MessageKind._('run');
const MessageKind _$card = const MessageKind._('card');
const MessageKind _$system = const MessageKind._('system');
const MessageKind _$unknownDefaultOpenApi = const MessageKind._(
  'unknownDefaultOpenApi',
);

MessageKind _$valueOf(String name) {
  switch (name) {
    case 'text':
      return _$text;
    case 'voice':
      return _$voice;
    case 'run':
      return _$run;
    case 'card':
      return _$card;
    case 'system':
      return _$system;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<MessageKind> _$values = BuiltSet<MessageKind>(
  const <MessageKind>[
    _$text,
    _$voice,
    _$run,
    _$card,
    _$system,
    _$unknownDefaultOpenApi,
  ],
);

class _$MessageKindMeta {
  const _$MessageKindMeta();
  MessageKind get text => _$text;
  MessageKind get voice => _$voice;
  MessageKind get run => _$run;
  MessageKind get card => _$card;
  MessageKind get system => _$system;
  MessageKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  MessageKind valueOf(String name) => _$valueOf(name);
  BuiltSet<MessageKind> get values => _$values;
}

mixin _$MessageKindMixin {
  // ignore: non_constant_identifier_names
  _$MessageKindMeta get MessageKind => const _$MessageKindMeta();
}

Serializer<MessageKind> _$messageKindSerializer = _$MessageKindSerializer();

class _$MessageKindSerializer implements PrimitiveSerializer<MessageKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'text': 'text',
    'voice': 'voice',
    'run': 'run',
    'card': 'card',
    'system': 'system',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'text': 'text',
    'voice': 'voice',
    'run': 'run',
    'card': 'card',
    'system': 'system',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[MessageKind];
  @override
  final String wireName = 'MessageKind';

  @override
  Object serialize(
    Serializers serializers,
    MessageKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  MessageKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => MessageKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
