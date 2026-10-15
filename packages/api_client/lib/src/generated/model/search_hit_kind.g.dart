// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_hit_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const SearchHitKind _$message = const SearchHitKind._('message');
const SearchHitKind _$thread = const SearchHitKind._('thread');
const SearchHitKind _$file = const SearchHitKind._('file');
const SearchHitKind _$unknownDefaultOpenApi = const SearchHitKind._(
  'unknownDefaultOpenApi',
);

SearchHitKind _$valueOf(String name) {
  switch (name) {
    case 'message':
      return _$message;
    case 'thread':
      return _$thread;
    case 'file':
      return _$file;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<SearchHitKind> _$values = BuiltSet<SearchHitKind>(
  const <SearchHitKind>[_$message, _$thread, _$file, _$unknownDefaultOpenApi],
);

class _$SearchHitKindMeta {
  const _$SearchHitKindMeta();
  SearchHitKind get message => _$message;
  SearchHitKind get thread => _$thread;
  SearchHitKind get file => _$file;
  SearchHitKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  SearchHitKind valueOf(String name) => _$valueOf(name);
  BuiltSet<SearchHitKind> get values => _$values;
}

mixin _$SearchHitKindMixin {
  // ignore: non_constant_identifier_names
  _$SearchHitKindMeta get SearchHitKind => const _$SearchHitKindMeta();
}

Serializer<SearchHitKind> _$searchHitKindSerializer =
    _$SearchHitKindSerializer();

class _$SearchHitKindSerializer implements PrimitiveSerializer<SearchHitKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'message': 'message',
    'thread': 'thread',
    'file': 'file',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'message': 'message',
    'thread': 'thread',
    'file': 'file',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[SearchHitKind];
  @override
  final String wireName = 'SearchHitKind';

  @override
  Object serialize(
    Serializers serializers,
    SearchHitKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  SearchHitKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => SearchHitKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
