// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'grant_subject_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const GrantSubjectKind _$user = const GrantSubjectKind._('user');
const GrantSubjectKind _$group = const GrantSubjectKind._('group');
const GrantSubjectKind _$agent = const GrantSubjectKind._('agent');
const GrantSubjectKind _$app = const GrantSubjectKind._('app');
const GrantSubjectKind _$unknownDefaultOpenApi = const GrantSubjectKind._(
  'unknownDefaultOpenApi',
);

GrantSubjectKind _$valueOf(String name) {
  switch (name) {
    case 'user':
      return _$user;
    case 'group':
      return _$group;
    case 'agent':
      return _$agent;
    case 'app':
      return _$app;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<GrantSubjectKind> _$values = BuiltSet<GrantSubjectKind>(
  const <GrantSubjectKind>[
    _$user,
    _$group,
    _$agent,
    _$app,
    _$unknownDefaultOpenApi,
  ],
);

class _$GrantSubjectKindMeta {
  const _$GrantSubjectKindMeta();
  GrantSubjectKind get user => _$user;
  GrantSubjectKind get group => _$group;
  GrantSubjectKind get agent => _$agent;
  GrantSubjectKind get app => _$app;
  GrantSubjectKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  GrantSubjectKind valueOf(String name) => _$valueOf(name);
  BuiltSet<GrantSubjectKind> get values => _$values;
}

mixin _$GrantSubjectKindMixin {
  // ignore: non_constant_identifier_names
  _$GrantSubjectKindMeta get GrantSubjectKind => const _$GrantSubjectKindMeta();
}

Serializer<GrantSubjectKind> _$grantSubjectKindSerializer =
    _$GrantSubjectKindSerializer();

class _$GrantSubjectKindSerializer
    implements PrimitiveSerializer<GrantSubjectKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'user': 'user',
    'group': 'group',
    'agent': 'agent',
    'app': 'app',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'user': 'user',
    'group': 'group',
    'agent': 'agent',
    'app': 'app',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[GrantSubjectKind];
  @override
  final String wireName = 'GrantSubjectKind';

  @override
  Object serialize(
    Serializers serializers,
    GrantSubjectKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  GrantSubjectKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => GrantSubjectKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
