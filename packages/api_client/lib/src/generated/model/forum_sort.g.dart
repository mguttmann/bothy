// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'forum_sort.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ForumSort _$lastActivity = const ForumSort._('lastActivity');
const ForumSort _$created = const ForumSort._('created');
const ForumSort _$unknownDefaultOpenApi = const ForumSort._(
  'unknownDefaultOpenApi',
);

ForumSort _$valueOf(String name) {
  switch (name) {
    case 'lastActivity':
      return _$lastActivity;
    case 'created':
      return _$created;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ForumSort> _$values = BuiltSet<ForumSort>(const <ForumSort>[
  _$lastActivity,
  _$created,
  _$unknownDefaultOpenApi,
]);

class _$ForumSortMeta {
  const _$ForumSortMeta();
  ForumSort get lastActivity => _$lastActivity;
  ForumSort get created => _$created;
  ForumSort get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ForumSort valueOf(String name) => _$valueOf(name);
  BuiltSet<ForumSort> get values => _$values;
}

mixin _$ForumSortMixin {
  // ignore: non_constant_identifier_names
  _$ForumSortMeta get ForumSort => const _$ForumSortMeta();
}

Serializer<ForumSort> _$forumSortSerializer = _$ForumSortSerializer();

class _$ForumSortSerializer implements PrimitiveSerializer<ForumSort> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'lastActivity': 'last_activity',
    'created': 'created',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'last_activity': 'lastActivity',
    'created': 'created',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ForumSort];
  @override
  final String wireName = 'ForumSort';

  @override
  Object serialize(
    Serializers serializers,
    ForumSort object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ForumSort deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ForumSort.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
