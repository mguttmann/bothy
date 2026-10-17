// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ChannelKind _$text = const ChannelKind._('text');
const ChannelKind _$forum = const ChannelKind._('forum');
const ChannelKind _$dm = const ChannelKind._('dm');
const ChannelKind _$unknownDefaultOpenApi = const ChannelKind._(
  'unknownDefaultOpenApi',
);

ChannelKind _$valueOf(String name) {
  switch (name) {
    case 'text':
      return _$text;
    case 'forum':
      return _$forum;
    case 'dm':
      return _$dm;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ChannelKind> _$values = BuiltSet<ChannelKind>(
  const <ChannelKind>[_$text, _$forum, _$dm, _$unknownDefaultOpenApi],
);

class _$ChannelKindMeta {
  const _$ChannelKindMeta();
  ChannelKind get text => _$text;
  ChannelKind get forum => _$forum;
  ChannelKind get dm => _$dm;
  ChannelKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ChannelKind valueOf(String name) => _$valueOf(name);
  BuiltSet<ChannelKind> get values => _$values;
}

mixin _$ChannelKindMixin {
  // ignore: non_constant_identifier_names
  _$ChannelKindMeta get ChannelKind => const _$ChannelKindMeta();
}

Serializer<ChannelKind> _$channelKindSerializer = _$ChannelKindSerializer();

class _$ChannelKindSerializer implements PrimitiveSerializer<ChannelKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'text': 'text',
    'forum': 'forum',
    'dm': 'dm',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'text': 'text',
    'forum': 'forum',
    'dm': 'dm',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ChannelKind];
  @override
  final String wireName = 'ChannelKind';

  @override
  Object serialize(
    Serializers serializers,
    ChannelKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ChannelKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ChannelKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
