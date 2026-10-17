// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gdpr_request_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const GdprRequestKind _$export_ = const GdprRequestKind._('export_');
const GdprRequestKind _$deletion = const GdprRequestKind._('deletion');
const GdprRequestKind _$rectification = const GdprRequestKind._(
  'rectification',
);
const GdprRequestKind _$unknownDefaultOpenApi = const GdprRequestKind._(
  'unknownDefaultOpenApi',
);

GdprRequestKind _$valueOf(String name) {
  switch (name) {
    case 'export_':
      return _$export_;
    case 'deletion':
      return _$deletion;
    case 'rectification':
      return _$rectification;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<GdprRequestKind> _$values = BuiltSet<GdprRequestKind>(
  const <GdprRequestKind>[
    _$export_,
    _$deletion,
    _$rectification,
    _$unknownDefaultOpenApi,
  ],
);

class _$GdprRequestKindMeta {
  const _$GdprRequestKindMeta();
  GdprRequestKind get export_ => _$export_;
  GdprRequestKind get deletion => _$deletion;
  GdprRequestKind get rectification => _$rectification;
  GdprRequestKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  GdprRequestKind valueOf(String name) => _$valueOf(name);
  BuiltSet<GdprRequestKind> get values => _$values;
}

mixin _$GdprRequestKindMixin {
  // ignore: non_constant_identifier_names
  _$GdprRequestKindMeta get GdprRequestKind => const _$GdprRequestKindMeta();
}

Serializer<GdprRequestKind> _$gdprRequestKindSerializer =
    _$GdprRequestKindSerializer();

class _$GdprRequestKindSerializer
    implements PrimitiveSerializer<GdprRequestKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'export_': 'export',
    'deletion': 'deletion',
    'rectification': 'rectification',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'export': 'export_',
    'deletion': 'deletion',
    'rectification': 'rectification',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[GdprRequestKind];
  @override
  final String wireName = 'GdprRequestKind';

  @override
  Object serialize(
    Serializers serializers,
    GdprRequestKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  GdprRequestKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => GdprRequestKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
