// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ImportKind _$discord = const ImportKind._('discord');
const ImportKind _$archive = const ImportKind._('archive');
const ImportKind _$unknownDefaultOpenApi = const ImportKind._(
  'unknownDefaultOpenApi',
);

ImportKind _$valueOf(String name) {
  switch (name) {
    case 'discord':
      return _$discord;
    case 'archive':
      return _$archive;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportKind> _$values = BuiltSet<ImportKind>(const <ImportKind>[
  _$discord,
  _$archive,
  _$unknownDefaultOpenApi,
]);

class _$ImportKindMeta {
  const _$ImportKindMeta();
  ImportKind get discord => _$discord;
  ImportKind get archive => _$archive;
  ImportKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ImportKind valueOf(String name) => _$valueOf(name);
  BuiltSet<ImportKind> get values => _$values;
}

mixin _$ImportKindMixin {
  // ignore: non_constant_identifier_names
  _$ImportKindMeta get ImportKind => const _$ImportKindMeta();
}

Serializer<ImportKind> _$importKindSerializer = _$ImportKindSerializer();

class _$ImportKindSerializer implements PrimitiveSerializer<ImportKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'discord': 'discord',
    'archive': 'archive',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'discord': 'discord',
    'archive': 'archive',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ImportKind];
  @override
  final String wireName = 'ImportKind';

  @override
  Object serialize(
    Serializers serializers,
    ImportKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
